package uplink

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// pusherProtocolVersion is the Pusher protocol version to use.
	pusherProtocolVersion = 7

	// receiveBufSize is the buffer size for the receive channel.
	receiveBufSize = 256

	// pusherPingTimeout is how long to wait for a pong after sending a ping.
	pusherPingTimeout = 30 * time.Second

	// pusherWriteTimeout is the timeout for WebSocket write operations.
	pusherWriteTimeout = 10 * time.Second
)

// pusherMessage is a Pusher protocol message (both sent and received).
type pusherMessage struct {
	Event   string          `json:"event"`
	Channel string          `json:"channel,omitempty"`
	Data    json.RawMessage `json:"data"`
}

// pusherConnectionData is the data field of pusher:connection_established.
type pusherConnectionData struct {
	SocketID        string `json:"socket_id"`
	ActivityTimeout int    `json:"activity_timeout"`
}

// pusherAuthResponse is the response from the broadcast auth endpoint.
type pusherAuthResponse struct {
	Auth string `json:"auth"`
}

// AuthFunc is a function that authenticates a Pusher channel subscription.
// It takes a socketID and channelName and returns the auth signature string.
type AuthFunc func(ctx context.Context, socketID, channelName string) (string, error)

// PusherClient connects to a Reverb/Pusher WebSocket and subscribes to a private channel.
type PusherClient struct {
	appKey   string
	host     string
	port     int
	scheme   string
	channel  string
	authFn   AuthFunc
	dialer   *websocket.Dialer

	mu       sync.Mutex
	conn     *websocket.Conn
	socketID string
	recvCh   chan json.RawMessage
	done     chan struct{}
	stopped  bool
}

// NewPusherClient creates a PusherClient configured to connect to Reverb.
//
// Parameters:
//   - cfg: Reverb connection config (key, host, port, scheme) from the connect response
//   - channel: the private channel to subscribe to (e.g., "private-chief-server.42")
//   - authFn: function to authenticate the channel subscription
func NewPusherClient(cfg ReverbConfig, channel string, authFn AuthFunc) *PusherClient {
	return &PusherClient{
		appKey:  cfg.Key,
		host:    cfg.Host,
		port:    cfg.Port,
		scheme:  cfg.Scheme,
		channel: channel,
		authFn:  authFn,
		dialer:  websocket.DefaultDialer,
		recvCh:  make(chan json.RawMessage, receiveBufSize),
		done:    make(chan struct{}),
	}
}

// Connect dials the Pusher WebSocket, waits for connection_established,
// subscribes to the private channel, and starts the read loop.
func (p *PusherClient) Connect(ctx context.Context) error {
	wsURL := p.buildURL()

	headers := http.Header{}
	headers.Set("Origin", fmt.Sprintf("%s://%s", p.scheme, p.host))

	conn, _, err := p.dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return fmt.Errorf("pusher dial: %w", err)
	}

	p.mu.Lock()
	p.conn = conn
	p.mu.Unlock()

	// Wait for pusher:connection_established.
	socketID, activityTimeout, err := p.waitForConnectionEstablished(ctx, conn)
	if err != nil {
		conn.Close()
		return err
	}

	p.mu.Lock()
	p.socketID = socketID
	p.mu.Unlock()

	// Subscribe to the private channel.
	if err := p.subscribe(ctx, conn, socketID); err != nil {
		conn.Close()
		return err
	}

	// Start read loop.
	go p.readLoop(ctx, conn, activityTimeout)

	return nil
}

// Receive returns a channel that delivers incoming command payloads.
// The channel is closed when the client shuts down.
func (p *PusherClient) Receive() <-chan json.RawMessage {
	return p.recvCh
}

// Close gracefully shuts down the Pusher client.
func (p *PusherClient) Close() error {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return nil
	}
	p.stopped = true
	conn := p.conn
	p.conn = nil
	p.mu.Unlock()

	var err error
	if conn != nil {
		deadline := time.Now().Add(5 * time.Second)
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		_ = conn.WriteControl(websocket.CloseMessage, closeMsg, deadline)
		err = conn.Close()
	}

	// Wait for readLoop to finish.
	<-p.done

	return err
}

// buildURL constructs the Pusher WebSocket URL.
func (p *PusherClient) buildURL() string {
	wsScheme := "wss"
	if p.scheme == "http" {
		wsScheme = "ws"
	}

	u := url.URL{
		Scheme:   wsScheme,
		Host:     fmt.Sprintf("%s:%d", p.host, p.port),
		Path:     fmt.Sprintf("/app/%s", p.appKey),
		RawQuery: fmt.Sprintf("protocol=%d", pusherProtocolVersion),
	}
	return u.String()
}

// waitForConnectionEstablished reads messages until it receives
// pusher:connection_established. Returns the socket ID and activity timeout.
func (p *PusherClient) waitForConnectionEstablished(ctx context.Context, conn *websocket.Conn) (string, int, error) {
	// Set a read deadline for the connection established message.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetReadDeadline(time.Time{}) // Clear deadline.

	for {
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return "", 0, fmt.Errorf("pusher: waiting for connection_established: %w", err)
		}

		var msg pusherMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue // Skip unparseable messages.
		}

		if msg.Event == "pusher:connection_established" {
			// The data field is a JSON-encoded string inside the outer JSON,
			// so we unmarshal twice: first to get the string, then to parse it.
			var dataStr string
			if err := json.Unmarshal(msg.Data, &dataStr); err != nil {
				return "", 0, fmt.Errorf("pusher: parsing connection data wrapper: %w", err)
			}
			var connData pusherConnectionData
			if err := json.Unmarshal([]byte(dataStr), &connData); err != nil {
				return "", 0, fmt.Errorf("pusher: parsing connection data: %w", err)
			}
			if connData.SocketID == "" {
				return "", 0, fmt.Errorf("pusher: empty socket_id in connection_established")
			}
			return connData.SocketID, connData.ActivityTimeout, nil
		}

		if msg.Event == "pusher:error" {
			return "", 0, fmt.Errorf("pusher: server error during connect: %s", string(msg.Data))
		}
	}
}

// subscribe authenticates and subscribes to the private channel.
func (p *PusherClient) subscribe(ctx context.Context, conn *websocket.Conn, socketID string) error {
	// Get auth signature from the auth endpoint.
	authSig, err := p.authFn(ctx, socketID, p.channel)
	if err != nil {
		return fmt.Errorf("pusher: channel auth failed: %w", err)
	}

	// Send subscribe message.
	subData, _ := json.Marshal(map[string]string{
		"auth":    authSig,
		"channel": p.channel,
	})
	subMsg := pusherMessage{
		Event: "pusher:subscribe",
		Data:  subData,
	}

	conn.SetWriteDeadline(time.Now().Add(pusherWriteTimeout))
	if err := conn.WriteJSON(subMsg); err != nil {
		return fmt.Errorf("pusher: sending subscribe: %w", err)
	}
	conn.SetWriteDeadline(time.Time{})

	// Wait for subscription_succeeded or error.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("pusher: waiting for subscription response: %w", err)
		}

		var msg pusherMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		if msg.Event == "pusher_internal:subscription_succeeded" && msg.Channel == p.channel {
			return nil
		}

		if msg.Event == "pusher:error" {
			return fmt.Errorf("pusher: subscription error: %s", string(msg.Data))
		}
	}
}

// readLoop reads messages from the WebSocket and dispatches command events.
func (p *PusherClient) readLoop(ctx context.Context, conn *websocket.Conn, activityTimeout int) {
	defer close(p.done)
	defer close(p.recvCh)

	// Set up a ping/pong timer based on the server's activity timeout.
	pingInterval := time.Duration(activityTimeout) * time.Second
	if pingInterval <= 0 {
		pingInterval = 120 * time.Second // Default Pusher activity timeout.
	}
	pingTimer := time.NewTimer(pingInterval)
	defer pingTimer.Stop()

	// Set up pong handler to confirm the connection is alive.
	pongCh := make(chan struct{}, 1)
	conn.SetPongHandler(func(string) error {
		select {
		case pongCh <- struct{}{}:
		default:
		}
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Non-blocking check for shutdown.
		p.mu.Lock()
		if p.stopped {
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()

		conn.SetReadDeadline(time.Now().Add(pingInterval + pusherPingTimeout))
		_, data, err := conn.ReadMessage()
		if err != nil {
			// Check if we're shutting down.
			select {
			case <-ctx.Done():
				return
			default:
			}

			p.mu.Lock()
			stopped := p.stopped
			p.mu.Unlock()
			if stopped {
				return
			}

			log.Printf("Pusher read error: %v", err)
			return
		}

		// Reset ping timer on any activity.
		if !pingTimer.Stop() {
			select {
			case <-pingTimer.C:
			default:
			}
		}
		pingTimer.Reset(pingInterval)

		var msg pusherMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Pusher: ignoring unparseable message: %v", err)
			continue
		}

		switch msg.Event {
		case "pusher:ping":
			// Respond with pong.
			p.mu.Lock()
			c := p.conn
			p.mu.Unlock()
			if c != nil {
				pongMsg := pusherMessage{Event: "pusher:pong", Data: json.RawMessage("{}")}
				c.SetWriteDeadline(time.Now().Add(pusherWriteTimeout))
				if err := c.WriteJSON(pongMsg); err != nil {
					log.Printf("Pusher: failed to send pong: %v", err)
				}
				c.SetWriteDeadline(time.Time{})
			}

		case "pusher:pong":
			// Server responded to our ping — connection is alive.
			select {
			case pongCh <- struct{}{}:
			default:
			}

		case "pusher:error":
			log.Printf("Pusher server error: %s", string(msg.Data))

		case "chief.command":
			// This is a command from the server — deliver to the receive channel.
			if msg.Channel == p.channel {
				// The data field contains the command payload.
				select {
				case p.recvCh <- msg.Data:
				default:
					log.Printf("Pusher: receive buffer full, dropping command")
				}
			}

		default:
			// Ignore other event types (subscription_succeeded during reconnect, etc.).
		}
	}
}

// BroadcastAuth authenticates a Pusher channel subscription via the uplink HTTP client.
// This creates an AuthFunc that calls POST /api/device/broadcasting/auth.
func (c *Client) BroadcastAuth(ctx context.Context, socketID, channelName string) (string, error) {
	body := broadcastAuthRequest{
		SocketID:    socketID,
		ChannelName: channelName,
	}

	var resp pusherAuthResponse
	if err := c.doJSON(ctx, "POST", "/api/device/broadcasting/auth", body, &resp); err != nil {
		return "", fmt.Errorf("broadcast auth: %w", err)
	}

	return resp.Auth, nil
}

// broadcastAuthRequest is the JSON body sent to POST /api/device/broadcasting/auth.
type broadcastAuthRequest struct {
	SocketID    string `json:"socket_id"`
	ChannelName string `json:"channel_name"`
}

// GenerateAuthSignature generates a Pusher private channel auth signature locally.
// This is used in tests to verify auth signatures without hitting the server.
func GenerateAuthSignature(appKey, appSecret, socketID, channelName string) string {
	data := socketID + ":" + channelName
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(data))
	sig := fmt.Sprintf("%x", mac.Sum(nil))
	return appKey + ":" + sig
}
