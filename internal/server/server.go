package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/gateway"
	"github.com/ye-dev/multigravity-cli/internal/profile"
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
	broker     *Broker
	gateway    *gateway.Gateway
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

	gw := gateway.NewGateway()
	if profs, err := profile.ListProfiles(); err == nil && len(profs) > 0 {
		gw.Router().SyncProfiles(profs)
	}

	s := &Server{
		cfg:     cfg,
		mux:     http.NewServeMux(),
		broker:  NewBroker(),
		gateway: gw,
	}

	s.broker.Start(2 * time.Second)
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

// Broker returns the active SSE event broker
func (s *Server) Broker() *Broker {
	return s.broker
}

// Gateway returns the OpenAI-compatible completions gateway
func (s *Server) Gateway() *gateway.Gateway {
	return s.gateway
}

// SetGateway overrides the gateway instance (primarily for testing)
func (s *Server) SetGateway(gw *gateway.Gateway) {
	s.gateway = gw
}

// Shutdown initiates graceful termination of the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.broker != nil {
		s.broker.Stop()
	}
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
