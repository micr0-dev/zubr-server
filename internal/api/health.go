package api

import (
	"encoding/json"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/micr0/zubr-server/internal/logger"
)

type HealthStatus struct {
	Status     string                 `json:"status"`      // "healthy", "degraded", "unhealthy"
	Timestamp  string                 `json:"timestamp"`
	Components map[string]ComponentStatus `json:"components"`
}

type ComponentStatus struct {
	Status  string `json:"status"`  // "up", "down", "unknown"
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Health check requested from %s", r.RemoteAddr)

	health := HealthStatus{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Components: make(map[string]ComponentStatus),
	}

	// Check API server (always up if this responds)
	health.Components["api"] = ComponentStatus{
		Status:  "up",
		Message: "API server is running",
	}

	// Check storage
	storageStatus := s.checkStorage()
	health.Components["storage"] = storageStatus

	// Check IRC database
	ircDBStatus := s.checkIRCDatabase()
	health.Components["irc_database"] = ircDBStatus

	// Check InspIRCd process
	inspircdStatus := s.checkInspIRCd()
	health.Components["inspircd"] = inspircdStatus

	// Determine overall status
	health.Status = determineOverallStatus(health.Components)

	// Set HTTP status code based on health
	statusCode := http.StatusOK
	if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if health.Status == "degraded" {
		statusCode = http.StatusOK // Still return 200 for degraded
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(health)

	logger.Debug("Health check completed: %s", health.Status)
}

func (s *Server) checkStorage() ComponentStatus {
	start := time.Now()

	// Try to get all users (tests read access)
	_ = s.store.GetAllUsers()
	latency := time.Since(start)

	return ComponentStatus{
		Status:  "up",
		Message: "Storage operational",
		Latency: latency.String(),
	}
}

func (s *Server) checkIRCDatabase() ComponentStatus {
	start := time.Now()

	// Try to check if a test user exists (tests DB connectivity)
	_, err := s.ircManager.UserExists("__healthcheck__")
	latency := time.Since(start)

	if err != nil {
		logger.Error("IRC database health check failed: %v", err)
		return ComponentStatus{
			Status:  "down",
			Message: "IRC database error: " + err.Error(),
		}
	}

	return ComponentStatus{
		Status:  "up",
		Message: "IRC database operational",
		Latency: latency.String(),
	}
}

func (s *Server) checkInspIRCd() ComponentStatus {
	start := time.Now()

	// Check 1: Process check
	cmd := exec.Command("pgrep", "-x", "inspircd")
	if err := cmd.Run(); err != nil {
		return ComponentStatus{
			Status:  "down",
			Message: "InspIRCd process not running",
		}
	}

	// Check 2: Port connectivity check (6667)
	conn, err := net.DialTimeout("tcp", "127.0.0.1:6667", 2*time.Second)
	latency := time.Since(start)

	if err != nil {
		logger.Error("InspIRCd port check failed: %v", err)
		return ComponentStatus{
			Status:  "degraded",
			Message: "InspIRCd process running but port 6667 not responding",
			Latency: latency.String(),
		}
	}
	conn.Close()

	return ComponentStatus{
		Status:  "up",
		Message: "InspIRCd running and accepting connections",
		Latency: latency.String(),
	}
}

func determineOverallStatus(components map[string]ComponentStatus) string {
	hasDown := false
	hasDegraded := false

	for _, comp := range components {
		if comp.Status == "down" {
			hasDown = true
		} else if comp.Status == "degraded" {
			hasDegraded = true
		}
	}

	if hasDown {
		return "unhealthy"
	} else if hasDegraded {
		return "degraded"
	}
	return "healthy"
}
