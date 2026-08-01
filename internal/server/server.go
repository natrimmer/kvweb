package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/natrimmer/kvweb/internal/api"
	"github.com/natrimmer/kvweb/internal/config"
	"github.com/natrimmer/kvweb/internal/valkey"
	"github.com/natrimmer/kvweb/internal/ws"
	"github.com/natrimmer/kvweb/static"
)

type Server struct {
	cfg         *config.Config
	client      *valkey.Client
	http        *http.Server
	wsHub       *ws.Hub
	apiHandler  *api.Handler
	keyEvents   <-chan valkey.KeyEvent
	liveUpdates atomic.Bool
	cancelFunc  context.CancelFunc
	ctx         context.Context
}

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

	// Static files (embedded Svelte app) — skip in dev mode
	if cfg.Dev {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "kvweb backend (dev mode)\n\nFrontend is at http://localhost:5173\nThis port only serves /api and /ws")
		})
	} else {
		mux.Handle("/", static.Handler())
	}

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
	current, err := s.client.GetNotifyKeyspaceEvents(ctx)
	if err != nil {
		slog.Warn("Could not check keyspace notifications", "error", err)
		return
	}

	// Auto-enable if flag set and not already enabled
	if s.cfg.Notifications && current == "" {
		// K = Keyspace events, E = Keyevent events
		// A = all commands (includes HyperLogLog which has no dedicated flag)
		// g = generic (DEL, EXPIRE, RENAME), e = expired, x = evicted
		if err := s.client.SetNotifyKeyspaceEvents(ctx, "KEAgex"); err != nil {
			slog.Warn("Could not enable keyspace notifications", "error", err)
			return
		}
		current = "KEAgex"
		slog.Info("Enabled Valkey keyspace notifications")
	}

	// Start subscriber if notifications are enabled
	if current != "" {
		events, err := s.client.SubscribeKeyspace(ctx, s.cfg.ValkeyDB)
		if err != nil {
			slog.Warn("Could not subscribe to keyspace notifications", "error", err)
			return
		}
		s.keyEvents = events
		s.liveUpdates.Store(true)
		slog.Info("Subscribed to Valkey keyspace notifications")
	}
}

func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.ctx = ctx

	s.initNotifications(ctx)

	go s.wsHub.Run()

	// Start event broadcaster if live updates enabled
	if s.liveUpdates.Load() {
		go s.runEventBroadcaster(ctx)
	}

	// Start stats broadcaster
	go s.runStatsBroadcaster(ctx)

	return s.http.ListenAndServe()
}

// enableLiveUpdates starts the keyspace subscription at runtime
func (s *Server) enableLiveUpdates() {
	if s.liveUpdates.Load() {
		return // Already enabled
	}

	if s.ctx == nil {
		return // Server not started yet
	}

	events, err := s.client.SubscribeKeyspace(s.ctx, s.cfg.ValkeyDB)
	if err != nil {
		slog.Warn("Could not subscribe to keyspace notifications", "error", err)
		return
	}

	s.keyEvents = events
	s.liveUpdates.Store(true)
	slog.Info("Live updates enabled at runtime")

	go s.runEventBroadcaster(s.ctx)

	// Broadcast updated status to all connected clients
	s.wsHub.Broadcast(ws.Message{
		Type: "status",
		Data: ws.StatusData{Live: true},
	})
}

// disableLiveUpdates stops the keyspace subscription at runtime
func (s *Server) disableLiveUpdates() {
	if !s.liveUpdates.Load() {
		return // Already disabled
	}

	s.liveUpdates.Store(false)
	slog.Info("Live updates disabled at runtime")

	// Broadcast updated status to all connected clients
	s.wsHub.Broadcast(ws.Message{
		Type: "status",
		Data: ws.StatusData{Live: false},
	})
}

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
			dbSize, err := s.client.DBSize(ctx)
			if err != nil {
				slog.Error("Stats broadcast failed", "op", "DBSize", "error", err)
			}
			memStats, err := s.client.GetMemoryStats(ctx)
			if err != nil {
				slog.Error("Stats broadcast failed", "op", "GetMemoryStats", "error", err)
			}

			statsData := ws.StatsData{
				DBSize:          dbSize,
				NotificationsOn: s.liveUpdates.Load(),
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

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{}
	if s.cfg.CORSOrigin != "" {
		opts.OriginPatterns = []string{s.cfg.CORSOrigin}
	}
	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		slog.Error("WebSocket accept failed", "error", err)
		return
	}

	client := ws.NewClient(s.wsHub, conn)
	s.wsHub.Register(client)

	// Send initial status
	status := ws.Message{
		Type: "status",
		Data: ws.StatusData{Live: s.liveUpdates.Load()},
	}
	if data, err := json.Marshal(status); err == nil {
		client.Send(data)
	}

	// Send initial stats
	dbSize, _ := s.client.DBSize(r.Context())
	memStats, _ := s.client.GetMemoryStats(r.Context())

	statsData := ws.StatsData{
		DBSize:          dbSize,
		NotificationsOn: s.liveUpdates.Load(),
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
