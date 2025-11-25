package irc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/micr0/zubr-server/internal/logger"
)

type Manager struct {
	inspircdPath string
	configPath   string
	domain       string
	apiAddr      string
	db           *sql.DB
	bot          *Bot
}

func NewManager(inspircdPath, configPath, domain, apiAddr string) *Manager {
	m := &Manager{
		inspircdPath: inspircdPath,
		configPath:   configPath,
		domain:       domain,
		apiAddr:      apiAddr,
	}

	// Initialize the IRC users database
	if err := m.initDB(); err != nil {
		logger.Error("Failed to initialize IRC user database: %v", err)
	}

	// Generate TLS certificates if they don't exist
	if err := m.ensureTLSCerts(); err != nil {
		logger.Error("Failed to generate TLS certificates: %v", err)
	}

	return m
}

// ensureTLSCerts generates self-signed TLS certificates if they don't exist
func (m *Manager) ensureTLSCerts() error {
	certPath := filepath.Join(m.configPath, "cert.pem")
	keyPath := filepath.Join(m.configPath, "key.pem")

	// Check if certs already exist
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			logger.Debug("TLS certificates already exist")
			return nil
		}
	}

	logger.Info("Generating self-signed TLS certificates...")

	// Generate ECDSA private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: m.domain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{m.domain},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate to file
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key to file
	keyFile, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	logger.Info("TLS certificates generated at %s and %s", certPath, keyPath)
	return nil
}

// hasTLSCerts checks if TLS certificates exist
func (m *Manager) hasTLSCerts() bool {
	certPath := filepath.Join(m.configPath, "cert.pem")
	keyPath := filepath.Join(m.configPath, "key.pem")

	if _, err := os.Stat(certPath); err != nil {
		return false
	}
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}
	return true
}

// hasSSLModule checks if the ssl_openssl module exists
func (m *Manager) hasSSLModule() bool {
	// Check common locations for the module
	inspircdDir := filepath.Dir(m.inspircdPath)
	modulePaths := []string{
		filepath.Join(inspircdDir, "modules", "m_ssl_openssl.so"),
		filepath.Join(inspircdDir, "..", "lib", "inspircd", "m_ssl_openssl.so"),
		"/usr/lib/inspircd/m_ssl_openssl.so",
		"/usr/lib64/inspircd/m_ssl_openssl.so",
	}

	for _, path := range modulePaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
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
        name="{{.domain}}"
        description="{{.networkName}} IRC Server"
        network="{{.networkName}}"
        id="001">

#-#-#-#-#-#-#-#-#-#-#-  DYNAMIC MOTD  #-#-#-#-#-#-#-#-#-#-#-

<execfiles motd="curl -s http://{{.apiAddr}}/api/motd.txt">

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
{{if .enableTLS}}
<bind
        address=""
        port="6697"
        type="clients"
        ssl="openssl">
{{end}}
#-#-#-#-#-#-#-#-#-#-#-  LOADMODULE  #-#-#-#-#-#-#-#-#-#-#-#-
# SQL authentication modules
<module name="sqlauth">
<module name="sqlite3">
<module name="password_hash">
<module name="sha256">

# Channel management modules
<module name="permchannels">
<module name="chanhistory">

# IRCv3 extensions
<module name="cap">
<module name="ircv3">
<module name="ircv3_batch">
<module name="ircv3_servertime">
<module name="ircv3_msgid">
<module name="setname">
{{if .enableTLS}}
# TLS/SSL support
<module name="ssl_openssl">

#-#-#-#-#-#-#-#-#-#-#-  TLS CONFIG  #-#-#-#-#-#-#-#-#-#-#-#-

<openssl
        certfile="{{.configPath}}/cert.pem"
        keyfile="{{.configPath}}/key.pem"
        dhfile=""
        hash="sha256"
        ciphersuites="TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256:TLS_AES_128_GCM_SHA256">
{{end}}
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
        allow="127.0.0.0/8"
        resolvehostnames="no"
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

#-#-#-#-#-#-#-#-#-#-#-  CHANNEL HISTORY  #-#-#-#-#-#-#-#-#-#-#

<chanhistory maxduration="4w"
             maxlines="100"
             prefixmsg="yes"
             savefrombots="yes"
             sendtobots="yes">

#-#-#-#-#-#-#-#-#-#-#-  PERMANENT CHANNELS  #-#-#-#-#-#-#-#-#-#

<permchannels channel="#general" modes="+ntH 100:4w" topic="Welcome to Zubr!">

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
        defaultmodes="+ntH 100:4w">

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

	// Replace 0.0.0.0 with 127.0.0.1 for curl to work
	apiAddr := strings.Replace(m.apiAddr, "0.0.0.0", "127.0.0.1", 1)

	// Check if TLS should be enabled (certs exist and module available)
	enableTLS := m.hasTLSCerts()
	if enableTLS {
		logger.Info("TLS certificates found, enabling TLS on port 6697")
	} else {
		logger.Info("TLS disabled (certificates not found)")
	}

	data := map[string]interface{}{
		"domain":      m.domain,
		"networkName": m.domain,
		"configPath":  m.configPath,
		"apiAddr":     apiAddr,
		"enableTLS":   enableTLS,
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

// WriteMOTD writes the message of the day to the motd.txt file
func (m *Manager) WriteMOTD(motd string) error {
	motdFile := filepath.Join(m.configPath, "motd.txt")
	if err := os.WriteFile(motdFile, []byte(motd), 0644); err != nil {
		logger.Error("Failed to write MOTD file: %v", err)
		return err
	}
	logger.Debug("Wrote MOTD to: %s", motdFile)
	return nil
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
