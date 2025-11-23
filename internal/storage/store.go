package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
)

type Store struct {
	usersFile      string
	settingsFile   string
	ircConfigPath  string
	users          map[string]*models.User
	settings       *models.InstanceSettings
	mu             sync.RWMutex
}

func New(usersFile, ircConfigPath string) (*Store, error) {
	logger.Debug("Initializing storage with file: %s", usersFile)
	s := &Store{
		usersFile:     usersFile,
		settingsFile:  "data/settings.json",
		ircConfigPath: ircConfigPath,
		users:         make(map[string]*models.User),
		settings:      models.DefaultInstanceSettings(),
	}

	// Load existing users
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to load users from %s: %v", usersFile, err)
		return nil, err
	}

	// Load existing settings
	if err := s.loadSettings(); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to load settings from %s: %v", s.settingsFile, err)
		return nil, err
	}

	logger.Info("Storage initialized with %d users", len(s.users))
	return s, nil
}

func (s *Store) load() error {
	logger.Debug("Loading users from file: %s", s.usersFile)
	data, err := os.ReadFile(s.usersFile)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("Users file does not exist yet, starting with empty store")
		}
		return err
	}

	if err := json.Unmarshal(data, &s.users); err != nil {
		logger.Error("Failed to unmarshal users data: %v", err)
		return err
	}

	logger.Debug("Loaded %d users from file", len(s.users))
	return nil
}

func (s *Store) save() error {
	logger.Debug("Saving %d users to file: %s", len(s.users), s.usersFile)
	data, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal users data: %v", err)
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		logger.Error("Failed to create data directory: %v", err)
		return err
	}

	if err := os.WriteFile(s.usersFile, data, 0644); err != nil {
		logger.Error("Failed to write users file: %v", err)
		return err
	}

	logger.Debug("Successfully saved users to file")
	return nil
}

func (s *Store) CreateUser(user *models.User) error {
	logger.Debug("Creating user: %s", user.Username)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.users[user.Username] = user
	if err := s.save(); err != nil {
		logger.Error("Failed to save user %s: %v", user.Username, err)
		return err
	}

	logger.Debug("Successfully created user: %s", user.Username)
	return nil
}

func (s *Store) GetUser(username string) (*models.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[username]
	return user, ok
}

func (s *Store) GetAllUsers() []*models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*models.User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

func (s *Store) UpdateUserConfig(username string, config *models.UserConfig) error {
	logger.Debug("Updating config for user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[username]
	if !ok {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	user.Config = config
	if err := s.save(); err != nil {
		logger.Error("Failed to save config for user %s: %v", username, err)
		return err
	}

	logger.Debug("Successfully updated config for user: %s", username)
	return nil
}

func (s *Store) UpdateUserRole(username string, role models.Role) error {
	logger.Debug("Updating role for user %s to %s", username, role)
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[username]
	if !ok {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	user.Role = role
	if err := s.save(); err != nil {
		logger.Error("Failed to save role for user %s: %v", username, err)
		return err
	}

	logger.Info("Successfully updated role for user %s to %s", username, role)
	return nil
}

func (s *Store) BanUser(username string) error {
	logger.Debug("Banning user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[username]
	if !ok {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	user.Banned = true
	if err := s.save(); err != nil {
		logger.Error("Failed to ban user %s: %v", username, err)
		return err
	}

	logger.Info("Successfully banned user: %s", username)
	return nil
}

func (s *Store) UnbanUser(username string) error {
	logger.Debug("Unbanning user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[username]
	if !ok {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	user.Banned = false
	if err := s.save(); err != nil {
		logger.Error("Failed to unban user %s: %v", username, err)
		return err
	}

	logger.Info("Successfully unbanned user: %s", username)
	return nil
}

func (s *Store) DeleteUser(username string) error {
	logger.Debug("Deleting user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[username]; !ok {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	delete(s.users, username)
	if err := s.save(); err != nil {
		logger.Error("Failed to delete user %s: %v", username, err)
		return err
	}

	logger.Info("Successfully deleted user: %s", username)
	return nil
}

func (s *Store) loadSettings() error {
	logger.Debug("Loading instance settings from file: %s", s.settingsFile)
	data, err := os.ReadFile(s.settingsFile)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("Settings file does not exist yet, using defaults")
			// Save defaults
			return s.saveSettings()
		}
		return err
	}

	if err := json.Unmarshal(data, s.settings); err != nil {
		logger.Error("Failed to unmarshal settings data: %v", err)
		return err
	}

	logger.Debug("Loaded instance settings")
	return nil
}

func (s *Store) saveSettings() error {
	logger.Debug("Saving instance settings to file: %s", s.settingsFile)
	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal settings data: %v", err)
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		logger.Error("Failed to create data directory: %v", err)
		return err
	}

	if err := os.WriteFile(s.settingsFile, data, 0644); err != nil {
		logger.Error("Failed to write settings file: %v", err)
		return err
	}

	// Sync MOTD to IRC config directory
	if err := s.syncMOTDFile(); err != nil {
		logger.Error("Failed to sync MOTD file: %v", err)
		// Don't fail the whole save if MOTD sync fails
	}

	logger.Debug("Successfully saved instance settings")
	return nil
}

func (s *Store) syncMOTDFile() error {
	if s.ircConfigPath == "" {
		logger.Debug("IRC config path not set, skipping MOTD sync")
		return nil
	}

	motdPath := filepath.Join(s.ircConfigPath, "motd.txt")
	logger.Debug("Syncing MOTD to: %s", motdPath)

	// Ensure IRC config directory exists
	if err := os.MkdirAll(s.ircConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create IRC config directory: %w", err)
	}

	// Write MOTD file
	if err := os.WriteFile(motdPath, []byte(s.settings.MOTD), 0644); err != nil {
		return fmt.Errorf("failed to write MOTD file: %w", err)
	}

	logger.Debug("Successfully synced MOTD file")
	return nil
}

func (s *Store) GetSettings() *models.InstanceSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	return &models.InstanceSettings{
		SignupMode:  s.settings.SignupMode,
		MOTD:        s.settings.MOTD,
		Domain:      s.settings.Domain,
		NetworkName: s.settings.NetworkName,
	}
}

func (s *Store) UpdateSettings(updates map[string]interface{}) error {
	logger.Debug("Updating instance settings")
	s.mu.Lock()
	defer s.mu.Unlock()

	// Apply updates
	if signupMode, ok := updates["signup_mode"].(string); ok {
		s.settings.SignupMode = models.SignupMode(signupMode)
		logger.Debug("Updated signup_mode to: %s", signupMode)
	}

	if motd, ok := updates["motd"].(string); ok {
		s.settings.MOTD = motd
		logger.Debug("Updated motd")
	}

	if domain, ok := updates["domain"].(string); ok {
		s.settings.Domain = domain
		logger.Debug("Updated domain to: %s", domain)
	}

	if networkName, ok := updates["network_name"].(string); ok {
		s.settings.NetworkName = networkName
		logger.Debug("Updated network_name to: %s", networkName)
	}

	if err := s.saveSettings(); err != nil {
		logger.Error("Failed to save settings: %v", err)
		return err
	}

	logger.Info("Successfully updated instance settings")
	return nil
}
