package valkey_test

import (
	"slices"
	"testing"
	"time"

	"github.com/natrimmer/kvweb/internal/testenv"
	"github.com/natrimmer/kvweb/internal/valkey"
)

// The API layer maps these Lua error replies onto 404 and 409 responses by
// comparing err.Error() literally, so their exact text is part of the contract
// between scripts.go and api.go.
const (
	errMemberMissing   = "Member does not exist"
	errMemberDuplicate = "New member already exists"
	errFieldMissing    = "Field does not exist"
	errFieldDuplicate  = "New field already exists"
)

func TestScriptSetAddIfNotExists(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()
		const key = "script:set"

		added, err := h.Client.SAddIfNotExists(ctx, key, "member1")
		if err != nil {
			t.Fatalf("SAddIfNotExists: %v", err)
		}
		if !added {
			t.Error("first add reported not added")
		}

		added, err = h.Client.SAddIfNotExists(ctx, key, "member1")
		if err != nil {
			t.Fatalf("SAddIfNotExists on a duplicate: %v", err)
		}
		if added {
			t.Error("duplicate add reported as added")
		}

		added, err = h.Client.SAddIfNotExists(ctx, key, "member2")
		if err != nil {
			t.Fatalf("SAddIfNotExists on a new member: %v", err)
		}
		if !added {
			t.Error("distinct member reported not added")
		}

		members := h.SetMembers(key)
		slices.Sort(members)
		if !slices.Equal(members, []string{"member1", "member2"}) {
			t.Errorf("members = %v, want [member1 member2]", members)
		}
	})
}

func TestScriptSetRename(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()
		const key = "script:set-rename"

		h.SeedSet(key, "old", "other")

		if err := h.Client.SRename(ctx, key, "old", "new"); err != nil {
			t.Fatalf("SRename: %v", err)
		}
		members := h.SetMembers(key)
		slices.Sort(members)
		if !slices.Equal(members, []string{"new", "other"}) {
			t.Errorf("members = %v, want [new other]", members)
		}

		t.Run("MissingMember", func(t *testing.T) {
			err := h.Client.SRename(ctx, key, "ghost", "whatever")
			if err == nil {
				t.Fatal("renaming a member that does not exist returned no error")
			}
			if err.Error() != errMemberMissing {
				t.Errorf("error = %q, want exactly %q (api.go matches this string)", err, errMemberMissing)
			}
		})

		t.Run("DuplicateTarget", func(t *testing.T) {
			err := h.Client.SRename(ctx, key, "new", "other")
			if err == nil {
				t.Fatal("renaming onto an existing member returned no error")
			}
			if err.Error() != errMemberDuplicate {
				t.Errorf("error = %q, want exactly %q (api.go matches this string)", err, errMemberDuplicate)
			}
		})

		t.Run("SetIsUnchangedAfterFailures", func(t *testing.T) {
			members := h.SetMembers(key)
			slices.Sort(members)
			if !slices.Equal(members, []string{"new", "other"}) {
				t.Errorf("members = %v after failed renames, want [new other]", members)
			}
		})
	})
}

func TestScriptZSetRename(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()
		const key = "script:zset-rename"

		h.SeedZSet(key, map[string]float64{"alice": 100.5, "bob": 200})

		score, err := h.Client.ZRename(ctx, key, "alice", "alice_new")
		if err != nil {
			t.Fatalf("ZRename: %v", err)
		}
		if score != 100.5 {
			t.Errorf("returned score = %v, want 100.5", score)
		}

		members := h.ZSetMembers(key)
		want := []valkey.ZMember{{Member: "alice_new", Score: 100.5}, {Member: "bob", Score: 200}}
		if len(members) != 2 {
			t.Fatalf("zset has %d members, want 2", len(members))
		}
		for i := range want {
			if members[i] != want[i] {
				t.Errorf("member %d = %+v, want %+v", i, members[i], want[i])
			}
		}

		t.Run("MissingMember", func(t *testing.T) {
			_, err := h.Client.ZRename(ctx, key, "ghost", "whatever")
			if err == nil {
				t.Fatal("renaming a member that does not exist returned no error")
			}
			if err.Error() != errMemberMissing {
				t.Errorf("error = %q, want exactly %q (api.go matches this string)", err, errMemberMissing)
			}
		})

		t.Run("DuplicateTarget", func(t *testing.T) {
			_, err := h.Client.ZRename(ctx, key, "alice_new", "bob")
			if err == nil {
				t.Fatal("renaming onto an existing member returned no error")
			}
			if err.Error() != errMemberDuplicate {
				t.Errorf("error = %q, want exactly %q (api.go matches this string)", err, errMemberDuplicate)
			}
		})

		t.Run("ZeroScoreMemberCanBeRenamed", func(t *testing.T) {
			// ZSCORE returns "0" here and ZRANK returns 0 for the first member;
			// both are truthy in Lua, but a translation to Go booleans would break.
			zero := "script:zset-zero"
			h.SeedZSet(zero, map[string]float64{"first": 0})
			score, err := h.Client.ZRename(ctx, zero, "first", "renamed")
			if err != nil {
				t.Fatalf("ZRename of a zero-score member: %v", err)
			}
			if score != 0 {
				t.Errorf("returned score = %v, want 0", score)
			}
		})

		t.Run("NegativeAndFractionalScoresSurvive", func(t *testing.T) {
			// The script hands the score back as a string that Go re-parses.
			fractional := "script:zset-fractional"
			h.SeedZSet(fractional, map[string]float64{"m": -12.75})
			score, err := h.Client.ZRename(ctx, fractional, "m", "m2")
			if err != nil {
				t.Fatalf("ZRename: %v", err)
			}
			if score != -12.75 {
				t.Errorf("returned score = %v, want -12.75", score)
			}
			if members := h.ZSetMembers(fractional); len(members) != 1 || members[0].Score != -12.75 {
				t.Errorf("stored members = %+v, want one member scored -12.75", members)
			}
		})
	})
}

func TestScriptHashRename(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()
		const key = "script:hash-rename"

		h.SeedHash(key, map[string]string{"name": "Alice", "age": "30"})

		value, err := h.Client.HRename(ctx, key, "name", "full_name")
		if err != nil {
			t.Fatalf("HRename: %v", err)
		}
		if value != "Alice" {
			t.Errorf("returned value = %q, want %q", value, "Alice")
		}

		fields := h.HashFields(key)
		if _, ok := fields["name"]; ok {
			t.Error("old field still exists")
		}
		if fields["full_name"] != "Alice" {
			t.Errorf("full_name = %q, want %q", fields["full_name"], "Alice")
		}

		t.Run("MissingField", func(t *testing.T) {
			_, err := h.Client.HRename(ctx, key, "ghost", "whatever")
			if err == nil {
				t.Fatal("renaming a field that does not exist returned no error")
			}
			if err.Error() != errFieldMissing {
				t.Errorf("error = %q, want exactly %q (api.go matches this string)", err, errFieldMissing)
			}
		})

		t.Run("DuplicateTarget", func(t *testing.T) {
			_, err := h.Client.HRename(ctx, key, "full_name", "age")
			if err == nil {
				t.Fatal("renaming onto an existing field returned no error")
			}
			if err.Error() != errFieldDuplicate {
				t.Errorf("error = %q, want exactly %q (api.go matches this string)", err, errFieldDuplicate)
			}
		})

		t.Run("EmptyValueIsNotMistakenForMissing", func(t *testing.T) {
			// HGET returns "" here, which is truthy in Lua but falsy in many
			// other languages; the script must still treat the field as present.
			empty := "script:hash-empty"
			h.SeedHash(empty, map[string]string{"blank": ""})
			value, err := h.Client.HRename(ctx, empty, "blank", "renamed")
			if err != nil {
				t.Fatalf("HRename of an empty-valued field: %v", err)
			}
			if value != "" {
				t.Errorf("returned value = %q, want an empty string", value)
			}
			if _, ok := h.HashFields(empty)["renamed"]; !ok {
				t.Error("renamed field is missing")
			}
		})
	})
}

func TestScriptGetKeyMetadata(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)
		ctx := h.Ctx()

		t.Run("MissingKeyReturnsNil", func(t *testing.T) {
			// The script returns Lua nil, which surfaces as a "valkey nil
			// message" error the client translates back into (nil, nil).
			meta, err := h.Client.GetKeyMetadata(ctx, "meta:missing")
			if err != nil {
				t.Fatalf("GetKeyMetadata: %v", err)
			}
			if meta != nil {
				t.Errorf("metadata = %+v for a missing key, want nil", meta)
			}
		})

		keys := h.SeedOneOfEachType("meta")

		sizes := map[string]int64{
			"string":      5, // "hello"
			"list":        3,
			"set":         3,
			"hash":        1,
			"zset":        2,
			"stream":      1,
			"hyperloglog": 0, // reported as a string; size is the raw byte length
		}

		for _, typeName := range []string{"string", "list", "set", "hash", "zset", "stream"} {
			t.Run(typeName, func(t *testing.T) {
				meta, err := h.Client.GetKeyMetadata(ctx, keys[typeName])
				if err != nil {
					t.Fatalf("GetKeyMetadata: %v", err)
				}
				if meta == nil {
					t.Fatal("metadata is nil for a key that exists")
				}
				if meta.Type != typeName {
					t.Errorf("type = %q, want %q", meta.Type, typeName)
				}
				if meta.Size != sizes[typeName] {
					t.Errorf("size = %d, want %d", meta.Size, sizes[typeName])
				}
				if meta.TTL != -1 {
					t.Errorf("ttl = %d, want -1 for a key with no expiry", meta.TTL)
				}
			})
		}

		t.Run("HyperLogLogReportsAsString", func(t *testing.T) {
			meta, err := h.Client.GetKeyMetadata(ctx, keys["hyperloglog"])
			if err != nil {
				t.Fatalf("GetKeyMetadata: %v", err)
			}
			if meta == nil {
				t.Fatal("metadata is nil for a HyperLogLog key")
			}
			if meta.Type != "string" {
				t.Errorf("type = %q, want %q", meta.Type, "string")
			}
			if meta.Size <= 0 {
				t.Errorf("size = %d, want the raw byte length", meta.Size)
			}
		})

		t.Run("TTLIsReported", func(t *testing.T) {
			h.SeedStringTTL("meta:ttl", "v", 60*time.Second)
			meta, err := h.Client.GetKeyMetadata(ctx, "meta:ttl")
			if err != nil {
				t.Fatalf("GetKeyMetadata: %v", err)
			}
			if meta == nil {
				t.Fatal("metadata is nil")
			}
			if meta.TTL <= 0 || meta.TTL > 60 {
				t.Errorf("ttl = %d, want a value in (0, 60]", meta.TTL)
			}
		})
	})
}

func TestScriptLoadingAndEvalShaFallback(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		// SCRIPT FLUSH wipes the server-wide script cache, so this needs its own
		// server or it would break scripts other tests are relying on.
		h := testenv.New(t, e, testenv.Exclusive())
		ctx := h.Ctx()

		t.Run("LoadAllScripts", func(t *testing.T) {
			// Each script's SHA1 is computed in Go; a mismatch here would mean
			// EVALSHA can never hit and every call silently falls back to EVAL.
			if err := valkey.LoadAllScripts(ctx, h.Client); err != nil {
				t.Fatalf("LoadAllScripts: %v", err)
			}
		})

		t.Run("EvalShaWorksAfterLoad", func(t *testing.T) {
			h.SeedSet("fallback:set", "a")
			if added, err := h.Client.SAddIfNotExists(ctx, "fallback:set", "b"); err != nil || !added {
				t.Fatalf("SAddIfNotExists after load: added=%v err=%v", added, err)
			}
		})

		t.Run("FallsBackToEvalAfterScriptFlush", func(t *testing.T) {
			// scripts.go recognises the NOSCRIPT reply by exact text. If either
			// engine reworded it, every script call would start failing outright
			// instead of falling back, so assert the fallback actually happens.
			if _, err := h.Client.Exec(ctx, []string{"SCRIPT", "FLUSH"}); err != nil {
				t.Fatalf("SCRIPT FLUSH: %v", err)
			}

			added, err := h.Client.SAddIfNotExists(ctx, "fallback:set", "c")
			if err != nil {
				t.Fatalf("SAddIfNotExists after SCRIPT FLUSH did not fall back to EVAL: %v", err)
			}
			if !added {
				t.Error("member was not added after the EVAL fallback")
			}
		})
	})
}
