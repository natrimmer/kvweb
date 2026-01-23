package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gnat/kvweb/internal/config"
	"github.com/gnat/kvweb/internal/valkey"
)

// Handler handles API requests
type Handler struct {
	cfg    *config.Config
	client *valkey.Client
	mux    *http.ServeMux
}

// New creates a new API handler
func New(cfg *config.Config, client *valkey.Client) *Handler {
	h := &Handler{
		cfg:    cfg,
		client: client,
		mux:    http.NewServeMux(),
	}

	// Register routes
	h.mux.HandleFunc("GET /api/config", h.handleConfig)
	h.mux.HandleFunc("GET /api/info", h.handleInfo)
	h.mux.HandleFunc("GET /api/keys", h.handleKeys)
	h.mux.HandleFunc("GET /api/key/{key}", h.handleGetKey)
	h.mux.HandleFunc("PUT /api/key/{key}", h.handleSetKey)
	h.mux.HandleFunc("DELETE /api/key/{key}", h.handleDeleteKey)
	h.mux.HandleFunc("POST /api/key/{key}/expire", h.handleExpire)
	h.mux.HandleFunc("POST /api/key/{key}/rename", h.handleRename)
	h.mux.HandleFunc("POST /api/flush", h.handleFlush)

	return h
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for development
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.mux.ServeHTTP(w, r)
}

// Response helpers

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// checkReadOnly returns true and sends an error response if in readonly mode
func (h *Handler) checkReadOnly(w http.ResponseWriter) bool {
	if h.cfg.ReadOnly {
		jsonError(w, "Server is in read-only mode", http.StatusForbidden)
		return true
	}
	return false
}

// Handlers

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]any{
		"readOnly": h.cfg.ReadOnly,
	})
}

func (h *Handler) handleInfo(w http.ResponseWriter, r *http.Request) {
	section := r.URL.Query().Get("section")

	info, err := h.client.Info(r.Context(), section)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dbSize, _ := h.client.DBSize(r.Context())

	jsonResponse(w, map[string]any{
		"info":   info,
		"dbSize": dbSize,
	})
}

func (h *Handler) handleKeys(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = "*"
	}

	cursorStr := r.URL.Query().Get("cursor")
	cursor := uint64(0)
	if cursorStr != "" {
		cursor, _ = strconv.ParseUint(cursorStr, 10, 64)
	}

	countStr := r.URL.Query().Get("count")
	count := int64(100)
	if countStr != "" {
		count, _ = strconv.ParseInt(countStr, 10, 64)
	}

	keys, nextCursor, err := h.client.Keys(r.Context(), pattern, cursor, count)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"keys":   keys,
		"cursor": nextCursor,
	})
}

func (h *Handler) handleGetKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	keyType, err := h.client.Type(r.Context(), key)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if keyType == "none" {
		jsonError(w, "Key not found", http.StatusNotFound)
		return
	}

	ttl, _ := h.client.TTL(r.Context(), key)

	var value any
	switch keyType {
	case "string":
		value, err = h.client.Get(r.Context(), key)
	default:
		// TODO: Handle other types (list, set, hash, zset, stream)
		value = "(complex type - not yet supported)"
	}

	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"key":   key,
		"type":  keyType,
		"value": value,
		"ttl":   ttl,
	})
}

func (h *Handler) handleSetKey(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")

	var body struct {
		Value string `json:"value"`
		TTL   int64  `json:"ttl"` // seconds, 0 = no expiry
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ttl := time.Duration(0)
	if body.TTL > 0 {
		ttl = time.Duration(body.TTL) * time.Second
	}

	if err := h.client.Set(r.Context(), key, body.Value, ttl); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")

	deleted, err := h.client.Del(r.Context(), key)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"deleted": deleted,
	})
}

func (h *Handler) handleExpire(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")

	var body struct {
		TTL int64 `json:"ttl"` // seconds, 0 = persist (remove TTL)
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var ok bool
	var err error

	if body.TTL == 0 {
		ok, err = h.client.Persist(r.Context(), key)
	} else {
		ok, err = h.client.Expire(r.Context(), key, time.Duration(body.TTL)*time.Second)
	}

	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]bool{"ok": ok})
}

func (h *Handler) handleRename(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")

	var body struct {
		NewKey string `json:"newKey"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	body.NewKey = strings.TrimSpace(body.NewKey)
	if body.NewKey == "" {
		jsonError(w, "New key name required", http.StatusBadRequest)
		return
	}

	if err := h.client.Rename(r.Context(), key, body.NewKey); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleFlush(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	if err := h.client.FlushDB(r.Context()); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}
