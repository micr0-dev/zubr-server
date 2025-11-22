package storage

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
)

type Store struct {
	usersFile string
	users     map[string]*models.User
	mu        sync.RWMutex
}

func New(usersFile string) (*Store, error) {
	logger.Debug("Initializing storage with file: %s", usersFile)
	s := &Store{
		usersFile: usersFile,
		users:     make(map[string]*models.User),
	}

	// Load existing users
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to load users from %s: %v", usersFile, err)
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
