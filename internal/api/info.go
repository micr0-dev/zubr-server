package api

import (
	"encoding/json"
	"net/http"

	"github.com/micr0/zubr-server/internal/logger"
)

const (
	ServerName    = "Zubr Server"
	ServerVersion = "0.2.0"
)

type ServerInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	API        string `json:"api"`
	SignupMode string `json:"signup_mode"`
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Server info request from %s", r.RemoteAddr)

	// Get instance settings for signup mode
	settings := s.store.GetSettings()

	info := ServerInfo{
		Name:       ServerName,
		Version:    ServerVersion,
		API:        "v1",
		SignupMode: string(settings.SignupMode),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
