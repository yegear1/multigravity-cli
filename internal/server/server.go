package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Config defines the configuration parameters for the HTTP server
type Config struct {
	Host      string
	Port      int
	Version   string
	StartTime time.Time
}

// Server encapsulates the HTTP server instance and router
type Server struct {
	cfg        Config
	httpServer *http.Server
	mux        *http.ServeMux
	listener   net.Listener
}

// NewServer creates a new HTTP server instance with configured routes
func NewServer(cfg Config) *Server {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port <= 0 {
		cfg.Port = 8989
	}
	if cfg.StartTime.IsZero() {
		cfg.StartTime = time.Now()
	}

	s := &Server{
		cfg: cfg,
		mux: http.NewServeMux(),
	}

	s.setupRoutes()
	return s
}

// Handler returns the HTTP handler with middleware
func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.mux)
}

// Addr returns the configured or active listener address
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
}

// Start binds the socket and starts serving requests (blocking)
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", addr, err)
	}
	s.listener = ln

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown initiates graceful termination of the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
