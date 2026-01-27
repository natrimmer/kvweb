package valkey

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gnat/kvweb/internal/config"
	"github.com/valkey-io/valkey-go"
)

// Client wraps the Valkey client with application-specific methods
type Client struct {
	client valkey.Client
	cfg    *config.Config
}

// New creates a new Valkey client
func New(cfg *config.Config) (*Client, error) {
	opts := valkey.ClientOption{
		InitAddress: []string{cfg.ValkeyURL},
	}

	if cfg.ValkeyPassword != "" {
		opts.Password = cfg.ValkeyPassword
	}

	if cfg.ValkeyDB != 0 {
		opts.SelectDB = cfg.ValkeyDB
	}

	client, err := valkey.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping server: %w", err)
	}

	return &Client{
		client: client,
		cfg:    cfg,
	}, nil
}

// Close closes the client connection
func (c *Client) Close() {
	c.client.Close()
}

// Raw returns the underlying valkey client for direct access
func (c *Client) Raw() valkey.Client {
	return c.client
}

// Ping tests the connection
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Do(ctx, c.client.B().Ping().Build()).Error()
}

// Info returns server information
func (c *Client) Info(ctx context.Context, section string) (string, error) {
	cmd := c.client.B().Info()
	if section != "" {
		cmd.Section(section)
	}
	return c.client.Do(ctx, cmd.Build()).ToString()
}

// DBSize returns the number of keys in the current database
func (c *Client) DBSize(ctx context.Context) (int64, error) {
	return c.client.Do(ctx, c.client.B().Dbsize().Build()).ToInt64()
}

// Keys returns keys matching the pattern
func (c *Client) Keys(ctx context.Context, pattern string, cursor uint64, count int64) ([]string, uint64, error) {
	result := c.client.Do(ctx, c.client.B().Scan().Cursor(cursor).Match(pattern).Count(count).Build())
	entry, err := result.AsScanEntry()
	if err != nil {
		return nil, 0, err
	}
	return entry.Elements, entry.Cursor, nil
}

// Get returns the value of a key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Do(ctx, c.client.B().Get().Key(key).Build()).ToString()
}

// Set sets the value of a key
func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	cmd := c.client.B().Set().Key(key).Value(value)
	if ttl > 0 {
		cmd.Ex(ttl)
	}
	return c.client.Do(ctx, cmd.Build()).Error()
}

// Del deletes keys
func (c *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Do(ctx, c.client.B().Del().Key(keys...).Build()).ToInt64()
}

// Type returns the type of a key
func (c *Client) Type(ctx context.Context, key string) (string, error) {
	return c.client.Do(ctx, c.client.B().Type().Key(key).Build()).ToString()
}

// TTL returns the TTL of a key in seconds (-1 if no TTL, -2 if key doesn't exist)
func (c *Client) TTL(ctx context.Context, key string) (int64, error) {
	return c.client.Do(ctx, c.client.B().Ttl().Key(key).Build()).ToInt64()
}

// Expire sets a TTL on a key
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	result, err := c.client.Do(ctx, c.client.B().Expire().Key(key).Seconds(int64(ttl.Seconds())).Build()).ToInt64()
	return result == 1, err
}

// Persist removes the TTL from a key
func (c *Client) Persist(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Do(ctx, c.client.B().Persist().Key(key).Build()).ToInt64()
	return result == 1, err
}

// Rename renames a key
func (c *Client) Rename(ctx context.Context, key, newkey string) error {
	return c.client.Do(ctx, c.client.B().Rename().Key(key).Newkey(newkey).Build()).Error()
}

// FlushDB deletes all keys in the current database
func (c *Client) FlushDB(ctx context.Context) error {
	return c.client.Do(ctx, c.client.B().Flushdb().Build()).Error()
}

// List operations

// LLen returns the length of a list
func (c *Client) LLen(ctx context.Context, key string) (int64, error) {
	return c.client.Do(ctx, c.client.B().Llen().Key(key).Build()).ToInt64()
}

// LRange returns elements from a list
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.Do(ctx, c.client.B().Lrange().Key(key).Start(start).Stop(stop).Build()).AsStrSlice()
}

// Set operations

// SCard returns the number of members in a set
func (c *Client) SCard(ctx context.Context, key string) (int64, error) {
	return c.client.Do(ctx, c.client.B().Scard().Key(key).Build()).ToInt64()
}

// SMembers returns all members of a set
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.client.Do(ctx, c.client.B().Smembers().Key(key).Build()).AsStrSlice()
}

// SScan returns members of a set using cursor-based pagination
func (c *Client) SScan(ctx context.Context, key string, cursor uint64, count int64) ([]string, uint64, error) {
	result := c.client.Do(ctx, c.client.B().Sscan().Key(key).Cursor(cursor).Count(count).Build())
	entry, err := result.AsScanEntry()
	if err != nil {
		return nil, 0, err
	}
	return entry.Elements, entry.Cursor, nil
}

// Hash operations

// HLen returns the number of fields in a hash
func (c *Client) HLen(ctx context.Context, key string) (int64, error) {
	return c.client.Do(ctx, c.client.B().Hlen().Key(key).Build()).ToInt64()
}

// HGetAll returns all fields and values in a hash
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.Do(ctx, c.client.B().Hgetall().Key(key).Build()).AsStrMap()
}

// HScan returns fields and values of a hash using cursor-based pagination
func (c *Client) HScan(ctx context.Context, key string, cursor uint64, count int64) (map[string]string, uint64, error) {
	result := c.client.Do(ctx, c.client.B().Hscan().Key(key).Cursor(cursor).Count(count).Build())
	entry, err := result.AsScanEntry()
	if err != nil {
		return nil, 0, err
	}
	// Convert flat slice [field1, value1, field2, value2, ...] to map
	m := make(map[string]string)
	for i := 0; i < len(entry.Elements); i += 2 {
		if i+1 < len(entry.Elements) {
			m[entry.Elements[i]] = entry.Elements[i+1]
		}
	}
	return m, entry.Cursor, nil
}

// Sorted set operations

// ZCard returns the number of members in a sorted set
func (c *Client) ZCard(ctx context.Context, key string) (int64, error) {
	return c.client.Do(ctx, c.client.B().Zcard().Key(key).Build()).ToInt64()
}

// ZMember represents a member with score in a sorted set
type ZMember struct {
	Member string  `json:"member"`
	Score  float64 `json:"score"`
}

// ZRangeWithScores returns members with scores from a sorted set
func (c *Client) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZMember, error) {
	result, err := c.client.Do(ctx, c.client.B().Zrange().Key(key).Min(toString(start)).Max(toString(stop)).Withscores().Build()).AsZScores()
	if err != nil {
		return nil, err
	}
	members := make([]ZMember, len(result))
	for i, z := range result {
		members[i] = ZMember{Member: z.Member, Score: z.Score}
	}
	return members, nil
}

func toString(i int64) string {
	return strconv.FormatInt(i, 10)
}

// Stream operations

// XLen returns the number of entries in a stream
func (c *Client) XLen(ctx context.Context, key string) (int64, error) {
	return c.client.Do(ctx, c.client.B().Xlen().Key(key).Build()).ToInt64()
}

// StreamEntry represents an entry in a stream
type StreamEntry struct {
	ID     string            `json:"id"`
	Fields map[string]string `json:"fields"`
}

// XRange returns entries from a stream
func (c *Client) XRange(ctx context.Context, key, start, stop string, count int64) ([]StreamEntry, error) {
	cmd := c.client.B().Xrange().Key(key).Start(start).End(stop)
	if count > 0 {
		cmd.Count(count)
	}
	result, err := c.client.Do(ctx, cmd.Build()).AsXRange()
	if err != nil {
		return nil, err
	}
	entries := make([]StreamEntry, len(result))
	for i, e := range result {
		entries[i] = StreamEntry{ID: e.ID, Fields: e.FieldValues}
	}
	return entries, nil
}

// Config operations

// GetNotifyKeyspaceEvents returns the current notify-keyspace-events setting
func (c *Client) GetNotifyKeyspaceEvents(ctx context.Context) (string, error) {
	result, err := c.client.Do(ctx, c.client.B().ConfigGet().Parameter("notify-keyspace-events").Build()).AsStrMap()
	if err != nil {
		return "", err
	}
	return result["notify-keyspace-events"], nil
}

// SetNotifyKeyspaceEvents enables/disables keyspace notifications
func (c *Client) SetNotifyKeyspaceEvents(ctx context.Context, value string) error {
	return c.client.Do(ctx, c.client.B().ConfigSet().ParameterValue().ParameterValue("notify-keyspace-events", value).Build()).Error()
}
