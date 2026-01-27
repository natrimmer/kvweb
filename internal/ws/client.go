package ws

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Send buffer size
	sendBufferSize = 256
)

// Client represents a WebSocket client connection
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// NewClient creates a new Client
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump(ctx context.Context) {
	defer func() {
		_ = c.conn.CloseNow()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Hub closed the channel
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// ReadPump reads messages from the WebSocket connection (mainly to detect disconnects)
func (c *Client) ReadPump(ctx context.Context) {
	defer c.hub.Unregister(c)

	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			break
		}
		// We don't process incoming messages currently
	}
}

// Send queues a message to be sent to this client
func (c *Client) Send(data []byte) bool {
	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}
