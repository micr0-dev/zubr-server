package irc

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/micr0/zubr-server/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

type Manager struct {
	inspircdPath string
	configPath   string
	networkName  string
	domain       string
	db           *sql.DB
}

func NewManager(inspircdPath, configPath, networkName, domain string) *Manager {
	m := &Manager{
		inspircdPath: inspircdPath,
		configPath:   configPath,
		networkName:  networkName,
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
	configFile := filepath.Join(m.configPath, "inspircd.conf")
	logger.Debug("Starting InspIRCd with config: %s", configFile)
	cmd := exec.Command(m.inspircdPath, "--config", configFile)
	err := cmd.Start()
	if err != nil {
		logger.Error("Failed to start InspIRCd: %v", err)
	} else {
		logger.Info("InspIRCd started successfully")
	}
	return err
}

func (m *Manager) Stop() error {
	cmd := exec.Command("killall", "inspircd")
	return cmd.Run()
}

func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	return m.Start()
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

	// Generate comprehensive InspIRCd config with SQL authentication
	tmpl := `<config format="xml">

<!-- Server Information -->
<server
    name="irc.{{.domain}}"
    description="{{.networkName}} IRC Server"
    network="{{.networkName}}"
    id="001">

<!-- Admin Information -->
<admin
    name="Server Admin"
    nick="admin"
    email="admin@{{.domain}}">

<!-- Network Binds -->
<bind
    address=""
    port="6667"
    type="clients">

<!-- Optional SSL Bind (requires SSL setup)
<bind
    address=""
    port="6697"
    type="clients"
    ssl="gnutls">
-->

<!-- Server Options -->
<options
    prefixquit="Quit: "
    suffixquit=""
    prefixpart="&quot;"
    suffixpart="&quot;"
    syntaxhints="yes"
    cyclehosts="yes"
    cyclehostsfromuser="no"
    ircumsgprefix="no"
    announcets="yes"
    allowmismatch="no"
    defaultbind="auto"
    hostintopic="yes"
    pingwarning="15"
    serverpingfreq="60"
    defaultmodes="nt"
    moronbanner="You're banned!"
    exemptchanops=""
    invitebypassmodes="yes">

<!-- Performance/Security -->
<performance
    netbuffersize="10240"
    somaxconn="128"
    softlimit="12800"
    quietbursts="yes"
    nouserdns="no">

<!-- Security Settings -->
<security
    announceinvites="dynamic"
    hidemodes="eI"
    hideulines="no"
    flatlinks="no"
    hidewhois=""
    hidebans="no"
    hidekills=""
    hidesplits="no"
    maxtargets="20"
    customversion=""
    operspywhois="no"
    restrictbannedusers="yes"
    genericoper="no"
    userstats="Pu">

<!-- Limits -->
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

<!-- Log Settings -->
<log method="file" type="*" level="default" target="/var/log/inspircd/ircd.log">

<!-- Load Required Modules -->
<module name="sqlauth">
<module name="sqlite3">
<module name="password_hash">
<module name="sha256">

<!-- SQLite Database Configuration -->
<database
    module="sqlite"
    hostname="{{.configPath}}/users.db"
    id="irc_users">

<!-- SQL Authentication -->
<sqlauth
    dbid="irc_users"
    query="SELECT password FROM users WHERE username='$username'"
    hash="sha256"
    allowpattern="*"
    verbose="yes">

<!-- Connect Class - Requires Authentication -->
<connect
    name="users"
    allow="*"
    timeout="60"
    flood="20"
    threshold="10"
    pingfreq="120"
    sendq="262144"
    recvq="8192"
    localmax="3"
    globalmax="3"
    maxchans="20"
    port="6667"
    requireaccount="yes">

<!-- Channels Configuration -->
<channels
    users="20">

<!-- DNS Settings -->
<dns
    server="8.8.8.8"
    timeout="5">

<!-- Disabled/Filtered Commands -->
<disabled
    commands=""
    chanmodes=""
    usermodes="">

</config>`

	data := map[string]string{
		"domain":      m.domain,
		"networkName": m.networkName,
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

// Close closes the database connection
func (m *Manager) Close() error {
	if m.db != nil {
		logger.Debug("Closing IRC user database")
		return m.db.Close()
	}
	return nil
}
