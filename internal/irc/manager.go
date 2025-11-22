package irc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/micr0/zubr-server/internal/logger"
)

type Manager struct {
	inspircdPath string
	configPath   string
	networkName  string
	domain       string
}

func NewManager(inspircdPath, configPath, networkName, domain string) *Manager {
	return &Manager{
		inspircdPath: inspircdPath,
		configPath:   configPath,
		networkName:  networkName,
		domain:       domain,
	}
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
	logger.Debug("IRC config path: %s", m.configPath)

	// For now, just add to opers.conf
	// Later: proper user management
	operConfig := filepath.Join(m.configPath, "opers.conf")
	logger.Debug("Oper config file path: %s", operConfig)

	// Read existing config
	data, err := os.ReadFile(operConfig)
	if err != nil {
		logger.Error("Failed to read opers.conf at %s: %v", operConfig, err)
		logger.Debug("Attempting to create opers.conf file and parent directories")

		// Try to create the directory if it doesn't exist
		if err := os.MkdirAll(m.configPath, 0755); err != nil {
			logger.Error("Failed to create config directory %s: %v", m.configPath, err)
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Initialize with empty data
		data = []byte{}
		logger.Debug("Created empty opers.conf")
	} else {
		logger.Debug("Successfully read existing opers.conf (%d bytes)", len(data))
	}

	// Append new user (simple for now)
	userBlock := fmt.Sprintf(`
<oper
    name="%s"
    password="%s"
    host="*@*"
    type="NetAdmin">
`, username, password)

	logger.Debug("Writing updated opers.conf with new user %s", username)
	if err := os.WriteFile(operConfig, append(data, []byte(userBlock)...), 0644); err != nil {
		logger.Error("Failed to write opers.conf: %v", err)
		return fmt.Errorf("failed to write opers.conf: %w", err)
	}

	logger.Info("Successfully created IRC user: %s", username)
	return nil
}

func (m *Manager) GenerateConfig() error {
	// Generate basic InspIRCd config
	tmpl := `<config format="xml">
<server name="irc.{{.Domain}}" description="{{.NetworkName}} IRC Server" network="{{.NetworkName}}">
<admin name="admin" description="Server Administrator" email="admin@{{.Domain}}">
<bind address="" port="6667" type="clients">
<bind address="" port="6697" type="clients" ssl="gnutls">
</config>`

	t, err := template.New("config").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(m.configPath, "inspircd.conf"))
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, m)
}
