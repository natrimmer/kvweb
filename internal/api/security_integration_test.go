package api_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/natrimmer/kvweb/internal/testenv"
)

// mutation is one write route, described well enough to fire at the API without
// knowing anything about the key it targets.
type mutation struct {
	name   string
	method string
	path   func(h *testenv.Harness, key string) string
	body   any
}

// mutations covers every route that changes data. Both the read-only and prefix
// tests walk this list, so a new write endpoint that forgets its guard shows up
// as a failure the moment it is added here.
var mutations = []mutation{
	{"SetKey", http.MethodPut, func(h *testenv.Harness, k string) string { return h.KeyPath(k) },
		map[string]any{"value": "written"}},
	{"DeleteKey", http.MethodDelete, func(h *testenv.Harness, k string) string { return h.KeyPath(k) }, nil},
	{"Increment", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/incr" },
		map[string]any{"amount": 1}},
	{"Expire", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/expire" },
		map[string]any{"ttl": 60}},
	{"Rename", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/rename" },
		map[string]any{"newKey": "renamed"}},
	{"ListAdd", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/list" },
		map[string]any{"value": "x"}},
	{"ListSet", http.MethodPut, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/list/0" },
		map[string]any{"value": "x"}},
	{"ListRemove", http.MethodDelete, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/list/0" }, nil},
	{"SetAdd", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/set" },
		map[string]any{"member": "x"}},
	{"SetRemove", http.MethodDelete, func(h *testenv.Harness, k string) string { return h.KeyPath(k, "set", "m") }, nil},
	{"SetRename", http.MethodPatch, func(h *testenv.Harness, k string) string { return h.KeyPath(k, "set", "m") },
		map[string]any{"newMember": "x"}},
	{"HashSet", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/hash" },
		map[string]any{"field": "f", "value": "v"}},
	{"HashRemove", http.MethodDelete, func(h *testenv.Harness, k string) string { return h.KeyPath(k, "hash", "f") }, nil},
	{"HashRename", http.MethodPatch, func(h *testenv.Harness, k string) string { return h.KeyPath(k, "hash", "f") },
		map[string]any{"newField": "g"}},
	{"ZSetAdd", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/zset" },
		map[string]any{"member": "m", "score": 1}},
	{"ZSetRemove", http.MethodDelete, func(h *testenv.Harness, k string) string { return h.KeyPath(k, "zset", "m") }, nil},
	{"ZSetRename", http.MethodPatch, func(h *testenv.Harness, k string) string { return h.KeyPath(k, "zset", "m") },
		map[string]any{"newMember": "x"}},
	{"ZSetIncrScore", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k, "zset", "m", "incr") },
		map[string]any{"amount": 1}},
	{"GeoAdd", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/geo" },
		map[string]any{"member": "m", "longitude": 1.0, "latitude": 1.0}},
	{"StreamAdd", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/stream" },
		map[string]any{"fields": map[string]string{"f": "v"}}},
	{"StreamRemove", http.MethodDelete, func(h *testenv.Harness, k string) string { return h.KeyPath(k, "stream", "1-1") }, nil},
	{"HLLAdd", http.MethodPost, func(h *testenv.Harness, k string) string { return h.KeyPath(k) + "/hll" },
		map[string]any{"element": "x"}},
}

func TestReadOnlyMode(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e, testenv.ReadOnly())

		// Seed through the client, which is not subject to the API's guards.
		h.SeedString("ro:string", "original")
		h.SeedList("ro:list", "a", "b")
		h.SeedSet("ro:set", "m")
		h.SeedHash("ro:hash", map[string]string{"f": "v"})
		h.SeedZSet("ro:zset", map[string]float64{"m": 1})
		before := h.AllKeys()
		slices.Sort(before)

		t.Run("ConfigAdvertisesReadOnly", func(t *testing.T) {
			if body := h.Get("/api/config").ExpectOK().Map(); body["readOnly"] != true {
				t.Errorf("readOnly = %v, want true", body["readOnly"])
			}
		})

		for _, m := range mutations {
			t.Run("Blocked_"+m.name, func(t *testing.T) {
				h.Request(m.method, m.path(h, "ro:string"), m.body).
					ExpectError(http.StatusForbidden, "read-only")
			})
		}

		t.Run("Blocked_DeleteKeys", func(t *testing.T) {
			h.Post("/api/keys/delete", map[string]any{"keys": []string{"ro:string"}}).
				ExpectError(http.StatusForbidden, "read-only")
		})

		t.Run("Blocked_Flush", func(t *testing.T) {
			h.Post("/api/flush", nil).ExpectError(http.StatusForbidden, "read-only")
		})

		t.Run("Blocked_SetNotifications", func(t *testing.T) {
			h.Post("/api/notifications", map[string]any{"enabled": true}).
				ExpectError(http.StatusForbidden, "read-only")
		})

		t.Run("ReadsStillWork", func(t *testing.T) {
			h.Get("/api/health").ExpectOK()
			h.Get("/api/info").ExpectOK()
			h.Get("/api/keys").ExpectOK()
			h.Get("/api/keys?meta=1").ExpectOK()
			h.Get("/api/prefixes").ExpectOK()
			h.Get("/api/slowlog").ExpectOK()
			h.Get("/api/notifications").ExpectOK()
			h.Get(h.KeyPath("ro:string")).ExpectOK()
			h.Get(h.KeyPath("ro:zset") + "/geo").ExpectOK()
			h.Post("/api/keys/memory", map[string]any{"keys": []string{"ro:string"}}).ExpectOK()
		})

		t.Run("NothingWasModified", func(t *testing.T) {
			after := h.AllKeys()
			slices.Sort(after)
			if !slices.Equal(before, after) {
				t.Errorf("keys changed under read-only mode: %v -> %v", before, after)
			}
			if got := h.GetString("ro:string"); got != "original" {
				t.Errorf("value = %q, want the original", got)
			}
			if items := h.ListItems("ro:list"); !slices.Equal(items, []string{"a", "b"}) {
				t.Errorf("list = %v, want [a b]", items)
			}
		})
	})
}

func TestPrefixRestriction(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		const allowed = "app:"
		h := testenv.New(t, e, testenv.Prefix(allowed))

		h.SeedString("app:mine", "in-prefix")
		h.SeedString("app:other", "in-prefix")
		h.SeedString("secret:theirs", "out-of-prefix")
		h.SeedList("secret:list", "a")

		t.Run("ConfigAdvertisesThePrefix", func(t *testing.T) {
			if body := h.Get("/api/config").ExpectOK().Map(); body["prefix"] != allowed {
				t.Errorf("prefix = %v, want %q", body["prefix"], allowed)
			}
		})

		for _, m := range mutations {
			t.Run("Blocked_"+m.name, func(t *testing.T) {
				h.Request(m.method, m.path(h, "secret:theirs"), m.body).
					ExpectError(http.StatusForbidden, "prefix")
			})
		}

		t.Run("Blocked_GetKey", func(t *testing.T) {
			h.Get(h.KeyPath("secret:theirs")).ExpectError(http.StatusForbidden, "prefix")
		})

		t.Run("Blocked_GeoGet", func(t *testing.T) {
			h.Get(h.KeyPath("secret:theirs")+"/geo").ExpectError(http.StatusForbidden, "prefix")
		})

		t.Run("Blocked_DeleteKeysIfAnyKeyIsOutside", func(t *testing.T) {
			h.Post("/api/keys/delete", map[string]any{
				"keys": []string{"app:mine", "secret:theirs"},
			}).ExpectError(http.StatusForbidden, "prefix")

			// The whole request is refused, so the in-prefix key survives too.
			if got := h.TypeOf("app:mine"); got != "string" {
				t.Error("an in-prefix key was deleted by a partially forbidden request")
			}
		})

		t.Run("Blocked_KeysMemoryIfAnyKeyIsOutside", func(t *testing.T) {
			h.Post("/api/keys/memory", map[string]any{
				"keys": []string{"app:mine", "secret:theirs"},
			}).ExpectError(http.StatusForbidden, "prefix")
		})

		t.Run("Blocked_RenameOutOfPrefix", func(t *testing.T) {
			// The destination is checked as well; otherwise a key could be
			// walked out of the sandbox one rename at a time.
			h.Post(h.KeyPath("app:mine")+"/rename", map[string]any{"newKey": "secret:escaped"}).
				ExpectError(http.StatusForbidden, "prefix")
			if got := h.TypeOf("secret:escaped"); got != "none" {
				t.Error("the key escaped the prefix")
			}
		})

		t.Run("AllowedKeysStillWork", func(t *testing.T) {
			h.Get(h.KeyPath("app:mine")).ExpectOK()
			h.Put(h.KeyPath("app:new"), map[string]any{"value": "v"}).ExpectOK()
			h.Post(h.KeyPath("app:mine")+"/rename", map[string]any{"newKey": "app:renamed"}).ExpectOK()
			h.Delete(h.KeyPath("app:renamed")).ExpectOK()
		})

		t.Run("KeyListingIsScoped", func(t *testing.T) {
			var body struct {
				Keys []string `json:"keys"`
			}
			h.Get("/api/keys").ExpectOK().Decode(&body)
			for _, k := range body.Keys {
				if !strings.HasPrefix(k, allowed) {
					t.Errorf("key listing leaked %q", k)
				}
			}
			if len(body.Keys) == 0 {
				t.Error("key listing returned nothing; the prefix scoping is too aggressive")
			}
		})

		t.Run("PatternsAreScoped", func(t *testing.T) {
			var body struct {
				Keys []string `json:"keys"`
			}
			// A caller-supplied pattern is prepended with the prefix, so it can
			// only ever narrow the search, never widen it.
			h.Get("/api/keys?pattern=*").ExpectOK().Decode(&body)
			for _, k := range body.Keys {
				if !strings.HasPrefix(k, allowed) {
					t.Errorf("pattern=* leaked %q", k)
				}
			}

			h.Get("/api/keys?pattern=secret:*").ExpectOK().Decode(&body)
			if len(body.Keys) != 0 {
				t.Errorf("pattern=secret:* returned %v, want nothing", body.Keys)
			}
		})

		t.Run("RegexSearchIsScoped", func(t *testing.T) {
			var body struct {
				Keys []string `json:"keys"`
			}
			// Regex mode scans with the prefix wildcard and filters afterwards,
			// so a regex that matches outside the prefix still sees nothing.
			h.Get("/api/keys?regex=1&pattern=.*").ExpectOK().Decode(&body)
			for _, k := range body.Keys {
				if !strings.HasPrefix(k, allowed) {
					t.Errorf("regex search leaked %q", k)
				}
			}

			h.Get("/api/keys?regex=1&pattern=^secret").ExpectOK().Decode(&body)
			if len(body.Keys) != 0 {
				t.Errorf("regex ^secret returned %v, want nothing", body.Keys)
			}
		})

		t.Run("PrefixTreeIsScoped", func(t *testing.T) {
			var body struct {
				Entries []prefixEntry `json:"entries"`
			}
			h.Get("/api/prefixes").ExpectOK().Decode(&body)
			for _, entry := range body.Entries {
				if !strings.HasPrefix(entry.Prefix, allowed) {
					t.Errorf("prefix tree leaked %q", entry.Prefix)
				}
			}
		})

		t.Run("FlushIsNotScopedToThePrefix", func(t *testing.T) {
			// Documented gap, not an endorsement: --prefix restricts which keys
			// the API will read or write, but FLUSHDB is a whole-database
			// command and clears out-of-prefix keys too. Deployments that rely
			// on --prefix as a boundary should also pass --disable-flush.
			flushed := testenv.New(t, e, testenv.Prefix(allowed))
			flushed.SeedString("app:inside", "v")
			flushed.SeedString("outside:key", "v")

			flushed.Post("/api/flush", nil).ExpectOK()

			if keys := flushed.AllKeys(); len(keys) != 0 {
				t.Errorf("keys remaining = %v, want the flush to have cleared everything", keys)
			}
		})

		t.Run("DisableFlushClosesThatGap", func(t *testing.T) {
			guarded := testenv.New(t, e, testenv.Prefix(allowed), testenv.DisableFlush())
			guarded.SeedString("outside:key", "v")
			guarded.Post("/api/flush", nil).ExpectError(http.StatusForbidden, "disabled")
			if got := guarded.TypeOf("outside:key"); got != "string" {
				t.Error("the out-of-prefix key was flushed despite --disable-flush")
			}
		})
	})
}

func TestDisableFlush(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e, testenv.DisableFlush())
		h.SeedKeys("keep:", 3)

		t.Run("ConfigAdvertisesIt", func(t *testing.T) {
			if body := h.Get("/api/config").ExpectOK().Map(); body["disableFlush"] != true {
				t.Errorf("disableFlush = %v, want true", body["disableFlush"])
			}
		})

		t.Run("FlushEndpointIsBlocked", func(t *testing.T) {
			h.Post("/api/flush", nil).ExpectError(http.StatusForbidden, "disabled")
		})

		t.Run("ConsoleFlushIsBlocked", func(t *testing.T) {
			for _, command := range []string{"FLUSHDB", "FLUSHALL", "flushdb", "flushall ASYNC"} {
				h.Post("/api/exec", map[string]any{"command": command}).
					ExpectError(http.StatusForbidden, "disabled")
			}
		})

		t.Run("DataSurvives", func(t *testing.T) {
			if keys := h.AllKeys(); len(keys) != 3 {
				t.Errorf("keys = %v, want the 3 seeded keys", keys)
			}
		})

		t.Run("OtherWritesStillWork", func(t *testing.T) {
			h.Put(h.KeyPath("keep:new"), map[string]any{"value": "v"}).ExpectOK()
		})
	})
}

func TestMaxKeysLimit(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e, testenv.MaxKeys(10))
		h.SeedKeys("limited:", 500)

		t.Run("ScanCountIsClamped", func(t *testing.T) {
			var body struct {
				Keys   []string `json:"keys"`
				Cursor uint64   `json:"cursor"`
			}
			// COUNT is only a hint, so the guarantee is "far fewer than all of
			// them, and a cursor to continue from" rather than an exact number.
			h.Get("/api/keys?count=1000").ExpectOK().Decode(&body)
			if len(body.Keys) >= 500 {
				t.Errorf("got %d keys, want the count clamped well below 500", len(body.Keys))
			}
			if body.Cursor == 0 {
				t.Error("cursor = 0, so the client cannot page past the limit")
			}
		})

		t.Run("SmallerRequestsAreUntouched", func(t *testing.T) {
			var body struct {
				Keys []string `json:"keys"`
			}
			h.Get("/api/keys?count=5").ExpectOK().Decode(&body)
			if len(body.Keys) > 100 {
				t.Errorf("count=5 returned %d keys", len(body.Keys))
			}
		})

		t.Run("PrefixTreeIsBounded", func(t *testing.T) {
			var body struct {
				Entries []prefixEntry `json:"entries"`
			}
			h.Get("/api/prefixes").ExpectOK().Decode(&body)
			if len(body.Entries) == 0 {
				t.Error("prefix tree is empty")
			}
		})
	})
}
