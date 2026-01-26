package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
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
	h.mux.HandleFunc("GET /api/prefixes", h.handlePrefixes)
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

// checkKeyPrefix returns true and sends an error response if key doesn't match prefix
func (h *Handler) checkKeyPrefix(w http.ResponseWriter, key string) bool {
	if h.cfg.Prefix != "" && !strings.HasPrefix(key, h.cfg.Prefix) {
		jsonError(w, "Key does not match required prefix", http.StatusForbidden)
		return true
	}
	return false
}

// applyPrefixToPattern prepends the configured prefix to a search pattern
func (h *Handler) applyPrefixToPattern(pattern string) string {
	if h.cfg.Prefix == "" {
		return pattern
	}
	// If pattern is "*", return "prefix*"
	// If pattern is "foo*", return "prefixfoo*"
	// This ensures we only see keys under our prefix
	if pattern == "*" {
		return h.cfg.Prefix + "*"
	}
	return h.cfg.Prefix + pattern
}

// Handlers

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]any{
		"readOnly":     h.cfg.ReadOnly,
		"prefix":       h.cfg.Prefix,
		"disableFlush": h.cfg.DisableFlush,
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

type keyMeta struct {
	Key  string `json:"key"`
	Type string `json:"type"`
	TTL  int64  `json:"ttl"`
}

func (h *Handler) handleKeys(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = "*"
	}

	useRegex := r.URL.Query().Get("regex") == "1"

	// If regex mode, validate and compile the pattern before applying prefix
	var re *regexp.Regexp
	if useRegex {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			jsonError(w, "Invalid regex: "+err.Error(), http.StatusBadRequest)
			return
		}
		// Use wildcard for SCAN, filter with regex after
		pattern = h.applyPrefixToPattern("*")
	} else {
		pattern = h.applyPrefixToPattern(pattern)
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

	// Apply max-keys limit if configured
	if h.cfg.MaxKeys > 0 && count > h.cfg.MaxKeys {
		count = h.cfg.MaxKeys
	}

	typeFilter := r.URL.Query().Get("type")
	withMeta := r.URL.Query().Get("meta") == "1"

	keys, nextCursor, err := h.client.Keys(r.Context(), pattern, cursor, count)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter by regex if in regex mode
	if re != nil {
		filtered := make([]string, 0, len(keys))
		for _, key := range keys {
			if re.MatchString(key) {
				filtered = append(filtered, key)
			}
		}
		keys = filtered
	}

	// Filter by type if requested
	if typeFilter != "" {
		filtered := make([]string, 0, len(keys))
		for _, key := range keys {
			keyType, err := h.client.Type(r.Context(), key)
			if err == nil && keyType == typeFilter {
				filtered = append(filtered, key)
			}
		}
		keys = filtered
	}

	// Return with metadata if requested (for sorting)
	if withMeta {
		metas := make([]keyMeta, 0, len(keys))
		for _, key := range keys {
			keyType, _ := h.client.Type(r.Context(), key)
			ttl, _ := h.client.TTL(r.Context(), key)
			metas = append(metas, keyMeta{Key: key, Type: keyType, TTL: ttl})
		}
		jsonResponse(w, map[string]any{
			"keys":   metas,
			"cursor": nextCursor,
		})
		return
	}

	jsonResponse(w, map[string]any{
		"keys":   keys,
		"cursor": nextCursor,
	})
}

type prefixEntry struct {
	Prefix   string `json:"prefix"`
	Count    int    `json:"count"`
	IsLeaf   bool   `json:"isLeaf"`
	FullKey  string `json:"fullKey,omitempty"`
	KeyType  string `json:"type,omitempty"`
}

func (h *Handler) handlePrefixes(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	if delimiter == "" {
		delimiter = ":"
	}

	// Build the search pattern
	pattern := h.applyPrefixToPattern("*")
	if prefix != "" {
		pattern = h.applyPrefixToPattern(prefix + "*")
	}

	// Scan all matching keys (with reasonable limit)
	var allKeys []string
	var cursor uint64
	limit := int64(10000)
	if h.cfg.MaxKeys > 0 && h.cfg.MaxKeys < limit {
		limit = h.cfg.MaxKeys
	}

	for {
		keys, nextCursor, err := h.client.Keys(r.Context(), pattern, cursor, 1000)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		allKeys = append(allKeys, keys...)
		cursor = nextCursor
		if cursor == 0 || int64(len(allKeys)) >= limit {
			break
		}
	}

	// Group by next prefix segment
	prefixLen := len(prefix)
	groups := make(map[string][]string)

	for _, key := range allKeys {
		// Remove the search prefix to get the remainder
		remainder := key
		if prefixLen > 0 && len(key) > prefixLen {
			remainder = key[prefixLen:]
		} else if prefixLen > 0 {
			remainder = ""
		}

		// Find the next delimiter
		delimIdx := strings.Index(remainder, delimiter)
		if delimIdx == -1 {
			// This is a leaf key
			groups[key] = nil
		} else {
			// This is a prefix group
			groupPrefix := prefix + remainder[:delimIdx+1]
			groups[groupPrefix] = append(groups[groupPrefix], key)
		}
	}

	// Build response
	entries := make([]prefixEntry, 0, len(groups))
	for groupKey, members := range groups {
		if members == nil {
			// Leaf key - get its type
			keyType, _ := h.client.Type(r.Context(), groupKey)
			entries = append(entries, prefixEntry{
				Prefix:  groupKey,
				Count:   1,
				IsLeaf:  true,
				FullKey: groupKey,
				KeyType: keyType,
			})
		} else {
			entries = append(entries, prefixEntry{
				Prefix: groupKey,
				Count:  len(members),
				IsLeaf: false,
			})
		}
	}

	// Sort by prefix
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Prefix < entries[j].Prefix
	})

	jsonResponse(w, map[string]any{
		"entries": entries,
		"prefix":  prefix,
	})
}

const maxItems = 100 // limit items returned for large collections

func (h *Handler) handleGetKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

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
	ctx := r.Context()

	var value any
	var length int64

	switch keyType {
	case "string":
		value, err = h.client.Get(ctx, key)
	case "list":
		length, _ = h.client.LLen(ctx, key)
		value, err = h.client.LRange(ctx, key, 0, maxItems-1)
	case "set":
		length, _ = h.client.SCard(ctx, key)
		value, err = h.client.SMembers(ctx, key)
	case "hash":
		length, _ = h.client.HLen(ctx, key)
		value, err = h.client.HGetAll(ctx, key)
	case "zset":
		length, _ = h.client.ZCard(ctx, key)
		value, err = h.client.ZRangeWithScores(ctx, key, 0, maxItems-1)
	case "stream":
		length, _ = h.client.XLen(ctx, key)
		value, err = h.client.XRange(ctx, key, "-", "+", maxItems)
	default:
		value = "(unsupported type)"
	}

	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"key":   key,
		"type":  keyType,
		"value": value,
		"ttl":   ttl,
	}

	if length > 0 {
		resp["length"] = length
	}

	jsonResponse(w, resp)
}

func (h *Handler) handleSetKey(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

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
	if h.checkKeyPrefix(w, key) {
		return
	}

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
	if h.checkKeyPrefix(w, key) {
		return
	}

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
	if h.checkKeyPrefix(w, key) {
		return
	}

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

	// Ensure new key also matches prefix
	if h.checkKeyPrefix(w, body.NewKey) {
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

	if h.cfg.DisableFlush {
		jsonError(w, "FLUSHDB is disabled", http.StatusForbidden)
		return
	}

	if err := h.client.FlushDB(r.Context()); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}
