package main

import (
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
		UsersFile string `toml:"users_file"`
	} `toml:"storage"`
	IRC struct {
		InspircdPath string `toml:"inspircd_path"`
		ConfigPath   string `toml:"config_path"`
		NetworkName  string `toml:"network_name"`
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
	store, err := storage.New(config.Storage.UsersFile)
	if err != nil {
		logger.Fatal("Error initializing storage: %v", err)
	}

	// Initialize IRC manager
	logger.Debug("Initializing IRC manager")
	ircManager := irc.NewManager(
		config.IRC.InspircdPath,
		config.IRC.ConfigPath,
		config.IRC.NetworkName,
		config.Server.Domain,
	)

	// Start IRC server
	if err := ircManager.Start(); err != nil {
		logger.Error("Warning: Could not start InspIRCd: %v", err)
	}

	// Start API server
	logger.Info("Starting API server on %s", config.Server.Address)
	server := api.NewServer(store, ircManager, config.Server.Address)
	if err := server.Start(); err != nil {
		logger.Fatal("Error starting API server: %v", err)
	}
}
