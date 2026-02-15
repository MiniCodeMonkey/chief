package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsURL converts an httptest.Server URL to a WebSocket URL.
func wsURL(s *httptest.Server) string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

// echoServer returns an httptest.Server that upgrades to WebSocket and echoes messages.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("echo server upgrade error: %v", err)
			return
		}
		defer conn.Close()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, data); err != nil {
				return
			}
		}
	}))
}

func TestConnectAndSendReceive(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	client := New(wsURL(srv))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Send a typed message.
	msg := map[string]string{
		"type":      "test",
		"id":        "abc-123",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      "hello",
	}
	if err := client.Send(msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Read it back.
	select {
	case received := <-client.Receive():
		if received.Type != "test" {
			t.Errorf("expected type=test, got %q", received.Type)
		}
		// Verify raw data round-trips.
		var raw map[string]string
		if err := json.Unmarshal(received.Raw, &raw); err != nil {
			t.Fatalf("unmarshal raw: %v", err)
		}
		if raw["data"] != "hello" {
			t.Errorf("expected data=hello, got %q", raw["data"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestGracefulClose(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	client := New(wsURL(srv))
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Receive channel should be closed.
	_, ok := <-client.Receive()
	if ok {
		t.Error("expected receive channel to be closed")
	}

	// Double close should be safe.
	if err := client.Close(); err != nil {
		t.Fatalf("double Close failed: %v", err)
	}
}

func TestSendWhenNotConnected(t *testing.T) {
	client := New("ws://localhost:1")

	err := client.Send(map[string]string{"type": "test"})
	if err == nil {
		t.Fatal("expected error sending on unconnected client")
	}
}

func TestReconnectOnServerClose(t *testing.T) {
	// Track connections.
	var connCount atomic.Int32
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		count := connCount.Add(1)
		if count == 1 {
			// First connection: close immediately to trigger reconnect.
			conn.Close()
			return
		}
		// Second connection: echo.
		defer conn.Close()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, data); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	var reconCalled atomic.Int32
	client := New(wsURL(srv), WithOnReconnect(func() {
		reconCalled.Add(1)
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Wait for reconnection.
	deadline := time.After(8 * time.Second)
	for reconCalled.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for reconnection callback")
		case <-time.After(100 * time.Millisecond):
		}
	}

	// After reconnect, send/receive should work.
	msg := map[string]string{"type": "ping_test"}
	if err := client.Send(msg); err != nil {
		t.Fatalf("Send after reconnect failed: %v", err)
	}

	select {
	case received := <-client.Receive():
		if received.Type != "ping_test" {
			t.Errorf("expected type=ping_test, got %q", received.Type)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message after reconnect")
	}
}

func TestContextCancellation(t *testing.T) {
	// Server that never accepts connections (unreachable port).
	ctx, cancel := context.WithCancel(context.Background())

	client := New("ws://127.0.0.1:1") // Port 1 — connection will fail
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Connect(ctx)
	}()

	// Cancel quickly.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from cancelled context")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: Connect did not respect context cancellation")
	}
}

func TestPingPong(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var pongReceived atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Set pong handler to track response.
		conn.SetPongHandler(func(string) error {
			pongReceived.Add(1)
			return nil
		})

		// Send a ping to the client.
		if err := conn.WriteControl(websocket.PingMessage, []byte("hello"), time.Now().Add(5*time.Second)); err != nil {
			t.Logf("ping send error: %v", err)
			return
		}

		// Read messages to keep connection alive and receive pong.
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	client := New(wsURL(srv))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Wait for the pong to be received.
	deadline := time.After(3 * time.Second)
	for pongReceived.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for pong response")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		minMs   int64
		maxMs   int64
	}{
		{1, 500, 1500},    // 1s * (0.5 to 1.5)
		{2, 1000, 3000},   // 2s * (0.5 to 1.5)
		{3, 2000, 6000},   // 4s * (0.5 to 1.5)
		{4, 4000, 12000},  // 8s * (0.5 to 1.5)
		{10, 30000, 90000}, // capped at 60s * (0.5 to 1.5)
	}

	for _, tt := range tests {
		d := backoff(tt.attempt)
		ms := d.Milliseconds()
		if ms < tt.minMs || ms > tt.maxMs {
			t.Errorf("backoff(%d) = %dms, want [%d, %d]ms", tt.attempt, ms, tt.minMs, tt.maxMs)
		}
	}
}

func TestDefaultURL(t *testing.T) {
	if DefaultURL != "wss://chiefloop.com/ws/server" {
		t.Errorf("DefaultURL = %q, want wss://chiefloop.com/ws/server", DefaultURL)
	}
}

func TestReceiveChannelBuffered(t *testing.T) {
	client := New("ws://localhost:1")
	ch := client.Receive()

	// Channel should be buffered.
	if cap(ch) != receiveBufSize {
		t.Errorf("receive channel capacity = %d, want %d", cap(ch), receiveBufSize)
	}
}

func TestMultipleMessages(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	client := New(wsURL(srv))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Send multiple messages.
	for i := 0; i < 5; i++ {
		msg := map[string]interface{}{
			"type": "test",
			"seq":  i,
		}
		if err := client.Send(msg); err != nil {
			t.Fatalf("Send %d failed: %v", i, err)
		}
	}

	// Receive all of them.
	for i := 0; i < 5; i++ {
		select {
		case received := <-client.Receive():
			if received.Type != "test" {
				t.Errorf("message %d: expected type=test, got %q", i, received.Type)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for message %d", i)
		}
	}
}
