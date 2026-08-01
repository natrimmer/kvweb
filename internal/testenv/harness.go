package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/natrimmer/kvweb/internal/api"
	"github.com/natrimmer/kvweb/internal/config"
	"github.com/natrimmer/kvweb/internal/valkey"
)

// Harness is one test's slice of the world: a config, a live client against a
// real server, and the API handler mounted on an httptest server.
//
// The database is flushed on setup and teardown, so tests start clean and leave
// nothing behind for whoever checks out this logical database next.
type Harness struct {
	T       *testing.T
	Engine  *Engine
	Addr    string
	DB      int
	Cfg     *config.Config
	Client  *valkey.Client
	API     *api.Handler
	Server  *httptest.Server
	BaseURL string

	http *http.Client
	ctx  context.Context
}

type options struct {
	exclusive  bool
	serverArgs []string
	cfgFns     []func(*config.Config)
}

// Option customises a Harness.
type Option func(*options)

// Exclusive gives the harness its own server process instead of the shared one.
// Required for anything that changes server-global state. extraArgs are passed
// to the server command line.
func Exclusive(extraArgs ...string) Option {
	return func(o *options) {
		o.exclusive = true
		o.serverArgs = append(o.serverArgs, extraArgs...)
	}
}

// WithConfig mutates the config before the client and handler are built.
func WithConfig(fn func(*config.Config)) Option {
	return func(o *options) { o.cfgFns = append(o.cfgFns, fn) }
}

// ReadOnly puts the API handler in read-only mode.
func ReadOnly() Option {
	return WithConfig(func(c *config.Config) { c.ReadOnly = true })
}

// Prefix restricts the API handler to keys under prefix.
func Prefix(prefix string) Option {
	return WithConfig(func(c *config.Config) { c.Prefix = prefix })
}

// DisableFlush blocks FLUSHDB through the API even in write mode.
func DisableFlush() Option {
	return WithConfig(func(c *config.Config) { c.DisableFlush = true })
}

// MaxKeys caps the SCAN count the API will accept.
func MaxKeys(n int64) Option {
	return WithConfig(func(c *config.Config) { c.MaxKeys = n })
}

// CORSOrigin sets the allowed CORS origin.
func CORSOrigin(origin string) Option {
	return WithConfig(func(c *config.Config) { c.CORSOrigin = origin })
}

// New builds a Harness against the given engine.
func New(t *testing.T, e *Engine, opts ...Option) *Harness {
	t.Helper()

	var o options
	for _, fn := range opts {
		fn(&o)
	}

	addr := e.Shared(t)
	db := 0
	if o.exclusive {
		addr = e.Exclusive(t, o.serverArgs...)
	} else {
		db = e.acquireDB(t)
	}

	cfg := config.New()
	cfg.ValkeyURL = addr
	cfg.ValkeyDB = db
	cfg.Dev = true
	cfg.Version = "test"
	cfg.Commit = "testcommit"
	for _, fn := range o.cfgFns {
		fn(cfg)
	}

	client, err := valkey.New(cfg)
	if err != nil {
		t.Fatalf("could not connect to %s at %s (db %d): %v", e.Name, addr, db, err)
	}
	t.Cleanup(client.Close)

	// One context for the whole test. Ctx is called from every seed helper, so
	// minting a fresh timer per call would pile up thousands of them.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	h := &Harness{
		T:      t,
		Engine: e,
		Addr:   addr,
		DB:     db,
		Cfg:    cfg,
		Client: client,
		http:   &http.Client{Timeout: 30 * time.Second},
		ctx:    ctx,
	}

	h.Flush()
	t.Cleanup(h.Flush)

	h.API = api.New(cfg, client)
	h.Server = httptest.NewServer(h.API)
	t.Cleanup(h.Server.Close)
	h.BaseURL = h.Server.URL

	return h
}

// With returns a shallow copy of the harness bound to t, sharing the same
// server, client and database. Use it inside a subtest so a failed assertion is
// reported against that subtest rather than the test that built the harness.
func (h *Harness) With(t *testing.T) *Harness {
	copied := *h
	copied.T = t
	return &copied
}

// Ctx returns the harness's context, cancelled when the test that built it ends.
func (h *Harness) Ctx() context.Context { return h.ctx }

// Flush empties the harness's logical database.
func (h *Harness) Flush() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := h.Client.FlushDB(ctx); err != nil {
		h.T.Errorf("FLUSHDB on %s db %d failed: %v", h.Engine.Name, h.DB, err)
	}
}

// HTTP request helpers.

// Res is a captured HTTP response with assertion helpers.
type Res struct {
	t      *testing.T
	Method string
	Path   string
	Status int
	Header http.Header
	Body   []byte
}

// Request performs an API request. body may be nil (no body), a string or
// []byte (sent verbatim), or any value that will be JSON-encoded.
func (h *Harness) Request(method, path string, body any) *Res {
	h.T.Helper()

	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = strings.NewReader(b)
	case []byte:
		reader = bytes.NewReader(b)
	default:
		encoded, err := json.Marshal(b)
		if err != nil {
			h.T.Fatalf("could not encode request body for %s %s: %v", method, path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, h.BaseURL+path, reader)
	if err != nil {
		h.T.Fatalf("could not build request %s %s: %v", method, path, err)
	}
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.http.Do(req)
	if err != nil {
		h.T.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		h.T.Fatalf("could not read response body for %s %s: %v", method, path, err)
	}

	return &Res{
		t:      h.T,
		Method: method,
		Path:   path,
		Status: resp.StatusCode,
		Header: resp.Header,
		Body:   data,
	}
}

// Get performs a GET request.
func (h *Harness) Get(path string) *Res { h.T.Helper(); return h.Request(http.MethodGet, path, nil) }

// Post performs a POST request.
func (h *Harness) Post(path string, body any) *Res {
	h.T.Helper()
	return h.Request(http.MethodPost, path, body)
}

// Put performs a PUT request.
func (h *Harness) Put(path string, body any) *Res {
	h.T.Helper()
	return h.Request(http.MethodPut, path, body)
}

// Patch performs a PATCH request.
func (h *Harness) Patch(path string, body any) *Res {
	h.T.Helper()
	return h.Request(http.MethodPatch, path, body)
}

// Delete performs a DELETE request.
func (h *Harness) Delete(path string) *Res {
	h.T.Helper()
	return h.Request(http.MethodDelete, path, nil)
}

// KeyPath builds /api/key/<key>[/segment...] with each segment escaped, so keys
// containing slashes, spaces or unicode survive the round trip.
func (h *Harness) KeyPath(key string, segments ...string) string {
	path := "/api/key/" + url.PathEscape(key)
	for _, s := range segments {
		path += "/" + url.PathEscape(s)
	}
	return path
}

// ExpectStatus fails the test unless the response has the given status.
func (r *Res) ExpectStatus(want int) *Res {
	r.t.Helper()
	if r.Status != want {
		r.t.Fatalf("%s %s: got status %d, want %d\nbody: %s",
			r.Method, r.Path, r.Status, want, bytes.TrimSpace(r.Body))
	}
	return r
}

// ExpectOK fails the test unless the response is 200.
func (r *Res) ExpectOK() *Res { r.t.Helper(); return r.ExpectStatus(http.StatusOK) }

// ExpectError asserts a status and that the JSON error message contains want.
func (r *Res) ExpectError(status int, want string) *Res {
	r.t.Helper()
	r.ExpectStatus(status)
	if msg := r.ErrorMessage(); !strings.Contains(msg, want) {
		r.t.Fatalf("%s %s: error %q does not contain %q", r.Method, r.Path, msg, want)
	}
	return r
}

// Decode unmarshals the response body into v.
func (r *Res) Decode(v any) *Res {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		r.t.Fatalf("%s %s: could not decode response: %v\nbody: %s", r.Method, r.Path, err, r.Body)
	}
	return r
}

// Map decodes the response body as a JSON object.
func (r *Res) Map() map[string]any {
	r.t.Helper()
	var m map[string]any
	r.Decode(&m)
	return m
}

// ErrorMessage returns the "error" field of a JSON error response.
func (r *Res) ErrorMessage() string {
	r.t.Helper()
	var m map[string]string
	if err := json.Unmarshal(r.Body, &m); err != nil {
		return string(bytes.TrimSpace(r.Body))
	}
	return m["error"]
}

// String renders the response for failure messages.
func (r *Res) String() string {
	return fmt.Sprintf("%s %s -> %d %s", r.Method, r.Path, r.Status, bytes.TrimSpace(r.Body))
}
