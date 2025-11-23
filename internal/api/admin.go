package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
)

type AdminResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type UserPermissionsResponse struct {
	TargetUser   string       `json:"target_user"`
	TargetRole   models.Role  `json:"target_role"`
	CanPromote   bool         `json:"can_promote"`
	CanDemote    bool         `json:"can_demote"`
	CanBan       bool         `json:"can_ban"`
	CanKick      bool         `json:"can_kick"`
	CanDelete    bool         `json:"can_delete"`
}

// extractUsername extracts username from URL path like /api/admin/users/username/action
func extractUsername(path string) string {
	// Expected format: /api/admin/users/:username/action
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 4 {
		return parts[3] // username is at index 3
	}
	return ""
}

func (s *Server) handlePromoteUser(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Promote user request from %s", r.RemoteAddr)

	targetUsername := extractUsername(r.URL.Path)
	if targetUsername == "" {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get requester info from context
	requester, ok := r.Context().Value("user").(*models.User)
	if !ok {
		logger.Error("Requester user not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get target user
	targetUser, exists := s.store.GetUser(targetUsername)
	if !exists {
		logger.Error("Target user %s not found", targetUsername)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Check if target is already admin or owner
	if targetUser.Role == models.RoleAdmin {
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "User is already an admin",
		})
		return
	}

	if targetUser.Role == models.RoleOwner {
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Cannot promote owner",
		})
		return
	}

	// Promote to admin
	if err := s.store.UpdateUserRole(targetUsername, models.RoleAdmin); err != nil {
		logger.Error("Failed to promote user %s: %v", targetUsername, err)
		http.Error(w, "Failed to promote user", http.StatusInternalServerError)
		return
	}

	logger.Info("User %s promoted to admin by %s", targetUsername, requester.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AdminResponse{
		Success: true,
		Message: "User promoted to admin",
	})
}

func (s *Server) handleDemoteUser(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Demote user request from %s", r.RemoteAddr)

	targetUsername := extractUsername(r.URL.Path)
	if targetUsername == "" {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get requester info from context
	requester, ok := r.Context().Value("user").(*models.User)
	if !ok {
		logger.Error("Requester user not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get target user
	targetUser, exists := s.store.GetUser(targetUsername)
	if !exists {
		logger.Error("Target user %s not found", targetUsername)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Permission checks
	// Cannot demote owner
	if targetUser.Role == models.RoleOwner {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Cannot demote owner",
		})
		return
	}

	// Admins cannot demote other admins (only owner can)
	if targetUser.Role == models.RoleAdmin && requester.Role != models.RoleOwner {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Only owner can demote admins",
		})
		return
	}

	// Check if already a regular user
	if targetUser.Role == models.RoleUser {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "User is already a regular user",
		})
		return
	}

	// Demote to user
	if err := s.store.UpdateUserRole(targetUsername, models.RoleUser); err != nil {
		logger.Error("Failed to demote user %s: %v", targetUsername, err)
		http.Error(w, "Failed to demote user", http.StatusInternalServerError)
		return
	}

	logger.Info("User %s demoted to user by %s", targetUsername, requester.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AdminResponse{
		Success: true,
		Message: "User demoted to regular user",
	})
}

func (s *Server) handleBanUser(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Ban user request from %s", r.RemoteAddr)

	targetUsername := extractUsername(r.URL.Path)
	if targetUsername == "" {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get requester info from context
	requester, ok := r.Context().Value("user").(*models.User)
	if !ok {
		logger.Error("Requester user not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get target user
	targetUser, exists := s.store.GetUser(targetUsername)
	if !exists {
		logger.Error("Target user %s not found", targetUsername)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Permission checks
	// Cannot ban owner
	if targetUser.Role == models.RoleOwner {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Cannot ban owner",
		})
		return
	}

	// Admins cannot ban other admins (only owner can)
	if targetUser.Role == models.RoleAdmin && requester.Role != models.RoleOwner {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Only owner can ban admins",
		})
		return
	}

	// Check if already banned
	if targetUser.Banned {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "User is already banned",
		})
		return
	}

	// Ban user
	if err := s.store.BanUser(targetUsername); err != nil {
		logger.Error("Failed to ban user %s: %v", targetUsername, err)
		http.Error(w, "Failed to ban user", http.StatusInternalServerError)
		return
	}

	// Also delete from IRC database
	if err := s.ircManager.DeleteUser(targetUsername); err != nil {
		logger.Error("Failed to remove %s from IRC: %v", targetUsername, err)
		// Continue anyway - user is banned from web UI
	}

	logger.Info("User %s banned by %s", targetUsername, requester.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AdminResponse{
		Success: true,
		Message: "User banned successfully",
	})
}

func (s *Server) handleKickUser(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Kick user request from %s", r.RemoteAddr)

	targetUsername := extractUsername(r.URL.Path)
	if targetUsername == "" {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get requester info from context
	requester, ok := r.Context().Value("user").(*models.User)
	if !ok {
		logger.Error("Requester user not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get target user
	targetUser, exists := s.store.GetUser(targetUsername)
	if !exists {
		logger.Error("Target user %s not found", targetUsername)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Permission checks
	// Cannot kick owner
	if targetUser.Role == models.RoleOwner {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Cannot kick owner",
		})
		return
	}

	// Admins cannot kick other admins (only owner can)
	if targetUser.Role == models.RoleAdmin && requester.Role != models.RoleOwner {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Only owner can kick admins",
		})
		return
	}

	// Temporarily remove from IRC (they can reconnect)
	if err := s.ircManager.DeleteUser(targetUsername); err != nil {
		logger.Error("Failed to kick %s from IRC: %v", targetUsername, err)
		http.Error(w, "Failed to kick user from IRC", http.StatusInternalServerError)
		return
	}

	logger.Info("User %s kicked from IRC by %s", targetUsername, requester.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AdminResponse{
		Success: true,
		Message: "User kicked from IRC server (they can reconnect)",
	})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Delete user request from %s", r.RemoteAddr)

	targetUsername := extractUsername(r.URL.Path)
	if targetUsername == "" {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get requester info from context
	requester, ok := r.Context().Value("user").(*models.User)
	if !ok {
		logger.Error("Requester user not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get target user
	targetUser, exists := s.store.GetUser(targetUsername)
	if !exists {
		logger.Error("Target user %s not found", targetUsername)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Permission checks
	// Cannot delete owner
	if targetUser.Role == models.RoleOwner {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Cannot delete owner",
		})
		return
	}

	// Admins cannot delete other admins (only owner can)
	if targetUser.Role == models.RoleAdmin && requester.Role != models.RoleOwner {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "Only owner can delete admins",
		})
		return
	}

	// Delete from IRC database first
	if err := s.ircManager.DeleteUser(targetUsername); err != nil {
		logger.Error("Failed to delete %s from IRC: %v", targetUsername, err)
		// Continue anyway - we still want to delete the account
	}

	// Delete user account
	if err := s.store.DeleteUser(targetUsername); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		logger.Error("Failed to delete user %s: %v", targetUsername, err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	logger.Info("User %s deleted by %s", targetUsername, requester.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AdminResponse{
		Success: true,
		Message: "User account deleted successfully",
	})
}

func (s *Server) handleGetUserPermissions(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Get user permissions request from %s", r.RemoteAddr)

	targetUsername := extractUsername(r.URL.Path)
	if targetUsername == "" {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get requester info from context
	requester, ok := r.Context().Value("user").(*models.User)
	if !ok {
		logger.Error("Requester user not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get target user
	targetUser, exists := s.store.GetUser(targetUsername)
	if !exists {
		logger.Error("Target user %s not found", targetUsername)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Calculate permissions based on roles
	permissions := UserPermissionsResponse{
		TargetUser:   targetUsername,
		TargetRole:   targetUser.Role,
		CanPromote:   false,
		CanDemote:    false,
		CanBan:       false,
		CanKick:      false,
		CanDelete:    false,
	}

	// Owner can do everything to everyone except themselves
	if requester.Role == models.RoleOwner {
		if targetUser.Role == models.RoleUser {
			permissions.CanPromote = true
			permissions.CanBan = true
			permissions.CanKick = true
			permissions.CanDelete = true
		} else if targetUser.Role == models.RoleAdmin {
			permissions.CanDemote = true
			permissions.CanBan = true
			permissions.CanKick = true
			permissions.CanDelete = true
		}
		// Can't do anything to another owner or self
	} else if requester.Role == models.RoleAdmin {
		// Admins can only affect regular users
		if targetUser.Role == models.RoleUser {
			permissions.CanPromote = true
			permissions.CanBan = true
			permissions.CanKick = true
			permissions.CanDelete = true
		}
		// Can't do anything to owner or other admins
	}
	// Regular users have no permissions (all false by default)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissions)

	logger.Debug("Returned permissions for %s targeting %s", requester.Username, targetUsername)
}
