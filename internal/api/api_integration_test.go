package api_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/natrimmer/kvweb/internal/testenv"
	"github.com/natrimmer/kvweb/internal/valkey"
)

func TestMetaEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		t.Run("Health", func(t *testing.T) {
			body := h.Get("/api/health").ExpectOK().Map()
			if body["status"] != "ok" {
				t.Errorf("status = %v, want ok", body["status"])
			}
			if body["database"] != true {
				t.Errorf("database = %v, want true", body["database"])
			}
			if ts, ok := body["timestamp"].(float64); !ok || ts <= 0 {
				t.Errorf("timestamp = %v, want a unix time", body["timestamp"])
			}
		})

		t.Run("Config", func(t *testing.T) {
			body := h.Get("/api/config").ExpectOK().Map()
			for _, field := range []string{"readOnly", "prefix", "disableFlush", "version", "commit", "dirty"} {
				if _, ok := body[field]; !ok {
					t.Errorf("config response is missing %q", field)
				}
			}
			if body["readOnly"] != false {
				t.Errorf("readOnly = %v, want false", body["readOnly"])
			}
			if body["version"] != "test" {
				t.Errorf("version = %v, want the configured build version", body["version"])
			}
		})

		t.Run("Info", func(t *testing.T) {
			h.SeedKeys("info:", 3)
			body := h.Get("/api/info").ExpectOK().Map()

			info, ok := body["info"].(string)
			if !ok || !strings.Contains(info, "redis_version:") {
				t.Errorf("info payload does not look like an INFO dump: %.80s", info)
			}
			if size, ok := body["dbSize"].(float64); !ok || int(size) != 3 {
				t.Errorf("dbSize = %v, want 3", body["dbSize"])
			}
		})

		t.Run("InfoSection", func(t *testing.T) {
			body := h.Get("/api/info?section=memory").ExpectOK().Map()
			info, _ := body["info"].(string)
			if !strings.Contains(info, "used_memory:") {
				t.Error("section=memory did not return the Memory section")
			}
			if strings.Contains(info, "# Server") {
				t.Error("section=memory returned other sections too")
			}
		})

		t.Run("SlowLog", func(t *testing.T) {
			body := h.Get("/api/slowlog").ExpectOK().Map()
			if _, ok := body["entries"]; !ok {
				t.Error("response is missing entries")
			}
			if _, ok := body["length"].(float64); !ok {
				t.Errorf("length = %v, want a number", body["length"])
			}
		})

		t.Run("SlowLogCountIsClamped", func(t *testing.T) {
			// Counts above 1000 are capped, and unparseable counts fall back to
			// the default rather than erroring.
			h.Get("/api/slowlog?count=999999").ExpectOK()
			h.Get("/api/slowlog?count=not-a-number").ExpectOK()
			h.Get("/api/slowlog?count=-5").ExpectOK()
		})
	})
}

func TestKeysEndpoint(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		types := h.SeedOneOfEachType("browse")
		h.SeedString("other:thing", "v")

		t.Run("ListsEveryKey", func(t *testing.T) {
			var body struct {
				Keys   []string `json:"keys"`
				Cursor uint64   `json:"cursor"`
			}
			h.Get("/api/keys").ExpectOK().Decode(&body)
			if len(body.Keys) != len(types)+1 {
				t.Errorf("got %d keys (%v), want %d", len(body.Keys), body.Keys, len(types)+1)
			}
		})

		t.Run("GlobPattern", func(t *testing.T) {
			var body struct {
				Keys []string `json:"keys"`
			}
			h.Get("/api/keys?pattern=browse:*").ExpectOK().Decode(&body)
			if len(body.Keys) != len(types) {
				t.Errorf("got %d keys, want %d", len(body.Keys), len(types))
			}
			for _, k := range body.Keys {
				if !strings.HasPrefix(k, "browse:") {
					t.Errorf("pattern browse:* matched %q", k)
				}
			}
		})

		t.Run("RegexPattern", func(t *testing.T) {
			var body struct {
				Keys []string `json:"keys"`
			}
			h.Get("/api/keys?regex=1&pattern=" + "browse:(list|set)$").ExpectOK().Decode(&body)
			slices.Sort(body.Keys)
			if !slices.Equal(body.Keys, []string{"browse:list", "browse:set"}) {
				t.Errorf("regex matched %v, want [browse:list browse:set]", body.Keys)
			}
		})

		t.Run("InvalidRegex", func(t *testing.T) {
			h.Get("/api/keys?regex=1&pattern=%5B").ExpectError(http.StatusBadRequest, "Invalid regex")
		})

		t.Run("InvalidCursor", func(t *testing.T) {
			h.Get("/api/keys?cursor=abc").ExpectError(http.StatusBadRequest, "invalid cursor")
		})

		t.Run("InvalidCount", func(t *testing.T) {
			h.Get("/api/keys?count=abc").ExpectError(http.StatusBadRequest, "invalid count")
		})

		t.Run("Metadata", func(t *testing.T) {
			var body struct {
				Keys []keyMeta `json:"keys"`
			}
			h.Get("/api/keys?meta=1&pattern=browse:*").ExpectOK().Decode(&body)
			if len(body.Keys) != len(types) {
				t.Fatalf("got %d keys, want %d", len(body.Keys), len(types))
			}

			byKey := map[string]keyMeta{}
			for _, m := range body.Keys {
				byKey[m.Key] = m
			}
			for typeName, key := range types {
				meta, ok := byKey[key]
				if !ok {
					t.Errorf("metadata is missing %q", key)
					continue
				}
				if meta.Type != typeName {
					t.Errorf("%q has type %q, want %q", key, meta.Type, typeName)
				}
				if meta.TTL != -1 {
					t.Errorf("%q has ttl %d, want -1", key, meta.TTL)
				}
			}
		})

		t.Run("MetadataReportsTTL", func(t *testing.T) {
			h.SeedStringTTL("browse:ttl", "v", 5*time.Minute)
			var body struct {
				Keys []keyMeta `json:"keys"`
			}
			h.Get("/api/keys?meta=1&pattern=browse:ttl").ExpectOK().Decode(&body)
			if len(body.Keys) != 1 {
				t.Fatalf("got %d keys, want 1", len(body.Keys))
			}
			if body.Keys[0].TTL <= 0 {
				t.Errorf("ttl = %d, want a positive value", body.Keys[0].TTL)
			}
		})

		for typeName, key := range types {
			t.Run("TypeFilter_"+typeName, func(t *testing.T) {
				var body struct {
					Keys []string `json:"keys"`
				}
				h.Get("/api/keys?type=" + typeName).ExpectOK().Decode(&body)
				if !slices.Contains(body.Keys, key) {
					t.Errorf("type=%s returned %v, want it to contain %q", typeName, body.Keys, key)
				}
				// hyperloglog keys are stored as strings; the filter must not
				// leak them into the string bucket or vice versa.
				for _, k := range body.Keys {
					if typeName == "string" && k == types["hyperloglog"] {
						t.Error("type=string returned the HyperLogLog key")
					}
					if typeName == "hyperloglog" && k == types["string"] {
						t.Error("type=hyperloglog returned the plain string key")
					}
				}
			})
		}

		t.Run("CountIsHonouredAsAPageSize", func(t *testing.T) {
			h.SeedKeys("page:", 200)
			var body struct {
				Keys   []string `json:"keys"`
				Cursor uint64   `json:"cursor"`
			}
			h.Get("/api/keys?count=10").ExpectOK().Decode(&body)
			if body.Cursor == 0 {
				t.Error("cursor = 0 on a partial scan; the client would stop paging early")
			}
			if len(body.Keys) > 100 {
				t.Errorf("count=10 returned %d keys, far past the requested page size", len(body.Keys))
			}
		})
	})
}

func TestPrefixesEndpoint(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		h.SeedString("app:users:1", "a")
		h.SeedString("app:users:2", "b")
		h.SeedString("app:sessions:x", "c")
		h.SeedList("standalone", "item")

		t.Run("TopLevel", func(t *testing.T) {
			var body struct {
				Entries []prefixEntry `json:"entries"`
				Prefix  string        `json:"prefix"`
			}
			h.Get("/api/prefixes").ExpectOK().Decode(&body)

			byPrefix := map[string]prefixEntry{}
			for _, entry := range body.Entries {
				byPrefix[entry.Prefix] = entry
			}

			group, ok := byPrefix["app:"]
			if !ok {
				t.Fatalf("no app: group in %v", body.Entries)
			}
			if group.IsLeaf {
				t.Error("app: is marked as a leaf")
			}
			if group.Count != 3 {
				t.Errorf("app: count = %d, want 3", group.Count)
			}

			leaf, ok := byPrefix["standalone"]
			if !ok {
				t.Fatalf("no standalone leaf in %v", body.Entries)
			}
			if !leaf.IsLeaf {
				t.Error("standalone is not marked as a leaf")
			}
			if leaf.FullKey != "standalone" || leaf.Type != "list" {
				t.Errorf("standalone leaf = %+v, want fullKey=standalone type=list", leaf)
			}
		})

		t.Run("Nested", func(t *testing.T) {
			var body struct {
				Entries []prefixEntry `json:"entries"`
				Prefix  string        `json:"prefix"`
			}
			h.Get("/api/prefixes?prefix=app:").ExpectOK().Decode(&body)
			if body.Prefix != "app:" {
				t.Errorf("echoed prefix = %q, want app:", body.Prefix)
			}

			var prefixes []string
			for _, entry := range body.Entries {
				prefixes = append(prefixes, entry.Prefix)
			}
			slices.Sort(prefixes)
			if !slices.Equal(prefixes, []string{"app:sessions:", "app:users:"}) {
				t.Errorf("entries = %v, want [app:sessions: app:users:]", prefixes)
			}
		})

		t.Run("Leaves", func(t *testing.T) {
			var body struct {
				Entries []prefixEntry `json:"entries"`
			}
			h.Get("/api/prefixes?prefix=app:users:").ExpectOK().Decode(&body)
			if len(body.Entries) != 2 {
				t.Fatalf("got %d entries, want 2", len(body.Entries))
			}
			for _, entry := range body.Entries {
				if !entry.IsLeaf {
					t.Errorf("%q is not a leaf", entry.Prefix)
				}
				if entry.Type != "string" {
					t.Errorf("%q has type %q, want string", entry.Prefix, entry.Type)
				}
			}
		})

		t.Run("CustomDelimiter", func(t *testing.T) {
			h.SeedString("dot.separated.key", "v")
			var body struct {
				Entries []prefixEntry `json:"entries"`
			}
			h.Get("/api/prefixes?delimiter=." + "&prefix=dot.").ExpectOK().Decode(&body)
			found := false
			for _, entry := range body.Entries {
				if entry.Prefix == "dot.separated." {
					found = true
				}
			}
			if !found {
				t.Errorf("entries = %v, want a dot.separated. group", body.Entries)
			}
		})

		t.Run("SortedByPrefix", func(t *testing.T) {
			var body struct {
				Entries []prefixEntry `json:"entries"`
			}
			h.Get("/api/prefixes").ExpectOK().Decode(&body)
			for i := 1; i < len(body.Entries); i++ {
				if body.Entries[i-1].Prefix > body.Entries[i].Prefix {
					t.Fatalf("entries are not sorted: %q came before %q",
						body.Entries[i-1].Prefix, body.Entries[i].Prefix)
				}
			}
		})
	})
}

func TestGetKeyByType(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		t.Run("NotFound", func(t *testing.T) {
			h.Get(h.KeyPath("missing")).ExpectError(http.StatusNotFound, "Key not found")
		})

		t.Run("String", func(t *testing.T) {
			h.SeedString("t:string", "hello world")
			resp := getKey(t, h, "t:string")
			if resp.Type != "string" {
				t.Errorf("type = %q, want string", resp.Type)
			}
			if got := decodeValue[string](t, resp); got != "hello world" {
				t.Errorf("value = %q, want %q", got, "hello world")
			}
			if resp.TTL != -1 {
				t.Errorf("ttl = %d, want -1", resp.TTL)
			}
			if resp.Memory <= 0 {
				t.Errorf("memory = %d, want a positive byte count", resp.Memory)
			}
		})

		t.Run("StringWithTTL", func(t *testing.T) {
			h.SeedStringTTL("t:ttl", "v", 10*time.Minute)
			resp := getKey(t, h, "t:ttl")
			if resp.TTL <= 0 || resp.TTL > 600 {
				t.Errorf("ttl = %d, want a value in (0, 600]", resp.TTL)
			}
		})

		t.Run("List", func(t *testing.T) {
			h.SeedList("t:list", "a", "b", "c")
			resp := getKey(t, h, "t:list")
			if resp.Type != "list" || resp.Length != 3 {
				t.Errorf("type = %q length = %d, want list and 3", resp.Type, resp.Length)
			}
			if got := decodeValue[[]string](t, resp); !slices.Equal(got, []string{"a", "b", "c"}) {
				t.Errorf("value = %v, want [a b c]", got)
			}
			if resp.Pagination == nil || resp.Pagination.Total != 3 || resp.Pagination.HasMore {
				t.Errorf("pagination = %+v, want total 3 and hasMore false", resp.Pagination)
			}
		})

		t.Run("Set", func(t *testing.T) {
			h.SeedSet("t:set", "x", "y", "z")
			resp := getKey(t, h, "t:set")
			if resp.Type != "set" || resp.Length != 3 {
				t.Errorf("type = %q length = %d, want set and 3", resp.Type, resp.Length)
			}
			members := decodeValue[[]string](t, resp)
			slices.Sort(members)
			if !slices.Equal(members, []string{"x", "y", "z"}) {
				t.Errorf("value = %v, want [x y z]", members)
			}
		})

		t.Run("Hash", func(t *testing.T) {
			h.SeedHash("t:hash", map[string]string{"b": "2", "a": "1"})
			resp := getKey(t, h, "t:hash")
			if resp.Type != "hash" || resp.Length != 2 {
				t.Errorf("type = %q length = %d, want hash and 2", resp.Type, resp.Length)
			}
			pairs := decodeValue[[]hashPair](t, resp)
			// The handler sorts fields so the editor renders a stable order.
			want := []hashPair{{Field: "a", Value: "1"}, {Field: "b", Value: "2"}}
			if !slices.Equal(pairs, want) {
				t.Errorf("value = %+v, want %+v", pairs, want)
			}
		})

		t.Run("ZSet", func(t *testing.T) {
			h.SeedZSet("t:zset", map[string]float64{"alice": 2.5, "bob": 1})
			resp := getKey(t, h, "t:zset")
			if resp.Type != "zset" || resp.Length != 2 {
				t.Errorf("type = %q length = %d, want zset and 2", resp.Type, resp.Length)
			}
			members := decodeValue[[]zMember](t, resp)
			want := []zMember{{Member: "bob", Score: 1}, {Member: "alice", Score: 2.5}}
			if !slices.Equal(members, want) {
				t.Errorf("value = %+v, want %+v (score order)", members, want)
			}
		})

		t.Run("Stream", func(t *testing.T) {
			ids := h.SeedStream("t:stream",
				map[string]string{"event": "created"},
				map[string]string{"event": "updated"},
			)
			resp := getKey(t, h, "t:stream")
			if resp.Type != "stream" || resp.Length != 2 {
				t.Errorf("type = %q length = %d, want stream and 2", resp.Type, resp.Length)
			}
			entries := decodeValue[[]streamEntry](t, resp)
			if len(entries) != 2 {
				t.Fatalf("got %d entries, want 2", len(entries))
			}
			if entries[0].ID != ids[0] || entries[0].Fields["event"] != "created" {
				t.Errorf("entry 0 = %+v, want id %q with event=created", entries[0], ids[0])
			}
		})

		t.Run("HyperLogLog", func(t *testing.T) {
			h.SeedHLL("t:hll", "a", "b", "c")
			resp := getKey(t, h, "t:hll")
			// The handler rewrites the reported type after sniffing the header.
			if resp.Type != "hyperloglog" {
				t.Errorf("type = %q, want hyperloglog", resp.Type)
			}
			value := decodeValue[map[string]int64](t, resp)
			if value["count"] != 3 {
				t.Errorf("count = %d, want 3", value["count"])
			}
		})

		t.Run("Geo", func(t *testing.T) {
			h.SeedGeo("t:geo", "palermo", 13.361389, 38.115556)
			h.SeedGeo("t:geo", "catania", 15.087269, 37.502669)

			var resp struct {
				Key        string      `json:"key"`
				Type       string      `json:"type"`
				Value      []geoMember `json:"value"`
				Length     int64       `json:"length"`
				Pagination *pagination `json:"pagination"`
			}
			h.Get(h.KeyPath("t:geo") + "/geo").ExpectOK().Decode(&resp)

			if resp.Type != "zset" || resp.Length != 2 {
				t.Errorf("type = %q length = %d, want zset and 2", resp.Type, resp.Length)
			}
			if len(resp.Value) != 2 {
				t.Fatalf("got %d members, want 2", len(resp.Value))
			}
			for _, m := range resp.Value {
				if m.Longitude == 0 || m.Latitude == 0 {
					t.Errorf("member %q has no coordinates: %+v", m.Member, m)
				}
			}
		})

		t.Run("PlainZSetRendersAsCoordinates", func(t *testing.T) {
			// Geo keys are ordinary sorted sets whose scores happen to be
			// geohashes, so GEOPOS decodes any score into a position. The geo
			// view therefore cannot reject a non-geo sorted set; it just shows
			// whatever the scores decode to. Pinning that here so the behaviour
			// is a decision rather than a surprise.
			h.SeedZSet("t:not-geo", map[string]float64{"plain": 1})
			var resp struct {
				Value  []geoMember `json:"value"`
				Length int64       `json:"length"`
			}
			h.Get(h.KeyPath("t:not-geo") + "/geo").ExpectOK().Decode(&resp)
			if resp.Length != 1 {
				t.Errorf("length = %d, want 1", resp.Length)
			}
			if len(resp.Value) != 1 {
				t.Fatalf("value = %+v, want the member with its decoded position", resp.Value)
			}
			if resp.Value[0].Member != "plain" {
				t.Errorf("member = %q, want plain", resp.Value[0].Member)
			}
		})

		t.Run("GeoOnAMissingKeyIsEmpty", func(t *testing.T) {
			var resp struct {
				Value  []geoMember `json:"value"`
				Length int64       `json:"length"`
			}
			h.Get(h.KeyPath("t:no-such-geo") + "/geo").ExpectOK().Decode(&resp)
			if resp.Length != 0 || len(resp.Value) != 0 {
				t.Errorf("response = %+v, want an empty result", resp)
			}
		})
	})
}

func TestGetKeyPagination(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		t.Run("ListPagesInOrder", func(t *testing.T) {
			items := make([]string, 25)
			for i := range items {
				items[i] = fmt.Sprintf("item-%02d", i)
			}
			h.SeedList("page:list", items...)

			var seen []string
			for page := 1; page <= 3; page++ {
				resp := getKey(t, h, "page:list", fmt.Sprintf("page=%d&pageSize=10", page))
				seen = append(seen, decodeValue[[]string](t, resp)...)
				wantMore := page < 3
				if resp.Pagination.HasMore != wantMore {
					t.Errorf("page %d hasMore = %v, want %v", page, resp.Pagination.HasMore, wantMore)
				}
				if resp.Pagination.Total != 25 {
					t.Errorf("page %d total = %d, want 25", page, resp.Pagination.Total)
				}
			}
			if !slices.Equal(seen, items) {
				t.Errorf("paging returned %v, want the seeded order", seen)
			}
		})

		t.Run("ZSetPagesInScoreOrder", func(t *testing.T) {
			members := map[string]float64{}
			for i := range 25 {
				members[fmt.Sprintf("m-%02d", i)] = float64(i)
			}
			h.SeedZSet("page:zset", members)

			var seen []string
			for page := 1; page <= 3; page++ {
				resp := getKey(t, h, "page:zset", fmt.Sprintf("page=%d&pageSize=10", page))
				for _, m := range decodeValue[[]zMember](t, resp) {
					seen = append(seen, m.Member)
				}
			}
			if len(seen) != 25 {
				t.Fatalf("paging returned %d members, want 25", len(seen))
			}
			if !slices.IsSorted(seen) {
				t.Errorf("members are not in score order: %v", seen)
			}
		})

		t.Run("HashPagesByCursor", func(t *testing.T) {
			fields := map[string]string{}
			for i := range 250 {
				fields[fmt.Sprintf("f-%03d", i)] = fmt.Sprintf("v-%03d", i)
			}
			h.SeedHash("page:hash", fields)

			seen := map[string]string{}
			cursor := "0"
			for range 100 {
				resp := getKey(t, h, "page:hash", "pageSize=25&cursor="+cursor)
				for _, pair := range decodeValue[[]hashPair](t, resp) {
					seen[pair.Field] = pair.Value
				}
				if resp.Pagination.Total != 250 {
					t.Fatalf("total = %d, want 250", resp.Pagination.Total)
				}
				if !resp.Pagination.HasMore {
					break
				}
				cursor = strings.Trim(string(resp.Pagination.NextCursor), `"`)
			}
			if len(seen) != 250 {
				t.Errorf("cursor paging saw %d fields, want 250", len(seen))
			}
			for field, want := range fields {
				if seen[field] != want {
					t.Errorf("field %q = %q, want %q", field, seen[field], want)
				}
			}
		})

		t.Run("SetPagesByCursor", func(t *testing.T) {
			members := make([]string, 250)
			for i := range members {
				members[i] = fmt.Sprintf("m-%03d", i)
			}
			h.SeedSet("page:set", members...)

			seen := map[string]bool{}
			cursor := "0"
			for range 100 {
				resp := getKey(t, h, "page:set", "pageSize=25&cursor="+cursor)
				for _, m := range decodeValue[[]string](t, resp) {
					seen[m] = true
				}
				if !resp.Pagination.HasMore {
					break
				}
				cursor = strings.Trim(string(resp.Pagination.NextCursor), `"`)
			}
			if len(seen) != 250 {
				t.Errorf("cursor paging saw %d members, want 250", len(seen))
			}
		})

		t.Run("StreamPagesInOrder", func(t *testing.T) {
			ids := h.SeedStreamN("page:stream", 35)

			var seen []string
			for page := 1; page <= 4; page++ {
				resp := getKey(t, h, "page:stream", fmt.Sprintf("page=%d&pageSize=10", page))
				for _, entry := range decodeValue[[]streamEntry](t, resp) {
					seen = append(seen, entry.ID)
				}
				wantMore := page < 4
				if resp.Pagination.HasMore != wantMore {
					t.Errorf("page %d hasMore = %v, want %v", page, resp.Pagination.HasMore, wantMore)
				}
			}
			if !slices.Equal(seen, ids) {
				t.Errorf("paging returned %d entries, want the %d seeded IDs in order", len(seen), len(ids))
			}
		})

		t.Run("InvalidPageParamsFallBackToDefaults", func(t *testing.T) {
			h.SeedList("page:defaults", "a", "b", "c")
			for _, query := range []string{
				"page=0", "page=-1", "page=abc",
				"pageSize=0", "pageSize=-1", "pageSize=abc", "pageSize=99999",
				"cursor=abc",
			} {
				resp := getKey(t, h, "page:defaults", query)
				if resp.Pagination == nil {
					t.Fatalf("%s: no pagination in the response", query)
				}
				if resp.Pagination.PageSize <= 0 || resp.Pagination.PageSize > 1000 {
					t.Errorf("%s: pageSize = %d, want a sane default", query, resp.Pagination.PageSize)
				}
			}
		})
	})
}

func TestStringCompression(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		original := strings.Repeat("compress me. ", 500)

		for _, encoding := range []string{"gzip", "zstd"} {
			t.Run(encoding, func(t *testing.T) {
				key := "compressed:" + encoding

				compressed, err := valkey.Compress(original, encoding)
				if err != nil {
					t.Fatalf("Compress: %v", err)
				}
				h.SeedString(key, compressed)

				resp := getKey(t, h, key)
				if resp.Encoding != encoding {
					t.Errorf("encoding = %q, want %q", resp.Encoding, encoding)
				}
				if got := decodeValue[string](t, resp); got != original {
					t.Error("GET did not return the decompressed value")
				}

				t.Run("EditPreservesCompression", func(t *testing.T) {
					// The editor sends back the plain text plus the encoding it
					// was given, and the handler re-compresses before storing.
					edited := original + " edited"
					h.Put(h.KeyPath(key), map[string]any{
						"value":    edited,
						"encoding": encoding,
					}).ExpectOK()

					stored := h.GetString(key)
					if valkey.DetectEncoding(stored) != encoding {
						t.Errorf("stored value is no longer %s-compressed", encoding)
					}
					roundTripped := getKey(t, h, key)
					if got := decodeValue[string](t, roundTripped); got != edited {
						t.Error("the edited value did not survive the round trip")
					}
				})
			})
		}

		t.Run("UnknownEncodingIsRejected", func(t *testing.T) {
			h.Put(h.KeyPath("compressed:bad"), map[string]any{
				"value":    "x",
				"encoding": "brotli",
			}).ExpectStatus(http.StatusInternalServerError)
		})

		t.Run("PlainValuesHaveNoEncodingField", func(t *testing.T) {
			h.SeedString("plain", "just text")
			if resp := getKey(t, h, "plain"); resp.Encoding != "" {
				t.Errorf("encoding = %q, want empty for an uncompressed value", resp.Encoding)
			}
		})
	})
}

func TestKeyMutationEndpoints(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		t.Run("SetAndOverwrite", func(t *testing.T) {
			h.Put(h.KeyPath("mut:string"), map[string]any{"value": "first"}).ExpectOK()
			if got := h.GetString("mut:string"); got != "first" {
				t.Errorf("value = %q, want first", got)
			}
			h.Put(h.KeyPath("mut:string"), map[string]any{"value": "second"}).ExpectOK()
			if got := h.GetString("mut:string"); got != "second" {
				t.Errorf("value = %q, want second", got)
			}
		})

		t.Run("SetWithTTL", func(t *testing.T) {
			h.Put(h.KeyPath("mut:ttl"), map[string]any{"value": "v", "ttl": 120}).ExpectOK()
			ttl, err := h.Client.TTL(h.Ctx(), "mut:ttl")
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			if ttl <= 0 || ttl > 120 {
				t.Errorf("ttl = %d, want a value in (0, 120]", ttl)
			}
		})

		t.Run("SetEmptyValue", func(t *testing.T) {
			h.Put(h.KeyPath("mut:empty"), map[string]any{"value": ""}).ExpectOK()
			if got := h.TypeOf("mut:empty"); got != "string" {
				t.Errorf("type = %q, want string; an empty value should still create the key", got)
			}
		})

		t.Run("Increment", func(t *testing.T) {
			h.SeedString("mut:counter", "10")
			var body struct {
				Value string `json:"value"`
			}
			h.Post(h.KeyPath("mut:counter")+"/incr", map[string]any{"amount": 5}).ExpectOK().Decode(&body)
			if body.Value != "15" {
				t.Errorf("value = %q, want 15", body.Value)
			}

			h.Post(h.KeyPath("mut:counter")+"/incr", map[string]any{"amount": -2.5}).ExpectOK().Decode(&body)
			if body.Value != "12.5" {
				t.Errorf("value = %q, want 12.5", body.Value)
			}
		})

		t.Run("IncrementNonNumeric", func(t *testing.T) {
			h.SeedString("mut:text", "abc")
			h.Post(h.KeyPath("mut:text")+"/incr", map[string]any{"amount": 1}).
				ExpectStatus(http.StatusInternalServerError)
		})

		t.Run("ExpireAndPersist", func(t *testing.T) {
			h.SeedString("mut:expire", "v")

			var body struct {
				OK bool `json:"ok"`
			}
			h.Post(h.KeyPath("mut:expire")+"/expire", map[string]any{"ttl": 300}).ExpectOK().Decode(&body)
			if !body.OK {
				t.Error("expire reported failure for an existing key")
			}

			// A ttl of 0 means "persist".
			h.Post(h.KeyPath("mut:expire")+"/expire", map[string]any{"ttl": 0}).ExpectOK().Decode(&body)
			if !body.OK {
				t.Error("persist reported failure for a key with a TTL")
			}
			ttl, err := h.Client.TTL(h.Ctx(), "mut:expire")
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			if ttl != -1 {
				t.Errorf("ttl = %d, want -1 after persist", ttl)
			}
		})

		t.Run("ExpireMissingKey", func(t *testing.T) {
			var body struct {
				OK bool `json:"ok"`
			}
			h.Post(h.KeyPath("mut:nothing")+"/expire", map[string]any{"ttl": 60}).ExpectOK().Decode(&body)
			if body.OK {
				t.Error("expire reported success for a key that does not exist")
			}
		})

		t.Run("Rename", func(t *testing.T) {
			h.SeedString("mut:old", "payload")
			h.Post(h.KeyPath("mut:old")+"/rename", map[string]any{"newKey": "mut:new"}).ExpectOK()
			if got := h.TypeOf("mut:old"); got != "none" {
				t.Errorf("old key still exists as %q", got)
			}
			if got := h.GetString("mut:new"); got != "payload" {
				t.Errorf("renamed value = %q, want payload", got)
			}
		})

		t.Run("RenameRequiresANewName", func(t *testing.T) {
			h.SeedString("mut:rename-blank", "v")
			for _, newKey := range []string{"", "   "} {
				h.Post(h.KeyPath("mut:rename-blank")+"/rename", map[string]any{"newKey": newKey}).
					ExpectError(http.StatusBadRequest, "New key name required")
			}
		})

		t.Run("RenameMissingKey", func(t *testing.T) {
			h.Post(h.KeyPath("mut:absent")+"/rename", map[string]any{"newKey": "mut:whatever"}).
				ExpectStatus(http.StatusInternalServerError)
		})

		t.Run("Delete", func(t *testing.T) {
			h.SeedString("mut:doomed", "v")
			var body struct {
				Deleted int64 `json:"deleted"`
			}
			h.Delete(h.KeyPath("mut:doomed")).ExpectOK().Decode(&body)
			if body.Deleted != 1 {
				t.Errorf("deleted = %d, want 1", body.Deleted)
			}

			h.Delete(h.KeyPath("mut:doomed")).ExpectOK().Decode(&body)
			if body.Deleted != 0 {
				t.Errorf("deleting a missing key reported %d, want 0", body.Deleted)
			}
		})

		t.Run("DeleteMany", func(t *testing.T) {
			h.SeedKeys("bulk:", 3)
			var body struct {
				Deleted int64 `json:"deleted"`
			}
			h.Post("/api/keys/delete", map[string]any{
				"keys": []string{"bulk:0", "bulk:1", "bulk:2", "bulk:missing"},
			}).ExpectOK().Decode(&body)
			if body.Deleted != 3 {
				t.Errorf("deleted = %d, want 3", body.Deleted)
			}
		})

		t.Run("DeleteManyWithoutKeys", func(t *testing.T) {
			h.Post("/api/keys/delete", map[string]any{"keys": []string{}}).
				ExpectError(http.StatusBadRequest, "No keys specified")
		})

		t.Run("Flush", func(t *testing.T) {
			h.SeedKeys("flush:", 5)
			h.Post("/api/flush", nil).ExpectOK()
			if keys := h.AllKeys(); len(keys) != 0 {
				t.Errorf("database still has %v after flush", keys)
			}
		})
	})
}

func TestKeysMemoryEndpoint(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		h.SeedString("mem:a", "x")
		h.SeedString("mem:b", strings.Repeat("x", 10_000))

		t.Run("ReportsPerKeyUsage", func(t *testing.T) {
			var body struct {
				Memory map[string]int64 `json:"memory"`
			}
			h.Post("/api/keys/memory", map[string]any{
				"keys": []string{"mem:a", "mem:b", "mem:missing"},
			}).ExpectOK().Decode(&body)

			if len(body.Memory) != 2 {
				t.Errorf("got %d entries, want 2 (missing keys are skipped)", len(body.Memory))
			}
			if body.Memory["mem:b"] <= body.Memory["mem:a"] {
				t.Errorf("memory = %v, want the larger value to use more", body.Memory)
			}
		})

		t.Run("RejectsEmptyRequest", func(t *testing.T) {
			h.Post("/api/keys/memory", map[string]any{"keys": []string{}}).
				ExpectError(http.StatusBadRequest, "No keys specified")
		})

		t.Run("RejectsOversizedRequest", func(t *testing.T) {
			keys := make([]string, 10_001)
			for i := range keys {
				keys[i] = "k"
			}
			h.Post("/api/keys/memory", map[string]any{"keys": keys}).
				ExpectError(http.StatusBadRequest, "Too many keys")
		})
	})
}

func TestSpecialKeyNames(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		// Key names reach the handler through the URL path, so anything that
		// needs percent-encoding is a chance to lose or mangle the name.
		keys := []string{
			"with/slash",
			"with spaces",
			"with:colons:like:namespaces",
			"with#hash",
			"with?question",
			"with%percent",
			"with+plus",
			"unicode:ключ:键:🔑",
			"with\"quote",
			"with\\backslash",
		}

		for _, key := range keys {
			t.Run(key, func(t *testing.T) {
				h.Put(h.KeyPath(key), map[string]any{"value": "v:" + key}).ExpectOK()

				if got := h.GetString(key); got != "v:"+key {
					t.Fatalf("stored under a different name; value = %q", got)
				}

				resp := getKey(t, h, key)
				if resp.Key != key {
					t.Errorf("returned key = %q, want %q", resp.Key, key)
				}
				if got := decodeValue[string](t, resp); got != "v:"+key {
					t.Errorf("value = %q, want %q", got, "v:"+key)
				}

				h.Delete(h.KeyPath(key)).ExpectOK()
				if got := h.TypeOf(key); got != "none" {
					t.Errorf("key still exists as %q after delete", got)
				}
			})
		}
	})
}

func TestMalformedRequests(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		h.SeedList("bad:list", "a")
		h.SeedSet("bad:set", "m")
		h.SeedHash("bad:hash", map[string]string{"f": "v"})
		h.SeedZSet("bad:zset", map[string]float64{"m": 1})

		t.Run("InvalidJSONBodies", func(t *testing.T) {
			endpoints := []struct {
				method string
				path   string
			}{
				{http.MethodPut, h.KeyPath("bad:string")},
				{http.MethodPost, h.KeyPath("bad:string") + "/incr"},
				{http.MethodPost, h.KeyPath("bad:string") + "/expire"},
				{http.MethodPost, h.KeyPath("bad:string") + "/rename"},
				{http.MethodPost, "/api/keys/delete"},
				{http.MethodPost, "/api/keys/memory"},
				{http.MethodPost, "/api/notifications"},
				{http.MethodPost, h.KeyPath("bad:list") + "/list"},
				{http.MethodPut, h.KeyPath("bad:list") + "/list/0"},
				{http.MethodPost, h.KeyPath("bad:set") + "/set"},
				{http.MethodPatch, h.KeyPath("bad:set") + "/set/m"},
				{http.MethodPost, h.KeyPath("bad:hash") + "/hash"},
				{http.MethodPatch, h.KeyPath("bad:hash") + "/hash/f"},
				{http.MethodPost, h.KeyPath("bad:zset") + "/zset"},
				{http.MethodPatch, h.KeyPath("bad:zset") + "/zset/m"},
				{http.MethodPost, h.KeyPath("bad:zset") + "/zset/m/incr"},
				{http.MethodPost, h.KeyPath("bad:geo") + "/geo"},
				{http.MethodPost, h.KeyPath("bad:stream") + "/stream"},
				{http.MethodPost, h.KeyPath("bad:hll") + "/hll"},
				{http.MethodPost, "/api/exec"},
			}
			for _, ep := range endpoints {
				h.Request(ep.method, ep.path, "{not json").
					ExpectError(http.StatusBadRequest, "Invalid request body")
			}
		})

		t.Run("InvalidListIndex", func(t *testing.T) {
			h.Put(h.KeyPath("bad:list")+"/list/abc", map[string]any{"value": "x"}).
				ExpectError(http.StatusBadRequest, "Invalid index")
			h.Delete(h.KeyPath("bad:list")+"/list/abc").
				ExpectError(http.StatusBadRequest, "Invalid index")
		})

		t.Run("OversizedBodyIsRejected", func(t *testing.T) {
			// The handler caps request bodies at 1MB to keep a single request
			// from exhausting memory.
			huge := map[string]any{"value": strings.Repeat("x", 2<<20)}
			resp := h.Put(h.KeyPath("bad:huge"), huge)
			if resp.Status != http.StatusBadRequest && resp.Status != http.StatusRequestEntityTooLarge {
				t.Errorf("status = %d, want 400 or 413 for an oversized body", resp.Status)
			}
			if got := h.TypeOf("bad:huge"); got != "none" {
				t.Error("the oversized value was stored anyway")
			}
		})

		t.Run("BodyJustUnderTheLimitIsAccepted", func(t *testing.T) {
			h.Put(h.KeyPath("bad:large"), map[string]any{
				"value": strings.Repeat("x", 900_000),
			}).ExpectOK()
		})

		t.Run("UnknownRouteIs404", func(t *testing.T) {
			h.Get("/api/nope").ExpectStatus(http.StatusNotFound)
		})

		t.Run("WrongMethodIsRejected", func(t *testing.T) {
			resp := h.Request(http.MethodPost, "/api/health", nil)
			if resp.Status != http.StatusMethodNotAllowed && resp.Status != http.StatusNotFound {
				t.Errorf("status = %d, want 405 or 404", resp.Status)
			}
		})
	})
}

func TestCORS(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		t.Run("DisabledByDefault", func(t *testing.T) {
			h := testenv.New(t, e)
			resp := h.Get("/api/health").ExpectOK()
			if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "" {
				t.Errorf("Access-Control-Allow-Origin = %q, want no header by default", origin)
			}
		})

		t.Run("EchoesConfiguredOrigin", func(t *testing.T) {
			h := testenv.New(t, e, testenv.CORSOrigin("https://example.test"))
			resp := h.Get("/api/health").ExpectOK()
			if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "https://example.test" {
				t.Errorf("Access-Control-Allow-Origin = %q, want https://example.test", origin)
			}
		})

		t.Run("PreflightShortCircuits", func(t *testing.T) {
			h := testenv.New(t, e, testenv.CORSOrigin("https://example.test"))
			resp := h.Request(http.MethodOptions, "/api/keys", nil).ExpectOK()
			if methods := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(methods, "DELETE") {
				t.Errorf("Access-Control-Allow-Methods = %q, want it to list DELETE", methods)
			}
			if headers := resp.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(headers, "Content-Type") {
				t.Errorf("Access-Control-Allow-Headers = %q, want Content-Type", headers)
			}
		})
	})
}
