package api_test

import (
	"net/http"
	"testing"

	"github.com/natrimmer/kvweb/internal/testenv"
)

// execResult mirrors the console's typed reply envelope.
type execResult struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

func exec(t *testing.T, h *testenv.Harness, command string) execResult {
	t.Helper()
	var result execResult
	h.With(t).Post("/api/exec", map[string]any{"command": command}).ExpectOK().Decode(&result)
	return result
}

func TestExecEndpoint(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		t.Run("Ping", func(t *testing.T) {
			result := exec(t, h, "PING")
			if result.Type != "string" || result.Value != "PONG" {
				t.Errorf("result = %+v, want string PONG", result)
			}
		})

		t.Run("SetThenGet", func(t *testing.T) {
			if result := exec(t, h, "SET console:key hello"); result.Value != "OK" {
				t.Errorf("SET returned %+v, want OK", result)
			}
			result := exec(t, h, "GET console:key")
			if result.Type != "string" || result.Value != "hello" {
				t.Errorf("GET returned %+v, want string hello", result)
			}
			if got := h.GetString("console:key"); got != "hello" {
				t.Errorf("the value was not actually stored: %q", got)
			}
		})

		t.Run("QuotedArguments", func(t *testing.T) {
			exec(t, h, `SET console:quoted "hello world"`)
			if got := h.GetString("console:quoted"); got != "hello world" {
				t.Errorf("value = %q, want %q", got, "hello world")
			}
		})

		t.Run("IntegerReply", func(t *testing.T) {
			result := exec(t, h, "STRLEN console:key")
			if result.Type != "integer" {
				t.Errorf("type = %q, want integer", result.Type)
			}
			if value, ok := result.Value.(float64); !ok || value != 5 {
				t.Errorf("value = %v, want 5", result.Value)
			}
		})

		t.Run("ArrayReply", func(t *testing.T) {
			h.SeedList("console:list", "a", "b")
			result := exec(t, h, "LRANGE console:list 0 -1")
			if result.Type != "array" {
				t.Fatalf("type = %q, want array", result.Type)
			}
			items, ok := result.Value.([]any)
			if !ok || len(items) != 2 {
				t.Fatalf("value = %v, want two items", result.Value)
			}
			first := items[0].(map[string]any)
			if first["type"] != "string" || first["value"] != "a" {
				t.Errorf("first item = %v, want string a", first)
			}
		})

		t.Run("CommandErrorsAreResultsNotHTTPErrors", func(t *testing.T) {
			// The console shows the server's error inline; a 500 would lose it.
			result := exec(t, h, "INCR console:key")
			if result.Type != "error" {
				t.Fatalf("type = %q, want error", result.Type)
			}
			if result.Value == "" {
				t.Error("the error message is empty")
			}
		})

		t.Run("UnknownCommandIsAnErrorResult", func(t *testing.T) {
			if result := exec(t, h, "NOSUCHCOMMAND"); result.Type != "error" {
				t.Errorf("type = %q, want error", result.Type)
			}
		})

		t.Run("EmptyCommand", func(t *testing.T) {
			h.Post("/api/exec", map[string]any{"command": ""}).
				ExpectError(http.StatusBadRequest, "Empty command")
			h.Post("/api/exec", map[string]any{"command": "   "}).
				ExpectError(http.StatusBadRequest, "Empty command")
		})

		t.Run("CommandNamesAreCaseInsensitive", func(t *testing.T) {
			if result := exec(t, h, "ping"); result.Value != "PONG" {
				t.Errorf("lowercase ping returned %+v", result)
			}
		})
	})
}

func TestExecBlockedCommands(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		// These are refused in write mode too: they would hang the request, take
		// the server down, or step outside the database this session is pinned to.
		blocked := []struct {
			group    string
			commands []string
		}{
			{"Blocking", []string{
				"SUBSCRIBE channel", "PSUBSCRIBE pattern*", "MONITOR", "WAIT 0 0",
				"BLPOP mylist 0", "BRPOP mylist 0", "BZPOPMIN myzset 0",
			}},
			{"ServerAdmin", []string{
				"SHUTDOWN NOSAVE", "DEBUG SLEEP 0", "REPLICAOF NO ONE",
				"FAILOVER ABORT", "SWAPDB 0 1", "MODULE LIST", "OBJECT ENCODING k",
			}},
			{"Scripting", []string{
				"EVAL return_1 0", "EVALSHA abc 0", "SCRIPT FLUSH",
				"FUNCTION LIST", "FCALL fn 0",
			}},
			{"Transactions", []string{"MULTI", "EXEC", "DISCARD", "WATCH k", "UNWATCH"}},
			{"Connection", []string{
				"AUTH password", "SELECT 3", "HELLO 3", "QUIT", "RESET",
			}},
		}

		for _, group := range blocked {
			t.Run(group.group, func(t *testing.T) {
				for _, command := range group.commands {
					h.Post("/api/exec", map[string]any{"command": command}).
						ExpectError(http.StatusForbidden, "not allowed")
				}
			})
		}

		t.Run("BlockedSubcommands", func(t *testing.T) {
			// The parent command stays available; only the mutating subcommand
			// is refused, so CONFIG GET keeps working while CONFIG SET does not.
			for _, command := range []string{
				"CONFIG SET maxmemory 0", "CONFIG REWRITE", "CONFIG RESETSTAT",
				"CLIENT KILL ID 1", "CLIENT PAUSE 100",
				"ACL SETUSER bob", "ACL DELUSER bob",
			} {
				h.Post("/api/exec", map[string]any{"command": command}).
					ExpectError(http.StatusForbidden, "not allowed")
			}
			exec(t, h, "CONFIG GET maxmemory")
			exec(t, h, "CLIENT ID")
		})

		t.Run("SelectCannotEscapeTheSessionDatabase", func(t *testing.T) {
			// The whole test harness relies on each connection staying pinned to
			// its own logical database.
			h.Post("/api/exec", map[string]any{"command": "SELECT 0"}).
				ExpectError(http.StatusForbidden, "not allowed")
		})
	})
}

func TestExecReadOnlyMode(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e, testenv.ReadOnly())
		h.SeedString("ro:key", "value")
		h.SeedList("ro:list", "a", "b")
		h.SeedHash("ro:hash", map[string]string{"f": "v"})
		h.SeedZSet("ro:zset", map[string]float64{"m": 1})
		h.SeedStream("ro:stream", map[string]string{"f": "v"})

		t.Run("ReadsAreAllowed", func(t *testing.T) {
			for _, command := range []string{
				"PING", "ECHO hi", "INFO", "DBSIZE", "TIME",
				"GET ro:key", "STRLEN ro:key", "TYPE ro:key", "TTL ro:key", "EXISTS ro:key",
				"SCAN 0", "KEYS *", "RANDOMKEY",
				"LLEN ro:list", "LRANGE ro:list 0 -1", "LINDEX ro:list 0",
				"HGETALL ro:hash", "HKEYS ro:hash", "HLEN ro:hash",
				"ZCARD ro:zset", "ZRANGE ro:zset 0 -1", "ZSCORE ro:zset m",
				"XLEN ro:stream", "XRANGE ro:stream - +",
				"MEMORY USAGE ro:key", "SLOWLOG LEN", "CONFIG GET maxmemory",
			} {
				result := exec(t, h, command)
				if result.Type == "error" {
					t.Errorf("%s was allowed but failed: %v", command, result.Value)
				}
			}
		})

		t.Run("WritesAreRefused", func(t *testing.T) {
			for _, command := range []string{
				"SET ro:key new", "DEL ro:key", "APPEND ro:key x", "INCR ro:counter",
				"EXPIRE ro:key 60", "PERSIST ro:key", "RENAME ro:key ro:other",
				"LPUSH ro:list x", "RPUSH ro:list x", "LSET ro:list 0 x", "LREM ro:list 1 a",
				"SADD ro:set x", "SREM ro:set x",
				"HSET ro:hash f2 v", "HDEL ro:hash f",
				"ZADD ro:zset 1 x", "ZREM ro:zset m", "ZINCRBY ro:zset 1 m",
				"XADD ro:stream * f v", "XDEL ro:stream 1-1",
				"PFADD ro:hll x",
				"FLUSHDB", "FLUSHALL",
			} {
				h.Post("/api/exec", map[string]any{"command": command}).
					ExpectError(http.StatusForbidden, "read-only")
			}
		})

		t.Run("WriteSubcommandsAreRefused", func(t *testing.T) {
			// CONFIG and SLOWLOG are read-only commands with mutating
			// subcommands; the refinement table has to catch those.
			for _, command := range []string{"SLOWLOG RESET", "MEMORY PURGE", "CLIENT SETNAME x"} {
				h.Post("/api/exec", map[string]any{"command": command}).
					ExpectError(http.StatusForbidden, "not allowed")
			}
		})

		t.Run("NothingWasWritten", func(t *testing.T) {
			if got := h.GetString("ro:key"); got != "value" {
				t.Errorf("value = %q, want the original", got)
			}
			if size, err := h.Client.DBSize(h.Ctx()); err != nil || size != 5 {
				t.Errorf("dbSize = %d (err %v), want the 5 seeded keys", size, err)
			}
		})
	})
}

func TestExecPrefixEnforcement(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e, testenv.Prefix("app:"))
		h.SeedString("app:key", "value")
		h.SeedString("other:key", "secret")

		t.Run("InPrefixKeysAreAllowed", func(t *testing.T) {
			if result := exec(t, h, "GET app:key"); result.Value != "value" {
				t.Errorf("result = %+v, want the stored value", result)
			}
			exec(t, h, "SET app:new written")
			if got := h.GetString("app:new"); got != "written" {
				t.Errorf("value = %q, want written", got)
			}
		})

		t.Run("OutOfPrefixKeysAreRefused", func(t *testing.T) {
			for _, command := range []string{
				"GET other:key", "SET other:key x", "DEL other:key", "TYPE other:key",
				"EXPIRE other:key 60", "LRANGE other:key 0 -1",
			} {
				h.Post("/api/exec", map[string]any{"command": command}).
					ExpectError(http.StatusForbidden, "prefix")
			}
			if got := h.GetString("other:key"); got != "secret" {
				t.Errorf("out-of-prefix value changed to %q", got)
			}
		})

		t.Run("MultiKeyCommandsCheckEveryKey", func(t *testing.T) {
			exec(t, h, "MGET app:key app:new")
			h.Post("/api/exec", map[string]any{"command": "MGET app:key other:key"}).
				ExpectError(http.StatusForbidden, "prefix")
			h.Post("/api/exec", map[string]any{"command": "DEL app:key other:key"}).
				ExpectError(http.StatusForbidden, "prefix")
		})

		t.Run("RenameChecksTheDestination", func(t *testing.T) {
			h.Post("/api/exec", map[string]any{"command": "RENAME app:key other:escaped"}).
				ExpectError(http.StatusForbidden, "prefix")
			if got := h.TypeOf("other:escaped"); got != "none" {
				t.Error("the key escaped the prefix through RENAME")
			}
		})

		t.Run("KeylessCommandsAreUnaffected", func(t *testing.T) {
			for _, command := range []string{"PING", "INFO", "DBSIZE", "TIME", "CONFIG GET maxmemory"} {
				if result := exec(t, h, command); result.Type == "error" {
					t.Errorf("%s failed: %v", command, result.Value)
				}
			}
		})

		t.Run("KeylessFirstArgumentsAreTreatedAsKeys", func(t *testing.T) {
			// keyPositions falls back to "argument 1 is the key", which is right
			// for most commands but not for SCAN's cursor or ECHO's message.
			// Under a prefix those become unrunnable in the console. The effect
			// is fail-closed, but it does mean SCAN cannot be used to browse.
			h.With(t).Post("/api/exec", map[string]any{"command": "SCAN 0"}).
				ExpectError(http.StatusForbidden, "prefix")
			h.With(t).Post("/api/exec", map[string]any{"command": "ECHO hello"}).
				ExpectError(http.StatusForbidden, "prefix")

			// A cursor that happens to start with the prefix would slip through,
			// which is only reachable because the argument is not really a key.
			exec(t, h, "KEYS app:*")
		})
	})
}
