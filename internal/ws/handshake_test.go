package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// handshakeServer returns a test server that performs the handshake protocol.
// responseType controls what the server responds with: "welcome", "incompatible", "auth_failed", or "none" (no response).
func handshakeServer(t *testing.T, responseType string) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("handshake server upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Read the hello message.
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Logf("handshake server read error: %v", err)
			return
		}

		var hello HelloMessage
		if err := json.Unmarshal(data, &hello); err != nil {
			t.Logf("handshake server unmarshal error: %v", err)
			return
		}

		if hello.Type != "hello" {
			t.Logf("handshake server: expected hello, got %q", hello.Type)
			return
		}

		switch responseType {
		case "welcome":
			resp := WelcomeMessage{
				Type:      "welcome",
				ID:        newUUID(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			respData, _ := json.Marshal(resp)
			conn.WriteMessage(websocket.TextMessage, respData)

		case "incompatible":
			resp := IncompatibleMessage{
				Type:      "incompatible",
				ID:        newUUID(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Message:   "Chief v0.3.0 is too old. Please update to v0.5.0 or later.",
			}
			respData, _ := json.Marshal(resp)
			conn.WriteMessage(websocket.TextMessage, respData)

		case "auth_failed":
			resp := AuthFailedMessage{
				Type:      "auth_failed",
				ID:        newUUID(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Message:   "invalid or expired token",
			}
			respData, _ := json.Marshal(resp)
			conn.WriteMessage(websocket.TextMessage, respData)

		case "none":
			// Don't respond — test timeout.
			time.Sleep(15 * time.Second)
		}

		// Keep connection open for a bit.
		time.Sleep(1 * time.Second)
	}))
}

func TestHandshakeSuccess(t *testing.T) {
	srv := handshakeServer(t, "welcome")
	defer srv.Close()

	client := New(wsURL(srv))
	ctx := testContext(t)

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	err := client.Handshake("test-token", "0.5.0", "test-device")
	if err != nil {
		t.Fatalf("Handshake failed: %v", err)
	}
}

func TestHandshakeIncompatible(t *testing.T) {
	srv := handshakeServer(t, "incompatible")
	defer srv.Close()

	client := New(wsURL(srv))
	ctx := testContext(t)

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	err := client.Handshake("test-token", "0.3.0", "test-device")
	if err == nil {
		t.Fatal("expected error for incompatible version")
	}

	incErr, ok := err.(*ErrIncompatible)
	if !ok {
		t.Fatalf("expected *ErrIncompatible, got %T: %v", err, err)
	}

	if !strings.Contains(incErr.Message, "too old") {
		t.Errorf("expected message about old version, got %q", incErr.Message)
	}
}

func TestHandshakeAuthFailed(t *testing.T) {
	srv := handshakeServer(t, "auth_failed")
	defer srv.Close()

	client := New(wsURL(srv))
	ctx := testContext(t)

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	err := client.Handshake("bad-token", "0.5.0", "test-device")
	if err == nil {
		t.Fatal("expected error for auth failure")
	}

	if err != ErrAuthFailed {
		t.Errorf("expected ErrAuthFailed, got %v", err)
	}
}

func TestHandshakeTimeout(t *testing.T) {
	srv := handshakeServer(t, "none")
	defer srv.Close()

	client := New(wsURL(srv))
	ctx := testContext(t)

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	err := client.Handshake("test-token", "0.5.0", "test-device")
	if err == nil {
		t.Fatal("expected error for timeout")
	}

	if err != ErrHandshakeTimeout {
		t.Errorf("expected ErrHandshakeTimeout, got %v", err)
	}
}

func TestHandshakeSendsCorrectHello(t *testing.T) {
	var receivedHello HelloMessage
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		json.Unmarshal(data, &receivedHello)

		// Respond with welcome.
		resp, _ := json.Marshal(WelcomeMessage{Type: "welcome"})
		conn.WriteMessage(websocket.TextMessage, resp)

		time.Sleep(1 * time.Second)
	}))
	defer srv.Close()

	client := New(wsURL(srv))
	ctx := testContext(t)

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	err := client.Handshake("my-access-token", "1.2.3", "my-server")
	if err != nil {
		t.Fatalf("Handshake failed: %v", err)
	}

	// Verify the hello message contents.
	if receivedHello.Type != "hello" {
		t.Errorf("expected type=hello, got %q", receivedHello.Type)
	}
	if receivedHello.ProtocolVersion != ProtocolVersion {
		t.Errorf("expected protocol_version=%d, got %d", ProtocolVersion, receivedHello.ProtocolVersion)
	}
	if receivedHello.ChiefVersion != "1.2.3" {
		t.Errorf("expected chief_version=1.2.3, got %q", receivedHello.ChiefVersion)
	}
	if receivedHello.DeviceName != "my-server" {
		t.Errorf("expected device_name=my-server, got %q", receivedHello.DeviceName)
	}
	if receivedHello.AccessToken != "my-access-token" {
		t.Errorf("expected access_token=my-access-token, got %q", receivedHello.AccessToken)
	}
	if receivedHello.OS == "" {
		t.Error("expected os to be set")
	}
	if receivedHello.Arch == "" {
		t.Error("expected arch to be set")
	}
	if receivedHello.ID == "" {
		t.Error("expected id (UUID) to be set")
	}
	if receivedHello.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}

	// Verify timestamp is valid ISO8601.
	if _, err := time.Parse(time.RFC3339, receivedHello.Timestamp); err != nil {
		t.Errorf("timestamp is not valid RFC3339: %q", receivedHello.Timestamp)
	}
}

func TestHandshakeConnectionClosed(t *testing.T) {
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Read hello then close immediately without responding.
		conn.ReadMessage()
		conn.Close()
	}))
	defer srv.Close()

	client := New(wsURL(srv))
	ctx := testContext(t)

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	err := client.Handshake("test-token", "0.5.0", "test-device")
	if err == nil {
		t.Fatal("expected error when connection closed during handshake")
	}
}

func TestNewUUIDFormat(t *testing.T) {
	uuid := newUUID()

	// UUID format: 8-4-4-4-12 hex characters.
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Fatalf("expected 5 UUID parts, got %d: %q", len(parts), uuid)
	}

	expectedLens := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != expectedLens[i] {
			t.Errorf("UUID part %d: expected %d chars, got %d: %q", i, expectedLens[i], len(part), part)
		}
	}

	// Version 4 check: third group starts with '4'.
	if parts[2][0] != '4' {
		t.Errorf("expected UUID v4 (third group starts with '4'), got %q", parts[2])
	}

	// Uniqueness check.
	uuid2 := newUUID()
	if uuid == uuid2 {
		t.Error("two consecutive UUIDs should be different")
	}
}

func TestNewMessage(t *testing.T) {
	msg := NewMessage("test_type")

	if msg.Type != "test_type" {
		t.Errorf("expected type=test_type, got %q", msg.Type)
	}
	if msg.ID == "" {
		t.Error("expected id to be set")
	}
	if msg.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
	if _, err := time.Parse(time.RFC3339, msg.Timestamp); err != nil {
		t.Errorf("timestamp is not valid RFC3339: %q", msg.Timestamp)
	}
}

// testContext returns a context with a reasonable timeout for tests.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}
