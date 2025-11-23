package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type SignupRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email,omitempty"`
	InviteToken string `json:"invite_token,omitempty"`
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

	// Get instance settings to check signup mode
	settings := s.store.GetSettings()
	logger.Debug("Signup mode: %s", settings.SignupMode)

	// Validate signup mode requirements
	if settings.SignupMode == models.SignupModeInvite {
		// Invite mode: require invite token
		if req.InviteToken == "" {
			logger.Debug("Invite token required but not provided")
			http.Error(w, "Invite token required", http.StatusForbidden)
			return
		}

		// Validate invite token
		invite, exists := s.store.GetInviteToken(req.InviteToken)
		if !exists {
			logger.Debug("Invalid invite token: %s", req.InviteToken)
			http.Error(w, "Invalid invite token", http.StatusForbidden)
			return
		}

		if invite.Used {
			logger.Debug("Invite token already used: %s", req.InviteToken)
			http.Error(w, "Invite token already used", http.StatusForbidden)
			return
		}

		logger.Debug("Valid invite token provided: %s", req.InviteToken)
	}

	// Determine role - first user is owner, rest are users
	allUsers := s.store.GetAllUsers()
	role := models.RoleUser
	if len(allUsers) == 0 {
		role = models.RoleOwner
		logger.Info("First user signing up - assigning Owner role to %s", req.Username)
	}

	logger.Debug("Hashing password for user %s", req.Username)
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Failed to hash password: %v", err)
		http.Error(w, "Error creating account", http.StatusInternalServerError)
		return
	}

	// Determine user status based on signup mode
	var active, pending bool
	switch settings.SignupMode {
	case models.SignupModePublic:
		active = true
		pending = false
	case models.SignupModeApproval:
		active = false
		pending = true
		logger.Debug("Approval mode: user %s will be created as pending", req.Username)
	case models.SignupModeInvite:
		active = true
		pending = false
	}

	// Create user
	user := &models.User{
		ID:           generateID(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Email:        req.Email,
		CreatedAt:    time.Now(),
		Active:       active,
		Pending:      pending,
		Role:         role,
		Config:       models.DefaultUserConfig(),
	}

	logger.Debug("Saving user %s to storage", req.Username)
	if err := s.store.CreateUser(user); err != nil {
		logger.Error("Failed to save user %s: %v", req.Username, err)
		http.Error(w, "Error saving user", http.StatusInternalServerError)
		return
	}

	// Mark invite token as used if in invite mode
	if settings.SignupMode == models.SignupModeInvite && req.InviteToken != "" {
		if err := s.store.UseInviteToken(req.InviteToken, req.Username); err != nil {
			logger.Error("Failed to mark invite token as used: %v", err)
			// Don't fail signup, just log the error
		}
	}

	// Only create IRC account if user is active
	if active {
		logger.Debug("Creating IRC account for user %s", req.Username)
		if err := s.ircManager.CreateUser(req.Username, req.Password); err != nil {
			logger.Error("Failed to create IRC account for %s: %v", req.Username, err)
			http.Error(w, "Error creating IRC account", http.StatusInternalServerError)
			return
		}
	} else {
		logger.Debug("User %s is pending approval, skipping IRC account creation", req.Username)
	}

	logger.Debug("Generating JWT token for user %s", req.Username)
	// Generate JWT
	token, err := s.generateToken(user)
	if err != nil {
		logger.Error("Failed to generate token for %s: %v", req.Username, err)
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	logger.Info("User %s successfully signed up (active: %v, pending: %v)", req.Username, active, pending)
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

// authMiddleware validates JWT token and adds username to context
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Auth middleware checking token for %s", r.URL.Path)

		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger.Debug("No Authorization header provided")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Extract token (format: "Bearer <token>")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			logger.Debug("Invalid Authorization header format")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte("your-secret-key"), nil // TODO: config
		})

		if err != nil || !token.Valid {
			logger.Debug("Invalid token: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Extract username from claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			logger.Debug("Invalid token claims")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		username, ok := claims["sub"].(string)
		if !ok {
			logger.Debug("No username in token claims")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		logger.Debug("Authenticated user: %s", username)

		// Add username to context
		ctx := context.WithValue(r.Context(), "username", username)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// optionalAuthMiddleware tries to authenticate but doesn't fail if no token is provided
func (s *Server) optionalAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No auth provided, continue without setting username in context
			next.ServeHTTP(w, r)
			return
		}

		// Extract token (format: "Bearer <token>")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Invalid format, continue without auth
			next.ServeHTTP(w, r)
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte("your-secret-key"), nil // TODO: config
		})

		if err != nil || !token.Valid {
			// Invalid token, continue without auth
			next.ServeHTTP(w, r)
			return
		}

		// Extract username from claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			// Invalid claims, continue without auth
			next.ServeHTTP(w, r)
			return
		}

		username, ok := claims["sub"].(string)
		if !ok {
			// No username in claims, continue without auth
			next.ServeHTTP(w, r)
			return
		}

		logger.Debug("Optional auth: authenticated user: %s", username)

		// Add username to context
		ctx := context.WithValue(r.Context(), "username", username)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// requireRole checks if the authenticated user has one of the required roles
func (s *Server) requireRole(allowedRoles []models.Role) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
			username, ok := r.Context().Value("username").(string)
			if !ok {
				logger.Error("Username not found in context")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Get user to check role
			user, exists := s.store.GetUser(username)
			if !exists {
				logger.Error("User %s not found", username)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if user has one of the allowed roles
			hasRole := false
			for _, role := range allowedRoles {
				if user.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				logger.Debug("User %s does not have required role. Has: %s, Required: %v", username, user.Role, allowedRoles)
				http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
				return
			}

			logger.Debug("User %s authorized with role %s", username, user.Role)

			// Add user and role to context for later use
			ctx := context.WithValue(r.Context(), "user", user)
			ctx = context.WithValue(ctx, "role", user.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
