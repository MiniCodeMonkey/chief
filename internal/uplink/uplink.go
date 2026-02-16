package uplink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	// heartbeatInterval is how often heartbeats are sent.
	heartbeatInterval = 30 * time.Second

	// heartbeatRetryDelay is the delay before retrying a failed heartbeat.
	heartbeatRetryDelay = 5 * time.Second

	// heartbeatSkipWindow is the duration after a message send within which
	// we skip the explicit heartbeat (server treats message receipt as implicit heartbeat).
	heartbeatSkipWindow = 25 * time.Second

	// heartbeatMaxFailures is the number of consecutive failures before triggering reconnection.
	heartbeatMaxFailures = 3
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

	// lastSendTime records when the batcher last successfully sent a batch.
	// Used by the heartbeat goroutine to skip heartbeats when messages
	// were recently sent (implicit heartbeat optimization).
	lastSendTime time.Time

	// Heartbeat timing (overridable for tests, default to package constants).
	hbInterval   time.Duration
	hbRetryDelay time.Duration
	hbSkipWindow time.Duration
	hbMaxFails   int

	// recvCh is a stable receive channel that outlives individual Pusher clients.
	// Commands from each Pusher client are forwarded into this channel so callers
	// of Receive() don't need to re-subscribe after reconnection.
	recvCh chan json.RawMessage

	// onReconnect is called after each successful reconnection.
	onReconnect func()

	// onAuthFailure is called when a 401 auth error occurs during reconnection.
	// The callback should perform a token refresh and call SetAccessToken() before returning.
	// If the callback returns nil, reconnection retries with the new token.
	// If it returns an error, reconnection aborts.
	onAuthFailure func() error

	// onHeartbeatMaxFailures is called when consecutive heartbeat failures
	// reach hbMaxFails. If nil, triggerReconnect() is called directly.
	// Tests can set this to override the default reconnection behavior.
	onHeartbeatMaxFailures func()

	// reconnecting tracks whether a reconnection is in progress to prevent concurrent reconnects.
	reconnecting bool

	// parentCtx is the context passed to Connect() — used as the parent for reconnection contexts.
	parentCtx context.Context

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

// WithOnAuthFailure sets a callback invoked when a 401 auth error occurs during
// reconnection. The callback should perform a token refresh and call
// SetAccessToken() before returning. Return nil to retry, or an error to abort.
func WithOnAuthFailure(fn func() error) UplinkOption {
	return func(u *Uplink) {
		u.onAuthFailure = fn
	}
}

// NewUplink creates a new Uplink that uses the given HTTP client.
// The Uplink does not connect until Connect is called.
func NewUplink(client *Client, opts ...UplinkOption) *Uplink {
	u := &Uplink{
		client:       client,
		hbInterval:   heartbeatInterval,
		hbRetryDelay: heartbeatRetryDelay,
		hbSkipWindow: heartbeatSkipWindow,
		hbMaxFails:   heartbeatMaxFailures,
		recvCh:       make(chan json.RawMessage, receiveBufSize),
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
//  4. Heartbeat start (sends periodic heartbeats to server)
//  5. Pusher monitor (detects disconnection and triggers reconnection)
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
	u.parentCtx = ctx
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
		if err == nil {
			u.mu.Lock()
			u.lastSendTime = time.Now()
			u.mu.Unlock()
		}
		return err
	})
	go u.batcher.Run(batchCtx)

	// Step 4: Start the heartbeat goroutine.
	go u.runHeartbeat(batchCtx)

	// Step 5: Monitor Pusher for disconnection.
	go u.monitorPusher(batchCtx)

	log.Printf("Uplink connected (device=%d, session=%s)", welcome.DeviceID, welcome.SessionID)
	return nil
}

// Send enqueues a message into the batcher for batched delivery.
// This replaces ws.Client.Send() — the batcher handles flush timing.
// During reconnection, messages are buffered locally in the batcher.
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

// Receive returns a channel that delivers incoming command payloads.
// This channel is stable across reconnections — new Pusher clients
// forward commands into the same channel.
func (u *Uplink) Receive() <-chan json.RawMessage {
	return u.recvCh
}

// Close performs graceful shutdown:
//  1. Stop the batcher (flushes remaining messages)
//  2. Close the Pusher client
//  3. HTTP disconnect
func (u *Uplink) Close() error {
	return u.doClose()
}

// CloseWithTimeout performs graceful shutdown with a deadline.
// If the timeout expires before the batcher flush completes, the flush is
// abandoned and shutdown continues with Pusher close and HTTP disconnect.
// This prevents shutdown from hanging when the server is unreachable.
func (u *Uplink) CloseWithTimeout(timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- u.doClose()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		log.Printf("uplink: graceful close timed out after %s — forcing shutdown", timeout)
		// Force-cancel the batcher/heartbeat/monitor contexts to unblock doClose.
		u.mu.Lock()
		u.connected = false
		u.mu.Unlock()
		if u.cancel != nil {
			u.cancel()
		}
		// Wait briefly for doClose to finish after cancellation.
		select {
		case err := <-done:
			return err
		case <-time.After(2 * time.Second):
			log.Printf("uplink: forced shutdown complete")
			return nil
		}
	}
}

// doClose is the internal close implementation shared by Close and CloseWithTimeout.
func (u *Uplink) doClose() error {
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

	// Cancel the batcher context to stop the Run loop, heartbeat, and Pusher monitor.
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

// monitorPusher watches the Pusher client's receive channel. When it closes
// (Pusher readLoop exited due to an error), it triggers a full reconnection.
func (u *Uplink) monitorPusher(ctx context.Context) {
	if u.pusher == nil {
		return
	}
	pusherRecv := u.pusher.Receive()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-pusherRecv:
			if !ok {
				// Pusher channel closed — readLoop exited.
				// Check if we're shutting down.
				select {
				case <-ctx.Done():
					return
				default:
				}

				u.mu.RLock()
				connected := u.connected
				u.mu.RUnlock()
				if !connected {
					return
				}

				u.triggerReconnect("Pusher disconnected")
				return
			}
			// Forward the command to the stable recvCh.
			select {
			case u.recvCh <- msg:
			default:
				log.Printf("uplink: receive buffer full, dropping command")
			}
		}
	}
}

// triggerReconnect initiates a reconnection attempt in the background.
// It is safe to call from multiple goroutines — only one reconnection runs at a time.
func (u *Uplink) triggerReconnect(reason string) {
	u.mu.Lock()
	if u.reconnecting || !u.connected {
		u.mu.Unlock()
		return
	}
	u.reconnecting = true
	parentCtx := u.parentCtx
	u.mu.Unlock()

	log.Printf("uplink: triggering reconnection (%s)", reason)
	go u.reconnect(parentCtx)
}

// reconnect tears down the existing connection and re-establishes it with backoff.
// On success, it fires the onReconnect callback so the caller can re-send state.
func (u *Uplink) reconnect(ctx context.Context) {
	defer func() {
		u.mu.Lock()
		u.reconnecting = false
		u.mu.Unlock()
	}()

	// Step 1: Tear down old batcher and Pusher.
	// Stop the batcher — this flushes remaining messages.
	if u.batcher != nil {
		u.batcher.Stop()
	}

	// Cancel old batcher context to stop the old Run loop, heartbeat, and monitor.
	if u.cancel != nil {
		u.cancel()
	}

	// Close the old Pusher client.
	if u.pusher != nil {
		if err := u.pusher.Close(); err != nil {
			log.Printf("uplink: error closing Pusher during reconnection: %v", err)
		}
	}

	// Step 2: Reconnect with exponential backoff.
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("uplink: reconnection cancelled")
			return
		default:
		}

		u.mu.RLock()
		connected := u.connected
		u.mu.RUnlock()
		if !connected {
			// Close() was called — stop reconnecting.
			return
		}

		attempt++
		delay := backoff(attempt)
		log.Printf("uplink: reconnection attempt %d — retrying in %s", attempt, delay.Round(time.Millisecond))

		select {
		case <-ctx.Done():
			log.Printf("uplink: reconnection cancelled")
			return
		case <-time.After(delay):
		}

		// Try HTTP connect.
		welcome, err := u.client.Connect(ctx)
		if err != nil {
			if errors.Is(err, ErrAuthFailed) {
				// Auth failure — try token refresh if callback is set.
				if u.onAuthFailure != nil {
					log.Printf("uplink: auth failed during reconnection — requesting token refresh")
					if refreshErr := u.onAuthFailure(); refreshErr != nil {
						log.Printf("uplink: token refresh failed: %v — aborting reconnection", refreshErr)
						return
					}
					// Token refreshed — retry without incrementing attempt.
					attempt--
					continue
				}
				log.Printf("uplink: auth failed during reconnection (no refresh callback) — aborting")
				return
			}
			log.Printf("uplink: reconnection attempt %d HTTP connect failed: %v", attempt, err)
			continue
		}

		// Update session/device.
		u.mu.Lock()
		u.sessionID = welcome.SessionID
		u.deviceID = welcome.DeviceID
		u.mu.Unlock()

		// Try Pusher connect.
		channel := fmt.Sprintf("private-chief-server.%d", welcome.DeviceID)
		pusher := NewPusherClient(welcome.Reverb, channel, u.client.BroadcastAuth)

		if err := pusher.Connect(ctx); err != nil {
			log.Printf("uplink: reconnection attempt %d Pusher connect failed: %v — disconnecting HTTP", attempt, err)
			disconnectCtx, cancel := context.WithTimeout(context.Background(), httpTimeout)
			if dErr := u.client.Disconnect(disconnectCtx); dErr != nil {
				log.Printf("uplink: failed to disconnect after Pusher reconnect error: %v", dErr)
			}
			cancel()
			continue
		}

		// Step 3: Start new batcher and heartbeat.
		batchCtx, batchCancel := context.WithCancel(ctx)

		u.mu.Lock()
		u.pusher = pusher
		u.cancel = batchCancel
		u.lastSendTime = time.Time{} // Reset — force next heartbeat to fire.
		u.mu.Unlock()

		u.batcher = NewBatcher(func(batchID string, messages []json.RawMessage) error {
			_, err := u.client.SendMessagesWithRetry(batchCtx, batchID, messages)
			if err == nil {
				u.mu.Lock()
				u.lastSendTime = time.Now()
				u.mu.Unlock()
			}
			return err
		})
		go u.batcher.Run(batchCtx)

		// Restart heartbeat.
		go u.runHeartbeat(batchCtx)

		// Restart Pusher monitor.
		go u.monitorPusher(batchCtx)

		log.Printf("Uplink reconnected (attempt %d, device=%d, session=%s)", attempt, welcome.DeviceID, welcome.SessionID)

		// Fire the OnReconnect callback so serve.go can re-send state.
		if u.onReconnect != nil {
			u.onReconnect()
		}

		return
	}
}

// runHeartbeat sends periodic heartbeats to the server every heartbeatInterval.
// It skips the heartbeat if a message batch was sent within heartbeatSkipWindow.
// On transient failure, it retries once after heartbeatRetryDelay.
// After heartbeatMaxFailures consecutive failures, it triggers reconnection.
func (u *Uplink) runHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(u.hbInterval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Skip heartbeat if a message batch was sent recently.
			u.mu.RLock()
			lastSend := u.lastSendTime
			u.mu.RUnlock()

			if !lastSend.IsZero() && time.Since(lastSend) < u.hbSkipWindow {
				consecutiveFailures = 0
				continue
			}

			// Send heartbeat.
			err := u.client.Heartbeat(ctx)
			if err == nil {
				consecutiveFailures = 0
				continue
			}

			// First failure — retry once after a short delay.
			log.Printf("uplink: heartbeat failed: %v — retrying in %s", err, u.hbRetryDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(u.hbRetryDelay):
			}

			err = u.client.Heartbeat(ctx)
			if err == nil {
				consecutiveFailures = 0
				continue
			}

			// Retry also failed — count as a failure.
			consecutiveFailures++
			log.Printf("uplink: heartbeat retry failed (%d/%d consecutive): %v", consecutiveFailures, u.hbMaxFails, err)

			if consecutiveFailures >= u.hbMaxFails {
				log.Printf("uplink: %d consecutive heartbeat failures — triggering reconnection", consecutiveFailures)
				if u.onHeartbeatMaxFailures != nil {
					u.onHeartbeatMaxFailures()
				} else {
					u.triggerReconnect("heartbeat failures")
				}
				consecutiveFailures = 0
			}
		}
	}
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
