package api

import (
	"net/http"
	"strings"

	"github.com/micr0/zubr-server/internal/irc"
	"github.com/micr0/zubr-server/internal/logger"
	"github.com/micr0/zubr-server/internal/models"
	"github.com/micr0/zubr-server/internal/storage"
)

type Server struct {
	store      *storage.Store
	ircManager *irc.Manager
	addr       string
}

func NewServer(store *storage.Store, ircManager *irc.Manager, addr string) *Server {
	return &Server{
		store:      store,
		ircManager: ircManager,
		addr:       addr,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Auth endpoints
	mux.HandleFunc("/api/signup", s.handleSignup)
	mux.HandleFunc("/api/login", s.handleLogin)

	// Current user endpoint (authenticated)
	mux.HandleFunc("/api/user/me", s.authMiddleware(s.handleGetMe))

	// User config endpoints (authenticated)
	mux.HandleFunc("/api/user/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.authMiddleware(s.handleGetUserConfig)(w, r)
		} else if r.Method == "PUT" {
			s.authMiddleware(s.handleUpdateUserConfig)(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// User list endpoint (conditionally authenticated based on signup mode)
	mux.HandleFunc("/api/users", s.optionalAuthMiddleware(s.handleGetUsers))

	// Voice endpoint not needed - using InspIRCd's +M mode instead
	// (Only authenticated users can speak in +M channels)

	// Admin endpoints (require owner or admin role)
	adminRole := []models.Role{models.RoleOwner, models.RoleAdmin}
	mux.HandleFunc("/api/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		// Route based on action suffix
		if strings.HasSuffix(r.URL.Path, "/promote") && r.Method == "POST" {
			s.requireRole(adminRole)(s.handlePromoteUser)(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/demote") && r.Method == "POST" {
			s.requireRole(adminRole)(s.handleDemoteUser)(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/ban") && r.Method == "POST" {
			s.requireRole(adminRole)(s.handleBanUser)(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/kick") && r.Method == "POST" {
			s.requireRole(adminRole)(s.handleKickUser)(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/approve") && r.Method == "POST" {
			s.requireRole(adminRole)(s.handleApproveUser)(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/permissions") && r.Method == "GET" {
			s.requireRole(adminRole)(s.handleGetUserPermissions)(w, r)
		} else if r.Method == "DELETE" && !strings.HasSuffix(r.URL.Path, "/") {
			s.requireRole(adminRole)(s.handleDeleteUser)(w, r)
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})

	// Admin invite generation endpoint
	mux.HandleFunc("/api/admin/invites/generate", s.requireRole(adminRole)(s.handleGenerateInvite))

	// Instance settings endpoints (require owner or admin role)
	mux.HandleFunc("/api/instance/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.requireRole(adminRole)(s.handleGetInstanceSettings)(w, r)
		} else if r.Method == "PATCH" {
			s.requireRole(adminRole)(s.handleUpdateInstanceSettings)(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Health check
	mux.HandleFunc("/api/health", s.handleHealth)

	// Server info
	mux.HandleFunc("/api/info", s.handleInfo)

	logger.Info("API server listening on %s", s.addr)
	return http.ListenAndServe(s.addr, s.loggingMiddleware(s.corsMiddleware(mux)))
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Incoming request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
