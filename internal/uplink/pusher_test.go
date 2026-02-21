package uplink

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// testPusherServer is a mock Pusher/Reverb WebSocket server for testing.
type testPusherServer struct {
	srv      *httptest.Server
	upgrader websocket.Upgrader

	mu   sync.Mutex
	conn *websocket.Conn

	// Configuration.
	appKey          string
	appSecret       string
	socketID        string
	activityTimeout int

	// Control channels.
	onSubscribe chan string // receives channel name when client subscribes
	onMessage   chan []byte // receives raw messages from client

	// Behavior flags.
	rejectAuth     bool
	rejectSubscribe bool
	skipEstablished bool
}

func newTestPusherServer(t *testing.T) *testPusherServer {
	t.Helper()

	ps := &testPusherServer{
		appKey:          "test-app-key",
		appSecret:       "test-app-secret",
		socketID:        "123456.7890",
		activityTimeout: 120,
		onSubscribe:     make(chan string, 10),
		onMessage:       make(chan []byte, 10),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	ps.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ps.handleWS(t, w, r)
	}))

	t.Cleanup(func() {
		ps.mu.Lock()
		if ps.conn != nil {
			ps.conn.Close()
		}
		ps.mu.Unlock()
		ps.srv.Close()
	})

	return ps
}

func (ps *testPusherServer) handleWS(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	// Verify the URL path matches Pusher format.
	expectedPath := fmt.Sprintf("/app/%s", ps.appKey)
	if !strings.HasPrefix(r.URL.Path, expectedPath) {
		http.Error(w, "invalid path", http.StatusNotFound)
		return
	}

	conn, err := ps.upgrader.Upgrade(w, r, nil)
	if err != nil {
		t.Logf("upgrade error: %v", err)
		return
	}

	ps.mu.Lock()
	ps.conn = conn
	ps.mu.Unlock()

	// Send connection_established unless configured not to.
	if !ps.skipEstablished {
		connDataJSON, _ := json.Marshal(pusherConnectionData{
			SocketID:        ps.socketID,
			ActivityTimeout: ps.activityTimeout,
		})
		// Real Pusher/Reverb double-encodes: the data field is a JSON string.
		connDataStr, _ := json.Marshal(string(connDataJSON))
		established := pusherMessage{
			Event: "pusher:connection_established",
			Data:  connDataStr,
		}
		if err := conn.WriteJSON(established); err != nil {
			t.Logf("write connection_established: %v", err)
			return
		}
	}

	// Read loop — handle subscribe messages and pass others to onMessage.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var msg pusherMessage
		if json.Unmarshal(data, &msg) != nil {
			continue
		}

		switch msg.Event {
		case "pusher:subscribe":
			var subData map[string]string
			json.Unmarshal(msg.Data, &subData)
			channel := subData["channel"]

			select {
			case ps.onSubscribe <- channel:
			default:
			}

			if ps.rejectSubscribe {
				errData, _ := json.Marshal(map[string]interface{}{
					"message": "subscription rejected",
					"code":    4009,
				})
				conn.WriteJSON(pusherMessage{
					Event: "pusher:error",
					Data:  errData,
				})
				continue
			}

			// Send subscription_succeeded.
			conn.WriteJSON(pusherMessage{
				Event:   "pusher_internal:subscription_succeeded",
				Channel: channel,
				Data:    json.RawMessage("{}"),
			})

		case "pusher:pong":
			select {
			case ps.onMessage <- data:
			default:
			}

		default:
			select {
			case ps.onMessage <- data:
			default:
			}
		}
	}
}

// sendCommand sends a chief.command event to the connected client.
func (ps *testPusherServer) sendCommand(channel string, command json.RawMessage) error {
	ps.mu.Lock()
	conn := ps.conn
	ps.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no client connected")
	}

	msg := pusherMessage{
		Event:   "chief.command",
		Channel: channel,
		Data:    command,
	}
	return conn.WriteJSON(msg)
}

// sendCommandStringEncoded sends a chief.command event where the data field
// is a JSON-encoded string, matching real Reverb/Pusher wire format.
func (ps *testPusherServer) sendCommandStringEncoded(channel string, command json.RawMessage) error {
	ps.mu.Lock()
	conn := ps.conn
	ps.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no client connected")
	}

	// Double-encode: wrap the JSON object as a JSON string.
	encoded, err := json.Marshal(string(command))
	if err != nil {
		return fmt.Errorf("encoding command: %w", err)
	}

	msg := pusherMessage{
		Event:   "chief.command",
		Channel: channel,
		Data:    encoded,
	}
	return conn.WriteJSON(msg)
}

// closeConnection closes the WebSocket connection from the server side,
// simulating a Pusher disconnection.
func (ps *testPusherServer) closeConnection() error {
	ps.mu.Lock()
	conn := ps.conn
	ps.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no client connected")
	}

	return conn.Close()
}

// sendPing sends a pusher:ping to the connected client.
func (ps *testPusherServer) sendPing() error {
	ps.mu.Lock()
	conn := ps.conn
	ps.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no client connected")
	}

	return conn.WriteJSON(pusherMessage{
		Event: "pusher:ping",
		Data:  json.RawMessage("{}"),
	})
}

// reverbConfig returns a ReverbConfig pointing at the test server.
func (ps *testPusherServer) reverbConfig() ReverbConfig {
	// Extract host and port from the test server URL.
	addr := ps.srv.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	host := parts[0]
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	return ReverbConfig{
		Key:    ps.appKey,
		Host:   host,
		Port:   port,
		Scheme: "http",
	}
}

// testAuthFn returns an AuthFunc that uses the test server's app key/secret.
func (ps *testPusherServer) testAuthFn() AuthFunc {
	return func(ctx context.Context, socketID, channelName string) (string, error) {
		return GenerateAuthSignature(ps.appKey, ps.appSecret, socketID, channelName), nil
	}
}

// failingAuthFn returns an AuthFunc that always fails.
func failingAuthFn() AuthFunc {
	return func(ctx context.Context, socketID, channelName string) (string, error) {
		return "", fmt.Errorf("auth endpoint unavailable")
	}
}

// --- Tests ---

func TestPusherClient_ConnectAndReceive(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Wait for subscription.
	select {
	case ch := <-ps.onSubscribe:
		if ch != channel {
			t.Errorf("subscribed to %q, want %q", ch, channel)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Send a command and verify receipt.
	cmd := json.RawMessage(`{"type":"start_run","project":"test"}`)
	if err := ps.sendCommand(channel, cmd); err != nil {
		t.Fatalf("sendCommand failed: %v", err)
	}

	select {
	case received := <-client.Receive():
		var parsed map[string]interface{}
		json.Unmarshal(received, &parsed)
		if parsed["type"] != "start_run" {
			t.Errorf("received type = %v, want start_run", parsed["type"])
		}
		if parsed["project"] != "test" {
			t.Errorf("received project = %v, want test", parsed["project"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for command")
	}
}

func TestPusherClient_MultipleCommands(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.99"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Send multiple commands.
	commands := []string{
		`{"type":"start_run","id":"1"}`,
		`{"type":"pause_run","id":"2"}`,
		`{"type":"stop_run","id":"3"}`,
	}

	for _, cmd := range commands {
		if err := ps.sendCommand(channel, json.RawMessage(cmd)); err != nil {
			t.Fatalf("sendCommand failed: %v", err)
		}
	}

	// Receive all commands in order.
	for i, expected := range commands {
		select {
		case received := <-client.Receive():
			var expectedMap, receivedMap map[string]interface{}
			json.Unmarshal([]byte(expected), &expectedMap)
			json.Unmarshal(received, &receivedMap)
			if receivedMap["id"] != expectedMap["id"] {
				t.Errorf("command %d: id = %v, want %v", i, receivedMap["id"], expectedMap["id"])
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for command %d", i)
		}
	}
}

func TestPusherClient_IgnoresOtherChannels(t *testing.T) {
	ps := newTestPusherServer(t)
	myChannel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), myChannel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Send a command on a different channel — should be ignored.
	if err := ps.sendCommand("private-chief-server.99", json.RawMessage(`{"type":"other"}`)); err != nil {
		t.Fatalf("sendCommand failed: %v", err)
	}

	// Send a command on our channel — should be received.
	if err := ps.sendCommand(myChannel, json.RawMessage(`{"type":"mine"}`)); err != nil {
		t.Fatalf("sendCommand failed: %v", err)
	}

	select {
	case received := <-client.Receive():
		var parsed map[string]interface{}
		json.Unmarshal(received, &parsed)
		if parsed["type"] != "mine" {
			t.Errorf("received type = %v, want mine (wrong channel message leaked through)", parsed["type"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for command")
	}
}

func TestPusherClient_PingPong(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Send a ping and verify pong response.
	if err := ps.sendPing(); err != nil {
		t.Fatalf("sendPing failed: %v", err)
	}

	// The client should send back a pusher:pong.
	select {
	case data := <-ps.onMessage:
		var msg pusherMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("failed to parse pong message: %v", err)
		}
		if msg.Event != "pusher:pong" {
			t.Errorf("response event = %q, want pusher:pong", msg.Event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for pong")
	}
}

func TestPusherClient_Close(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Close and verify the receive channel closes.
	if err := client.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Receive channel should be closed.
	select {
	case _, ok := <-client.Receive():
		if ok {
			t.Error("expected receive channel to be closed after Close()")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for receive channel to close")
	}

	// Double-close should be safe.
	if err := client.Close(); err != nil {
		t.Fatalf("double Close() failed: %v", err)
	}
}

func TestPusherClient_ContextCancellation(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Cancel context — the read loop should stop.
	cancel()

	// Give the readLoop time to notice the cancellation and close.
	select {
	case <-client.Receive():
		// Channel closed or drained — good.
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shutdown after context cancellation")
	}
}

func TestPusherClient_AuthFailure(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, failingAuthFn())

	ctx := testContext(t)
	err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error when auth fails, got nil")
		client.Close()
	}
	if !strings.Contains(err.Error(), "auth endpoint unavailable") {
		t.Errorf("error = %v, want containing 'auth endpoint unavailable'", err)
	}
}

func TestPusherClient_SubscriptionRejected(t *testing.T) {
	ps := newTestPusherServer(t)
	ps.rejectSubscribe = true
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error when subscription is rejected, got nil")
		client.Close()
	}
	if !strings.Contains(err.Error(), "subscription error") {
		t.Errorf("error = %v, want containing 'subscription error'", err)
	}
}

func TestPusherClient_BuildURL(t *testing.T) {
	tests := []struct {
		name   string
		cfg    ReverbConfig
		expect string
	}{
		{
			name: "HTTPS scheme",
			cfg: ReverbConfig{
				Key:    "my-key",
				Host:   "reverb.example.com",
				Port:   443,
				Scheme: "https",
			},
			expect: "wss://reverb.example.com:443/app/my-key?protocol=7",
		},
		{
			name: "HTTP scheme",
			cfg: ReverbConfig{
				Key:    "local-key",
				Host:   "localhost",
				Port:   8080,
				Scheme: "http",
			},
			expect: "ws://localhost:8080/app/local-key?protocol=7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPusherClient(tt.cfg, "private-test", nil)
			got := p.buildURL()
			if got != tt.expect {
				t.Errorf("buildURL() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestPusherClient_ReceiveChannelBuffered(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// The receive channel should be buffered.
	if cap(client.recvCh) != receiveBufSize {
		t.Errorf("receive channel capacity = %d, want %d", cap(client.recvCh), receiveBufSize)
	}
}

func TestPusherClient_ServerError(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Send a pusher:error — client should log it but not crash.
	ps.mu.Lock()
	conn := ps.conn
	ps.mu.Unlock()

	errData, _ := json.Marshal(map[string]interface{}{"message": "test error", "code": 4100})
	conn.WriteJSON(pusherMessage{
		Event: "pusher:error",
		Data:  errData,
	})

	// Send a command after the error — client should still be functioning.
	if err := ps.sendCommand(channel, json.RawMessage(`{"type":"after_error"}`)); err != nil {
		t.Fatalf("sendCommand failed: %v", err)
	}

	select {
	case received := <-client.Receive():
		var parsed map[string]interface{}
		json.Unmarshal(received, &parsed)
		if parsed["type"] != "after_error" {
			t.Errorf("received type = %v, want after_error", parsed["type"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for command after error")
	}
}

func TestBroadcastAuth_Success(t *testing.T) {
	var receivedSocketID, receivedChannel string
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/device/broadcasting/auth" {
			http.NotFound(w, r)
			return
		}

		receivedAuth = r.Header.Get("Authorization")

		var body broadcastAuthRequest
		json.NewDecoder(r.Body).Decode(&body)
		receivedSocketID = body.SocketID
		receivedChannel = body.ChannelName

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pusherAuthResponse{
			Auth: "app-key:test-signature",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "test-token")
	ctx := testContext(t)

	auth, err := client.BroadcastAuth(ctx, "12345.67890", "private-chief-server.42")
	if err != nil {
		t.Fatalf("BroadcastAuth() failed: %v", err)
	}

	if receivedAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer test-token")
	}
	if receivedSocketID != "12345.67890" {
		t.Errorf("socket_id = %q, want %q", receivedSocketID, "12345.67890")
	}
	if receivedChannel != "private-chief-server.42" {
		t.Errorf("channel_name = %q, want %q", receivedChannel, "private-chief-server.42")
	}
	if auth != "app-key:test-signature" {
		t.Errorf("auth = %q, want %q", auth, "app-key:test-signature")
	}
}

func TestBroadcastAuth_AuthFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "bad-token")
	ctx := testContext(t)

	_, err := client.BroadcastAuth(ctx, "12345.67890", "private-chief-server.42")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestGenerateAuthSignature(t *testing.T) {
	// Known test vectors for Pusher private channel auth.
	sig := GenerateAuthSignature("278d425bdf160313ff76", "7ad3773142a6692b25b8", "1234.1234", "private-foobar")

	// The format should be "key:hex_signature".
	if !strings.HasPrefix(sig, "278d425bdf160313ff76:") {
		t.Errorf("signature should start with app key, got %q", sig)
	}

	parts := strings.SplitN(sig, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("signature should have format key:sig, got %q", sig)
	}
	if len(parts[1]) != 64 { // SHA256 hex = 64 chars
		t.Errorf("signature hex length = %d, want 64", len(parts[1]))
	}
}

func TestPusherClient_ConnectionEstablishedTimeout(t *testing.T) {
	ps := newTestPusherServer(t)
	ps.skipEstablished = true // Server won't send connection_established.
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error when connection_established is not received, got nil")
		client.Close()
	}
}

func TestPusherClient_ConcurrentClose(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Close from multiple goroutines concurrently.
	var wg sync.WaitGroup
	var closeErrors atomic.Int32
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := client.Close(); err != nil {
				closeErrors.Add(1)
			}
		}()
	}
	wg.Wait()

	// At most one goroutine should get an error (the connection close), rest should be nil.
	// This test mainly verifies no panics from concurrent access.
}

func TestPusherClient_DialFailure(t *testing.T) {
	// Connect to a non-existent server.
	cfg := ReverbConfig{
		Key:    "test-key",
		Host:   "127.0.0.1",
		Port:   1, // Port 1 should be unreachable.
		Scheme: "http",
	}
	channel := "private-chief-server.42"
	client := NewPusherClient(cfg, channel, failingAuthFn())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error connecting to unreachable server, got nil")
		client.Close()
	}
}

// TestPusherClient_DoubleEncodedData verifies that commands with Pusher's
// real wire format (data as JSON string) are correctly unwrapped.
func TestPusherClient_DoubleEncodedData(t *testing.T) {
	ps := newTestPusherServer(t)
	channel := "private-chief-server.42"

	client := NewPusherClient(ps.reverbConfig(), channel, ps.testAuthFn())

	ctx := testContext(t)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Wait for subscription.
	select {
	case <-ps.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Send command using Reverb's real format (data as JSON string).
	cmd := json.RawMessage(`{"type":"start_run","payload":{"project_slug":"my-project"}}`)
	if err := ps.sendCommandStringEncoded(channel, cmd); err != nil {
		t.Fatalf("sendCommandStringEncoded failed: %v", err)
	}

	select {
	case received := <-client.Receive():
		var parsed map[string]interface{}
		if err := json.Unmarshal(received, &parsed); err != nil {
			t.Fatalf("failed to parse received command: %v (raw: %s)", err, string(received))
		}
		if parsed["type"] != "start_run" {
			t.Errorf("received type = %v, want start_run", parsed["type"])
		}
		payload, ok := parsed["payload"].(map[string]interface{})
		if !ok {
			t.Fatalf("payload is not an object: %v", parsed["payload"])
		}
		if payload["project_slug"] != "my-project" {
			t.Errorf("received project_slug = %v, want my-project", payload["project_slug"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for command")
	}
}
