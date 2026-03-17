package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/glennprays/dbeasebackup/internal/backup"
	"github.com/glennprays/log"
)

// Server provides health check HTTP endpoint
type Server struct {
	backupService *backup.Service
	server       *http.Server
	logger       *log.Logger
	ready        bool
	mu           sync.RWMutex
}

// NewServer creates a new health check server
func NewServer(backupService *backup.Service, logger *log.Logger) *Server {
	return &Server{
		backupService: backupService,
		logger:       logger,
	}
}

// Start begins serving the health check endpoint
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.HandleFunc("/livez", s.handleLivez)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler:  mux,
	}

	s.setReady(true)

	s.logger.Info("health-server-start", "Health check server starting", nil,
		log.Int("port", port),
	)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("health-server-error", "Health check server failed", nil, log.Error(err))
		}
	}()

	return nil
}

// Stop gracefully shuts down the health check server
func (s *Server) Stop(ctx context.Context) error {
	s.setReady(false)

	if s.server != nil {
		s.logger.Info("health-server-stop", "Health check server stopping", nil)
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handleHealth responds to /health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status := s.checkHealth(ctx)

	if status.Healthy {
		s.logger.Debug("health-check", "Health check passed", nil)
		writeJSON(w, http.StatusOK, status)
	} else {
		s.logger.Warn("health-check", "Health check failed", nil, log.Any("issues", status.Issues))
		writeJSON(w, http.StatusServiceUnavailable, status)
	}
}

// handleReadyz responds to /readyz endpoint (readiness probe)
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status := s.checkHealth(ctx)

	if status.Healthy && s.isReady() {
		s.logger.Debug("readiness-check", "Readiness check passed", nil)
		writeJSON(w, http.StatusOK, map[string]any{"ready": true})
	} else {
		s.logger.Warn("readiness-check", "Readiness check failed", nil, log.Any("ready", s.isReady()))
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ready": false})
	}
}

// handleLivez responds to /livez endpoint (liveness probe)
func (s *Server) handleLivez(w http.ResponseWriter, r *http.Request) {
	// For liveness, we just return 200 if the server is running
	// This doesn't check dependencies - just that the process is alive
	s.logger.Debug("liveness-check", "Liveness check passed", nil)
	writeJSON(w, http.StatusOK, map[string]any{"alive": true})
}

// checkHealth performs health checks on dependencies
func (s *Server) checkHealth(ctx context.Context) HealthStatus {
	var issues []string

	// Check database health
	if err := s.backupService.Health(ctx); err != nil {
		issues = append(issues, fmt.Sprintf("database: %v", err))
	}

	return HealthStatus{
		Healthy: len(issues) == 0,
		Issues:  issues,
	}
}

// setReady sets the ready flag
func (s *Server) setReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = ready
}

// isReady returns the ready flag
func (s *Server) isReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// HealthStatus represents the result of a health check
type HealthStatus struct {
	Healthy bool     `json:"healthy"`
	Issues  []string `json:"issues,omitempty"`
}

// writeJSON writes a JSON response with status code
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
