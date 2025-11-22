package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/micr0/zubr-server/internal/logger"
)

type IRCConfigResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Config  string `json:"config,omitempty"`
}

// handleGenerateIRCConfig generates a new InspIRCd configuration file
func (s *Server) handleGenerateIRCConfig(w http.ResponseWriter, r *http.Request) {
	logger.Debug("IRC config generation requested from %s", r.RemoteAddr)

	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate the config
	if err := s.ircManager.GenerateConfig(); err != nil {
		logger.Error("Failed to generate IRC config: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(IRCConfigResponse{
			Success: false,
			Message: "Failed to generate IRC config: " + err.Error(),
		})
		return
	}

	logger.Info("IRC config generated successfully")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(IRCConfigResponse{
		Success: true,
		Message: "InspIRCd configuration generated successfully",
	})
}

// handleGetIRCConfig returns the current InspIRCd configuration
func (s *Server) handleGetIRCConfig(w http.ResponseWriter, r *http.Request) {
	logger.Debug("IRC config view requested from %s", r.RemoteAddr)

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the config file path
	configPath := s.ircManager.GetConfigPath()
	configFile := filepath.Join(configPath, "inspircd.conf")

	logger.Debug("Reading IRC config from: %s", configFile)

	// Read the config file
	content, err := os.ReadFile(configFile)
	if err != nil {
		logger.Error("Failed to read IRC config: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(IRCConfigResponse{
			Success: false,
			Message: "IRC config not found. Generate it first using POST /api/irc/config/generate",
		})
		return
	}

	logger.Debug("Successfully read IRC config (%d bytes)", len(content))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(IRCConfigResponse{
		Success: true,
		Message: "IRC configuration retrieved successfully",
		Config:  string(content),
	})
}
