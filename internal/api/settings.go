package api

import (
	"encoding/json"
	"net/http"

	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
)

func (s *Server) handleGetInstanceSettings(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Get instance settings request from %s", r.RemoteAddr)

	settings := s.store.GetSettings()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)

	logger.Debug("Returned instance settings")
}

func (s *Server) handleUpdateInstanceSettings(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Update instance settings request from %s", r.RemoteAddr)

	// Get requester info from context
	requester, ok := r.Context().Value("user").(*models.User)
	if !ok {
		logger.Error("Requester user not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body as generic map for PATCH
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		logger.Error("Failed to decode settings update: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate signup_mode if present
	if signupMode, ok := updates["signup_mode"].(string); ok {
		validMode := false
		for _, mode := range []models.SignupMode{models.SignupModePublic, models.SignupModeApproval, models.SignupModeInvite} {
			if models.SignupMode(signupMode) == mode {
				validMode = true
				break
			}
		}
		if !validMode {
			http.Error(w, "Invalid signup_mode. Must be 'public', 'approval', or 'invite'", http.StatusBadRequest)
			return
		}
	}

	// Update settings
	if err := s.store.UpdateSettings(updates); err != nil {
		logger.Error("Failed to update settings: %v", err)
		http.Error(w, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	logger.Info("Instance settings updated by %s", requester.Username)

	// Return updated settings
	settings := s.store.GetSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}
