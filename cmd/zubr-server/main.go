package main

import (
	"time"

	"github.com/BurntSushi/toml"
	"github.com/micr0/zubr-server/internal/api"
	"github.com/micr0/zubr-server/internal/irc"
	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/storage"
)

type Config struct {
	Server struct {
		Address string `toml:"address"`
		Domain  string `toml:"domain"`
	} `toml:"server"`
	Storage struct {
		DatabasePath string `toml:"database_path"`
	} `toml:"storage"`
	IRC struct {
		InspircdPath string `toml:"inspircd_path"`
		ConfigPath   string `toml:"config_path"`
		AutoStart    bool   `toml:"auto_start"`
	} `toml:"irc"`
}

func main() {
	logger.Info("Starting Zubr Server...")
	logger.Debug("Production mode: %v", logger.IsProduction())

	// Load config
	logger.Debug("Loading configuration from config.toml")
	var config Config
	if _, err := toml.DecodeFile("config.toml", &config); err != nil {
		logger.Fatal("Error loading config: %v", err)
	}
	logger.Debug("Configuration loaded successfully")
	logger.Debug("Server address: %s", config.Server.Address)
	logger.Debug("Server domain: %s", config.Server.Domain)
	logger.Debug("IRC config path: %s", config.IRC.ConfigPath)

	// Initialize storage
	store, err := storage.New(config.Storage.DatabasePath, config.IRC.ConfigPath)
	if err != nil {
		logger.Fatal("Error initializing storage: %v", err)
	}

	// Initialize IRC manager
	logger.Debug("Initializing IRC manager")
	ircManager := irc.NewManager(
		config.IRC.InspircdPath,
		config.IRC.ConfigPath,
		config.Server.Domain,
		config.Server.Address,
	)

	// Generate InspIRCd configuration
	logger.Info("Generating InspIRCd configuration...")
	if err := ircManager.GenerateConfig(); err != nil {
		logger.Fatal("Failed to generate InspIRCd config: %v", err)
	}

	// Start API server first (in goroutine so InspIRCd can curl it)
	logger.Info("Starting API server on %s", config.Server.Address)
	server := api.NewServer(store, ircManager, config.Server.Address)
	go func() {
		if err := server.Start(); err != nil {
			logger.Fatal("Error starting API server: %v", err)
		}
	}()

	// Wait for API to be ready before starting InspIRCd
	time.Sleep(500 * time.Millisecond)

	// Start IRC server (if auto_start is enabled)
	if config.IRC.AutoStart {
		logger.Info("Auto-starting InspIRCd...")
		if err := ircManager.Start(); err != nil {
			logger.Error("Warning: Could not start InspIRCd: %v", err)
			logger.Info("You may need to start InspIRCd manually")
		}
	} else {
		logger.Info("InspIRCd auto-start is disabled")
		logger.Info("Manage InspIRCd separately (systemd, manual, etc.)")
		logger.Debug("Set 'auto_start = true' in config.toml to enable auto-start")
	}

	// Keep main running
	select {}
}
