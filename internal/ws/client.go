package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// DefaultURL is the default WebSocket server URL.
	DefaultURL = "wss://chiefloop.com/ws/server"

	// maxBackoff is the maximum reconnection delay.
	maxBackoff = 60 * time.Second

	// initialBackoff is the starting reconnection delay.
	initialBackoff = 1 * time.Second

	// receiveBufSize is the buffer size for the receive channel.
	receiveBufSize = 256
)

// Message represents a WebSocket protocol message.
type Message struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// Client is a WebSocket client with automatic reconnection.
type Client struct {
	url     string
	conn    *websocket.Conn
	mu      sync.Mutex
	recvCh  chan Message
	done    chan struct{}
	onRecon func() // called after each successful reconnection

	// For clean shutdown
	cancel  context.CancelFunc
	stopped bool
}

// Option configures a Client.
type Option func(*Client)

// WithOnReconnect sets a callback invoked after each successful reconnection.
// The serve command uses this to re-send a full state snapshot.
func WithOnReconnect(fn func()) Option {
	return func(c *Client) {
		c.onRecon = fn
	}
}

// New creates a new WebSocket client for the given URL.
// The client does not connect until Connect is called.
func New(url string, opts ...Option) *Client {
	c := &Client{
		url:    url,
		recvCh: make(chan Message, receiveBufSize),
		done:   make(chan struct{}),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Connect dials the WebSocket server and starts read/write loops.
// It blocks until the initial connection succeeds or the context is cancelled.
func (c *Client) Connect(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)

	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// Set up ping/pong handler — respond to server pings automatically.
	conn.SetPongHandler(func(string) error { return nil })
	conn.SetPingHandler(func(appData string) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.conn != nil {
			return c.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
		}
		return nil
	})

	go c.readLoop(ctx)

	return nil
}

// Send sends a message over the WebSocket connection.
// Returns an error if the connection is closed.
func (c *Client) Send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Receive returns a channel that delivers incoming messages.
// The channel is closed when the client shuts down.
func (c *Client) Receive() <-chan Message {
	return c.recvCh
}

// Close gracefully shuts down the WebSocket client.
// It sends a close frame to the server and stops all goroutines.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	c.stopped = true
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	var err error
	if conn != nil {
		// Send close frame.
		deadline := time.Now().Add(5 * time.Second)
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		_ = conn.WriteControl(websocket.CloseMessage, closeMsg, deadline)
		err = conn.Close()
	}

	// Wait for readLoop to finish.
	<-c.done

	return err
}

// dial connects to the WebSocket server, retrying with exponential backoff.
func (c *Client) dial(ctx context.Context) (*websocket.Conn, error) {
	attempt := 0
	for {
		conn, resp, err := websocket.DefaultDialer.Dial(c.url, nil)
		if err == nil {
			return conn, nil
		}

		attempt++
		delay := backoff(attempt)
		if resp != nil {
			body := make([]byte, 512)
			n, _ := resp.Body.Read(body)
			resp.Body.Close()
			log.Printf("WebSocket connection failed (attempt %d): %v (HTTP %d: %s) — retrying in %s",
				attempt, err, resp.StatusCode, string(body[:n]), delay.Round(time.Millisecond))
		} else {
			log.Printf("WebSocket connection failed (attempt %d): %v — retrying in %s", attempt, err, delay.Round(time.Millisecond))
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// readLoop reads messages from the WebSocket and pushes them to recvCh.
// On read errors it reconnects with exponential backoff.
func (c *Client) readLoop(ctx context.Context) {
	defer close(c.done)
	defer close(c.recvCh)

	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			// Check if we're shutting down.
			select {
			case <-ctx.Done():
				return
			default:
			}

			log.Printf("WebSocket read error: %v — reconnecting", err)
			conn.Close()

			newConn, dialErr := c.dial(ctx)
			if dialErr != nil {
				// Context cancelled during reconnect.
				return
			}

			c.mu.Lock()
			c.conn = newConn
			c.mu.Unlock()

			// Set up ping handler on new connection.
			newConn.SetPongHandler(func(string) error { return nil })
			newConn.SetPingHandler(func(appData string) error {
				c.mu.Lock()
				defer c.mu.Unlock()
				if c.conn != nil {
					return c.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
				}
				return nil
			})

			log.Printf("WebSocket reconnected to %s", c.url)
			if c.onRecon != nil {
				c.onRecon()
			}
			continue
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("WebSocket: ignoring unparseable message: %v", err)
			continue
		}
		msg.Raw = data

		select {
		case c.recvCh <- msg:
		default:
			log.Printf("WebSocket: receive buffer full, dropping message type=%s", msg.Type)
		}
	}
}

// backoff returns a duration for the given attempt using exponential backoff + jitter.
func backoff(attempt int) time.Duration {
	base := float64(initialBackoff) * math.Pow(2, float64(attempt-1))
	if base > float64(maxBackoff) {
		base = float64(maxBackoff)
	}
	// Add jitter: 0.5x to 1.5x
	jitter := 0.5 + rand.Float64()
	return time.Duration(base * jitter)
}
