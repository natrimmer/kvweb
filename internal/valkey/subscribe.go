package valkey

import (
	"context"
	"fmt"
	"strings"

	"github.com/valkey-io/valkey-go"
)

// KeyEvent represents a keyspace notification event
type KeyEvent struct {
	Operation string // "set", "del", "expire", "expired", "rename_from", "rename_to", etc.
	Key       string
}

// SubscribeKeyspace subscribes to keyspace notifications for a specific database.
// Returns a channel that emits KeyEvent for each key operation.
// The channel is closed when the context is cancelled or an error occurs.
func (c *Client) SubscribeKeyspace(ctx context.Context, db int) (<-chan KeyEvent, error) {
	events := make(chan KeyEvent, 100)

	// Subscribe to __keyspace@{db}__:* pattern
	pattern := fmt.Sprintf("__keyspace@%d__:*", db)
	prefix := fmt.Sprintf("__keyspace@%d__:", db)

	go func() {
		defer close(events)

		err := c.client.Receive(ctx, c.client.B().Psubscribe().Pattern(pattern).Build(),
			func(msg valkey.PubSubMessage) {
				// Channel format: __keyspace@0__:mykey
				// Message: set, del, expire, expired, rename_from, rename_to, etc.
				key := strings.TrimPrefix(msg.Channel, prefix)
				select {
				case events <- KeyEvent{
					Operation: msg.Message,
					Key:       key,
				}:
				case <-ctx.Done():
					return
				}
			})
		// On error, channel closes via defer; err is intentionally ignored
		// when context is cancelled (normal shutdown)
		_ = err
	}()

	return events, nil
}
