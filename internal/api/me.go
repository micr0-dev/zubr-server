package api

import (
	"encoding/json"
	"net/http"

	"github.com/micr0/zubr-server/internal/logger"
)

type MeResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	CreatedAt string `json:"created_at"`
	Active    bool   `json:"active"`
	Banned    bool   `json:"banned"`
	Role      string `json:"role"`
}

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Get current user request from %s", r.RemoteAddr)

	// Get username from context (set by authMiddleware)
	username, ok := r.Context().Value("username").(string)
	if !ok {
		logger.Error("Username not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user from storage
	user, exists := s.store.GetUser(username)
	if !exists {
		logger.Error("User %s not found", username)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Build response (exclude password hash)
	response := MeResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Active:    user.Active,
		Banned:    user.Banned,
		Role:      string(user.Role),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	logger.Debug("Returned current user info for: %s", username)
}
