package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
)

type Store struct {
	db            *sql.DB
	dbPath        string
	ircConfigPath string
	mu            sync.RWMutex
}

func New(dbPath, ircConfigPath string) (*Store, error) {
	logger.Debug("Initializing SQLite storage with database: %s", dbPath)

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		logger.Error("Failed to create data directory: %v", err)
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		logger.Error("Failed to open database: %v", err)
		return nil, err
	}

	s := &Store{
		db:            db,
		dbPath:        dbPath,
		ircConfigPath: ircConfigPath,
	}

	// Initialize schema
	if err := s.initSchema(); err != nil {
		logger.Error("Failed to initialize database schema: %v", err)
		db.Close()
		return nil, err
	}

	// Initialize default settings if not present
	if err := s.initDefaultSettings(); err != nil {
		logger.Error("Failed to initialize default settings: %v", err)
		db.Close()
		return nil, err
	}

	// Count users for logging
	var userCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); err == nil {
		logger.Info("Storage initialized with %d users", userCount)
	}

	return s, nil
}

func (s *Store) initSchema() error {
	logger.Debug("Initializing database schema")

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		id TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		email TEXT,
		created_at DATETIME NOT NULL,
		active BOOLEAN NOT NULL DEFAULT 0,
		banned BOOLEAN NOT NULL DEFAULT 0,
		pending BOOLEAN NOT NULL DEFAULT 0,
		role TEXT NOT NULL DEFAULT 'user',
		config TEXT
	);

	CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		signup_mode TEXT NOT NULL DEFAULT 'public',
		motd TEXT NOT NULL DEFAULT '',
		domain TEXT NOT NULL DEFAULT 'localhost',
		network_name TEXT NOT NULL DEFAULT 'zubr'
	);

	CREATE TABLE IF NOT EXISTS invites (
		token TEXT PRIMARY KEY,
		created_by TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		used BOOLEAN NOT NULL DEFAULT 0,
		used_by TEXT,
		used_at DATETIME
	);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		logger.Error("Failed to create schema: %v", err)
		return err
	}

	logger.Debug("Database schema initialized successfully")
	return nil
}

func (s *Store) initDefaultSettings() error {
	defaults := models.DefaultInstanceSettings()

	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO settings (id, signup_mode, motd, domain, network_name)
		VALUES (1, ?, ?, ?, ?)
	`, defaults.SignupMode, defaults.MOTD, defaults.Domain, defaults.NetworkName)

	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

// User operations

func (s *Store) CreateUser(user *models.User) error {
	logger.Debug("Creating user: %s", user.Username)
	s.mu.Lock()
	defer s.mu.Unlock()

	var configJSON []byte
	var err error
	if user.Config != nil {
		configJSON, err = json.Marshal(user.Config)
		if err != nil {
			logger.Error("Failed to marshal user config: %v", err)
			return err
		}
	}

	_, err = s.db.Exec(`
		INSERT INTO users (username, id, password_hash, email, created_at, active, banned, pending, role, config)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.Username, user.ID, user.PasswordHash, user.Email, user.CreatedAt, user.Active, user.Banned, user.Pending, user.Role, configJSON)

	if err != nil {
		logger.Error("Failed to create user %s: %v", user.Username, err)
		return err
	}

	logger.Debug("Successfully created user: %s", user.Username)
	return nil
}

func (s *Store) GetUser(username string) (*models.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user := &models.User{}
	var configJSON sql.NullString
	var email sql.NullString

	err := s.db.QueryRow(`
		SELECT username, id, password_hash, email, created_at, active, banned, pending, role, config
		FROM users WHERE username = ?
	`, username).Scan(&user.Username, &user.ID, &user.PasswordHash, &email, &user.CreatedAt, &user.Active, &user.Banned, &user.Pending, &user.Role, &configJSON)

	if err != nil {
		if err != sql.ErrNoRows {
			logger.Error("Failed to get user %s: %v", username, err)
		}
		return nil, false
	}

	if email.Valid {
		user.Email = email.String
	}

	if configJSON.Valid && configJSON.String != "" {
		user.Config = &models.UserConfig{}
		if err := json.Unmarshal([]byte(configJSON.String), user.Config); err != nil {
			logger.Error("Failed to unmarshal user config for %s: %v", username, err)
		}
	}

	return user, true
}

func (s *Store) GetAllUsers() []*models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT username, id, password_hash, email, created_at, active, banned, pending, role, config
		FROM users
	`)
	if err != nil {
		logger.Error("Failed to get all users: %v", err)
		return nil
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		var configJSON sql.NullString
		var email sql.NullString

		err := rows.Scan(&user.Username, &user.ID, &user.PasswordHash, &email, &user.CreatedAt, &user.Active, &user.Banned, &user.Pending, &user.Role, &configJSON)
		if err != nil {
			logger.Error("Failed to scan user row: %v", err)
			continue
		}

		if email.Valid {
			user.Email = email.String
		}

		if configJSON.Valid && configJSON.String != "" {
			user.Config = &models.UserConfig{}
			if err := json.Unmarshal([]byte(configJSON.String), user.Config); err != nil {
				logger.Error("Failed to unmarshal user config: %v", err)
			}
		}

		users = append(users, user)
	}

	return users
}

func (s *Store) UpdateUserConfig(username string, config *models.UserConfig) error {
	logger.Debug("Updating config for user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	configJSON, err := json.Marshal(config)
	if err != nil {
		logger.Error("Failed to marshal user config: %v", err)
		return err
	}

	result, err := s.db.Exec(`UPDATE users SET config = ? WHERE username = ?`, configJSON, username)
	if err != nil {
		logger.Error("Failed to update config for user %s: %v", username, err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	logger.Debug("Successfully updated config for user: %s", username)
	return nil
}

func (s *Store) UpdateUserRole(username string, role models.Role) error {
	logger.Debug("Updating role for user %s to %s", username, role)
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`UPDATE users SET role = ? WHERE username = ?`, role, username)
	if err != nil {
		logger.Error("Failed to update role for user %s: %v", username, err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	logger.Info("Successfully updated role for user %s to %s", username, role)
	return nil
}

func (s *Store) BanUser(username string) error {
	logger.Debug("Banning user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`UPDATE users SET banned = 1 WHERE username = ?`, username)
	if err != nil {
		logger.Error("Failed to ban user %s: %v", username, err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	logger.Info("Successfully banned user: %s", username)
	return nil
}

func (s *Store) UnbanUser(username string) error {
	logger.Debug("Unbanning user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`UPDATE users SET banned = 0 WHERE username = ?`, username)
	if err != nil {
		logger.Error("Failed to unban user %s: %v", username, err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	logger.Info("Successfully unbanned user: %s", username)
	return nil
}

func (s *Store) DeleteUser(username string) error {
	logger.Debug("Deleting user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		logger.Error("Failed to delete user %s: %v", username, err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	logger.Info("Successfully deleted user: %s", username)
	return nil
}

func (s *Store) ApproveUser(username string) error {
	logger.Debug("Approving user: %s", username)
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`UPDATE users SET pending = 0, active = 1 WHERE username = ?`, username)
	if err != nil {
		logger.Error("Failed to approve user %s: %v", username, err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("User %s not found", username)
		return os.ErrNotExist
	}

	logger.Info("Successfully approved user: %s", username)
	return nil
}

// Settings operations

func (s *Store) GetSettings() *models.InstanceSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings := &models.InstanceSettings{}

	err := s.db.QueryRow(`
		SELECT signup_mode, motd, domain, network_name FROM settings WHERE id = 1
	`).Scan(&settings.SignupMode, &settings.MOTD, &settings.Domain, &settings.NetworkName)

	if err != nil {
		logger.Error("Failed to get settings: %v", err)
		return models.DefaultInstanceSettings()
	}

	return settings
}

func (s *Store) UpdateSettings(updates map[string]interface{}) error {
	logger.Debug("Updating instance settings")
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current settings first
	settings := &models.InstanceSettings{}
	err := s.db.QueryRow(`
		SELECT signup_mode, motd, domain, network_name FROM settings WHERE id = 1
	`).Scan(&settings.SignupMode, &settings.MOTD, &settings.Domain, &settings.NetworkName)
	if err != nil {
		logger.Error("Failed to get current settings: %v", err)
		return err
	}

	// Apply updates
	if signupMode, ok := updates["signup_mode"].(string); ok {
		settings.SignupMode = models.SignupMode(signupMode)
		logger.Debug("Updated signup_mode to: %s", signupMode)
	}

	if motd, ok := updates["motd"].(string); ok {
		settings.MOTD = motd
		logger.Debug("Updated motd")
	}

	if domain, ok := updates["domain"].(string); ok {
		settings.Domain = domain
		logger.Debug("Updated domain to: %s", domain)
	}

	if networkName, ok := updates["network_name"].(string); ok {
		settings.NetworkName = networkName
		logger.Debug("Updated network_name to: %s", networkName)
	}

	// Save updated settings
	_, err = s.db.Exec(`
		UPDATE settings SET signup_mode = ?, motd = ?, domain = ?, network_name = ? WHERE id = 1
	`, settings.SignupMode, settings.MOTD, settings.Domain, settings.NetworkName)

	if err != nil {
		logger.Error("Failed to save settings: %v", err)
		return err
	}

	// Sync MOTD to IRC config directory
	if err := s.syncMOTDFile(settings.MOTD); err != nil {
		logger.Error("Failed to sync MOTD file: %v", err)
		// Don't fail the whole save if MOTD sync fails
	}

	logger.Info("Successfully updated instance settings")
	return nil
}

func (s *Store) syncMOTDFile(motd string) error {
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
	if err := os.WriteFile(motdPath, []byte(motd), 0644); err != nil {
		return fmt.Errorf("failed to write MOTD file: %w", err)
	}

	logger.Debug("Successfully synced MOTD file")
	return nil
}

// Invite token operations

func (s *Store) CreateInviteToken(token *models.InviteToken) error {
	logger.Debug("Creating invite token: %s", token.Token)
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO invites (token, created_by, created_at, used, used_by, used_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token.Token, token.CreatedBy, token.CreatedAt, token.Used, token.UsedBy, token.UsedAt)

	if err != nil {
		logger.Error("Failed to create invite token %s: %v", token.Token, err)
		return err
	}

	logger.Info("Successfully created invite token: %s", token.Token)
	return nil
}

func (s *Store) GetInviteToken(token string) (*models.InviteToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	invite := &models.InviteToken{}
	var usedBy sql.NullString
	var usedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT token, created_by, created_at, used, used_by, used_at
		FROM invites WHERE token = ?
	`, token).Scan(&invite.Token, &invite.CreatedBy, &invite.CreatedAt, &invite.Used, &usedBy, &usedAt)

	if err != nil {
		if err != sql.ErrNoRows {
			logger.Error("Failed to get invite token %s: %v", token, err)
		}
		return nil, false
	}

	if usedBy.Valid {
		invite.UsedBy = usedBy.String
	}
	if usedAt.Valid {
		invite.UsedAt = usedAt.Time
	}

	return invite, true
}

func (s *Store) UseInviteToken(token, username string) error {
	logger.Debug("Marking invite token %s as used by %s", token, username)
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE invites SET used = 1, used_by = ?, used_at = ? WHERE token = ?
	`, username, now, token)

	if err != nil {
		logger.Error("Failed to use invite token %s: %v", token, err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("Invite token %s not found", token)
		return os.ErrNotExist
	}

	logger.Info("Successfully marked invite token %s as used", token)
	return nil
}
