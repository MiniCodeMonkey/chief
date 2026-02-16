package uplink

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

// Uplink composes the HTTP client, message batcher, and Pusher client
// into a unified Send/Receive interface that matches the ws.Client API.
type Uplink struct {
	client  *Client
	batcher *Batcher
	pusher  *PusherClient

	mu        sync.RWMutex
	sessionID string
	deviceID  int
	connected bool

	// onReconnect is called after each successful reconnection.
	onReconnect func()

	// cancel stops the batcher run loop and heartbeat goroutine.
	cancel context.CancelFunc
}

// UplinkOption configures an Uplink.
type UplinkOption func(*Uplink)

// WithOnReconnect sets a callback invoked after each successful reconnection.
// This matches the ws.WithOnReconnect pattern — serve.go uses it to re-send
// a full state snapshot after reconnecting.
func WithOnReconnect(fn func()) UplinkOption {
	return func(u *Uplink) {
		u.onReconnect = fn
	}
}

// NewUplink creates a new Uplink that uses the given HTTP client.
// The Uplink does not connect until Connect is called.
func NewUplink(client *Client, opts ...UplinkOption) *Uplink {
	u := &Uplink{
		client: client,
	}
	for _, o := range opts {
		o(u)
	}
	return u
}

// Connect establishes the full uplink connection:
//  1. HTTP connect (registers device, gets session ID + Reverb config)
//  2. Pusher connect (subscribes to private command channel)
//  3. Batcher start (begins background flush loop)
//
// Heartbeat is started by US-019 — the heartbeat goroutine will be added
// to this lifecycle in a subsequent story.
func (u *Uplink) Connect(ctx context.Context) error {
	// Step 1: HTTP connect to register the device.
	welcome, err := u.client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("uplink connect: %w", err)
	}

	u.mu.Lock()
	u.sessionID = welcome.SessionID
	u.deviceID = welcome.DeviceID
	u.connected = true
	u.mu.Unlock()

	// Step 2: Start the Pusher client for receiving commands.
	channel := fmt.Sprintf("private-chief-server.%d", welcome.DeviceID)
	u.pusher = NewPusherClient(welcome.Reverb, channel, u.client.BroadcastAuth)

	if err := u.pusher.Connect(ctx); err != nil {
		// Clean up: disconnect from HTTP since Pusher failed.
		disconnectCtx, cancel := context.WithTimeout(context.Background(), httpTimeout)
		defer cancel()
		if dErr := u.client.Disconnect(disconnectCtx); dErr != nil {
			log.Printf("uplink: failed to disconnect after Pusher error: %v", dErr)
		}
		return fmt.Errorf("uplink pusher connect: %w", err)
	}

	// Step 3: Start the batcher for outgoing messages.
	batchCtx, batchCancel := context.WithCancel(ctx)
	u.cancel = batchCancel

	u.batcher = NewBatcher(func(batchID string, messages []json.RawMessage) error {
		_, err := u.client.SendMessagesWithRetry(batchCtx, batchID, messages)
		return err
	})
	go u.batcher.Run(batchCtx)

	log.Printf("Uplink connected (device=%d, session=%s)", welcome.DeviceID, welcome.SessionID)
	return nil
}

// Send enqueues a message into the batcher for batched delivery.
// This replaces ws.Client.Send() — the batcher handles flush timing.
func (u *Uplink) Send(msg json.RawMessage, msgType string) {
	u.mu.RLock()
	connected := u.connected
	u.mu.RUnlock()

	if !connected {
		log.Printf("uplink: dropping message (type=%s) — not connected", msgType)
		return
	}

	u.batcher.Enqueue(msg, msgType)
}

// Receive returns a channel that delivers incoming command payloads from
// the Pusher client. This replaces ws.Client.Receive().
// The channel is closed when the Pusher client shuts down.
func (u *Uplink) Receive() <-chan json.RawMessage {
	return u.pusher.Receive()
}

// Close performs graceful shutdown:
//  1. Stop the batcher (flushes remaining messages)
//  2. Close the Pusher client
//  3. HTTP disconnect
func (u *Uplink) Close() error {
	u.mu.Lock()
	if !u.connected {
		u.mu.Unlock()
		return nil
	}
	u.connected = false
	u.mu.Unlock()

	// Step 1: Stop the batcher — this flushes remaining messages.
	if u.batcher != nil {
		u.batcher.Stop()
	}

	// Cancel the batcher context to stop the Run loop.
	if u.cancel != nil {
		u.cancel()
	}

	// Step 2: Close the Pusher client.
	var pusherErr error
	if u.pusher != nil {
		pusherErr = u.pusher.Close()
	}

	// Step 3: HTTP disconnect.
	disconnectCtx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	if err := u.client.Disconnect(disconnectCtx); err != nil {
		log.Printf("uplink: disconnect failed: %v", err)
	}

	log.Printf("Uplink disconnected")
	return pusherErr
}

// SessionID returns the current session ID from the connect response.
func (u *Uplink) SessionID() string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.sessionID
}

// DeviceID returns the device ID from the connect response.
func (u *Uplink) DeviceID() int {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.deviceID
}

// SetAccessToken updates the access token on the HTTP client.
// This is called after a token refresh — the new token will be used
// for subsequent HTTP requests and Pusher auth calls.
func (u *Uplink) SetAccessToken(token string) {
	u.client.SetAccessToken(token)
}
