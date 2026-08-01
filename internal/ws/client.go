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

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
	}
}

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

// ReadPump exists to notice when the peer goes away; the frames themselves are
// discarded.
func (c *Client) ReadPump(ctx context.Context) {
	defer c.hub.Unregister(c)
	c.conn.SetReadLimit(4096) // We don't process incoming messages; cap to prevent abuse

	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			break
		}
	}
}

func (c *Client) Send(data []byte) bool {
	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}
