package valkey

import (
	"context"
	"testing"
	"time"

	"github.com/natrimmer/kvweb/internal/config"
)

// TestLuaScripts tests all Lua script operations
// This requires a running Valkey/Redis instance
func TestLuaScripts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cfg := &config.Config{
		ValkeyURL: "localhost:6379",
		ValkeyDB:  15, // Use DB 15 for testing
	}

	client, err := New(cfg)
	if err != nil {
		t.Skip("Valkey not available:", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Clean up test keys
	defer func() {
		_, _ = client.Del(ctx, "test:list", "test:set", "test:zset", "test:hash", "test:meta")
	}()

	t.Run("ListRemoveByIndex", func(t *testing.T) {
		// Setup: Create a list with 5 elements
		key := "test:list"
		_, _ = client.Del(ctx, key)
		if err := client.RPush(ctx, key, "a", "b", "c", "d", "e"); err != nil {
			t.Fatalf("RPush failed: %v", err)
		}

		// Remove element at index 2 (value "c")
		err := client.LRemByIndex(ctx, key, 2)
		if err != nil {
			t.Fatalf("LRemByIndex failed: %v", err)
		}

		// Verify the list is now [a, b, d, e]
		items, err := client.LRange(ctx, key, 0, -1)
		if err != nil {
			t.Fatalf("LRange failed: %v", err)
		}

		expected := []string{"a", "b", "d", "e"}
		if len(items) != len(expected) {
			t.Fatalf("expected %d items, got %d", len(expected), len(items))
		}
		for i, item := range items {
			if item != expected[i] {
				t.Errorf("expected items[%d] = %q, got %q", i, expected[i], item)
			}
		}
	})

	t.Run("SetAddIfNotExists", func(t *testing.T) {
		key := "test:set"
		_, _ = client.Del(ctx, key)

		// First add should succeed
		added, err := client.SAddIfNotExists(ctx, key, "member1")
		if err != nil {
			t.Fatalf("SAddIfNotExists failed: %v", err)
		}
		if !added {
			t.Error("expected first add to succeed")
		}

		// Second add of same member should fail
		added, err = client.SAddIfNotExists(ctx, key, "member1")
		if err != nil {
			t.Fatalf("SAddIfNotExists failed: %v", err)
		}
		if added {
			t.Error("expected duplicate add to fail")
		}

		// Different member should succeed
		added, err = client.SAddIfNotExists(ctx, key, "member2")
		if err != nil {
			t.Fatalf("SAddIfNotExists failed: %v", err)
		}
		if !added {
			t.Error("expected different member add to succeed")
		}
	})

	t.Run("ZSetRename", func(t *testing.T) {
		key := "test:zset"
		_, _ = client.Del(ctx, key)

		// Setup: Add a member with score
		if err := client.ZAdd(ctx, key, "alice", 100.5); err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}
		if err := client.ZAdd(ctx, key, "bob", 200.0); err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}

		// Rename alice to alice_new
		score, err := client.ZRename(ctx, key, "alice", "alice_new")
		if err != nil {
			t.Fatalf("ZRename failed: %v", err)
		}
		if score != 100.5 {
			t.Errorf("expected score 100.5, got %v", score)
		}

		// Verify alice is gone and alice_new exists with same score
		members, err := client.ZRangeWithScores(ctx, key, 0, -1)
		if err != nil {
			t.Fatalf("ZRangeWithScores failed: %v", err)
		}

		found := false
		for _, m := range members {
			if m.Member == "alice" {
				t.Error("old member 'alice' still exists")
			}
			if m.Member == "alice_new" {
				found = true
				if m.Score != 100.5 {
					t.Errorf("expected score 100.5, got %v", m.Score)
				}
			}
		}
		if !found {
			t.Error("new member 'alice_new' not found")
		}

		// Try to rename to existing member (should fail)
		_, err = client.ZRename(ctx, key, "alice_new", "bob")
		if err == nil {
			t.Error("expected error when renaming to existing member")
		}
	})

	t.Run("HashRename", func(t *testing.T) {
		key := "test:hash"
		_, _ = client.Del(ctx, key)

		// Setup: Add fields
		if err := client.HSet(ctx, key, "name", "Alice"); err != nil {
			t.Fatalf("HSet failed: %v", err)
		}
		if err := client.HSet(ctx, key, "age", "30"); err != nil {
			t.Fatalf("HSet failed: %v", err)
		}

		// Rename name to full_name
		value, err := client.HRename(ctx, key, "name", "full_name")
		if err != nil {
			t.Fatalf("HRename failed: %v", err)
		}
		if value != "Alice" {
			t.Errorf("expected value 'Alice', got %q", value)
		}

		// Verify name is gone and full_name exists
		fields, err := client.HGetAll(ctx, key)
		if err != nil {
			t.Fatalf("HGetAll failed: %v", err)
		}

		if _, exists := fields["name"]; exists {
			t.Error("old field 'name' still exists")
		}
		if v, exists := fields["full_name"]; !exists {
			t.Error("new field 'full_name' not found")
		} else if v != "Alice" {
			t.Errorf("expected value 'Alice', got %q", v)
		}

		// Try to rename to existing field (should fail)
		_, err = client.HRename(ctx, key, "full_name", "age")
		if err == nil {
			t.Error("expected error when renaming to existing field")
		}
	})

	t.Run("GetKeyMetadata", func(t *testing.T) {
		key := "test:meta"
		_, _ = client.Del(ctx, key)

		// Test with non-existent key
		meta, err := client.GetKeyMetadata(ctx, key)
		if err != nil {
			t.Fatalf("GetKeyMetadata failed: %v", err)
		}
		if meta != nil {
			t.Error("expected nil metadata for non-existent key")
		}

		// Create a list with 3 items and TTL
		if err := client.RPush(ctx, key, "a", "b", "c"); err != nil {
			t.Fatalf("RPush failed: %v", err)
		}
		if _, err := client.Expire(ctx, key, 60*time.Second); err != nil {
			t.Fatalf("Expire failed: %v", err)
		}

		// Get metadata
		meta, err = client.GetKeyMetadata(ctx, key)
		if err != nil {
			t.Fatalf("GetKeyMetadata failed: %v", err)
		}
		if meta == nil {
			t.Fatal("expected metadata, got nil")
		}

		if meta.Type != "list" {
			t.Errorf("expected type 'list', got %q", meta.Type)
		}
		if meta.Size != 3 {
			t.Errorf("expected size 3, got %d", meta.Size)
		}
		if meta.TTL <= 0 || meta.TTL > 60 {
			t.Errorf("expected TTL around 60, got %d", meta.TTL)
		}
	})
}
