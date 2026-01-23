package valkey

import (
	"context"
	"fmt"
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
