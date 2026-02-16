package cmd

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

// mockUplinkServer is a combined HTTP API + Pusher WebSocket server for testing
// the serve command with the uplink transport. It replaces the old WebSocket-only
// test server used before the uplink refactor.
type mockUplinkServer struct {
	httpSrv   *httptest.Server
	pusherSrv *mockPusherServer

	mu             sync.Mutex
	messageBatches []mockMessageBatch
	connectBody    map[string]interface{}

	connectCalls    atomic.Int32
	disconnectCalls atomic.Int32
	heartbeatCalls  atomic.Int32
	messagesCalls   atomic.Int32

	// connectStatus controls the HTTP status returned by /api/device/connect.
	// 0 means success (200).
	connectStatus atomic.Int32
}

type mockMessageBatch struct {
	BatchID  string            `json:"batch_id"`
	Messages []json.RawMessage `json:"messages"`
}

// newMockUplinkServer creates a new combined test server.
func newMockUplinkServer(t *testing.T) *mockUplinkServer {
	t.Helper()

	ps := newMockPusherServer(t)

	ms := &mockUplinkServer{
		pusherSrv: ps,
	}

	reverbCfg := ps.reverbConfig()

	ms.httpSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.handleHTTP(w, r, reverbCfg)
	}))
	t.Cleanup(func() { ms.httpSrv.Close() })

	return ms
}

func (ms *mockUplinkServer) handleHTTP(w http.ResponseWriter, r *http.Request, reverbCfg mockReverbConfig) {
	// Check auth header.
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing token"})
		return
	}

	// Check for simulated auth failure.
	if r.URL.Path == "/api/device/connect" {
		status := int(ms.connectStatus.Load())
		if status >= 400 {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": "auth failed"})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/api/device/connect":
		ms.connectCalls.Add(1)

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		ms.mu.Lock()
		ms.connectBody = body
		ms.mu.Unlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"type":             "welcome",
			"protocol_version": 1,
			"device_id":        42,
			"session_id":       "test-session-1",
			"reverb": map[string]interface{}{
				"key":    reverbCfg.Key,
				"host":   reverbCfg.Host,
				"port":   reverbCfg.Port,
				"scheme": reverbCfg.Scheme,
			},
		})

	case "/api/device/disconnect":
		ms.disconnectCalls.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})

	case "/api/device/heartbeat":
		ms.heartbeatCalls.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	case "/api/device/messages":
		ms.messagesCalls.Add(1)

		var req struct {
			BatchID  string            `json:"batch_id"`
			Messages []json.RawMessage `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		ms.mu.Lock()
		ms.messageBatches = append(ms.messageBatches, mockMessageBatch{
			BatchID:  req.BatchID,
			Messages: req.Messages,
		})
		ms.mu.Unlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"accepted":   len(req.Messages),
			"batch_id":   req.BatchID,
			"session_id": "test-session-1",
		})

	case "/api/device/broadcasting/auth":
		var body struct {
			SocketID    string `json:"socket_id"`
			ChannelName string `json:"channel_name"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		sig := generateTestAuthSignature(
			ms.pusherSrv.appKey,
			ms.pusherSrv.appSecret,
			body.SocketID,
			body.ChannelName,
		)
		json.NewEncoder(w).Encode(map[string]string{"auth": sig})

	default:
		http.NotFound(w, r)
	}
}

// getConnectBody returns the last connect request body.
func (ms *mockUplinkServer) getConnectBody() map[string]interface{} {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.connectBody
}

// getMessages returns all messages received across all batches, flattened.
func (ms *mockUplinkServer) getMessages() []json.RawMessage {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	var msgs []json.RawMessage
	for _, b := range ms.messageBatches {
		msgs = append(msgs, b.Messages...)
	}
	return msgs
}

// waitForMessageType waits for a message of the given type to arrive.
// Returns the first matching message or an error on timeout.
func (ms *mockUplinkServer) waitForMessageType(msgType string, timeout time.Duration) (json.RawMessage, error) {
	deadline := time.After(timeout)
	for {
		msgs := ms.getMessages()
		for _, raw := range msgs {
			var envelope struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &envelope) == nil && envelope.Type == msgType {
				return raw, nil
			}
		}
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for message type %q (got %d messages total)", msgType, len(msgs))
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// waitForMessages waits until at least n messages have been received.
func (ms *mockUplinkServer) waitForMessages(n int, timeout time.Duration) ([]json.RawMessage, error) {
	deadline := time.After(timeout)
	for {
		msgs := ms.getMessages()
		if len(msgs) >= n {
			return msgs, nil
		}
		select {
		case <-deadline:
			return msgs, fmt.Errorf("timeout waiting for %d messages (got %d)", n, len(msgs))
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// sendCommand sends a command to the CLI via the Pusher server.
func (ms *mockUplinkServer) sendCommand(command interface{}) error {
	data, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("marshaling command: %w", err)
	}
	channel := fmt.Sprintf("private-chief-server.%d", 42) // device ID 42
	return ms.pusherSrv.sendCommand(channel, data)
}

// waitForPusherSubscribe waits for the CLI to subscribe to its Pusher channel.
func (ms *mockUplinkServer) waitForPusherSubscribe(timeout time.Duration) error {
	select {
	case <-ms.pusherSrv.onSubscribe:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for Pusher subscription")
	}
}

// generateTestAuthSignature generates a Pusher auth signature for testing.
func generateTestAuthSignature(appKey, appSecret, socketID, channelName string) string {
	toSign := socketID + ":" + channelName
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(toSign))
	sig := hex.EncodeToString(mac.Sum(nil))
	return appKey + ":" + sig
}

// mockPusherServer is a minimal Pusher protocol WebSocket server for testing.
type mockPusherServer struct {
	srv      *httptest.Server
	upgrader websocket.Upgrader

	mu   sync.Mutex
	conn *websocket.Conn

	appKey          string
	appSecret       string
	socketID        string
	activityTimeout int

	onSubscribe chan string
}

type mockReverbConfig struct {
	Key    string `json:"key"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme"`
}

func newMockPusherServer(t *testing.T) *mockPusherServer {
	t.Helper()

	ps := &mockPusherServer{
		appKey:          "test-app-key",
		appSecret:       "test-app-secret",
		socketID:        "123456.7890",
		activityTimeout: 120,
		onSubscribe:     make(chan string, 10),
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

type pusherMsg struct {
	Event   string          `json:"event"`
	Data    json.RawMessage `json:"data,omitempty"`
	Channel string          `json:"channel,omitempty"`
}

type pusherConnData struct {
	SocketID        string `json:"socket_id"`
	ActivityTimeout int    `json:"activity_timeout"`
}

func (ps *mockPusherServer) handleWS(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	expectedPath := fmt.Sprintf("/app/%s", ps.appKey)
	if !strings.HasPrefix(r.URL.Path, expectedPath) {
		http.Error(w, "invalid path", http.StatusNotFound)
		return
	}

	conn, err := ps.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	ps.mu.Lock()
	ps.conn = conn
	ps.mu.Unlock()

	// Send connection_established.
	connData, _ := json.Marshal(pusherConnData{
		SocketID:        ps.socketID,
		ActivityTimeout: ps.activityTimeout,
	})
	conn.WriteJSON(pusherMsg{
		Event: "pusher:connection_established",
		Data:  connData,
	})

	// Read loop.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var msg pusherMsg
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

			conn.WriteJSON(pusherMsg{
				Event:   "pusher_internal:subscription_succeeded",
				Channel: channel,
				Data:    json.RawMessage("{}"),
			})

		case "pusher:pong":
			// Ignore pong responses.
		}
	}
}

// sendCommand sends a chief.command event to the connected client.
func (ps *mockPusherServer) sendCommand(channel string, command json.RawMessage) error {
	ps.mu.Lock()
	conn := ps.conn
	ps.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no client connected")
	}

	return conn.WriteJSON(pusherMsg{
		Event:   "chief.command",
		Channel: channel,
		Data:    command,
	})
}

// reverbConfig returns a config pointing at the test server.
func (ps *mockPusherServer) reverbConfig() mockReverbConfig {
	addr := ps.srv.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	host := parts[0]
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	return mockReverbConfig{
		Key:    ps.appKey,
		Host:   host,
		Port:   port,
		Scheme: "http",
	}
}
