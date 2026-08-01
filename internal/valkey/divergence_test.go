package valkey_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/natrimmer/kvweb/internal/testenv"
	"github.com/natrimmer/kvweb/internal/valkey"
)

// Tests in this file pin down the places where kvweb depends on the exact shape
// of a server reply rather than on a typed API: parsed INFO fields, string
// comparisons on error text, magic byte headers, notification channel names.
// Those are the joints where Redis and Valkey can drift apart without any
// compiler or client-library complaint.

func TestDivergenceEngineIdentity(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		info, err := h.Client.Info(h.Ctx(), "server")
		if err != nil {
			t.Fatalf("Info(server): %v", err)
		}

		// Valkey reports redis_version too, for compatibility, so identity has
		// to come from the Valkey-specific fields.
		hasValkeyFields := strings.Contains(info, "valkey_version:") ||
			strings.Contains(info, "server_name:valkey")

		if e.IsValkey() && !hasValkeyFields {
			t.Error("Valkey INFO has neither valkey_version nor server_name:valkey")
		}
		if e.IsRedis() && hasValkeyFields {
			t.Error("Redis INFO reports Valkey fields; the engines may be mixed up")
		}
		if !strings.Contains(info, "redis_version:") {
			t.Error("INFO server has no redis_version field; version detection would break")
		}
	})
}

func TestDivergenceInfoParsing(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		info, err := h.Client.Info(h.Ctx(), "memory")
		if err != nil {
			t.Fatalf("Info(memory): %v", err)
		}

		// GetMemoryStats splits INFO on \r\n and on the first colon. Both parts
		// of that assumption are worth pinning.
		if !strings.Contains(info, "\r\n") {
			t.Error("INFO is not CRLF-delimited; GetMemoryStats splits on \\r\\n")
		}
		for _, field := range []string{"used_memory:", "used_memory_human:"} {
			if !strings.Contains(info, field) {
				t.Errorf("INFO memory has no %s field", field)
			}
		}

		stats, err := h.Client.GetMemoryStats(h.Ctx())
		if err != nil {
			t.Fatalf("GetMemoryStats: %v", err)
		}
		if stats.UsedMemory <= 0 || stats.UsedMemoryHuman == "" {
			t.Errorf("parsed stats = %+v, want both fields populated", stats)
		}
	})
}

func TestDivergenceNotifyKeyspaceEventsConfig(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		// notify-keyspace-events is server-global.
		h := testenv.New(t, e, testenv.Exclusive())
		ctx := h.Ctx()

		t.Run("StartsDisabled", func(t *testing.T) {
			value, err := h.Client.GetNotifyKeyspaceEvents(ctx)
			if err != nil {
				t.Fatalf("GetNotifyKeyspaceEvents: %v", err)
			}
			if value != "" {
				t.Errorf("value = %q on a fresh server, want empty", value)
			}
		})

		t.Run("SetAndReadBack", func(t *testing.T) {
			// The API writes "KEAgex" and decides "enabled" by testing the
			// read-back value against "". Servers normalise these flags, so the
			// string that comes back is not the string that went in.
			if err := h.Client.SetNotifyKeyspaceEvents(ctx, "KEAgex"); err != nil {
				t.Fatalf("SetNotifyKeyspaceEvents: %v", err)
			}
			value, err := h.Client.GetNotifyKeyspaceEvents(ctx)
			if err != nil {
				t.Fatalf("GetNotifyKeyspaceEvents: %v", err)
			}
			if value == "" {
				t.Fatal("value is empty after enabling; the API would report notifications as off")
			}
			for _, flag := range []string{"K", "E"} {
				if !strings.Contains(value, flag) {
					t.Errorf("normalised value %q is missing the %s flag", value, flag)
				}
			}
			t.Logf("%s normalised KEAgex to %q", e.Name, value)
		})

		t.Run("Disable", func(t *testing.T) {
			if err := h.Client.SetNotifyKeyspaceEvents(ctx, ""); err != nil {
				t.Fatalf("SetNotifyKeyspaceEvents(\"\"): %v", err)
			}
			value, err := h.Client.GetNotifyKeyspaceEvents(ctx)
			if err != nil {
				t.Fatalf("GetNotifyKeyspaceEvents: %v", err)
			}
			if value != "" {
				t.Errorf("value = %q after disabling, want empty", value)
			}
		})
	})
}

func TestDivergenceKeyspaceNotifications(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e, testenv.Exclusive())
		ctx := h.Ctx()

		if err := h.Client.SetNotifyKeyspaceEvents(ctx, "KEAgex"); err != nil {
			t.Fatalf("SetNotifyKeyspaceEvents: %v", err)
		}

		events, err := h.Client.SubscribeKeyspace(ctx, h.DB)
		if err != nil {
			t.Fatalf("SubscribeKeyspace: %v", err)
		}
		// PSUBSCRIBE is asynchronous; give it a moment to register before
		// generating traffic, otherwise the first events are missed.
		time.Sleep(250 * time.Millisecond)

		t.Run("SetProducesAKeyEvent", func(t *testing.T) {
			h.SeedString("notify:key", "value")
			event := awaitEvent(t, events, func(ev valkey.KeyEvent) bool {
				return ev.Key == "notify:key" && ev.Operation == "set"
			})
			// The channel name carries the database number; subscribe.go strips
			// the "__keyspace@N__:" prefix to recover the key.
			if event.Key != "notify:key" {
				t.Errorf("key = %q, want %q; the channel prefix may not have been stripped", event.Key, "notify:key")
			}
		})

		t.Run("DeleteProducesADelEvent", func(t *testing.T) {
			h.SeedString("notify:doomed", "value")
			if _, err := h.Client.Del(ctx, "notify:doomed"); err != nil {
				t.Fatalf("Del: %v", err)
			}
			awaitEvent(t, events, func(ev valkey.KeyEvent) bool {
				return ev.Key == "notify:doomed" && ev.Operation == "del"
			})
		})

		t.Run("ExpiryProducesAnExpiredEvent", func(t *testing.T) {
			h.SeedString("notify:ephemeral", "value")
			if _, err := h.Client.Expire(ctx, "notify:ephemeral", time.Second); err != nil {
				t.Fatalf("Expire: %v", err)
			}
			awaitEvent(t, events, func(ev valkey.KeyEvent) bool {
				return ev.Key == "notify:ephemeral" && ev.Operation == "expire"
			})

			// Expired keys are only reported once something touches them or the
			// active expiry cycle catches up, so poll rather than wait blindly.
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := h.Client.Type(ctx, "notify:ephemeral"); err != nil {
					t.Fatalf("Type: %v", err)
				}
				select {
				case ev := <-events:
					if ev.Key == "notify:ephemeral" && ev.Operation == "expired" {
						return
					}
				case <-time.After(200 * time.Millisecond):
				}
			}
			t.Error("never saw an 'expired' event for a key whose TTL elapsed")
		})

		t.Run("HyperLogLogWritesAreReported", func(t *testing.T) {
			// PFADD has no dedicated notification class, which is why the API
			// enables the catch-all "A" flag. Without it, live updates would
			// silently miss HyperLogLog edits.
			h.SeedHLL("notify:hll", "element")
			awaitEvent(t, events, func(ev valkey.KeyEvent) bool {
				return ev.Key == "notify:hll"
			})
		})
	})
}

// awaitEvent drains the channel until match returns true or the wait times out.
func awaitEvent(t *testing.T, events <-chan valkey.KeyEvent, match func(valkey.KeyEvent) bool) valkey.KeyEvent {
	t.Helper()
	deadline := time.After(10 * time.Second)
	var seen []string
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("event channel closed; saw %v", seen)
			}
			seen = append(seen, ev.Operation+" "+ev.Key)
			if match(ev) {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for a matching keyspace event; saw %v", seen)
		}
	}
}

func TestDivergenceHyperLogLogHeader(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		h.SeedHLL("hll:key", "a", "b", "c")

		// The API has no other way to tell a HyperLogLog from a string, so this
		// magic header is load-bearing for type detection in the key list.
		raw := h.GetString("hll:key")
		if len(raw) < 4 || raw[:4] != "HYLL" {
			t.Fatalf("HyperLogLog value does not start with HYLL: % x", raw[:min(8, len(raw))])
		}
		if got := h.TypeOf("hll:key"); got != "string" {
			t.Errorf("TYPE = %q, want %q", got, "string")
		}

		// A plain string must not be mistaken for one.
		h.SeedString("plain:key", "HYL")
		if raw := h.GetString("plain:key"); len(raw) >= 4 && raw[:4] == "HYLL" {
			t.Error("a short plain string was misdetected as a HyperLogLog")
		}
	})
}

func TestDivergenceTypeOfMissingKey(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		// The API returns 404 based on this exact string.
		if got := h.TypeOf("definitely:missing"); got != "none" {
			t.Errorf("TYPE of a missing key = %q, want %q", got, "none")
		}
	})
}

func TestDivergenceCompressedValuesSurviveTheServer(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		original := strings.Repeat("kvweb compresses large string values. ", 200)

		for _, encoding := range []string{"gzip", "zstd"} {
			t.Run(encoding, func(t *testing.T) {
				compressed, err := valkey.Compress(original, encoding)
				if err != nil {
					t.Fatalf("Compress: %v", err)
				}

				h.SeedString("compressed:"+encoding, compressed)
				stored := h.GetString("compressed:" + encoding)

				if stored != compressed {
					t.Fatalf("compressed bytes changed in transit (%d bytes stored, %d retrieved)",
						len(compressed), len(stored))
				}
				if got := valkey.DetectEncoding(stored); got != encoding {
					t.Errorf("DetectEncoding after the round trip = %q, want %q", got, encoding)
				}

				decompressed, err := valkey.Decompress(stored, encoding)
				if err != nil {
					t.Fatalf("Decompress: %v", err)
				}
				if decompressed != original {
					t.Error("value did not survive compress -> store -> fetch -> decompress")
				}
			})
		}
	})
}

func TestDivergenceExecReplyShapes(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		// The console renders whatever Exec returns by switching on Go types, so
		// the mapping from RESP replies to Go types is part of the contract.
		h.SeedString("exec:string", "value")
		h.SeedList("exec:list", "a", "b")

		cases := []struct {
			name string
			args []string
			want string // Go type name expected from Exec
		}{
			{"SimpleString", []string{"PING"}, "string"},
			{"BulkString", []string{"GET", "exec:string"}, "string"},
			{"Integer", []string{"STRLEN", "exec:string"}, "int64"},
			{"Array", []string{"LRANGE", "exec:list", "0", "-1"}, "[]interface {}"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := h.Client.Exec(ctx, tc.args)
				if err != nil {
					t.Fatalf("Exec %v: %v", tc.args, err)
				}
				if got := fmt.Sprintf("%T", result); got != tc.want {
					t.Errorf("Exec %v returned %s, want %s", tc.args, got, tc.want)
				}
			})
		}

		t.Run("NilReply", func(t *testing.T) {
			// A missing key produces an error rather than a nil value, which is
			// why the console renders errors instead of a nil result here.
			if _, err := h.Client.Exec(ctx, []string{"GET", "exec:missing"}); err == nil {
				t.Error("GET on a missing key returned no error")
			}
		})

		t.Run("CommandErrorsComeBackAsErrors", func(t *testing.T) {
			// valkey-go strips the leading "ERR " code, so the console renders
			// the bare message. Both engines word this one identically.
			_, err := h.Client.Exec(ctx, []string{"INCR", "exec:string"})
			if err == nil {
				t.Fatal("INCR on a non-numeric string returned no error")
			}
			if err.Error() != "value is not an integer or out of range" {
				t.Errorf("error = %q, want %q", err, "value is not an integer or out of range")
			}
		})
	})
}

func TestDivergenceSlowLogShape(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e, testenv.Exclusive("--slowlog-log-slower-than", "0"))

		h.SeedString("slowlog:key", "value")

		entries, err := h.Client.SlowLogGet(h.Ctx(), 5)
		if err != nil {
			t.Fatalf("SlowLogGet: %v", err)
		}
		if len(entries) == 0 {
			t.Fatal("slow log is empty")
		}

		// The client indexes fields 0-5 positionally. Redis added the client
		// address and name in 4.0; if either engine dropped or reordered them,
		// the parse would silently yield blank columns in the UI.
		entry := entries[0]
		if entry.Timestamp <= 0 {
			t.Errorf("timestamp = %d, want a unix time", entry.Timestamp)
		}
		if len(entry.Args) == 0 {
			t.Error("args are empty; field 3 is not the command array")
		}
		if entry.ClientAddr == "" {
			t.Error("client address is empty; field 4 is missing")
		}
		if !strings.Contains(entry.ClientAddr, ":") {
			t.Errorf("client address %q is not host:port", entry.ClientAddr)
		}
	})
}

func TestDivergenceGeoPositionNils(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		h.SeedGeo("geo:key", "known", 13.361389, 38.115556)

		// GEOPOS returns a nil array element for unknown members. The client
		// detects that with IsNil; a change to an empty array would produce
		// bogus (0,0) pins on the map instead of dropping the member.
		positions, err := h.Client.GeoPos(h.Ctx(), "geo:key", "unknown", "known")
		if err != nil {
			t.Fatalf("GeoPos: %v", err)
		}
		if len(positions) != 2 {
			t.Fatalf("GeoPos returned %d positions for 2 members", len(positions))
		}
		if positions[0] != nil {
			t.Errorf("unknown member returned %+v, want nil", *positions[0])
		}
		if positions[1] == nil {
			t.Error("known member returned nil")
		}
	})
}

func TestDivergenceIncrByFloatFormatting(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		// INCRBYFLOAT replies with a decimal string that the client parses to a
		// float and reformats with %g. Trailing-zero handling differs between
		// the raw reply and the formatted value, and the UI shows the latter.
		cases := []struct {
			start  string
			amount float64
			want   string
		}{
			{"10", 2.5, "12.5"},
			{"10", 0.1, "10.1"},
			{"0", -1.5, "-1.5"},
			{"1.5", 1.5, "3"},
			{"100", 0, "100"},
		}

		for i, tc := range cases {
			key := fmt.Sprintf("incr:%d", i)
			h.SeedString(key, tc.start)
			got, err := h.Client.IncrByFloat(ctx, key, tc.amount)
			if err != nil {
				t.Fatalf("IncrByFloat(%s, %v): %v", tc.start, tc.amount, err)
			}
			if got != tc.want {
				t.Errorf("IncrByFloat(%s, %v) = %q, want %q", tc.start, tc.amount, got, tc.want)
			}
		}
	})
}
