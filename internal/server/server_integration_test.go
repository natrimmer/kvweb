package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/natrimmer/kvweb/internal/config"
	"github.com/natrimmer/kvweb/internal/server"
	"github.com/natrimmer/kvweb/internal/testenv"
	"github.com/natrimmer/kvweb/internal/valkey"
)

func TestMain(m *testing.M) { testenv.Main(m) }

// message is a decoded frame from the /ws endpoint.
type message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type statusData struct {
	Live bool   `json:"live"`
	Msg  string `json:"msg"`
}

type statsData struct {
	DBSize          int64  `json:"dbSize"`
	UsedMemory      int64  `json:"usedMemory"`
	UsedMemoryHuman string `json:"usedMemoryHuman"`
	NotificationsOn bool   `json:"notificationsOn"`
}

type keyEventData struct {
	Op  string `json:"op"`
	Key string `json:"key"`
}

// liveServer is a real kvweb server listening on a loopback port, with a client
// connected to the same database.
type liveServer struct {
	BaseURL string
	Client  *valkey.Client
	Cfg     *config.Config
}

// startServer boots the full server the way main.go does, on an ephemeral port.
// Notifications are server-global, so every case gets its own backing server.
func startServer(t *testing.T, e *testenv.Engine, configure ...func(*config.Config)) *liveServer {
	t.Helper()

	addr := e.Exclusive(t)

	cfg := config.New()
	cfg.ValkeyURL = addr
	cfg.Host = "127.0.0.1"
	cfg.Port = freePort(t)
	cfg.Dev = true // skip the embedded frontend; these tests are about /api and /ws
	for _, fn := range configure {
		fn(cfg)
	}

	client, err := valkey.New(cfg)
	if err != nil {
		t.Fatalf("could not connect to %s: %v", e.Name, err)
	}
	t.Cleanup(client.Close)

	srv := server.New(cfg, client)
	errs := make(chan error, 1)
	go func() { errs <- srv.Start() }()
	t.Cleanup(func() {
		if err := srv.Shutdown(); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
		if err := <-errs; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Start returned %v", err)
		}
	})

	base := fmt.Sprintf("http://%s", cfg.Addr())
	waitForHealth(t, base)

	return &liveServer{BaseURL: base, Client: client, Cfg: cfg}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not reserve a port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("could not release the reserved port: %v", err)
	}
	return port
}

func waitForHealth(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/api/health") //nolint:noctx // short-lived readiness poll
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("server at %s never became healthy", base)
}

// wsClient reads frames from /ws in the background so tests can wait on them.
type wsClient struct {
	t      *testing.T
	conn   *websocket.Conn
	frames chan message
}

func (s *liveServer) connectWS(t *testing.T) *wsClient {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	url := "ws://" + strings.TrimPrefix(s.BaseURL, "http://") + "/ws"
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("could not open a WebSocket to %s: %v", url, err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	c := &wsClient{t: t, conn: conn, frames: make(chan message, 256)}
	go func() {
		defer close(c.frames)
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var msg message
			if err := json.Unmarshal(data, &msg); err != nil {
				return
			}
			select {
			case c.frames <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()
	return c
}

// await returns the first frame satisfying match, or fails after timeout.
func (c *wsClient) await(t *testing.T, timeout time.Duration, what string, match func(message) bool) message {
	t.Helper()
	deadline := time.After(timeout)
	var seen []string
	for {
		select {
		case msg, ok := <-c.frames:
			if !ok {
				t.Fatalf("the WebSocket closed while waiting for %s; saw %v", what, seen)
			}
			seen = append(seen, msg.Type)
			if match(msg) {
				return msg
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s; saw frames %v", what, seen)
		}
	}
}

func (c *wsClient) awaitType(t *testing.T, timeout time.Duration, frameType string) message {
	t.Helper()
	return c.await(t, timeout, frameType+" frame", func(m message) bool { return m.Type == frameType })
}

func decode[T any](t *testing.T, msg message) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(msg.Data, &v); err != nil {
		t.Fatalf("could not decode a %s frame: %v\nraw: %s", msg.Type, err, msg.Data)
	}
	return v
}

// post sends a JSON request to the live server's API.
func (s *liveServer) post(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("could not encode the request body: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.BaseURL+path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("could not build the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestServerServesAPIAndDevRoot(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		s := startServer(t, e)

		t.Run("HealthEndpoint", func(t *testing.T) {
			resp, err := http.Get(s.BaseURL + "/api/health") //nolint:noctx // simple smoke request
			if err != nil {
				t.Fatalf("GET /api/health: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want 200", resp.StatusCode)
			}
		})

		t.Run("DevRootExplainsItself", func(t *testing.T) {
			// In dev mode the root path is a note pointing at the Vite server
			// rather than the embedded frontend.
			resp, err := http.Get(s.BaseURL + "/") //nolint:noctx // simple smoke request
			if err != nil {
				t.Fatalf("GET /: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			body := make([]byte, 256)
			n, _ := resp.Body.Read(body)
			if !strings.Contains(string(body[:n]), "dev mode") {
				t.Errorf("body = %q, want the dev-mode notice", body[:n])
			}
		})
	})
}

func TestWebSocketHandshake(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		s := startServer(t, e)
		if err := s.Client.Set(context.Background(), "ws:seed", "v", 0); err != nil {
			t.Fatalf("seed: %v", err)
		}

		ws := s.connectWS(t)

		t.Run("SendsInitialStatus", func(t *testing.T) {
			msg := ws.awaitType(t, 10*time.Second, "status")
			status := decode[statusData](t, msg)
			// Notifications were not enabled at startup for this server.
			if status.Live {
				t.Error("live = true, but keyspace notifications are off")
			}
		})

		t.Run("SendsInitialStats", func(t *testing.T) {
			msg := ws.awaitType(t, 10*time.Second, "stats")
			stats := decode[statsData](t, msg)
			if stats.DBSize != 1 {
				t.Errorf("dbSize = %d, want 1", stats.DBSize)
			}
			if stats.UsedMemory <= 0 {
				t.Errorf("usedMemory = %d, want a positive byte count", stats.UsedMemory)
			}
			if stats.UsedMemoryHuman == "" {
				t.Error("usedMemoryHuman is empty")
			}
			if stats.NotificationsOn {
				t.Error("notificationsOn = true, but they are off")
			}
		})

		t.Run("SecondClientAlsoGetsAnInitialSnapshot", func(t *testing.T) {
			other := s.connectWS(t)
			other.awaitType(t, 10*time.Second, "status")
			other.awaitType(t, 10*time.Second, "stats")
		})
	})
}

func TestWebSocketLiveUpdatesFromStartup(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		// --notifications makes the server enable keyspace events and subscribe
		// before it starts listening.
		s := startServer(t, e, func(c *config.Config) { c.Notifications = true })
		ws := s.connectWS(t)

		t.Run("StatusReportsLive", func(t *testing.T) {
			msg := ws.awaitType(t, 10*time.Second, "status")
			if status := decode[statusData](t, msg); !status.Live {
				t.Error("live = false, but --notifications was set")
			}
		})

		t.Run("KeyWritesAreBroadcast", func(t *testing.T) {
			if err := s.Client.Set(context.Background(), "live:key", "v", 0); err != nil {
				t.Fatalf("Set: %v", err)
			}
			msg := ws.await(t, 10*time.Second, "a key_event for live:key", func(m message) bool {
				if m.Type != "key_event" {
					return false
				}
				var ev keyEventData
				return json.Unmarshal(m.Data, &ev) == nil && ev.Key == "live:key"
			})
			if ev := decode[keyEventData](t, msg); ev.Op != "set" {
				t.Errorf("op = %q, want set", ev.Op)
			}
		})

		t.Run("DeletesAreBroadcast", func(t *testing.T) {
			if _, err := s.Client.Del(context.Background(), "live:key"); err != nil {
				t.Fatalf("Del: %v", err)
			}
			msg := ws.await(t, 10*time.Second, "a del key_event", func(m message) bool {
				if m.Type != "key_event" {
					return false
				}
				var ev keyEventData
				return json.Unmarshal(m.Data, &ev) == nil && ev.Op == "del"
			})
			if ev := decode[keyEventData](t, msg); ev.Key != "live:key" {
				t.Errorf("key = %q, want live:key", ev.Key)
			}
		})

		t.Run("EveryConnectedClientSeesTheEvent", func(t *testing.T) {
			second := s.connectWS(t)
			second.awaitType(t, 10*time.Second, "status")

			if err := s.Client.Set(context.Background(), "live:broadcast", "v", 0); err != nil {
				t.Fatalf("Set: %v", err)
			}
			for name, client := range map[string]*wsClient{"first": ws, "second": second} {
				client.await(t, 10*time.Second, name+" seeing live:broadcast", func(m message) bool {
					if m.Type != "key_event" {
						return false
					}
					var ev keyEventData
					return json.Unmarshal(m.Data, &ev) == nil && ev.Key == "live:broadcast"
				})
			}
		})
	})
}

func TestWebSocketNotificationsToggle(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		// Notifications start off; turning them on through the API has to
		// subscribe, start the broadcaster and tell connected clients.
		s := startServer(t, e)
		ws := s.connectWS(t)

		if status := decode[statusData](t, ws.awaitType(t, 10*time.Second, "status")); status.Live {
			t.Fatal("live = true before notifications were enabled")
		}

		t.Run("EnablingBroadcastsStatus", func(t *testing.T) {
			resp := s.post(t, "/api/notifications", map[string]any{"enabled": true})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("POST /api/notifications: status %d", resp.StatusCode)
			}
			msg := ws.await(t, 10*time.Second, "a live status frame", func(m message) bool {
				if m.Type != "status" {
					return false
				}
				var status statusData
				return json.Unmarshal(m.Data, &status) == nil && status.Live
			})
			if status := decode[statusData](t, msg); !status.Live {
				t.Error("live = false after enabling")
			}
		})

		t.Run("EventsFlowAfterEnabling", func(t *testing.T) {
			// The subscription is established asynchronously.
			time.Sleep(250 * time.Millisecond)
			if err := s.Client.Set(context.Background(), "toggle:key", "v", 0); err != nil {
				t.Fatalf("Set: %v", err)
			}
			ws.await(t, 10*time.Second, "a key_event for toggle:key", func(m message) bool {
				if m.Type != "key_event" {
					return false
				}
				var ev keyEventData
				return json.Unmarshal(m.Data, &ev) == nil && ev.Key == "toggle:key"
			})
		})

		t.Run("DisablingBroadcastsStatus", func(t *testing.T) {
			resp := s.post(t, "/api/notifications", map[string]any{"enabled": false})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("POST /api/notifications: status %d", resp.StatusCode)
			}
			ws.await(t, 10*time.Second, "a not-live status frame", func(m message) bool {
				if m.Type != "status" {
					return false
				}
				var status statusData
				return json.Unmarshal(m.Data, &status) == nil && !status.Live
			})

			value, err := s.Client.GetNotifyKeyspaceEvents(context.Background())
			if err != nil {
				t.Fatalf("GetNotifyKeyspaceEvents: %v", err)
			}
			if value != "" {
				t.Errorf("notify-keyspace-events = %q, want it cleared", value)
			}
		})
	})
}

func TestWebSocketPrefixFiltersEvents(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		s := startServer(t, e, func(c *config.Config) {
			c.Notifications = true
			c.Prefix = "app:"
		})
		ws := s.connectWS(t)
		ws.awaitType(t, 10*time.Second, "status")

		ctx := context.Background()
		if err := s.Client.Set(ctx, "other:hidden", "v", 0); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := s.Client.Set(ctx, "app:visible", "v", 0); err != nil {
			t.Fatalf("Set: %v", err)
		}

		// Waiting for the in-prefix event proves the out-of-prefix one was
		// filtered: it was written first, so it would have arrived first.
		msg := ws.await(t, 10*time.Second, "a key_event for app:visible", func(m message) bool {
			if m.Type != "key_event" {
				return false
			}
			var ev keyEventData
			return json.Unmarshal(m.Data, &ev) == nil && ev.Key == "app:visible"
		})
		if ev := decode[keyEventData](t, msg); ev.Key != "app:visible" {
			t.Errorf("key = %q, want app:visible", ev.Key)
		}

		// Nothing for the hidden key should be queued behind it either.
		select {
		case extra, ok := <-ws.frames:
			if ok && extra.Type == "key_event" {
				var ev keyEventData
				if json.Unmarshal(extra.Data, &ev) == nil && strings.HasPrefix(ev.Key, "other:") {
					t.Errorf("an out-of-prefix key leaked over the WebSocket: %+v", ev)
				}
			}
		case <-time.After(500 * time.Millisecond):
		}
	})
}

func TestWebSocketPeriodicStats(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		s := startServer(t, e)
		ws := s.connectWS(t)

		// The initial stats frame arrives on connect; the broadcaster then
		// pushes another every five seconds.
		ws.awaitType(t, 10*time.Second, "stats")

		if err := s.Client.Set(context.Background(), "stats:key", "v", 0); err != nil {
			t.Fatalf("Set: %v", err)
		}

		msg := ws.await(t, 15*time.Second, "a periodic stats frame reflecting the new key", func(m message) bool {
			if m.Type != "stats" {
				return false
			}
			var stats statsData
			return json.Unmarshal(m.Data, &stats) == nil && stats.DBSize >= 1
		})
		if stats := decode[statsData](t, msg); stats.UsedMemory <= 0 {
			t.Errorf("usedMemory = %d, want a positive byte count", stats.UsedMemory)
		}
	})
}

func TestServerShutdownIsClean(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		addr := e.Exclusive(t)

		cfg := config.New()
		cfg.ValkeyURL = addr
		cfg.Host = "127.0.0.1"
		cfg.Port = freePort(t)
		cfg.Dev = true

		client, err := valkey.New(cfg)
		if err != nil {
			t.Fatalf("could not connect: %v", err)
		}
		defer client.Close()

		srv := server.New(cfg, client)
		errs := make(chan error, 1)
		go func() { errs <- srv.Start() }()
		waitForHealth(t, "http://"+cfg.Addr())

		if err := srv.Shutdown(); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
		if err := <-errs; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Start returned %v, want ErrServerClosed", err)
		}

		// The port must actually be released, or a restart would fail.
		if _, err := http.Get("http://" + cfg.Addr() + "/api/health"); err == nil { //nolint:noctx // post-shutdown probe
			t.Error("the server still answers after Shutdown")
		}
	})
}
