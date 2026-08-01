package valkey_test

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/natrimmer/kvweb/internal/testenv"
	"github.com/natrimmer/kvweb/internal/valkey"
)

func TestStringOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("SetGet", func(t *testing.T) {
			if err := h.Client.Set(ctx, "s", "hello", 0); err != nil {
				t.Fatalf("Set: %v", err)
			}
			got, err := h.Client.Get(ctx, "s")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got != "hello" {
				t.Errorf("Get = %q, want %q", got, "hello")
			}
		})

		t.Run("SetOverwritesAndClearsTTL", func(t *testing.T) {
			if err := h.Client.Set(ctx, "overwrite", "first", time.Minute); err != nil {
				t.Fatalf("Set with ttl: %v", err)
			}
			if err := h.Client.Set(ctx, "overwrite", "second", 0); err != nil {
				t.Fatalf("Set without ttl: %v", err)
			}
			ttl, err := h.Client.TTL(ctx, "overwrite")
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			// SET without KEEPTTL discards the previous expiry on both engines.
			if ttl != -1 {
				t.Errorf("TTL after plain SET = %d, want -1 (no expiry)", ttl)
			}
		})

		t.Run("GetMissingKey", func(t *testing.T) {
			if _, err := h.Client.Get(ctx, "does-not-exist"); err == nil {
				t.Error("Get on a missing key returned no error")
			}
		})

		t.Run("BinaryValuesRoundTrip", func(t *testing.T) {
			// Values pass through the client as Go strings; anything that
			// mangles bytes here would corrupt compressed and HLL values.
			binary := string([]byte{0x00, 0x1f, 0x8b, 0xff, 0xfe, '\n', '\r', 0x7f})
			if err := h.Client.Set(ctx, "binary", binary, 0); err != nil {
				t.Fatalf("Set binary: %v", err)
			}
			got, err := h.Client.Get(ctx, "binary")
			if err != nil {
				t.Fatalf("Get binary: %v", err)
			}
			if got != binary {
				t.Errorf("binary round trip changed the value: % x -> % x", binary, got)
			}
		})

		t.Run("IncrByFloat", func(t *testing.T) {
			if err := h.Client.Set(ctx, "counter", "10", 0); err != nil {
				t.Fatalf("Set: %v", err)
			}
			got, err := h.Client.IncrByFloat(ctx, "counter", 2.5)
			if err != nil {
				t.Fatalf("IncrByFloat: %v", err)
			}
			if got != "12.5" {
				t.Errorf("IncrByFloat = %q, want %q", got, "12.5")
			}

			// Negative amounts decrement, and whole results stay integral.
			got, err = h.Client.IncrByFloat(ctx, "counter", -2.5)
			if err != nil {
				t.Fatalf("IncrByFloat negative: %v", err)
			}
			if got != "10" {
				t.Errorf("IncrByFloat = %q, want %q", got, "10")
			}
		})

		t.Run("IncrByFloatOnNonNumeric", func(t *testing.T) {
			h.SeedString("not-a-number", "abc")
			if _, err := h.Client.IncrByFloat(ctx, "not-a-number", 1); err == nil {
				t.Error("IncrByFloat on a non-numeric value returned no error")
			}
		})

		t.Run("Delete", func(t *testing.T) {
			h.SeedString("del-a", "1")
			h.SeedString("del-b", "2")
			deleted, err := h.Client.Del(ctx, "del-a", "del-b", "del-missing")
			if err != nil {
				t.Fatalf("Del: %v", err)
			}
			if deleted != 2 {
				t.Errorf("Del reported %d deleted, want 2", deleted)
			}
		})
	})
}

func TestTTLOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("MissingKeyReturnsMinusTwo", func(t *testing.T) {
			ttl, err := h.Client.TTL(ctx, "nope")
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			if ttl != -2 {
				t.Errorf("TTL of a missing key = %d, want -2", ttl)
			}
		})

		t.Run("KeyWithoutTTLReturnsMinusOne", func(t *testing.T) {
			h.SeedString("no-ttl", "v")
			ttl, err := h.Client.TTL(ctx, "no-ttl")
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			if ttl != -1 {
				t.Errorf("TTL of a persistent key = %d, want -1", ttl)
			}
		})

		t.Run("ExpireThenPersist", func(t *testing.T) {
			h.SeedString("expiring", "v")

			ok, err := h.Client.Expire(ctx, "expiring", 90*time.Second)
			if err != nil {
				t.Fatalf("Expire: %v", err)
			}
			if !ok {
				t.Fatal("Expire returned false for an existing key")
			}

			ttl, err := h.Client.TTL(ctx, "expiring")
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			if ttl <= 0 || ttl > 90 {
				t.Errorf("TTL = %d, want a value in (0, 90]", ttl)
			}

			ok, err = h.Client.Persist(ctx, "expiring")
			if err != nil {
				t.Fatalf("Persist: %v", err)
			}
			if !ok {
				t.Fatal("Persist returned false for a key with a TTL")
			}

			ttl, err = h.Client.TTL(ctx, "expiring")
			if err != nil {
				t.Fatalf("TTL after Persist: %v", err)
			}
			if ttl != -1 {
				t.Errorf("TTL after Persist = %d, want -1", ttl)
			}
		})

		t.Run("ExpireOnMissingKey", func(t *testing.T) {
			ok, err := h.Client.Expire(ctx, "missing", time.Minute)
			if err != nil {
				t.Fatalf("Expire: %v", err)
			}
			if ok {
				t.Error("Expire returned true for a missing key")
			}
		})

		t.Run("PersistOnKeyWithoutTTL", func(t *testing.T) {
			h.SeedString("plain", "v")
			ok, err := h.Client.Persist(ctx, "plain")
			if err != nil {
				t.Fatalf("Persist: %v", err)
			}
			if ok {
				t.Error("Persist returned true for a key that had no TTL")
			}
		})
	})
}

func TestKeyScanning(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		const total = 250
		h.SeedKeys("scan:item:", total)
		h.SeedString("other:key", "v")

		t.Run("FullScanSeesEveryKey", func(t *testing.T) {
			// SCAN guarantees full coverage across a complete cursor cycle even
			// though COUNT is only a hint, so assert on the union, not one page.
			keys := h.ScanAll("scan:item:*")
			if len(keys) != total {
				t.Fatalf("full scan returned %d keys, want %d", len(keys), total)
			}
			seen := map[string]bool{}
			for _, k := range keys {
				if seen[k] {
					t.Fatalf("scan returned %q twice", k)
				}
				seen[k] = true
			}
			for i := range total {
				want := fmt.Sprintf("scan:item:%d", i)
				if !seen[want] {
					t.Errorf("scan never returned %q", want)
				}
			}
		})

		t.Run("PatternExcludesNonMatchingKeys", func(t *testing.T) {
			for _, k := range h.ScanAll("scan:item:*") {
				if !strings.HasPrefix(k, "scan:item:") {
					t.Errorf("scan returned %q, which does not match the pattern", k)
				}
			}
		})

		t.Run("CursorRoundTrips", func(t *testing.T) {
			_, cursor, err := h.Client.Keys(h.Ctx(), "*", 0, 10)
			if err != nil {
				t.Fatalf("Keys: %v", err)
			}
			if cursor == 0 {
				t.Skip("server returned everything in one page; nothing to resume")
			}
			if _, _, err := h.Client.Keys(h.Ctx(), "*", cursor, 10); err != nil {
				t.Fatalf("Keys with a resumed cursor: %v", err)
			}
		})
	})
}

func TestListOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("PushRangeLen", func(t *testing.T) {
			key := "list:basic"
			if err := h.Client.RPush(ctx, key, "b", "c"); err != nil {
				t.Fatalf("RPush: %v", err)
			}
			if err := h.Client.LPush(ctx, key, "a"); err != nil {
				t.Fatalf("LPush: %v", err)
			}

			length, err := h.Client.LLen(ctx, key)
			if err != nil {
				t.Fatalf("LLen: %v", err)
			}
			if length != 3 {
				t.Errorf("LLen = %d, want 3", length)
			}

			items, err := h.Client.LRange(ctx, key, 0, -1)
			if err != nil {
				t.Fatalf("LRange: %v", err)
			}
			if !slices.Equal(items, []string{"a", "b", "c"}) {
				t.Errorf("LRange = %v, want [a b c]", items)
			}
		})

		t.Run("RangeWindow", func(t *testing.T) {
			key := "list:window"
			h.SeedList(key, "0", "1", "2", "3", "4")
			items, err := h.Client.LRange(ctx, key, 1, 3)
			if err != nil {
				t.Fatalf("LRange: %v", err)
			}
			if !slices.Equal(items, []string{"1", "2", "3"}) {
				t.Errorf("LRange(1,3) = %v, want [1 2 3]", items)
			}
		})

		t.Run("RangePastEndIsEmpty", func(t *testing.T) {
			key := "list:short"
			h.SeedList(key, "a")
			items, err := h.Client.LRange(ctx, key, 100, 200)
			if err != nil {
				t.Fatalf("LRange: %v", err)
			}
			if len(items) != 0 {
				t.Errorf("LRange past the end = %v, want empty", items)
			}
		})

		t.Run("Set", func(t *testing.T) {
			key := "list:set"
			h.SeedList(key, "a", "b", "c")
			if err := h.Client.LSet(ctx, key, 1, "B"); err != nil {
				t.Fatalf("LSet: %v", err)
			}
			if items := h.ListItems(key); !slices.Equal(items, []string{"a", "B", "c"}) {
				t.Errorf("list = %v, want [a B c]", items)
			}
		})

		t.Run("SetOutOfRange", func(t *testing.T) {
			key := "list:oob"
			h.SeedList(key, "a")
			if err := h.Client.LSet(ctx, key, 9, "x"); err == nil {
				t.Error("LSet past the end returned no error")
			}
		})

		t.Run("RemoveByIndex", func(t *testing.T) {
			key := "list:remove"
			h.SeedList(key, "a", "b", "c", "d", "e")
			if err := h.Client.LRemByIndex(ctx, key, 2); err != nil {
				t.Fatalf("LRemByIndex: %v", err)
			}
			if items := h.ListItems(key); !slices.Equal(items, []string{"a", "b", "d", "e"}) {
				t.Errorf("list = %v, want [a b d e]", items)
			}
		})

		t.Run("RemoveByIndexKeepsDuplicates", func(t *testing.T) {
			// The Lua script tombstones the target slot rather than using LREM
			// by value; duplicates of the removed value must survive.
			key := "list:dupes"
			h.SeedList(key, "x", "x", "x")
			if err := h.Client.LRemByIndex(ctx, key, 1); err != nil {
				t.Fatalf("LRemByIndex: %v", err)
			}
			if items := h.ListItems(key); !slices.Equal(items, []string{"x", "x"}) {
				t.Errorf("list = %v, want [x x]", items)
			}
		})

		t.Run("RemoveFirstAndLast", func(t *testing.T) {
			key := "list:ends"
			h.SeedList(key, "a", "b", "c")
			if err := h.Client.LRemByIndex(ctx, key, 0); err != nil {
				t.Fatalf("LRemByIndex(0): %v", err)
			}
			if err := h.Client.LRemByIndex(ctx, key, 1); err != nil {
				t.Fatalf("LRemByIndex(last): %v", err)
			}
			if items := h.ListItems(key); !slices.Equal(items, []string{"b"}) {
				t.Errorf("list = %v, want [b]", items)
			}
		})

		t.Run("RemoveByIndexOutOfRange", func(t *testing.T) {
			key := "list:remove-oob"
			h.SeedList(key, "a")
			if err := h.Client.LRemByIndex(ctx, key, 5); err == nil {
				t.Error("LRemByIndex past the end returned no error")
			}
		})
	})
}

func TestSetOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("AddMembersCard", func(t *testing.T) {
			key := "set:basic"
			if err := h.Client.SAdd(ctx, key, "a", "b", "c"); err != nil {
				t.Fatalf("SAdd: %v", err)
			}
			card, err := h.Client.SCard(ctx, key)
			if err != nil {
				t.Fatalf("SCard: %v", err)
			}
			if card != 3 {
				t.Errorf("SCard = %d, want 3", card)
			}

			members := h.SetMembers(key)
			slices.Sort(members)
			if !slices.Equal(members, []string{"a", "b", "c"}) {
				t.Errorf("SMembers = %v, want [a b c]", members)
			}
		})

		t.Run("AddIsIdempotent", func(t *testing.T) {
			key := "set:dupe"
			h.SeedSet(key, "a")
			h.SeedSet(key, "a")
			card, err := h.Client.SCard(ctx, key)
			if err != nil {
				t.Fatalf("SCard: %v", err)
			}
			if card != 1 {
				t.Errorf("SCard after re-adding = %d, want 1", card)
			}
		})

		t.Run("IsMember", func(t *testing.T) {
			key := "set:member"
			h.SeedSet(key, "present")

			ok, err := h.Client.SIsMember(ctx, key, "present")
			if err != nil {
				t.Fatalf("SIsMember: %v", err)
			}
			if !ok {
				t.Error("SIsMember = false for a member that exists")
			}

			ok, err = h.Client.SIsMember(ctx, key, "absent")
			if err != nil {
				t.Fatalf("SIsMember: %v", err)
			}
			if ok {
				t.Error("SIsMember = true for a member that does not exist")
			}
		})

		t.Run("Remove", func(t *testing.T) {
			key := "set:remove"
			h.SeedSet(key, "a", "b")
			if err := h.Client.SRem(ctx, key, "a"); err != nil {
				t.Fatalf("SRem: %v", err)
			}
			if members := h.SetMembers(key); !slices.Equal(members, []string{"b"}) {
				t.Errorf("members = %v, want [b]", members)
			}
		})

		t.Run("ScanCoversEveryMember", func(t *testing.T) {
			key := "set:scan"
			members := make([]string, 300)
			for i := range members {
				members[i] = fmt.Sprintf("member-%03d", i)
			}
			h.SeedSet(key, members...)

			seen := map[string]bool{}
			var cursor uint64
			for range 100 {
				page, next, err := h.Client.SScan(ctx, key, cursor, 25)
				if err != nil {
					t.Fatalf("SScan: %v", err)
				}
				for _, m := range page {
					seen[m] = true
				}
				cursor = next
				if cursor == 0 {
					break
				}
			}
			if cursor != 0 {
				t.Fatal("SSCAN did not finish within 100 iterations")
			}
			if len(seen) != len(members) {
				t.Errorf("SSCAN saw %d distinct members, want %d", len(seen), len(members))
			}
		})
	})
}

func TestHashOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("SetGetAllLen", func(t *testing.T) {
			key := "hash:basic"
			h.SeedHash(key, map[string]string{"name": "Alice", "age": "30"})

			length, err := h.Client.HLen(ctx, key)
			if err != nil {
				t.Fatalf("HLen: %v", err)
			}
			if length != 2 {
				t.Errorf("HLen = %d, want 2", length)
			}

			fields := h.HashFields(key)
			if fields["name"] != "Alice" || fields["age"] != "30" {
				t.Errorf("HGetAll = %v, want name=Alice age=30", fields)
			}
		})

		t.Run("SetOverwrites", func(t *testing.T) {
			key := "hash:overwrite"
			h.SeedHash(key, map[string]string{"f": "old"})
			h.SeedHash(key, map[string]string{"f": "new"})
			if got := h.HashFields(key)["f"]; got != "new" {
				t.Errorf("field = %q, want %q", got, "new")
			}
		})

		t.Run("EmptyFieldNameIsAllowedByTheServer", func(t *testing.T) {
			// The API rejects empty field names, but the server itself accepts
			// them; keeping the layers distinct matters when reading old data.
			key := "hash:empty-field"
			if err := h.Client.HSet(ctx, key, "", "value"); err != nil {
				t.Fatalf("HSet with an empty field name: %v", err)
			}
			if got := h.HashFields(key)[""]; got != "value" {
				t.Errorf("empty-named field = %q, want %q", got, "value")
			}
		})

		t.Run("Exists", func(t *testing.T) {
			key := "hash:exists"
			h.SeedHash(key, map[string]string{"here": "1"})

			ok, err := h.Client.HExists(ctx, key, "here")
			if err != nil {
				t.Fatalf("HExists: %v", err)
			}
			if !ok {
				t.Error("HExists = false for a field that exists")
			}

			ok, err = h.Client.HExists(ctx, key, "gone")
			if err != nil {
				t.Fatalf("HExists: %v", err)
			}
			if ok {
				t.Error("HExists = true for a field that does not exist")
			}
		})

		t.Run("Delete", func(t *testing.T) {
			key := "hash:delete"
			h.SeedHash(key, map[string]string{"a": "1", "b": "2"})
			if err := h.Client.HDel(ctx, key, "a"); err != nil {
				t.Fatalf("HDel: %v", err)
			}
			fields := h.HashFields(key)
			if _, ok := fields["a"]; ok {
				t.Error("deleted field is still present")
			}
			if len(fields) != 1 {
				t.Errorf("hash has %d fields, want 1", len(fields))
			}
		})

		t.Run("ScanCoversEveryFieldWithValues", func(t *testing.T) {
			key := "hash:scan"
			want := map[string]string{}
			for i := range 300 {
				want[fmt.Sprintf("field-%03d", i)] = fmt.Sprintf("value-%03d", i)
			}
			h.SeedHash(key, want)

			// HSCAN returns a flat field/value array; the client pairs it up, so
			// a mispaired page would show as a wrong value here, not a wrong count.
			got := map[string]string{}
			var cursor uint64
			for range 100 {
				page, next, err := h.Client.HScan(ctx, key, cursor, 25)
				if err != nil {
					t.Fatalf("HScan: %v", err)
				}
				for field, value := range page {
					got[field] = value
				}
				cursor = next
				if cursor == 0 {
					break
				}
			}
			if cursor != 0 {
				t.Fatal("HSCAN did not finish within 100 iterations")
			}
			if len(got) != len(want) {
				t.Fatalf("HSCAN saw %d fields, want %d", len(got), len(want))
			}
			for field, value := range want {
				if got[field] != value {
					t.Errorf("field %q = %q, want %q", field, got[field], value)
				}
			}
		})
	})
}

func TestZSetOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("AddCardRange", func(t *testing.T) {
			key := "zset:basic"
			h.SeedZSet(key, map[string]float64{"alice": 1.5, "bob": 2.5, "carol": 0.5})

			card, err := h.Client.ZCard(ctx, key)
			if err != nil {
				t.Fatalf("ZCard: %v", err)
			}
			if card != 3 {
				t.Errorf("ZCard = %d, want 3", card)
			}

			members := h.ZSetMembers(key)
			want := []valkey.ZMember{
				{Member: "carol", Score: 0.5},
				{Member: "alice", Score: 1.5},
				{Member: "bob", Score: 2.5},
			}
			if len(members) != len(want) {
				t.Fatalf("ZRange returned %d members, want %d", len(members), len(want))
			}
			for i := range want {
				if members[i] != want[i] {
					t.Errorf("member %d = %+v, want %+v", i, members[i], want[i])
				}
			}
		})

		t.Run("RangeWindow", func(t *testing.T) {
			key := "zset:window"
			h.SeedZSet(key, map[string]float64{"a": 1, "b": 2, "c": 3, "d": 4})
			members, err := h.Client.ZRangeWithScores(ctx, key, 1, 2)
			if err != nil {
				t.Fatalf("ZRangeWithScores: %v", err)
			}
			if len(members) != 2 || members[0].Member != "b" || members[1].Member != "c" {
				t.Errorf("ZRange(1,2) = %+v, want members b and c", members)
			}
		})

		t.Run("NegativeAndFractionalScores", func(t *testing.T) {
			key := "zset:scores"
			h.SeedZSet(key, map[string]float64{"neg": -12.75, "tiny": 0.000001})
			members := h.ZSetMembers(key)
			if len(members) != 2 {
				t.Fatalf("ZRange returned %d members, want 2", len(members))
			}
			if members[0].Score != -12.75 {
				t.Errorf("negative score round trip = %v, want -12.75", members[0].Score)
			}
			if members[1].Score != 0.000001 {
				t.Errorf("fractional score round trip = %v, want 0.000001", members[1].Score)
			}
		})

		t.Run("IncrementScore", func(t *testing.T) {
			key := "zset:incr"
			h.SeedZSet(key, map[string]float64{"alice": 10})

			score, err := h.Client.ZIncrBy(ctx, key, "alice", 2.5)
			if err != nil {
				t.Fatalf("ZIncrBy: %v", err)
			}
			if score != 12.5 {
				t.Errorf("ZIncrBy = %v, want 12.5", score)
			}

			// Incrementing an absent member creates it at the increment value.
			score, err = h.Client.ZIncrBy(ctx, key, "newcomer", 3)
			if err != nil {
				t.Fatalf("ZIncrBy on a new member: %v", err)
			}
			if score != 3 {
				t.Errorf("ZIncrBy on a new member = %v, want 3", score)
			}
		})

		t.Run("Remove", func(t *testing.T) {
			key := "zset:remove"
			h.SeedZSet(key, map[string]float64{"a": 1, "b": 2})
			if err := h.Client.ZRem(ctx, key, "a"); err != nil {
				t.Fatalf("ZRem: %v", err)
			}
			members := h.ZSetMembers(key)
			if len(members) != 1 || members[0].Member != "b" {
				t.Errorf("members = %+v, want only b", members)
			}
		})
	})
}

func TestGeoOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		const key = "geo:cities"
		// Geohash encoding is lossy, so compare to roughly a metre.
		const tolerance = 0.00002

		h.SeedGeo(key, "palermo", 13.361389, 38.115556)
		h.SeedGeo(key, "catania", 15.087269, 37.502669)

		t.Run("PositionsRoundTrip", func(t *testing.T) {
			positions, err := h.Client.GeoPos(ctx, key, "palermo", "catania")
			if err != nil {
				t.Fatalf("GeoPos: %v", err)
			}
			if len(positions) != 2 {
				t.Fatalf("GeoPos returned %d positions, want 2", len(positions))
			}
			for i, want := range []valkey.GeoPosition{
				{Longitude: 13.361389, Latitude: 38.115556},
				{Longitude: 15.087269, Latitude: 37.502669},
			} {
				if positions[i] == nil {
					t.Fatalf("position %d is nil", i)
				}
				if math.Abs(positions[i].Longitude-want.Longitude) > tolerance ||
					math.Abs(positions[i].Latitude-want.Latitude) > tolerance {
					t.Errorf("position %d = %+v, want ~%+v", i, *positions[i], want)
				}
			}
		})

		t.Run("MissingMemberIsNil", func(t *testing.T) {
			positions, err := h.Client.GeoPos(ctx, key, "palermo", "atlantis")
			if err != nil {
				t.Fatalf("GeoPos: %v", err)
			}
			if len(positions) != 2 {
				t.Fatalf("GeoPos returned %d positions, want 2", len(positions))
			}
			if positions[0] == nil {
				t.Error("position for an existing member is nil")
			}
			if positions[1] != nil {
				t.Errorf("position for a missing member = %+v, want nil", *positions[1])
			}
		})

		t.Run("StoredAsZSet", func(t *testing.T) {
			// The UI treats geo keys as sorted sets with a coordinate view; if
			// this ever changed, the geo editor would silently stop loading.
			if got := h.TypeOf(key); got != "zset" {
				t.Errorf("TYPE of a geo key = %q, want %q", got, "zset")
			}
		})

		t.Run("ExtremeCoordinates", func(t *testing.T) {
			extreme := "geo:extremes"
			h.SeedGeo(extreme, "west", -180, 0)
			h.SeedGeo(extreme, "north", 0, 85.05112878)
			h.SeedGeo(extreme, "south", 0, -85.05112878)

			positions, err := h.Client.GeoPos(ctx, extreme, "west", "north", "south")
			if err != nil {
				t.Fatalf("GeoPos: %v", err)
			}
			for i, p := range positions {
				if p == nil {
					t.Errorf("position %d is nil; the server rejected a coordinate the API allows", i)
				}
			}
		})
	})
}

func TestStreamOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("AddLenRange", func(t *testing.T) {
			key := "stream:basic"
			ids := h.SeedStream(key,
				map[string]string{"event": "created", "user": "alice"},
				map[string]string{"event": "updated", "user": "bob"},
			)

			length, err := h.Client.XLen(ctx, key)
			if err != nil {
				t.Fatalf("XLen: %v", err)
			}
			if length != 2 {
				t.Errorf("XLen = %d, want 2", length)
			}

			entries, err := h.Client.XRange(ctx, key, "-", "+", 0)
			if err != nil {
				t.Fatalf("XRange: %v", err)
			}
			if len(entries) != 2 {
				t.Fatalf("XRange returned %d entries, want 2", len(entries))
			}
			if entries[0].ID != ids[0] || entries[1].ID != ids[1] {
				t.Errorf("XRange IDs = %q, %q, want %q, %q",
					entries[0].ID, entries[1].ID, ids[0], ids[1])
			}
			if entries[0].Fields["event"] != "created" || entries[0].Fields["user"] != "alice" {
				t.Errorf("entry fields = %v, want event=created user=alice", entries[0].Fields)
			}
		})

		t.Run("AddWithoutFieldsIsRejected", func(t *testing.T) {
			if _, err := h.Client.XAddMulti(ctx, "stream:empty", map[string]string{}); err == nil {
				t.Error("XAddMulti with no fields returned no error")
			}
		})

		t.Run("Delete", func(t *testing.T) {
			key := "stream:delete"
			ids := h.SeedStreamN(key, 3)

			deleted, err := h.Client.XDel(ctx, key, ids[1])
			if err != nil {
				t.Fatalf("XDel: %v", err)
			}
			if deleted != 1 {
				t.Errorf("XDel reported %d deleted, want 1", deleted)
			}

			deleted, err = h.Client.XDel(ctx, key, ids[1])
			if err != nil {
				t.Fatalf("XDel on an already-deleted entry: %v", err)
			}
			if deleted != 0 {
				t.Errorf("XDel of a missing entry reported %d deleted, want 0", deleted)
			}
		})

		t.Run("DeleteWithoutIDsIsRejected", func(t *testing.T) {
			if _, err := h.Client.XDel(ctx, "stream:delete"); err == nil {
				t.Error("XDel with no IDs returned no error")
			}
		})

		t.Run("PageThroughEveryEntry", func(t *testing.T) {
			// The stream editor walks pages by cursor; every entry must appear
			// exactly once, and the walk must terminate on its own.
			key := "stream:paged"
			const total = 35
			const pageSize = 10
			ids := h.SeedStreamN(key, total)

			var (
				seen   []string
				cursor string
			)
			for page := range 20 {
				entries, next, err := h.Client.XRangePage(ctx, key, cursor, pageSize)
				if err != nil {
					t.Fatalf("XRangePage page %d: %v", page, err)
				}
				for _, entry := range entries {
					seen = append(seen, entry.ID)
				}
				if next == "" {
					break
				}
				if next == cursor {
					t.Fatalf("cursor %q did not advance", next)
				}
				cursor = next
			}

			if !slices.Equal(seen, ids) {
				t.Errorf("paging returned %d entries, want the %d seeded IDs in order\ngot:  %v\nwant: %v",
					len(seen), len(ids), seen, ids)
			}
		})

		t.Run("PageSizeDefaultsWhenNonPositive", func(t *testing.T) {
			key := "stream:default-page"
			h.SeedStreamN(key, 25)
			entries, _, err := h.Client.XRangePage(ctx, key, "", 0)
			if err != nil {
				t.Fatalf("XRangePage: %v", err)
			}
			if len(entries) != 10 {
				t.Errorf("XRangePage with pageSize 0 returned %d entries, want the default 10", len(entries))
			}
		})
	})
}

func TestHyperLogLogOperations(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		const key = "hll:visitors"

		t.Run("CountApproximatesCardinality", func(t *testing.T) {
			elements := make([]string, 1000)
			for i := range elements {
				elements[i] = fmt.Sprintf("visitor-%d", i)
			}
			h.SeedHLL(key, elements...)

			count, err := h.Client.PFCount(ctx, key)
			if err != nil {
				t.Fatalf("PFCount: %v", err)
			}
			// The standard error is ~0.81%; 5% is loose enough never to flake.
			if count < 950 || count > 1050 {
				t.Errorf("PFCount = %d, want roughly 1000", count)
			}
		})

		t.Run("StoredAsStringWithHYLLHeader", func(t *testing.T) {
			// The API identifies HyperLogLogs by this magic header rather than by
			// TYPE, because the server reports them as plain strings.
			if got := h.TypeOf(key); got != "string" {
				t.Errorf("TYPE of a HyperLogLog = %q, want %q", got, "string")
			}
			raw := h.GetString(key)
			if !strings.HasPrefix(raw, "HYLL") {
				t.Errorf("raw value starts with %q, want the HYLL magic header", raw[:min(4, len(raw))])
			}
		})

		t.Run("RepeatedElementsDoNotInflateCount", func(t *testing.T) {
			repeat := "hll:repeat"
			for range 100 {
				h.SeedHLL(repeat, "same")
			}
			count, err := h.Client.PFCount(ctx, repeat)
			if err != nil {
				t.Fatalf("PFCount: %v", err)
			}
			if count != 1 {
				t.Errorf("PFCount after adding one element 100 times = %d, want 1", count)
			}
		})
	})
}

func TestRenameAndFlush(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("RenamePreservesValueAndTTL", func(t *testing.T) {
			h.SeedStringTTL("rename:old", "payload", 5*time.Minute)
			if err := h.Client.Rename(ctx, "rename:old", "rename:new"); err != nil {
				t.Fatalf("Rename: %v", err)
			}
			if got := h.TypeOf("rename:old"); got != "none" {
				t.Errorf("old key still exists as %q", got)
			}
			if got := h.GetString("rename:new"); got != "payload" {
				t.Errorf("renamed value = %q, want %q", got, "payload")
			}
			ttl, err := h.Client.TTL(ctx, "rename:new")
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			if ttl <= 0 {
				t.Errorf("TTL after rename = %d, want the original expiry to survive", ttl)
			}
		})

		t.Run("RenameMissingKeyFails", func(t *testing.T) {
			if err := h.Client.Rename(ctx, "rename:missing", "rename:whatever"); err == nil {
				t.Error("Rename of a missing key returned no error")
			}
		})

		t.Run("FlushDBEmptiesOnlyThisDatabase", func(t *testing.T) {
			h.SeedKeys("flush:", 10)
			if err := h.Client.FlushDB(ctx); err != nil {
				t.Fatalf("FlushDB: %v", err)
			}
			size, err := h.Client.DBSize(ctx)
			if err != nil {
				t.Fatalf("DBSize: %v", err)
			}
			if size != 0 {
				t.Errorf("DBSize after FLUSHDB = %d, want 0", size)
			}
		})
	})
}

func TestServerIntrospection(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("DBSizeCountsKeys", func(t *testing.T) {
			h.SeedKeys("size:", 7)
			size, err := h.Client.DBSize(ctx)
			if err != nil {
				t.Fatalf("DBSize: %v", err)
			}
			if size != 7 {
				t.Errorf("DBSize = %d, want 7", size)
			}
		})

		t.Run("InfoSections", func(t *testing.T) {
			all, err := h.Client.Info(ctx, "")
			if err != nil {
				t.Fatalf("Info: %v", err)
			}
			if !strings.Contains(all, "# Server") || !strings.Contains(all, "# Memory") {
				t.Error("INFO without a section is missing expected sections")
			}

			memory, err := h.Client.Info(ctx, "memory")
			if err != nil {
				t.Fatalf("Info(memory): %v", err)
			}
			if !strings.Contains(memory, "used_memory:") {
				t.Error("INFO memory is missing used_memory")
			}
			if strings.Contains(memory, "# Server") {
				t.Error("INFO memory returned sections other than Memory")
			}
		})

		t.Run("MemoryStatsParse", func(t *testing.T) {
			stats, err := h.Client.GetMemoryStats(ctx)
			if err != nil {
				t.Fatalf("GetMemoryStats: %v", err)
			}
			if stats.UsedMemory <= 0 {
				t.Errorf("UsedMemory = %d, want a positive byte count", stats.UsedMemory)
			}
			if stats.UsedMemoryHuman == "" {
				t.Error("UsedMemoryHuman is empty; the INFO field name may have changed")
			}
		})

		t.Run("MemoryUsagePerKey", func(t *testing.T) {
			h.SeedString("mem:small", "x")
			h.SeedString("mem:large", strings.Repeat("x", 50_000))

			small, err := h.Client.MemoryUsage(ctx, "mem:small")
			if err != nil {
				t.Fatalf("MemoryUsage: %v", err)
			}
			large, err := h.Client.MemoryUsage(ctx, "mem:large")
			if err != nil {
				t.Fatalf("MemoryUsage: %v", err)
			}
			if small <= 0 || large <= small {
				t.Errorf("MemoryUsage small=%d large=%d, want positive values with large > small", small, large)
			}
		})

		t.Run("MemoryUsageBatchSkipsMissingKeys", func(t *testing.T) {
			h.SeedString("batch:a", "1")
			h.SeedString("batch:b", "2")

			usage, err := h.Client.MemoryUsageBatch(ctx, []string{"batch:a", "batch:b", "batch:missing"})
			if err != nil {
				t.Fatalf("MemoryUsageBatch: %v", err)
			}
			if len(usage) != 2 {
				t.Errorf("batch returned %d entries, want 2 (missing keys are skipped)", len(usage))
			}
			for _, key := range []string{"batch:a", "batch:b"} {
				if usage[key] <= 0 {
					t.Errorf("usage[%q] = %d, want a positive byte count", key, usage[key])
				}
			}
		})

		t.Run("MemoryUsageBatchOnEmptyInput", func(t *testing.T) {
			usage, err := h.Client.MemoryUsageBatch(ctx, nil)
			if err != nil {
				t.Fatalf("MemoryUsageBatch: %v", err)
			}
			if len(usage) != 0 {
				t.Errorf("batch of no keys returned %d entries, want 0", len(usage))
			}
		})
	})
}

func TestSlowLog(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		// slowlog-log-slower-than is server-global, so this needs its own server.
		// Zero means "log every command", which makes the test deterministic.
		h := testenv.New(t, e, testenv.Exclusive("--slowlog-log-slower-than", "0"))
		ctx := h.Ctx()

		h.SeedString("slow:key", "value")

		length, err := h.Client.SlowLogLen(ctx)
		if err != nil {
			t.Fatalf("SlowLogLen: %v", err)
		}
		if length == 0 {
			t.Fatal("slow log is empty even with slowlog-log-slower-than 0")
		}

		entries, err := h.Client.SlowLogGet(ctx, 10)
		if err != nil {
			t.Fatalf("SlowLogGet: %v", err)
		}
		if len(entries) == 0 {
			t.Fatal("SlowLogGet returned no entries")
		}

		for _, entry := range entries {
			if len(entry.Args) == 0 {
				t.Errorf("entry %d has no command args; the SLOWLOG reply shape may have changed", entry.ID)
			}
			if entry.Timestamp <= 0 {
				t.Errorf("entry %d has timestamp %d, want a unix time", entry.ID, entry.Timestamp)
			}
			if entry.Duration < 0 {
				t.Errorf("entry %d has duration %d, want a non-negative microsecond count", entry.ID, entry.Duration)
			}
		}

		// Fields 5 and 6 (client address and name) were added in Redis 4.0 and
		// the API exposes them; make sure both engines still send them.
		if entries[0].ClientAddr == "" {
			t.Error("slow log entry has no client address")
		}
	})
}
