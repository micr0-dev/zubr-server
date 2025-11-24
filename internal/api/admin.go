package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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

	// Kick user from IRC server (temporary - they can reconnect)
	kickReason := fmt.Sprintf("Kicked by %s", requester.Username)
	if err := s.ircManager.KickUser(targetUsername, kickReason); err != nil {
		logger.Error("Failed to kick %s from IRC: %v", targetUsername, err)
		// Continue anyway - the IRC kick is best effort
		logger.Debug("Continuing despite IRC kick failure")
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

func (s *Server) handleApproveUser(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Approve user request from %s", r.RemoteAddr)

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

	// Check if user is pending
	if !targetUser.Pending {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AdminResponse{
			Success: false,
			Message: "User is not pending approval",
		})
		return
	}

	// Approve user
	if err := s.store.ApproveUser(targetUsername); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		logger.Error("Failed to approve user %s: %v", targetUsername, err)
		http.Error(w, "Failed to approve user", http.StatusInternalServerError)
		return
	}

	// Create IRC account for newly approved user
	// Note: We need to get the password, but we can't since it's hashed.
	// The user will need to reset their password or we store it temporarily.
	// For now, we'll create the IRC account without a password and log a warning.
	logger.Debug("User %s approved, creating IRC account", targetUsername)
	// TODO: Handle IRC account creation for approved users
	// This might require storing the password temporarily or having users reset it

	logger.Info("User %s approved by %s", targetUsername, requester.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AdminResponse{
		Success: true,
		Message: "User approved successfully",
	})
}

func (s *Server) handleGenerateInvite(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Generate invite token request from %s", r.RemoteAddr)

	// Get requester info from context
	requester, ok := r.Context().Value("user").(*models.User)
	if !ok {
		logger.Error("Requester user not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate random invite token
	token := generateInviteToken()

	invite := &models.InviteToken{
		Token:     token,
		CreatedBy: requester.Username,
		CreatedAt: time.Now(),
		Used:      false,
	}

	if err := s.store.CreateInviteToken(invite); err != nil {
		logger.Error("Failed to create invite token: %v", err)
		http.Error(w, "Failed to generate invite token", http.StatusInternalServerError)
		return
	}

	logger.Info("Invite token generated by %s: %s", requester.Username, token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
	})
}

func generateInviteToken() string {
	// Generate a random token (8 characters)
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
