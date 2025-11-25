package api

import (
	"net/http"

	"github.com/micr0/zubr-server/internal/logger"
)

func (s *Server) handleMOTD(w http.ResponseWriter, r *http.Request) {
	logger.Debug("MOTD request from %s", r.RemoteAddr)

	settings := s.store.GetSettings()

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(settings.MOTD))
}
