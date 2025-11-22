package api

import (
	"net/http"

	"github.com/micr0/zubr-server/internal/irc"
	"github.com/micr0/zubr-server/internal/logger"
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

	// IRC config management endpoints
	mux.HandleFunc("/api/irc/config/generate", s.handleGenerateIRCConfig)
	mux.HandleFunc("/api/irc/config", s.handleGetIRCConfig)

	// Health check
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Health check requested")
		w.Write([]byte("OK"))
	})

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
