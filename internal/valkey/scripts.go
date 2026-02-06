package valkey

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

// Script represents a Lua script that can be executed atomically
type Script struct {
	script string
	sha1   string
}

// NewScript creates a new Script with the given Lua code
func NewScript(script string) *Script {
	hash := sha1.Sum([]byte(script))
	return &Script{
		script: script,
		sha1:   hex.EncodeToString(hash[:]),
	}
}

// Eval executes the script with the given keys and args
// Uses EVALSHA for efficiency, falls back to EVAL if script not cached
func (s *Script) Eval(ctx context.Context, c *Client, keys []string, args []string) (interface{}, error) {
	// Build EVALSHA command with all keys and args
	allArgs := []string{"EVALSHA", s.sha1, fmt.Sprintf("%d", len(keys))}
	allArgs = append(allArgs, keys...)
	allArgs = append(allArgs, args...)

	result := c.client.Do(ctx, c.client.B().Arbitrary(allArgs...).Build())
	err := result.Error()

	// If script not found, load it with EVAL
	if err != nil && isNoScriptError(err) {
		return s.evalScript(ctx, c, keys, args)
	}

	if err != nil {
		return nil, err
	}

	return result.ToAny()
}

// evalScript executes the script using EVAL (loads and runs in one command)
func (s *Script) evalScript(ctx context.Context, c *Client, keys []string, args []string) (interface{}, error) {
	// Build EVAL command with all keys and args
	allArgs := []string{"EVAL", s.script, fmt.Sprintf("%d", len(keys))}
	allArgs = append(allArgs, keys...)
	allArgs = append(allArgs, args...)

	result := c.client.Do(ctx, c.client.B().Arbitrary(allArgs...).Build())
	if err := result.Error(); err != nil {
		return nil, err
	}

	return result.ToAny()
}

// Load preloads the script on the server using SCRIPT LOAD
// This is optional but can improve performance if the script will be used many times
func (s *Script) Load(ctx context.Context, c *Client) error {
	sha, err := c.client.Do(ctx, c.client.B().ScriptLoad().Script(s.script).Build()).ToString()
	if err != nil {
		return err
	}
	if sha != s.sha1 {
		return fmt.Errorf("script SHA1 mismatch: expected %s, got %s", s.sha1, sha)
	}
	return nil
}

// isNoScriptError checks if the error is a "NOSCRIPT" error from Redis/Valkey
func isNoScriptError(err error) bool {
	if err == nil {
		return false
	}
	// Valkey/Redis returns "NOSCRIPT No matching script." or similar
	errMsg := err.Error()
	return errMsg == "NOSCRIPT No matching script. Please use EVAL." ||
		errMsg == "NOSCRIPT No matching script."
}

// Built-in scripts

var (
	// scriptListRemoveByIndex atomically removes a list element at a specific index
	// KEYS[1] = key name
	// ARGV[1] = index to remove
	// ARGV[2] = tombstone suffix (for uniqueness)
	// Returns: 1 on success, 0 if key doesn't exist or wrong type
	scriptListRemoveByIndex = NewScript(`
		local key = KEYS[1]
		local index = tonumber(ARGV[1])
		local tombstone = "__KVWEB_TOMBSTONE_" .. ARGV[2]

		-- Check if key exists and is a list
		if redis.call('EXISTS', key) == 0 then
			return 0
		end
		if redis.call('TYPE', key)['ok'] ~= 'list' then
			return 0
		end

		-- Set tombstone at index, then remove it
		local ok, err = pcall(function()
			redis.call('LSET', key, index, tombstone)
			redis.call('LREM', key, 1, tombstone)
		end)

		if not ok then
			return 0
		end

		return 1
	`)

	// scriptSetAddIfNotExists atomically adds a member to a set only if it doesn't exist
	// KEYS[1] = key name
	// ARGV[1] = member to add
	// Returns: 1 if added, 0 if already exists
	scriptSetAddIfNotExists = NewScript(`
		local key = KEYS[1]
		local member = ARGV[1]

		-- Check if member already exists
		if redis.call('SISMEMBER', key, member) == 1 then
			return 0
		end

		-- Add member
		redis.call('SADD', key, member)
		return 1
	`)

	// scriptSetRename atomically renames a set member (removes old, adds new)
	// KEYS[1] = key name
	// ARGV[1] = old member value
	// ARGV[2] = new member value
	// Returns: 1 on success, error if old member doesn't exist or new member already exists
	scriptSetRename = NewScript(`
		local key = KEYS[1]
		local oldMember = ARGV[1]
		local newMember = ARGV[2]

		-- Check if old member exists
		if redis.call('SISMEMBER', key, oldMember) == 0 then
			return redis.error_reply('Member does not exist')
		end

		-- Check if new member already exists
		if redis.call('SISMEMBER', key, newMember) == 1 then
			return redis.error_reply('New member already exists')
		end

		-- Remove old member and add new member
		redis.call('SREM', key, oldMember)
		redis.call('SADD', key, newMember)

		return 1
	`)

	// scriptZSetRename atomically renames a sorted set member
	// KEYS[1] = key name
	// ARGV[1] = old member name
	// ARGV[2] = new member name
	// Returns: score on success, nil if old member doesn't exist, error if new member exists
	scriptZSetRename = NewScript(`
		local key = KEYS[1]
		local oldMember = ARGV[1]
		local newMember = ARGV[2]

		-- Get score of old member
		local score = redis.call('ZSCORE', key, oldMember)
		if not score then
			return redis.error_reply('Member does not exist')
		end

		-- Check if new member already exists
		if redis.call('ZRANK', key, newMember) then
			return redis.error_reply('New member already exists')
		end

		-- Remove old member and add with new name
		redis.call('ZREM', key, oldMember)
		redis.call('ZADD', key, score, newMember)

		return score
	`)

	// scriptHashRename atomically renames a hash field
	// KEYS[1] = key name
	// ARGV[1] = old field name
	// ARGV[2] = new field name
	// Returns: value on success, nil if old field doesn't exist, error if new field exists
	scriptHashRename = NewScript(`
		local key = KEYS[1]
		local oldField = ARGV[1]
		local newField = ARGV[2]

		-- Get value of old field
		local value = redis.call('HGET', key, oldField)
		if not value then
			return redis.error_reply('Field does not exist')
		end

		-- Check if new field already exists
		if redis.call('HEXISTS', key, newField) == 1 then
			return redis.error_reply('New field already exists')
		end

		-- Delete old field and set new field
		redis.call('HDEL', key, oldField)
		redis.call('HSET', key, newField, value)

		return value
	`)

	// scriptGetKeyMetadata atomically gets key type, size, and TTL
	// KEYS[1] = key name
	// Returns: {type, size, ttl} or nil if key doesn't exist
	scriptGetKeyMetadata = NewScript(`
		local key = KEYS[1]

		-- Check if key exists
		if redis.call('EXISTS', key) == 0 then
			return nil
		end

		local ktype = redis.call('TYPE', key)['ok']
		local size = 0
		local ttl = redis.call('TTL', key)

		-- Get size based on type
		if ktype == 'string' then
			local val = redis.call('GET', key)
			if val then
				size = string.len(val)
			end
		elseif ktype == 'list' then
			size = redis.call('LLEN', key)
		elseif ktype == 'set' then
			size = redis.call('SCARD', key)
		elseif ktype == 'hash' then
			size = redis.call('HLEN', key)
		elseif ktype == 'zset' then
			size = redis.call('ZCARD', key)
		elseif ktype == 'stream' then
			size = redis.call('XLEN', key)
		end

		return {ktype, size, ttl}
	`)
)

// LoadAllScripts preloads all built-in scripts on the server
// This is optional but improves performance by avoiding EVAL fallback
func LoadAllScripts(ctx context.Context, c *Client) error {
	scripts := []*Script{
		scriptListRemoveByIndex,
		scriptSetAddIfNotExists,
		scriptSetRename,
		scriptZSetRename,
		scriptHashRename,
		scriptGetKeyMetadata,
	}

	for _, script := range scripts {
		if err := script.Load(ctx, c); err != nil {
			return fmt.Errorf("failed to load script: %w", err)
		}
	}

	return nil
}
