package irc

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/micr0/zubr-server/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

type Manager struct {
	inspircdPath string
	configPath   string
	domain       string
	db           *sql.DB
	bot          *Bot
}

func NewManager(inspircdPath, configPath, domain string) *Manager {
	m := &Manager{
		inspircdPath: inspircdPath,
		configPath:   configPath,
		domain:       domain,
	}

	// Initialize the IRC users database
	if err := m.initDB(); err != nil {
		logger.Error("Failed to initialize IRC user database: %v", err)
	}

	return m
}

// initDB initializes the SQLite database for IRC user authentication
func (m *Manager) initDB() error {
	// Ensure config directory exists
	if err := os.MkdirAll(m.configPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	dbPath := filepath.Join(m.configPath, "users.db")
	logger.Debug("Initializing IRC user database at: %s", dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Create users table
	createTable := `
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(createTable); err != nil {
		db.Close()
		return fmt.Errorf("failed to create users table: %w", err)
	}

	m.db = db
	logger.Info("IRC user database initialized")
	return nil
}

func (m *Manager) Start() error {
	logger.Debug("Starting InspIRCd with path: %s", m.inspircdPath)

	// Check if InspIRCd is already running by attempting to start it
	cmd := exec.Command(m.inspircdPath, "start")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// If output indicates already running, restart to apply new config
	if strings.Contains(outputStr, "already running") {
		logger.Info("InspIRCd is already running, restarting to apply new configuration...")
		if err := m.Restart(); err != nil {
			logger.Error("Failed to restart InspIRCd: %v", err)
			return err
		}
		logger.Info("InspIRCd restarted successfully")
		return nil
	}

	// Handle other errors
	if err != nil {
		logger.Error("Failed to start InspIRCd: %v, output: %s", err, outputStr)
		return err
	}

	logger.Info("InspIRCd started successfully: %s", outputStr)

	// Bot is not needed - using InspIRCd's built-in +M mode (m_services_account)
	// Channels with +M mode only allow authenticated users to speak
	// Unauthenticated users can join and read but not send messages

	return nil
}

// startBot initializes and connects the services bot
func (m *Manager) startBot() error {
	// Create bot user in database first
	botPassword := "zubrservices"
	if err := m.CreateUser("ZubrBot", botPassword); err != nil {
		logger.Error("Failed to create bot user: %v", err)
	}

	// Create and connect bot
	m.bot = NewBot("127.0.0.1:6667", "ZubrBot", botPassword)
	if err := m.bot.Connect(); err != nil {
		return fmt.Errorf("failed to connect bot: %w", err)
	}

	logger.Info("Services bot connected successfully")
	return nil
}

func (m *Manager) Stop() error {
	logger.Debug("Stopping InspIRCd")
	cmd := exec.Command(m.inspircdPath, "stop")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to stop InspIRCd: %v, output: %s", err, string(output))
	} else {
		logger.Info("InspIRCd stopped: %s", string(output))
	}
	return err
}

func (m *Manager) Restart() error {
	logger.Debug("Restarting InspIRCd")
	cmd := exec.Command(m.inspircdPath, "restart")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to restart InspIRCd: %v, output: %s", err, string(output))
	} else {
		logger.Info("InspIRCd restarted: %s", string(output))
	}
	return err
}

func (m *Manager) CreateUser(username, password string) error {
	logger.Debug("Creating IRC user: %s", username)

	if m.db == nil {
		return fmt.Errorf("IRC user database not initialized")
	}

	// Hash the password with SHA256 for InspIRCd
	hasher := sha256.New()
	hasher.Write([]byte(password))
	hashedPassword := hex.EncodeToString(hasher.Sum(nil))

	logger.Debug("Adding user %s to IRC database", username)

	// Insert or replace user in database
	query := `INSERT OR REPLACE INTO users (username, password) VALUES (?, ?)`
	_, err := m.db.Exec(query, username, hashedPassword)
	if err != nil {
		logger.Error("Failed to create IRC user %s: %v", username, err)
		return fmt.Errorf("failed to insert user into database: %w", err)
	}

	logger.Info("Successfully created IRC user: %s", username)
	return nil
}

// DeleteUser removes a user from the IRC authentication database
func (m *Manager) DeleteUser(username string) error {
	logger.Debug("Deleting IRC user: %s", username)

	if m.db == nil {
		return fmt.Errorf("IRC user database not initialized")
	}

	query := `DELETE FROM users WHERE username = ?`
	result, err := m.db.Exec(query, username)
	if err != nil {
		logger.Error("Failed to delete IRC user %s: %v", username, err)
		return fmt.Errorf("failed to delete user from database: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Debug("User %s not found in IRC database", username)
		return fmt.Errorf("user not found")
	}

	logger.Info("Successfully deleted IRC user: %s", username)
	return nil
}

// UserExists checks if a user exists in the IRC database
func (m *Manager) UserExists(username string) (bool, error) {
	if m.db == nil {
		return false, fmt.Errorf("IRC user database not initialized")
	}

	var count int
	query := `SELECT COUNT(*) FROM users WHERE username = ?`
	err := m.db.QueryRow(query, username).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (m *Manager) GenerateConfig() error {
	logger.Debug("Generating InspIRCd configuration")

	// Generate InspIRCd config in their format (using # for comments)
	tmpl := `#-#-#-#-#-#-#-#-#-#-  ZUBR SERVER CONFIG  #-#-#-#-#-#-#-#-#-#-#-
# Auto-generated by Zubr Server
# Network: {{.networkName}}

#-#-#-#-#-#-#-#-#-#-  SERVER DESCRIPTION  #-#-#-#-#-#-#-#-#-#-#

<server
        name="irc.{{.domain}}"
        description="{{.networkName}} IRC Server"
        network="{{.networkName}}"
        id="001"
        motd="{{.configPath}}/motd.txt">

#-#-#-#-#-#-#-#-#-#-#-  ADMIN INFO  #-#-#-#-#-#-#-#-#-#-#-#-#-

<admin
        name="Server Admin"
        nick="admin"
        email="admin@{{.domain}}">

#-#-#-#-#-#-#-#-#-#-#-#-  PORT CONFIG  #-#-#-#-#-#-#-#-#-#-#-

<bind
        address=""
        port="6667"
        type="clients">

#-#-#-#-#-#-#-#-#-#-#-  LOADMODULE  #-#-#-#-#-#-#-#-#-#-#-#-
# SQL authentication modules
<module name="sqlauth">
<module name="sqlite3">
<module name="password_hash">
<module name="sha256">

# Channel moderation modules (removed - allowing all users to speak)

#-#-#-#-#-#-#-#-#-#-#-  DATABASE  #-#-#-#-#-#-#-#-#-#-#-#-#-

<database
        module="sqlite"
        hostname="{{.configPath}}/users.db"
        id="irc_users">

#-#-#-#-#-#-#-#-#-#-  SQL AUTHENTICATION  #-#-#-#-#-#-#-#-#-#

<sqlauth
        dbid="irc_users"
        query="SELECT password FROM users WHERE username='$username'"
        hash="sha256"
        allowpattern="*"
        verbose="yes">

#-#-#-#-#-#-#-#-#-#-#-#-  CONNECT  #-#-#-#-#-#-#-#-#-#-#-#-#

# Localhost exempt from connection limits
<connect
        name="localhost"
        allow="127.0.0.1"
        maxchans="20"
        timeout="60"
        limit="5000"
        localmax="5000"
        globalmax="5000"
        exempt="*">

# Authenticated users (have logged in via zubr-server)
<connect
        name="authenticated"
        allow="*"
        timeout="60"
        flood="20"
        threshold="10"
        pingfreq="120"
        sendq="262144"
        recvq="8192"
        localmax="100"
        globalmax="100"
        maxchans="20"
        port="6667">

# Guest users (unauthenticated, read-only in +M channels)
# Note: This is commented out by default. Uncomment for public mode.
# <connect
#         name="guests"
#         allow="*"
#         timeout="60"
#         pingfreq="120"
#         sendq="262144"
#         recvq="8192"
#         localmax="50"
#         globalmax="50"
#         maxchans="10"
#         port="6667">

#-#-#-#-#-#-#-#-#-#-#-#-  CHANNELS  #-#-#-#-#-#-#-#-#-#-#-#-

<channels
        users="20">

#-#-#-#-#-#-#-#-#-#-#-#-  DNS  #-#-#-#-#-#-#-#-#-#-#-#-#-#-

<dns
        server="8.8.8.8"
        timeout="5">

#-#-#-#-#-#-#-#-#-#-#-#-  OPTIONS  #-#-#-#-#-#-#-#-#-#-#-#-

<options
        prefixquit="Quit: "
        suffixquit=""
        syntaxhints="yes"
        defaultbind="auto"
        hostintopic="yes"
        pingwarning="15"
        serverpingfreq="60"
        defaultmodes="nt">

#-#-#-#-#-#-#-#-#-#-#-  PERFORMANCE  #-#-#-#-#-#-#-#-#-#-#-

<performance
        netbuffersize="10240"
        somaxconn="128"
        softlimit="12800"
        quietbursts="yes"
        nouserdns="no">

#-#-#-#-#-#-#-#-#-#-#-#-  SECURITY  #-#-#-#-#-#-#-#-#-#-#-#

<security
        announceinvites="dynamic"
        hidemodes="eI"
        flatlinks="no"
        maxtargets="20"
        restrictbannedusers="yes">

#-#-#-#-#-#-#-#-#-#-#-#-  LIMITS  #-#-#-#-#-#-#-#-#-#-#-#-#

<limits
        maxnick="30"
        maxchan="64"
        maxmodes="20"
        maxident="11"
        maxquit="255"
        maxtopic="307"
        maxkick="255"
        maxgecos="128"
        maxaway="200">
`

	data := map[string]string{
		"domain":      m.domain,
		"networkName": m.domain,
		"configPath":  m.configPath,
	}

	t, err := template.New("config").Parse(tmpl)
	if err != nil {
		logger.Error("Failed to parse config template: %v", err)
		return err
	}

	configFile := filepath.Join(m.configPath, "inspircd.conf")
	f, err := os.Create(configFile)
	if err != nil {
		logger.Error("Failed to create config file: %v", err)
		return err
	}
	defer f.Close()

	if err := t.Execute(f, data); err != nil {
		logger.Error("Failed to write config file: %v", err)
		return err
	}

	logger.Info("Generated InspIRCd configuration at: %s", configFile)
	return nil
}

// GetConfigPath returns the IRC config directory path
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// GiveVoice grants voice (+v) to a user in a channel
func (m *Manager) GiveVoice(channel, username string) error {
	if m.bot == nil || !m.bot.IsConnected() {
		return fmt.Errorf("services bot not connected")
	}
	logger.Debug("Granting voice to %s in %s", username, channel)
	return m.bot.GiveVoice(channel, username)
}

// RemoveVoice removes voice (+v) from a user in a channel
func (m *Manager) RemoveVoice(channel, username string) error {
	if m.bot == nil || !m.bot.IsConnected() {
		return fmt.Errorf("services bot not connected")
	}
	logger.Debug("Removing voice from %s in %s", username, channel)
	return m.bot.RemoveVoice(channel, username)
}

// SetChannelMode sets a mode on a channel
func (m *Manager) SetChannelMode(channel, mode string) error {
	if m.bot == nil || !m.bot.IsConnected() {
		return fmt.Errorf("services bot not connected")
	}
	logger.Debug("Setting mode %s on channel %s", mode, channel)
	return m.bot.SetChannelMode(channel, mode)
}

// KickUser kicks a user from the IRC server
func (m *Manager) KickUser(username, reason string) error {
	if m.bot == nil || !m.bot.IsConnected() {
		return fmt.Errorf("services bot not connected")
	}
	logger.Debug("Kicking user %s from IRC server: %s", username, reason)
	return m.bot.KillUser(username, reason)
}

// Close closes the database connection
func (m *Manager) Close() error {
	if m.db != nil {
		logger.Debug("Closing IRC user database")
		return m.db.Close()
	}
	return nil
}
