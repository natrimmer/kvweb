package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gnat/kvweb/internal/api"
	"github.com/gnat/kvweb/internal/config"
	"github.com/gnat/kvweb/internal/valkey"
	"github.com/gnat/kvweb/static"
)

// Server represents the HTTP server
type Server struct {
	cfg    *config.Config
	client *valkey.Client
	http   *http.Server
}

// New creates a new Server
func New(cfg *config.Config, client *valkey.Client) *Server {
	s := &Server{
		cfg:    cfg,
		client: client,
	}

	mux := http.NewServeMux()

	// API routes
	apiHandler := api.New(client)
	mux.Handle("/api/", apiHandler)

	// WebSocket for real-time updates
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Static files (embedded Svelte app)
	mux.Handle("/", static.Handler())

	s.http = &http.Server{
		Addr:         cfg.Addr(),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}

// handleWebSocket handles WebSocket connections for real-time updates
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement WebSocket handling for real-time key updates
	// This will be implemented with gorilla/websocket or similar
	http.Error(w, "WebSocket not yet implemented", http.StatusNotImplemented)
}
