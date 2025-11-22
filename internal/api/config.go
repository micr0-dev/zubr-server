package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
)

func (s *Server) handleGetUserConfig(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Get user config request from %s", r.RemoteAddr)

	// Get username from JWT context
	username, ok := r.Context().Value("username").(string)
	if !ok {
		logger.Error("Failed to get username from context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logger.Debug("Fetching config for user: %s", username)

	// Get user from storage
	user, exists := s.store.GetUser(username)
	if !exists {
		logger.Error("User %s not found", username)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// If user has no config, return default
	config := user.Config
	if config == nil {
		logger.Debug("User %s has no saved config, returning default", username)
		config = models.DefaultUserConfig()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(config); err != nil {
		logger.Error("Failed to encode config for user %s: %v", username, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("Successfully returned config for user: %s", username)
}

func (s *Server) handleUpdateUserConfig(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Update user config request from %s", r.RemoteAddr)

	// Get username from JWT context
	username, ok := r.Context().Value("username").(string)
	if !ok {
		logger.Error("Failed to get username from context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logger.Debug("Updating config for user: %s", username)

	// Parse request body
	var config models.UserConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		logger.Error("Failed to decode config for user %s: %v", username, err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure clientSettings is initialized (never nil)
	if config.ClientSettings == nil {
		config.ClientSettings = make(map[string]interface{})
	}

	// Ensure networks is initialized (never nil)
	if config.Networks == nil {
		config.Networks = []models.Network{}
	}

	// Update user config in storage
	if err := s.store.UpdateUserConfig(username, &config); err != nil {
		if os.IsNotExist(err) {
			logger.Error("User %s not found", username)
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		logger.Error("Failed to update config for user %s: %v", username, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuration updated successfully",
	})

	logger.Info("Successfully updated config for user: %s", username)
}
