package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/natrimmer/kvweb/internal/api"
	"github.com/natrimmer/kvweb/internal/config"
	"github.com/natrimmer/kvweb/internal/valkey"
	"github.com/natrimmer/kvweb/internal/ws"
	"github.com/natrimmer/kvweb/static"
)

// Server represents the HTTP server
type Server struct {
	cfg         *config.Config
	client      *valkey.Client
	http        *http.Server
	wsHub       *ws.Hub
	apiHandler  *api.Handler
	keyEvents   <-chan valkey.KeyEvent
	liveUpdates bool
	cancelFunc  context.CancelFunc
	ctx         context.Context
}

// New creates a new Server
func New(cfg *config.Config, client *valkey.Client) *Server {
	s := &Server{
		cfg:    cfg,
		client: client,
		wsHub:  ws.NewHub(),
	}

	mux := http.NewServeMux()

	// API routes
	s.apiHandler = api.New(cfg, client)
	s.apiHandler.SetOnNotificationsEnabled(s.enableLiveUpdates)
	s.apiHandler.SetOnNotificationsDisabled(s.disableLiveUpdates)
	mux.Handle("/api/", s.apiHandler)

	// WebSocket for real-time updates
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Static files (embedded Svelte app)
	mux.Handle("/", static.Handler())

	s.http = &http.Server{
		Addr:         cfg.Addr(),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // Disable for WebSocket
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// initNotifications checks and optionally enables keyspace notifications
func (s *Server) initNotifications(ctx context.Context) {
	// Check current setting
	current, err := s.client.GetNotifyKeyspaceEvents(ctx)
	if err != nil {
		log.Printf("Warning: Could not check keyspace notifications: %v", err)
		return
	}

	// Auto-enable if flag set and not already enabled
	if s.cfg.Notifications && current == "" {
		// K = Keyspace events, E = Keyevent events
		// A = all commands (includes HyperLogLog which has no dedicated flag)
		// g = generic (DEL, EXPIRE, RENAME), e = expired, x = evicted
		if err := s.client.SetNotifyKeyspaceEvents(ctx, "KEAgex"); err != nil {
			log.Printf("Warning: Could not enable keyspace notifications: %v", err)
			return
		}
		current = "KEAgex"
		log.Println("Enabled Valkey keyspace notifications")
	}

	// Start subscriber if notifications are enabled
	if current != "" {
		events, err := s.client.SubscribeKeyspace(ctx, s.cfg.ValkeyDB)
		if err != nil {
			log.Printf("Warning: Could not subscribe to keyspace notifications: %v", err)
			return
		}
		s.keyEvents = events
		s.liveUpdates = true
		log.Println("Subscribed to Valkey keyspace notifications")
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.ctx = ctx

	// Initialize notifications
	s.initNotifications(ctx)

	// Start WebSocket hub
	go s.wsHub.Run()

	// Start event broadcaster if live updates enabled
	if s.liveUpdates {
		go s.runEventBroadcaster(ctx)
	}

	// Start stats broadcaster
	go s.runStatsBroadcaster(ctx)

	return s.http.ListenAndServe()
}

// enableLiveUpdates starts the keyspace subscription at runtime
func (s *Server) enableLiveUpdates() {
	if s.liveUpdates {
		return // Already enabled
	}

	if s.ctx == nil {
		return // Server not started yet
	}

	events, err := s.client.SubscribeKeyspace(s.ctx, s.cfg.ValkeyDB)
	if err != nil {
		log.Printf("Warning: Could not subscribe to keyspace notifications: %v", err)
		return
	}

	s.keyEvents = events
	s.liveUpdates = true
	log.Println("Live updates enabled at runtime")

	// Start the event broadcaster
	go s.runEventBroadcaster(s.ctx)

	// Broadcast updated status to all connected clients
	s.wsHub.Broadcast(ws.Message{
		Type: "status",
		Data: ws.StatusData{Live: true},
	})
}

// disableLiveUpdates stops the keyspace subscription at runtime
func (s *Server) disableLiveUpdates() {
	if !s.liveUpdates {
		return // Already disabled
	}

	s.liveUpdates = false
	log.Println("Live updates disabled at runtime")

	// Broadcast updated status to all connected clients
	s.wsHub.Broadcast(ws.Message{
		Type: "status",
		Data: ws.StatusData{Live: false},
	})
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}

// runEventBroadcaster broadcasts keyspace events to all WebSocket clients
func (s *Server) runEventBroadcaster(ctx context.Context) {
	for {
		select {
		case event, ok := <-s.keyEvents:
			if !ok {
				return
			}
			// Filter by prefix if configured
			if s.cfg.Prefix != "" && !strings.HasPrefix(event.Key, s.cfg.Prefix) {
				continue
			}
			s.wsHub.Broadcast(ws.Message{
				Type: "key_event",
				Data: ws.KeyEventData{
					Op:  event.Operation,
					Key: event.Key,
				},
			})
		case <-ctx.Done():
			return
		}
	}
}

// runStatsBroadcaster periodically broadcasts stats to all WebSocket clients
func (s *Server) runStatsBroadcaster(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dbSize, _ := s.client.DBSize(ctx)
			memStats, _ := s.client.GetMemoryStats(ctx)

			statsData := ws.StatsData{
				DBSize:          dbSize,
				NotificationsOn: s.liveUpdates,
			}

			if memStats != nil {
				statsData.UsedMemory = memStats.UsedMemory
				statsData.UsedMemoryHuman = memStats.UsedMemoryHuman
			}

			s.wsHub.Broadcast(ws.Message{
				Type: "stats",
				Data: statsData,
			})
		case <-ctx.Done():
			return
		}
	}
}

// handleWebSocket handles WebSocket connections for real-time updates
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{}
	if s.cfg.CORSOrigin != "" {
		opts.OriginPatterns = []string{s.cfg.CORSOrigin}
	}
	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		return
	}

	client := ws.NewClient(s.wsHub, conn)
	s.wsHub.Register(client)

	// Send initial status
	status := ws.Message{
		Type: "status",
		Data: ws.StatusData{Live: s.liveUpdates},
	}
	if data, err := json.Marshal(status); err == nil {
		client.Send(data)
	}

	// Send initial stats
	dbSize, _ := s.client.DBSize(r.Context())
	memStats, _ := s.client.GetMemoryStats(r.Context())

	statsData := ws.StatsData{
		DBSize:          dbSize,
		NotificationsOn: s.liveUpdates,
	}

	if memStats != nil {
		statsData.UsedMemory = memStats.UsedMemory
		statsData.UsedMemoryHuman = memStats.UsedMemoryHuman
	}

	stats := ws.Message{
		Type: "stats",
		Data: statsData,
	}
	if data, err := json.Marshal(stats); err == nil {
		client.Send(data)
	}

	ctx := r.Context()
	go client.WritePump(ctx)
	client.ReadPump(ctx) // Blocks until disconnect
}
