package testenv

import (
	"fmt"
	"testing"
	"time"

	"github.com/natrimmer/kvweb/internal/valkey"
)

// Seeding helpers. Each fails the test on error, so callers can seed without
// error-checking noise and every failure points at the seeding line.

// SeedString stores a string value.
func (h *Harness) SeedString(key, value string) {
	h.T.Helper()
	if err := h.Client.Set(h.Ctx(), key, value, 0); err != nil {
		h.T.Fatalf("seed string %q: %v", key, err)
	}
}

// SeedStringTTL stores a string value with a TTL.
func (h *Harness) SeedStringTTL(key, value string, ttl time.Duration) {
	h.T.Helper()
	if err := h.Client.Set(h.Ctx(), key, value, ttl); err != nil {
		h.T.Fatalf("seed string %q with ttl: %v", key, err)
	}
}

// SeedList appends items to a list, left to right.
func (h *Harness) SeedList(key string, items ...string) {
	h.T.Helper()
	if len(items) == 0 {
		return
	}
	if err := h.Client.RPush(h.Ctx(), key, items...); err != nil {
		h.T.Fatalf("seed list %q: %v", key, err)
	}
}

// SeedSet adds members to a set.
func (h *Harness) SeedSet(key string, members ...string) {
	h.T.Helper()
	if len(members) == 0 {
		return
	}
	if err := h.Client.SAdd(h.Ctx(), key, members...); err != nil {
		h.T.Fatalf("seed set %q: %v", key, err)
	}
}

// SeedHash sets fields on a hash.
func (h *Harness) SeedHash(key string, fields map[string]string) {
	h.T.Helper()
	for field, value := range fields {
		if err := h.Client.HSet(h.Ctx(), key, field, value); err != nil {
			h.T.Fatalf("seed hash %q field %q: %v", key, field, err)
		}
	}
}

// SeedZSet adds scored members to a sorted set.
func (h *Harness) SeedZSet(key string, members map[string]float64) {
	h.T.Helper()
	for member, score := range members {
		if err := h.Client.ZAdd(h.Ctx(), key, member, score); err != nil {
			h.T.Fatalf("seed zset %q member %q: %v", key, member, err)
		}
	}
}

// SeedGeo adds a positioned member to a geospatial index.
func (h *Harness) SeedGeo(key, member string, lon, lat float64) {
	h.T.Helper()
	if err := h.Client.GeoAdd(h.Ctx(), key, lon, lat, member); err != nil {
		h.T.Fatalf("seed geo %q member %q: %v", key, member, err)
	}
}

// SeedStream appends entries to a stream and returns their generated IDs.
func (h *Harness) SeedStream(key string, entries ...map[string]string) []string {
	h.T.Helper()
	ids := make([]string, 0, len(entries))
	for i, fields := range entries {
		id, err := h.Client.XAddMulti(h.Ctx(), key, fields)
		if err != nil {
			h.T.Fatalf("seed stream %q entry %d: %v", key, i, err)
		}
		ids = append(ids, id)
	}
	return ids
}

// SeedStreamN appends n entries of the form {"seq": "<i>"} and returns their IDs.
func (h *Harness) SeedStreamN(key string, n int) []string {
	h.T.Helper()
	entries := make([]map[string]string, n)
	for i := range n {
		entries[i] = map[string]string{"seq": fmt.Sprintf("%d", i)}
	}
	return h.SeedStream(key, entries...)
}

// SeedHLL adds elements to a HyperLogLog.
func (h *Harness) SeedHLL(key string, elements ...string) {
	h.T.Helper()
	if err := h.Client.PFAdd(h.Ctx(), key, elements...); err != nil {
		h.T.Fatalf("seed hll %q: %v", key, err)
	}
}

// SeedKeys stores n string keys named <prefix><i>.
func (h *Harness) SeedKeys(prefix string, n int) []string {
	h.T.Helper()
	keys := make([]string, n)
	for i := range n {
		keys[i] = fmt.Sprintf("%s%d", prefix, i)
		h.SeedString(keys[i], fmt.Sprintf("value-%d", i))
	}
	return keys
}

// SeedOneOfEachType creates one key of every supported type, named
// "<prefix>:<type>", and returns the map of type name to key.
func (h *Harness) SeedOneOfEachType(prefix string) map[string]string {
	h.T.Helper()
	keys := map[string]string{
		"string":      prefix + ":string",
		"list":        prefix + ":list",
		"set":         prefix + ":set",
		"hash":        prefix + ":hash",
		"zset":        prefix + ":zset",
		"stream":      prefix + ":stream",
		"hyperloglog": prefix + ":hll",
	}
	h.SeedString(keys["string"], "hello")
	h.SeedList(keys["list"], "a", "b", "c")
	h.SeedSet(keys["set"], "x", "y", "z")
	h.SeedHash(keys["hash"], map[string]string{"field": "value"})
	h.SeedZSet(keys["zset"], map[string]float64{"alice": 1.5, "bob": 2.5})
	h.SeedStream(keys["stream"], map[string]string{"event": "created"})
	h.SeedHLL(keys["hyperloglog"], "a", "b", "c")
	return keys
}

// Raw runs an arbitrary command through the client and fails on error.
func (h *Harness) Raw(args ...string) any {
	h.T.Helper()
	result, err := h.Client.Exec(h.Ctx(), args)
	if err != nil {
		h.T.Fatalf("command %v failed: %v", args, err)
	}
	return result
}

// RawErr runs an arbitrary command and returns the result and error unchanged.
func (h *Harness) RawErr(args ...string) (any, error) {
	h.T.Helper()
	return h.Client.Exec(h.Ctx(), args)
}

// TypeOf returns the server-reported type of a key.
func (h *Harness) TypeOf(key string) string {
	h.T.Helper()
	t, err := h.Client.Type(h.Ctx(), key)
	if err != nil {
		h.T.Fatalf("TYPE %q: %v", key, err)
	}
	return t
}

// GetString returns a string key's value.
func (h *Harness) GetString(key string) string {
	h.T.Helper()
	v, err := h.Client.Get(h.Ctx(), key)
	if err != nil {
		h.T.Fatalf("GET %q: %v", key, err)
	}
	return v
}

// ListItems returns a list's full contents.
func (h *Harness) ListItems(key string) []string {
	h.T.Helper()
	items, err := h.Client.LRange(h.Ctx(), key, 0, -1)
	if err != nil {
		h.T.Fatalf("LRANGE %q: %v", key, err)
	}
	return items
}

// SetMembers returns a set's full contents.
func (h *Harness) SetMembers(key string) []string {
	h.T.Helper()
	members, err := h.Client.SMembers(h.Ctx(), key)
	if err != nil {
		h.T.Fatalf("SMEMBERS %q: %v", key, err)
	}
	return members
}

// HashFields returns a hash's full contents.
func (h *Harness) HashFields(key string) map[string]string {
	h.T.Helper()
	fields, err := h.Client.HGetAll(h.Ctx(), key)
	if err != nil {
		h.T.Fatalf("HGETALL %q: %v", key, err)
	}
	return fields
}

// ZSetMembers returns a sorted set's full contents with scores.
func (h *Harness) ZSetMembers(key string) []valkey.ZMember {
	h.T.Helper()
	members, err := h.Client.ZRangeWithScores(h.Ctx(), key, 0, -1)
	if err != nil {
		h.T.Fatalf("ZRANGE %q: %v", key, err)
	}
	return members
}

// AllKeys scans the whole database and returns every key.
func (h *Harness) AllKeys() []string {
	h.T.Helper()
	return h.ScanAll("*")
}

// ScanAll runs SCAN to completion for a pattern and returns every match.
func (h *Harness) ScanAll(pattern string) []string {
	h.T.Helper()
	var (
		all    []string
		cursor uint64
	)
	for range 1000 {
		keys, next, err := h.Client.Keys(h.Ctx(), pattern, cursor, 100)
		if err != nil {
			h.T.Fatalf("SCAN %q: %v", pattern, err)
		}
		all = append(all, keys...)
		cursor = next
		if cursor == 0 {
			return all
		}
	}
	h.T.Fatalf("SCAN %q did not finish within 1000 iterations", pattern)
	return nil
}

// RequireNoError fails the test if err is non-nil.
func RequireNoError(t *testing.T, err error, format string, args ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf(format+": %v", append(args, err)...)
	}
}
