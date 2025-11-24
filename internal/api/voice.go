package api

import (
	"encoding/json"
	"net/http"

	"github.com/micr0/zubr-server/internal/logger"
)

type VoiceRequest struct {
	Channel string `json:"channel"`
}

type VoiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// handleGrantVoice grants voice to the authenticated user in a channel
func (s *Server) handleGrantVoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get username from context (set by authMiddleware)
	username, ok := r.Context().Value("username").(string)
	if !ok {
		logger.Error("Username not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req VoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Debug("Failed to decode voice request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Channel == "" {
		http.Error(w, "Channel is required", http.StatusBadRequest)
		return
	}

	logger.Debug("Granting voice to %s in %s", username, req.Channel)

	// Grant voice to user in channel
	if err := s.ircManager.GiveVoice(req.Channel, username); err != nil {
		logger.Error("Failed to grant voice to %s in %s: %v", username, req.Channel, err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VoiceResponse{
			Success: false,
			Message: "Failed to grant voice: " + err.Error(),
		})
		return
	}

	logger.Info("Granted voice to %s in %s", username, req.Channel)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VoiceResponse{
		Success: true,
		Message: "Voice granted successfully",
	})
}
