package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// execRequest is the request body for POST /api/exec
type execRequest struct {
	Command string `json:"command"`
}

// handleExec executes a raw Valkey command and returns the result.
func (h *Handler) handleExec(w http.ResponseWriter, r *http.Request) {
	var body execRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	args := parseCommand(body.Command)
	if len(args) == 0 {
		jsonError(w, "Empty command", http.StatusBadRequest)
		return
	}

	cmd := strings.ToUpper(args[0])

	// Check always-blocked commands first
	if blockedCommands[cmd] {
		jsonError(w, "Command not allowed: "+cmd, http.StatusForbidden)
		return
	}

	// Check subcommand-level blocks (e.g. CONFIG SET)
	if subs, ok := blockedSubcommands[cmd]; ok && len(args) > 1 {
		sub := strings.ToUpper(args[1])
		if subs[sub] {
			jsonError(w, "Command not allowed: "+cmd+" "+sub, http.StatusForbidden)
			return
		}
	}

	// Readonly mode: only allow known read-only commands
	if h.cfg.ReadOnly {
		if !readOnlyCommands[cmd] {
			jsonError(w, "Command not allowed in read-only mode: "+cmd, http.StatusForbidden)
			return
		}
		// For commands with subcommands, check if the specific subcommand is read-only
		if subs, ok := readOnlySubcommands[cmd]; ok && len(args) > 1 {
			sub := strings.ToUpper(args[1])
			if !subs[sub] {
				jsonError(w, "Subcommand not allowed in read-only mode: "+cmd+" "+sub, http.StatusForbidden)
				return
			}
		}
	}

	// FLUSHDB/FLUSHALL blocked when DisableFlush is set
	if h.cfg.DisableFlush && (cmd == "FLUSHDB" || cmd == "FLUSHALL") {
		jsonError(w, "FLUSHDB/FLUSHALL is disabled", http.StatusForbidden)
		return
	}

	// Prefix enforcement: check key arguments
	if h.cfg.Prefix != "" {
		if !checkPrefixArgs(cmd, args, h.cfg.Prefix) {
			jsonError(w, "Key does not match required prefix: "+h.cfg.Prefix, http.StatusForbidden)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := h.client.Exec(ctx, args)
	if err != nil {
		// Return valkey errors as formatted results, not HTTP errors
		jsonResponse(w, formatResult(err))
		return
	}

	jsonResponse(w, formatResult(result))
}

// parseCommand splits a command string into arguments, respecting double-quoted strings.
func parseCommand(input string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch {
		case ch == '"':
			inQuotes = !inQuotes
		case ch == '\\' && inQuotes && i+1 < len(input):
			// Handle escaped characters inside quotes
			i++
			switch input[i] {
			case '"':
				current.WriteByte('"')
			case '\\':
				current.WriteByte('\\')
			case 'n':
				current.WriteByte('\n')
			case 't':
				current.WriteByte('\t')
			default:
				current.WriteByte('\\')
				current.WriteByte(input[i])
			}
		case ch == ' ' && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// formatResult converts a Valkey response to a typed JSON structure.
func formatResult(v any) map[string]any {
	if v == nil {
		return map[string]any{"type": "nil", "value": nil}
	}
	switch val := v.(type) {
	case error:
		return map[string]any{"type": "error", "value": val.Error()}
	case string:
		return map[string]any{"type": "string", "value": val}
	case int64:
		return map[string]any{"type": "integer", "value": val}
	case float64:
		return map[string]any{"type": "string", "value": val}
	case bool:
		if val {
			return map[string]any{"type": "integer", "value": int64(1)}
		}
		return map[string]any{"type": "nil", "value": nil}
	case []any:
		items := make([]map[string]any, len(val))
		for i, item := range val {
			items[i] = formatResult(item)
		}
		return map[string]any{"type": "array", "value": items}
	default:
		return map[string]any{"type": "string", "value": val}
	}
}

// checkPrefixArgs validates that key arguments match the required prefix.
func checkPrefixArgs(cmd string, args []string, prefix string) bool {
	positions := keyPositions(cmd, len(args))
	for _, pos := range positions {
		if pos < len(args) && !strings.HasPrefix(args[pos], prefix) {
			return false
		}
	}
	return true
}

// keyPositions returns the argument indices (0-based) that are key arguments for the given command.
func keyPositions(cmd string, argCount int) []int {
	// Commands with no key arguments
	switch cmd {
	case "PING", "INFO", "DBSIZE", "TIME", "LASTSAVE", "BGSAVE", "BGREWRITEAOF",
		"CONFIG", "CLIENT", "SLOWLOG", "COMMAND", "MEMORY", "LATENCY",
		"CLUSTER", "ACL", "DEBUG", "OBJECT":
		return nil
	}

	// Commands where all args after the command name are keys
	switch cmd {
	case "MGET", "DEL", "EXISTS", "UNLINK", "TOUCH", "WATCH":
		positions := make([]int, argCount-1)
		for i := range positions {
			positions[i] = i + 1
		}
		return positions
	}

	// RENAME: both args are keys
	if cmd == "RENAME" || cmd == "RENAMENX" {
		if argCount > 2 {
			return []int{1, 2}
		}
		if argCount > 1 {
			return []int{1}
		}
		return nil
	}

	// Default: first arg after command is the key
	if argCount > 1 {
		return []int{1}
	}
	return nil
}

// blockedCommands are always blocked regardless of readonly mode.
var blockedCommands = map[string]bool{
	// Blocking/streaming
	"SUBSCRIBE": true, "PSUBSCRIBE": true, "SSUBSCRIBE": true,
	"UNSUBSCRIBE": true, "PUNSUBSCRIBE": true, "SUNSUBSCRIBE": true,
	"MONITOR": true, "WAIT": true, "WAITAOF": true,
	"BLPOP": true, "BRPOP": true, "BLMOVE": true, "BRPOPLPUSH": true,
	"BLMPOP": true, "BZPOPMIN": true, "BZPOPMAX": true, "BZMPOP": true,
	// Server admin
	"SHUTDOWN": true, "DEBUG": true, "SLAVEOF": true, "REPLICAOF": true,
	"FAILOVER": true, "MODULE": true, "SWAPDB": true,
	"MIGRATE": true, "OBJECT": true,
	// Scripting
	"EVAL": true, "EVALSHA": true, "EVAL_RO": true, "EVALSHA_RO": true,
	"SCRIPT": true, "FUNCTION": true, "FCALL": true, "FCALL_RO": true,
	// Transaction
	"MULTI": true, "EXEC": true, "DISCARD": true,
	"WATCH": true, "UNWATCH": true,
	// Connection
	"AUTH": true, "SELECT": true, "HELLO": true, "QUIT": true, "RESET": true,
}

// blockedSubcommands are subcommands blocked even in write mode.
var blockedSubcommands = map[string]map[string]bool{
	"CLIENT": {"KILL": true, "NO-EVICT": true, "NO-TOUCH": true, "PAUSE": true, "UNPAUSE": true},
	"CONFIG": {"SET": true, "REWRITE": true, "RESETSTAT": true},
	"ACL":    {"SETUSER": true, "DELUSER": true, "SAVE": true, "LOAD": true},
}

// readOnlyCommands are commands allowed in readonly mode.
var readOnlyCommands = map[string]bool{
	// Generic
	"PING": true, "ECHO": true,
	// Server info
	"INFO": true, "DBSIZE": true, "TIME": true, "LASTSAVE": true,
	"COMMAND": true, "MEMORY": true, "LATENCY": true,
	"CONFIG": true, "CLIENT": true, "SLOWLOG": true, "CLUSTER": true, "ACL": true,
	// Key inspection
	"EXISTS": true, "TYPE": true, "TTL": true, "PTTL": true,
	"DUMP": true, "OBJECT": true, "RANDOMKEY": true,
	"SCAN": true, "KEYS": true,
	// String
	"GET": true, "MGET": true, "GETRANGE": true, "STRLEN": true,
	"SUBSTR": true,
	// Hash
	"HGET": true, "HMGET": true, "HGETALL": true, "HKEYS": true,
	"HVALS": true, "HLEN": true, "HEXISTS": true, "HSCAN": true,
	"HRANDFIELD": true,
	// List
	"LINDEX": true, "LLEN": true, "LRANGE": true, "LPOS": true,
	// Set
	"SCARD": true, "SISMEMBER": true, "SMEMBERS": true,
	"SMISMEMBER": true, "SRANDMEMBER": true, "SSCAN": true,
	"SINTER": true, "SUNION": true, "SDIFF": true,
	// Sorted set
	"ZCARD": true, "ZCOUNT": true, "ZLEXCOUNT": true,
	"ZRANGE": true, "ZRANGEBYLEX": true, "ZRANGEBYSCORE": true,
	"ZRANK": true, "ZREVRANGE": true, "ZREVRANGEBYLEX": true,
	"ZREVRANGEBYSCORE": true, "ZREVRANK": true, "ZSCORE": true,
	"ZMSCORE": true, "ZRANDMEMBER": true, "ZSCAN": true,
	// Stream
	"XINFO": true, "XLEN": true, "XRANGE": true, "XREVRANGE": true,
	"XREAD": true, "XPENDING": true,
	// Geo
	"GEOPOS": true, "GEODIST": true, "GEOHASH": true,
	"GEORADIUS_RO": true, "GEORADIUSBYMEMBER_RO": true, "GEOSEARCH": true,
	// HyperLogLog
	"PFCOUNT": true,
}

// readOnlySubcommands refines which subcommands are allowed in readonly mode
// for commands that have both read and write subcommands.
var readOnlySubcommands = map[string]map[string]bool{
	"CONFIG":  {"GET": true},
	"CLIENT":  {"GETNAME": true, "ID": true, "INFO": true, "LIST": true},
	"SLOWLOG": {"GET": true, "LEN": true},
	"CLUSTER": {"INFO": true, "NODES": true, "SLOTS": true, "MYID": true, "KEYSLOT": true},
	"ACL":     {"LIST": true, "GETUSER": true, "WHOAMI": true, "CAT": true, "LOG": true},
	"MEMORY":  {"USAGE": true, "DOCTOR": true, "STATS": true},
	"LATENCY": {"LATEST": true, "HISTORY": true},
	"COMMAND": {"COUNT": true, "DOCS": true, "INFO": true, "LIST": true, "GETKEYS": true},
	"XINFO":   {"STREAM": true, "GROUPS": true, "CONSUMERS": true},
}
