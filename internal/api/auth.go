package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type SignupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Signup request received from %s", r.RemoteAddr)

	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Debug("Failed to decode signup request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	logger.Debug("Signup request for username: %s", req.Username)

	// Validate
	if req.Username == "" || req.Password == "" {
		logger.Debug("Validation failed: username or password empty")
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	// Check if user exists
	if _, exists := s.store.GetUser(req.Username); exists {
		logger.Debug("Username %s already exists", req.Username)
		http.Error(w, "Username already taken", http.StatusConflict)
		return
	}

	logger.Debug("Hashing password for user %s", req.Username)
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Failed to hash password: %v", err)
		http.Error(w, "Error creating account", http.StatusInternalServerError)
		return
	}

	// Create user
	user := &models.User{
		ID:           generateID(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Email:        req.Email,
		CreatedAt:    time.Now(),
		Active:       true, // Auto-activate for now
	}

	logger.Debug("Saving user %s to storage", req.Username)
	if err := s.store.CreateUser(user); err != nil {
		logger.Error("Failed to save user %s: %v", req.Username, err)
		http.Error(w, "Error saving user", http.StatusInternalServerError)
		return
	}

	logger.Debug("Creating IRC account for user %s", req.Username)
	// Create IRC account
	if err := s.ircManager.CreateUser(req.Username, req.Password); err != nil {
		logger.Error("Failed to create IRC account for %s: %v", req.Username, err)
		http.Error(w, "Error creating IRC account", http.StatusInternalServerError)
		return
	}

	logger.Debug("Generating JWT token for user %s", req.Username)
	// Generate JWT
	token, err := s.generateToken(user)
	if err != nil {
		logger.Error("Failed to generate token for %s: %v", req.Username, err)
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	logger.Info("User %s successfully signed up", req.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token:    token,
		Username: user.Username,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Login request received from %s", r.RemoteAddr)

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Debug("Failed to decode login request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	logger.Debug("Login attempt for username: %s", req.Username)

	// Get user
	user, exists := s.store.GetUser(req.Username)
	if !exists {
		logger.Debug("User %s not found", req.Username)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		logger.Debug("Invalid password for user %s", req.Username)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check if active
	if !user.Active {
		logger.Debug("User %s account is not active", req.Username)
		http.Error(w, "Account not active", http.StatusForbidden)
		return
	}

	logger.Debug("Generating JWT token for user %s", req.Username)
	// Generate JWT
	token, err := s.generateToken(user)
	if err != nil {
		logger.Error("Failed to generate token for %s: %v", req.Username, err)
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	logger.Info("User %s successfully logged in", req.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token:    token,
		Username: user.Username,
	})
}

func (s *Server) generateToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub": user.Username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte("your-secret-key")) // TODO: config
}

func generateID() string {
	return time.Now().Format("20060102150405")
}
