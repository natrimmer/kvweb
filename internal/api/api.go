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
	cfg                     *config.Config
	client                  *valkey.Client
	mux                     *http.ServeMux
	onNotificationsEnabled  func() // Callback when notifications are enabled at runtime
	onNotificationsDisabled func() // Callback when notifications are disabled at runtime
}

// New creates a new API handler
func New(cfg *config.Config, client *valkey.Client) *Handler {
	h := &Handler{
		cfg:    cfg,
		client: client,
		mux:    http.NewServeMux(),
	}

	// Register routes
	h.mux.HandleFunc("GET /api/health", h.handleHealth)
	h.mux.HandleFunc("GET /api/config", h.handleConfig)
	h.mux.HandleFunc("GET /api/info", h.handleInfo)
	h.mux.HandleFunc("GET /api/keys", h.handleKeys)
	h.mux.HandleFunc("GET /api/prefixes", h.handlePrefixes)
	h.mux.HandleFunc("GET /api/key/{key}", h.handleGetKey)
	h.mux.HandleFunc("PUT /api/key/{key}", h.handleSetKey)
	h.mux.HandleFunc("DELETE /api/key/{key}", h.handleDeleteKey)
	h.mux.HandleFunc("POST /api/key/{key}/incr", h.handleIncrKey)
	h.mux.HandleFunc("POST /api/key/{key}/expire", h.handleExpire)
	h.mux.HandleFunc("POST /api/key/{key}/rename", h.handleRename)
	h.mux.HandleFunc("POST /api/flush", h.handleFlush)
	h.mux.HandleFunc("GET /api/notifications", h.handleGetNotifications)
	h.mux.HandleFunc("POST /api/notifications", h.handleSetNotifications)

	// Complex type CRUD endpoints
	// List operations
	h.mux.HandleFunc("POST /api/key/{key}/list", h.handleListAdd)
	h.mux.HandleFunc("PUT /api/key/{key}/list/{index}", h.handleListSet)
	h.mux.HandleFunc("DELETE /api/key/{key}/list/{index}", h.handleListRemove)

	// Set operations
	h.mux.HandleFunc("POST /api/key/{key}/set", h.handleSetAdd)
	h.mux.HandleFunc("DELETE /api/key/{key}/set/{member}", h.handleSetRemove)
	h.mux.HandleFunc("PATCH /api/key/{key}/set/{member}", h.handleSetRename)

	// Hash operations
	h.mux.HandleFunc("POST /api/key/{key}/hash", h.handleHashSet)
	h.mux.HandleFunc("DELETE /api/key/{key}/hash/{field}", h.handleHashRemove)
	h.mux.HandleFunc("PATCH /api/key/{key}/hash/{field}", h.handleHashRename)

	// ZSet operations
	h.mux.HandleFunc("POST /api/key/{key}/zset", h.handleZSetAdd)
	h.mux.HandleFunc("DELETE /api/key/{key}/zset/{member}", h.handleZSetRemove)
	h.mux.HandleFunc("PATCH /api/key/{key}/zset/{member}", h.handleZSetRename)

	// Geo operations (uses zset internally, provides coordinate view)
	h.mux.HandleFunc("GET /api/key/{key}/geo", h.handleGeoGet)
	h.mux.HandleFunc("POST /api/key/{key}/geo", h.handleGeoAdd)
	// DELETE uses handleZSetRemove - same underlying operation

	// Stream operations
	h.mux.HandleFunc("POST /api/key/{key}/stream", h.handleStreamAdd)
	h.mux.HandleFunc("DELETE /api/key/{key}/stream/{id}", h.handleStreamRemove)

	// HyperLogLog operations
	h.mux.HandleFunc("POST /api/key/{key}/hll", h.handleHLLAdd)

	return h
}

// SetOnNotificationsEnabled sets the callback for when notifications are enabled at runtime
func (h *Handler) SetOnNotificationsEnabled(fn func()) {
	h.onNotificationsEnabled = fn
}

// SetOnNotificationsDisabled sets the callback for when notifications are disabled at runtime
func (h *Handler) SetOnNotificationsDisabled(fn func()) {
	h.onNotificationsDisabled = fn
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
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
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

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check database connectivity by pinging
	err := h.client.Ping(r.Context())

	status := "ok"
	dbConnected := true

	if err != nil {
		status = "degraded"
		dbConnected = false
	}

	jsonResponse(w, map[string]any{
		"status":    status,
		"database":  dbConnected,
		"timestamp": time.Now().Unix(),
	})
}

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
			if err != nil {
				continue
			}
			// Detect HyperLogLog (stored as string with HYLL magic header)
			if keyType == "string" {
				val, err := h.client.Get(r.Context(), key)
				if err == nil && len(val) >= 4 && val[:4] == "HYLL" {
					keyType = "hyperloglog"
				}
			}
			if keyType == typeFilter {
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
			// Detect HyperLogLog (stored as string with HYLL magic header)
			if keyType == "string" {
				val, err := h.client.Get(r.Context(), key)
				if err == nil && len(val) >= 4 && val[:4] == "HYLL" {
					keyType = "hyperloglog"
				}
			}
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
	Prefix  string `json:"prefix"`
	Count   int    `json:"count"`
	IsLeaf  bool   `json:"isLeaf"`
	FullKey string `json:"fullKey,omitempty"`
	KeyType string `json:"type,omitempty"`
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

const defaultPageSize = 100 // default page size for collections

func (h *Handler) handleGetKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	// Parse pagination params
	pageStr := r.URL.Query().Get("page")
	page := int64(1)
	if pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 64); err == nil && p > 0 {
			page = p
		}
	}

	pageSizeStr := r.URL.Query().Get("pageSize")
	pageSize := int64(defaultPageSize)
	if pageSizeStr != "" {
		if ps, err := strconv.ParseInt(pageSizeStr, 10, 64); err == nil && ps > 0 && ps <= 1000 {
			pageSize = ps
		}
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
	var pagination map[string]any

	switch keyType {
	case "string":
		val, getErr := h.client.Get(ctx, key)
		if getErr != nil {
			err = getErr
		} else if len(val) >= 4 && val[:4] == "HYLL" {
			// HyperLogLog detected by magic header
			keyType = "hyperloglog"
			count, _ := h.client.PFCount(ctx, key)
			value = map[string]any{"count": count}
		} else {
			value = val
		}
	case "list":
		length, _ = h.client.LLen(ctx, key)
		start := (page - 1) * pageSize
		stop := start + pageSize - 1
		value, err = h.client.LRange(ctx, key, start, stop)
		if err == nil {
			pagination = map[string]any{
				"page":     page,
				"pageSize": pageSize,
				"total":    length,
				"hasMore":  stop < length-1,
			}
		}
	case "set":
		length, _ = h.client.SCard(ctx, key)
		// For sets, we use cursor-based pagination
		// We need to scan through pages to get to the requested page
		cursor := uint64(0)
		var allMembers []string
		membersNeeded := page * pageSize

		for {
			members, nextCursor, scanErr := h.client.SScan(ctx, key, cursor, pageSize)
			if scanErr != nil {
				err = scanErr
				break
			}
			allMembers = append(allMembers, members...)
			cursor = nextCursor

			// Stop if we have enough members or reached the end
			if int64(len(allMembers)) >= membersNeeded || cursor == 0 {
				break
			}
		}

		if err == nil {
			start := (page - 1) * pageSize
			end := start + pageSize
			if start < int64(len(allMembers)) {
				if end > int64(len(allMembers)) {
					end = int64(len(allMembers))
				}
				value = allMembers[start:end]
			} else {
				value = []string{}
			}
			pagination = map[string]any{
				"page":     page,
				"pageSize": pageSize,
				"total":    length,
				"hasMore":  cursor != 0 || end < int64(len(allMembers)),
			}
		}
	case "hash":
		length, _ = h.client.HLen(ctx, key)
		// For hashes, we use cursor-based pagination similar to sets
		cursor := uint64(0)
		allFields := make(map[string]string)
		fieldsNeeded := page * pageSize

		for {
			fields, nextCursor, scanErr := h.client.HScan(ctx, key, cursor, pageSize)
			if scanErr != nil {
				err = scanErr
				break
			}
			for k, v := range fields {
				allFields[k] = v
			}
			cursor = nextCursor

			// Stop if we have enough fields or reached the end
			if int64(len(allFields)) >= fieldsNeeded || cursor == 0 {
				break
			}
		}

		if err == nil {
			// Convert to slice of key-value pairs for pagination
			type hashPair struct {
				Field string `json:"field"`
				Value string `json:"value"`
			}
			var allPairs []hashPair
			for field, val := range allFields {
				allPairs = append(allPairs, hashPair{Field: field, Value: val})
			}
			// Sort by field name for consistent ordering
			sort.Slice(allPairs, func(i, j int) bool {
				return allPairs[i].Field < allPairs[j].Field
			})

			start := (page - 1) * pageSize
			end := start + pageSize
			if start < int64(len(allPairs)) {
				if end > int64(len(allPairs)) {
					end = int64(len(allPairs))
				}
				value = allPairs[start:end]
			} else {
				value = []hashPair{}
			}
			pagination = map[string]any{
				"page":     page,
				"pageSize": pageSize,
				"total":    length,
				"hasMore":  cursor != 0 || end < int64(len(allPairs)),
			}
		}
	case "zset":
		length, _ = h.client.ZCard(ctx, key)
		start := (page - 1) * pageSize
		stop := start + pageSize - 1
		value, err = h.client.ZRangeWithScores(ctx, key, start, stop)
		if err == nil {
			pagination = map[string]any{
				"page":     page,
				"pageSize": pageSize,
				"total":    length,
				"hasMore":  stop < length-1,
			}
		}
	case "stream":
		length, _ = h.client.XLen(ctx, key)
		// Streams use ID-based pagination for efficiency
		// We fetch only the entries needed using XRANGE with cursor

		// To support page jumping, we need to find the starting ID for the requested page
		// For page 1, start from beginning. For others, we need to skip entries.
		var startAfterID string
		if page > 1 {
			// We need to skip (page-1) * pageSize entries to find the start ID
			skipCount := (page - 1) * pageSize
			if skipCount < length {
				// Fetch entries up to but not including the target page to get the cursor
				skipEntries, err := h.client.XRange(ctx, key, "-", "+", skipCount)
				if err != nil {
					jsonError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if len(skipEntries) > 0 {
					startAfterID = skipEntries[len(skipEntries)-1].ID
				}
			}
		}

		// Now fetch the actual page using the cursor
		entries, nextCursor, err := h.client.XRangePage(ctx, key, startAfterID, pageSize)
		if err == nil {
			value = entries
			pagination = map[string]any{
				"page":       page,
				"pageSize":   pageSize,
				"total":      length,
				"hasMore":    nextCursor != "",
				"nextCursor": nextCursor,
			}
		}
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

	if pagination != nil {
		resp["pagination"] = pagination
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

func (h *Handler) handleIncrKey(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	var body struct {
		Amount float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	newValue, err := h.client.IncrByFloat(r.Context(), key, body.Amount)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{
		"value": newValue,
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

func (h *Handler) handleGetNotifications(w http.ResponseWriter, r *http.Request) {
	val, err := h.client.GetNotifyKeyspaceEvents(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{
		"enabled": val != "",
		"value":   val,
	})
}

func (h *Handler) handleSetNotifications(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	val := ""
	if body.Enabled {
		// K = Keyspace events, E = Keyevent events
		// A = all commands (includes HyperLogLog which has no dedicated flag)
		// g = generic (DEL, EXPIRE, RENAME), e = expired, x = evicted
		val = "KEAgex"
	}

	if err := h.client.SetNotifyKeyspaceEvents(r.Context(), val); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Trigger callbacks based on enabled/disabled state
	if body.Enabled && h.onNotificationsEnabled != nil {
		h.onNotificationsEnabled()
	} else if !body.Enabled && h.onNotificationsDisabled != nil {
		h.onNotificationsDisabled()
	}

	jsonResponse(w, map[string]any{
		"ok":      true,
		"enabled": body.Enabled,
	})
}

// List operation handlers

func (h *Handler) handleListAdd(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	var body struct {
		Value    string `json:"value"`
		Position string `json:"position"` // "head" or "tail"
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var err error
	if body.Position == "head" {
		err = h.client.LPush(r.Context(), key, body.Value)
	} else {
		err = h.client.RPush(r.Context(), key, body.Value)
	}

	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleListSet(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	indexStr := r.PathValue("index")
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid index", http.StatusBadRequest)
		return
	}

	var body struct {
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.client.LSet(r.Context(), key, index, body.Value); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleListRemove(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	indexStr := r.PathValue("index")
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid index", http.StatusBadRequest)
		return
	}

	if err := h.client.LRemByIndex(r.Context(), key, index); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

// Set operation handlers

func (h *Handler) handleSetAdd(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	var body struct {
		Member string `json:"member"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Member == "" {
		jsonError(w, "Member cannot be empty", http.StatusBadRequest)
		return
	}

	// Check for duplicate
	exists, err := h.client.SIsMember(r.Context(), key, body.Member)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists {
		jsonError(w, "Member already exists in set", http.StatusConflict)
		return
	}

	if err := h.client.SAdd(r.Context(), key, body.Member); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleSetRemove(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	member := r.PathValue("member")
	if member == "" {
		jsonError(w, "Member cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.client.SRem(r.Context(), key, member); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleSetRename(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	oldMember := r.PathValue("member")
	if oldMember == "" {
		jsonError(w, "Member cannot be empty", http.StatusBadRequest)
		return
	}

	var body struct {
		NewMember string `json:"newMember"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.NewMember == "" {
		jsonError(w, "New member cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.client.SRename(r.Context(), key, oldMember, body.NewMember); err != nil {
		// Check for specific error messages from Lua script
		switch err.Error() {
		case "Member does not exist":
			jsonError(w, "Member does not exist", http.StatusNotFound)
		case "New member already exists":
			jsonError(w, "New member already exists", http.StatusConflict)
		default:
			jsonError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

// Hash operation handlers

func (h *Handler) handleHashSet(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	var body struct {
		Field string `json:"field"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Field == "" {
		jsonError(w, "Field name cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.client.HSet(r.Context(), key, body.Field, body.Value); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleHashRemove(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	field := r.PathValue("field")
	if field == "" {
		jsonError(w, "Field name cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.client.HDel(r.Context(), key, field); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleHashRename(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	oldField := r.PathValue("field")
	if oldField == "" {
		jsonError(w, "Field name cannot be empty", http.StatusBadRequest)
		return
	}

	var body struct {
		NewField string `json:"newField"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.NewField == "" {
		jsonError(w, "New field name cannot be empty", http.StatusBadRequest)
		return
	}

	value, err := h.client.HRename(r.Context(), key, oldField, body.NewField)
	if err != nil {
		// Check for specific error messages from Lua script
		switch err.Error() {
		case "Field does not exist":
			jsonError(w, "Field does not exist", http.StatusNotFound)
		case "New field already exists":
			jsonError(w, "New field already exists", http.StatusConflict)
		default:
			jsonError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	jsonResponse(w, map[string]interface{}{
		"status": "ok",
		"value":  value,
	})
}

// ZSet operation handlers

func (h *Handler) handleZSetAdd(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	var body struct {
		Member string  `json:"member"`
		Score  float64 `json:"score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Member == "" {
		jsonError(w, "Member cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.client.ZAdd(r.Context(), key, body.Member, body.Score); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleZSetRemove(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	member := r.PathValue("member")
	if member == "" {
		jsonError(w, "Member cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.client.ZRem(r.Context(), key, member); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleZSetRename(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	oldMember := r.PathValue("member")
	if oldMember == "" {
		jsonError(w, "Member cannot be empty", http.StatusBadRequest)
		return
	}

	var body struct {
		NewMember string `json:"newMember"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.NewMember == "" {
		jsonError(w, "New member name cannot be empty", http.StatusBadRequest)
		return
	}

	score, err := h.client.ZRename(r.Context(), key, oldMember, body.NewMember)
	if err != nil {
		// Check for specific error messages from Lua script
		switch err.Error() {
		case "Member does not exist":
			jsonError(w, "Member does not exist", http.StatusNotFound)
		case "New member already exists":
			jsonError(w, "New member already exists", http.StatusConflict)
		default:
			jsonError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	jsonResponse(w, map[string]interface{}{
		"status": "ok",
		"score":  score,
	})
}

// Geo operation handlers

func (h *Handler) handleGeoGet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	// Parse pagination params
	pageStr := r.URL.Query().Get("page")
	page := int64(1)
	if pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 64); err == nil && p > 0 {
			page = p
		}
	}

	pageSizeStr := r.URL.Query().Get("pageSize")
	pageSize := int64(defaultPageSize)
	if pageSizeStr != "" {
		if ps, err := strconv.ParseInt(pageSizeStr, 10, 64); err == nil && ps > 0 && ps <= 1000 {
			pageSize = ps
		}
	}

	ctx := r.Context()

	// Get total count
	length, _ := h.client.ZCard(ctx, key)

	// Get paginated members
	start := (page - 1) * pageSize
	stop := start + pageSize - 1

	zMembers, err := h.client.ZRangeWithScores(ctx, key, start, stop)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract member names for GEOPOS
	memberNames := make([]string, len(zMembers))
	for i, m := range zMembers {
		memberNames[i] = m.Member
	}

	// Get coordinates
	positions, err := h.client.GeoPos(ctx, key, memberNames...)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Combine into response (only include members with valid positions)
	geoMembers := make([]valkey.GeoMember, 0, len(zMembers))
	for i, m := range zMembers {
		if i < len(positions) && positions[i] != nil {
			geoMembers = append(geoMembers, valkey.GeoMember{
				Member:    m.Member,
				Longitude: positions[i].Longitude,
				Latitude:  positions[i].Latitude,
			})
		}
	}

	ttl, _ := h.client.TTL(ctx, key)

	jsonResponse(w, map[string]any{
		"key":    key,
		"type":   "zset",
		"value":  geoMembers,
		"ttl":    ttl,
		"length": length,
		"pagination": map[string]any{
			"page":     page,
			"pageSize": pageSize,
			"total":    length,
			"hasMore":  stop < length-1,
		},
	})
}

func (h *Handler) handleGeoAdd(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	var body struct {
		Member    string  `json:"member"`
		Longitude float64 `json:"longitude"`
		Latitude  float64 `json:"latitude"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Member == "" {
		jsonError(w, "Member cannot be empty", http.StatusBadRequest)
		return
	}

	// Validate coordinates (Redis geo limits)
	if body.Longitude < -180 || body.Longitude > 180 {
		jsonError(w, "Longitude must be between -180 and 180", http.StatusBadRequest)
		return
	}
	if body.Latitude < -85.05112878 || body.Latitude > 85.05112878 {
		jsonError(w, "Latitude must be between -85.05112878 and 85.05112878", http.StatusBadRequest)
		return
	}

	if err := h.client.GeoAdd(r.Context(), key, body.Longitude, body.Latitude, body.Member); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

// Stream operation handlers

func (h *Handler) handleStreamAdd(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	var body struct {
		Fields map[string]string `json:"fields"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(body.Fields) == 0 {
		jsonError(w, "At least one field is required", http.StatusBadRequest)
		return
	}

	// Validate field names and values are non-empty
	for field, value := range body.Fields {
		if field == "" {
			jsonError(w, "Field name cannot be empty", http.StatusBadRequest)
			return
		}
		if value == "" {
			jsonError(w, "Field value cannot be empty", http.StatusBadRequest)
			return
		}
	}

	id, err := h.client.XAddMulti(r.Context(), key, body.Fields)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok", "id": id})
}

func (h *Handler) handleStreamRemove(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		jsonError(w, "Entry ID cannot be empty", http.StatusBadRequest)
		return
	}

	deleted, err := h.client.XDel(r.Context(), key, id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if deleted == 0 {
		jsonError(w, "Entry not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

// HyperLogLog operation handlers

func (h *Handler) handleHLLAdd(w http.ResponseWriter, r *http.Request) {
	if h.checkReadOnly(w) {
		return
	}

	key := r.PathValue("key")
	if h.checkKeyPrefix(w, key) {
		return
	}

	var body struct {
		Element string `json:"element"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Element == "" {
		jsonError(w, "Element cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.client.PFAdd(r.Context(), key, body.Element); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}
