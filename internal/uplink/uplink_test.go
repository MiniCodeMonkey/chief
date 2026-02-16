package uplink

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testUplinkServer combines a mock HTTP API server and a mock Pusher WebSocket server
// for end-to-end Uplink testing.
type testUplinkServer struct {
	httpSrv   *httptest.Server
	pusherSrv *testPusherServer

	mu             sync.Mutex
	connectCalls   atomic.Int32
	disconnectCalls atomic.Int32
	heartbeatCalls atomic.Int32
	messageBatches []messageBatch

	// Last received connect metadata.
	lastConnectBody map[string]interface{}
}

type messageBatch struct {
	BatchID  string
	Messages []json.RawMessage
}

func newTestUplinkServer(t *testing.T) *testUplinkServer {
	t.Helper()

	ps := newTestPusherServer(t)

	us := &testUplinkServer{
		pusherSrv: ps,
	}

	// Build the Reverb config from the Pusher server.
	reverbCfg := ps.reverbConfig()

	us.httpSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		us.handleHTTP(t, w, r, reverbCfg)
	}))
	t.Cleanup(func() { us.httpSrv.Close() })

	return us
}

func (us *testUplinkServer) handleHTTP(t *testing.T, w http.ResponseWriter, r *http.Request, reverbCfg ReverbConfig) {
	t.Helper()

	// Check auth header.
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/api/device/connect":
		us.connectCalls.Add(1)

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		us.mu.Lock()
		us.lastConnectBody = body
		us.mu.Unlock()

		json.NewEncoder(w).Encode(WelcomeResponse{
			Type:            "welcome",
			ProtocolVersion: 1,
			DeviceID:        42,
			SessionID:       "test-session-abc",
			Reverb:          reverbCfg,
		})

	case "/api/device/disconnect":
		us.disconnectCalls.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})

	case "/api/device/heartbeat":
		us.heartbeatCalls.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	case "/api/device/messages":
		var req ingestRequest
		json.NewDecoder(r.Body).Decode(&req)

		us.mu.Lock()
		us.messageBatches = append(us.messageBatches, messageBatch{
			BatchID:  req.BatchID,
			Messages: req.Messages,
		})
		us.mu.Unlock()

		json.NewEncoder(w).Encode(IngestResponse{
			Accepted:  len(req.Messages),
			BatchID:   req.BatchID,
			SessionID: "test-session-abc",
		})

	case "/api/device/broadcasting/auth":
		var body broadcastAuthRequest
		json.NewDecoder(r.Body).Decode(&body)

		sig := GenerateAuthSignature(
			us.pusherSrv.appKey,
			us.pusherSrv.appSecret,
			body.SocketID,
			body.ChannelName,
		)
		json.NewEncoder(w).Encode(pusherAuthResponse{Auth: sig})

	default:
		http.NotFound(w, r)
	}
}

func (us *testUplinkServer) getMessageBatches() []messageBatch {
	us.mu.Lock()
	defer us.mu.Unlock()
	result := make([]messageBatch, len(us.messageBatches))
	copy(result, us.messageBatches)
	return result
}

// newTestUplink creates an Uplink connected to the test servers.
func newTestUplink(t *testing.T, us *testUplinkServer, opts ...UplinkOption) *Uplink {
	t.Helper()

	client := newTestClient(t, us.httpSrv.URL, "test-token")
	u := NewUplink(client, opts...)
	return u
}

// --- Tests ---

func TestUplink_FullLifecycle(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	ctx := testContext(t)

	// Connect.
	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Wait for Pusher subscription.
	select {
	case <-us.pusherSrv.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Pusher subscription")
	}

	// Verify connect was called.
	if got := us.connectCalls.Load(); got != 1 {
		t.Errorf("connect calls = %d, want 1", got)
	}

	// Verify session/device IDs.
	if got := u.SessionID(); got != "test-session-abc" {
		t.Errorf("SessionID() = %q, want %q", got, "test-session-abc")
	}
	if got := u.DeviceID(); got != 42 {
		t.Errorf("DeviceID() = %d, want 42", got)
	}

	// Send a message (immediate tier — should flush right away).
	msg := json.RawMessage(`{"type":"run_complete","project":"test"}`)
	u.Send(msg, "run_complete")

	// Wait for the batcher to flush.
	deadline := time.After(5 * time.Second)
	for {
		batches := us.getMessageBatches()
		if len(batches) > 0 {
			if len(batches[0].Messages) != 1 {
				t.Errorf("batch has %d messages, want 1", len(batches[0].Messages))
			}
			var parsed map[string]interface{}
			json.Unmarshal(batches[0].Messages[0], &parsed)
			if parsed["type"] != "run_complete" {
				t.Errorf("message type = %v, want run_complete", parsed["type"])
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for message batch to be sent")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Receive a command from the server via Pusher.
	channel := fmt.Sprintf("private-chief-server.%d", u.DeviceID())
	cmd := json.RawMessage(`{"type":"start_run","project":"myapp"}`)
	if err := us.pusherSrv.sendCommand(channel, cmd); err != nil {
		t.Fatalf("sendCommand failed: %v", err)
	}

	select {
	case received := <-u.Receive():
		var parsed map[string]interface{}
		json.Unmarshal(received, &parsed)
		if parsed["type"] != "start_run" {
			t.Errorf("received type = %v, want start_run", parsed["type"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for command")
	}

	// Close.
	if err := u.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Verify disconnect was called.
	if got := us.disconnectCalls.Load(); got != 1 {
		t.Errorf("disconnect calls = %d, want 1", got)
	}
}

func TestUplink_SessionIDAndDeviceID(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	// Before connect, values should be zero/empty.
	if got := u.SessionID(); got != "" {
		t.Errorf("SessionID() before connect = %q, want empty", got)
	}
	if got := u.DeviceID(); got != 0 {
		t.Errorf("DeviceID() before connect = %d, want 0", got)
	}

	ctx := testContext(t)
	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Wait for Pusher subscription.
	select {
	case <-us.pusherSrv.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	if got := u.SessionID(); got != "test-session-abc" {
		t.Errorf("SessionID() = %q, want %q", got, "test-session-abc")
	}
	if got := u.DeviceID(); got != 42 {
		t.Errorf("DeviceID() = %d, want 42", got)
	}

	u.Close()
}

func TestUplink_SendEnqueuesToBatcher(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	ctx := testContext(t)
	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer u.Close()

	// Wait for Pusher subscription.
	select {
	case <-us.pusherSrv.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Send multiple messages of different tiers.
	u.Send(json.RawMessage(`{"type":"error","msg":"oops"}`), "error")                     // immediate
	u.Send(json.RawMessage(`{"type":"claude_output","data":"hello"}`), "claude_output")    // standard
	u.Send(json.RawMessage(`{"type":"project_state","data":"state"}`), "project_state")    // low priority

	// The immediate message triggers a flush that drains all tiers.
	deadline := time.After(5 * time.Second)
	for {
		batches := us.getMessageBatches()
		if len(batches) > 0 {
			// All three messages should be in the first batch (immediate triggers drain of all).
			if len(batches[0].Messages) != 3 {
				t.Errorf("batch has %d messages, want 3", len(batches[0].Messages))
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for batched messages")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestUplink_SendBeforeConnect(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	// Send before connect — should be silently dropped.
	u.Send(json.RawMessage(`{"type":"error"}`), "error")

	// No crash, no messages sent.
	time.Sleep(100 * time.Millisecond)
	batches := us.getMessageBatches()
	if len(batches) != 0 {
		t.Errorf("expected 0 batches before connect, got %d", len(batches))
	}
}

func TestUplink_ReceiveFromPusher(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	ctx := testContext(t)
	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer u.Close()

	// Wait for Pusher subscription.
	select {
	case <-us.pusherSrv.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	channel := fmt.Sprintf("private-chief-server.%d", u.DeviceID())

	// Send 3 commands.
	for i := 0; i < 3; i++ {
		cmd := json.RawMessage(fmt.Sprintf(`{"type":"cmd","id":"%d"}`, i))
		if err := us.pusherSrv.sendCommand(channel, cmd); err != nil {
			t.Fatalf("sendCommand(%d) failed: %v", i, err)
		}
	}

	// Receive all 3 in order.
	for i := 0; i < 3; i++ {
		select {
		case received := <-u.Receive():
			var parsed map[string]interface{}
			json.Unmarshal(received, &parsed)
			want := fmt.Sprintf("%d", i)
			if parsed["id"] != want {
				t.Errorf("command %d: id = %v, want %v", i, parsed["id"], want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for command %d", i)
		}
	}
}

func TestUplink_Close_FlushesAndDisconnects(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	ctx := testContext(t)
	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Wait for Pusher subscription.
	select {
	case <-us.pusherSrv.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Enqueue a low-priority message (wouldn't normally flush for 1s).
	u.Send(json.RawMessage(`{"type":"settings","data":"config"}`), "settings")

	// Close should flush the remaining message before disconnecting.
	if err := u.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Verify the message was flushed.
	batches := us.getMessageBatches()
	if len(batches) == 0 {
		t.Error("expected at least 1 batch after Close(), got 0")
	} else {
		found := false
		for _, batch := range batches {
			for _, msg := range batch.Messages {
				var parsed map[string]interface{}
				json.Unmarshal(msg, &parsed)
				if parsed["type"] == "settings" {
					found = true
				}
			}
		}
		if !found {
			t.Error("settings message was not flushed on Close()")
		}
	}

	// Verify disconnect was called.
	if got := us.disconnectCalls.Load(); got != 1 {
		t.Errorf("disconnect calls = %d, want 1", got)
	}
}

func TestUplink_Close_DoubleCloseIsSafe(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	ctx := testContext(t)
	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Wait for Pusher subscription.
	select {
	case <-us.pusherSrv.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// First close.
	if err := u.Close(); err != nil {
		t.Fatalf("first Close() failed: %v", err)
	}

	// Second close should be a no-op.
	if err := u.Close(); err != nil {
		t.Fatalf("second Close() failed: %v", err)
	}

	// Only one disconnect call.
	if got := us.disconnectCalls.Load(); got != 1 {
		t.Errorf("disconnect calls = %d, want 1", got)
	}
}

func TestUplink_SetAccessToken(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	ctx := testContext(t)
	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer u.Close()

	// Wait for Pusher subscription.
	select {
	case <-us.pusherSrv.onSubscribe:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}

	// Update the token.
	u.SetAccessToken("new-token-xyz")

	// The internal client should use the new token.
	// We can verify this by checking the client's token directly.
	u.client.mu.RLock()
	token := u.client.accessToken
	u.client.mu.RUnlock()

	if token != "new-token-xyz" {
		t.Errorf("accessToken = %q, want %q", token, "new-token-xyz")
	}
}

func TestUplink_OnReconnectCallback(t *testing.T) {
	us := newTestUplinkServer(t)

	var callCount atomic.Int32
	u := newTestUplink(t, us, WithOnReconnect(func() {
		callCount.Add(1)
	}))

	// Verify the callback is stored.
	if u.onReconnect == nil {
		t.Fatal("onReconnect should be set")
	}

	// The callback itself is used by the reconnection logic (US-020).
	// For now just verify it can be invoked.
	u.onReconnect()
	if got := callCount.Load(); got != 1 {
		t.Errorf("callback count = %d, want 1", got)
	}
}

func TestUplink_ConnectFailure_HTTPError(t *testing.T) {
	// HTTP server that rejects connect.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "bad-token")
	u := NewUplink(client)

	ctx := testContext(t)
	err := u.Connect(ctx)
	if err == nil {
		t.Fatal("expected error when connect fails, got nil")
	}
	if !strings.Contains(err.Error(), "uplink connect") {
		t.Errorf("error = %v, want containing 'uplink connect'", err)
	}

	// Should not be connected.
	if u.SessionID() != "" {
		t.Error("SessionID should be empty after failed connect")
	}
}

func TestUplink_ConnectFailure_PusherError(t *testing.T) {
	// HTTP server that succeeds for connect but Pusher server that rejects auth.
	ps := newTestPusherServer(t)
	ps.rejectSubscribe = true
	reverbCfg := ps.reverbConfig()

	var disconnectCalled atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/device/connect":
			json.NewEncoder(w).Encode(WelcomeResponse{
				Type:            "welcome",
				ProtocolVersion: 1,
				DeviceID:        42,
				SessionID:       "sess-123",
				Reverb:          reverbCfg,
			})
		case "/api/device/disconnect":
			disconnectCalled.Add(1)
			json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})
		case "/api/device/broadcasting/auth":
			sig := GenerateAuthSignature(ps.appKey, ps.appSecret, "unused", "unused")
			json.NewEncoder(w).Encode(pusherAuthResponse{Auth: sig})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, "test-token")
	u := NewUplink(client)

	ctx := testContext(t)
	err := u.Connect(ctx)
	if err == nil {
		t.Fatal("expected error when Pusher subscription fails, got nil")
	}
	if !strings.Contains(err.Error(), "pusher") {
		t.Errorf("error = %v, want containing 'pusher'", err)
	}

	// HTTP disconnect should have been called as cleanup.
	time.Sleep(100 * time.Millisecond)
	if got := disconnectCalled.Load(); got != 1 {
		t.Errorf("disconnect calls after Pusher failure = %d, want 1", got)
	}
}

func TestUplink_ChannelName(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplink(t, us)

	ctx := testContext(t)
	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer u.Close()

	// Verify the Pusher client subscribes to the correct channel.
	select {
	case channel := <-us.pusherSrv.onSubscribe:
		expected := "private-chief-server.42"
		if channel != expected {
			t.Errorf("subscribed to %q, want %q", channel, expected)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription")
	}
}
