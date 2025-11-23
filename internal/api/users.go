package api

import (
	"encoding/json"
	"net/http"

	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
)

type UserListItem struct {
	ID        string       `json:"id"`
	Username  string       `json:"username"`
	Email     string       `json:"email,omitempty"`
	CreatedAt string       `json:"created_at"`
	Active    bool         `json:"active"`
	Role      models.Role  `json:"role"`
}

type UserListResponse struct {
	Users []UserListItem `json:"users"`
	Total int            `json:"total"`
}

func (s *Server) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Get users request from %s", r.RemoteAddr)

	// Get all users from storage
	users := s.store.GetAllUsers()

	// Convert to response format (exclude sensitive data like password hash)
	userList := make([]UserListItem, 0, len(users))
	for _, user := range users {
		userList = append(userList, UserListItem{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Active:    user.Active,
			Role:      user.Role,
		})
	}

	response := UserListResponse{
		Users: userList,
		Total: len(userList),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	logger.Info("Returned %d users", len(userList))
}
