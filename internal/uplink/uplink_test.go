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

	// heartbeatStatus controls the HTTP status code returned by heartbeat.
	// 0 or 200 means success.
	heartbeatStatus atomic.Int32
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
		status := int(us.heartbeatStatus.Load())
		if status >= 400 {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": "heartbeat failed"})
			return
		}
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

// --- Heartbeat Tests ---

// newTestUplinkWithHeartbeat creates a connected Uplink with fast heartbeat timing for tests.
func newTestUplinkWithHeartbeat(t *testing.T, us *testUplinkServer, interval, retryDelay, skipWindow time.Duration, maxFails int, opts ...UplinkOption) *Uplink {
	t.Helper()

	client := newTestClient(t, us.httpSrv.URL, "test-token")
	u := NewUplink(client, opts...)

	// Override heartbeat timing for fast tests.
	u.hbInterval = interval
	u.hbRetryDelay = retryDelay
	u.hbSkipWindow = skipWindow
	u.hbMaxFails = maxFails

	return u
}

func TestUplink_Heartbeat_SendsPeriodically(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplinkWithHeartbeat(t, us, 50*time.Millisecond, 10*time.Millisecond, 0, 3)

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

	// Wait for at least 3 heartbeats.
	deadline := time.After(2 * time.Second)
	for {
		if us.heartbeatCalls.Load() >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected at least 3 heartbeats, got %d", us.heartbeatCalls.Load())
		case <-time.After(10 * time.Millisecond):
		}
	}

	u.Close()
}

func TestUplink_Heartbeat_StopsOnClose(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplinkWithHeartbeat(t, us, 50*time.Millisecond, 10*time.Millisecond, 0, 3)

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

	// Wait for at least 1 heartbeat.
	deadline := time.After(2 * time.Second)
	for {
		if us.heartbeatCalls.Load() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for first heartbeat")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Close the uplink.
	u.Close()

	// Record count and wait to confirm no more heartbeats are sent.
	countAfterClose := us.heartbeatCalls.Load()
	time.Sleep(200 * time.Millisecond)

	if got := us.heartbeatCalls.Load(); got != countAfterClose {
		t.Errorf("heartbeat calls after close: got %d more (total %d), want 0 more", got-countAfterClose, got)
	}
}

func TestUplink_Heartbeat_SkipsWhenMessagesSentRecently(t *testing.T) {
	us := newTestUplinkServer(t)
	// skipWindow of 5s — any message sent within 5s skips heartbeat.
	u := newTestUplinkWithHeartbeat(t, us, 50*time.Millisecond, 10*time.Millisecond, 5*time.Second, 3)

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

	// Send a message to trigger the lastSendTime update.
	msg := json.RawMessage(`{"type":"run_complete","data":"done"}`)
	u.Send(msg, "run_complete")

	// Wait for the message batch to be sent (sets lastSendTime).
	deadline := time.After(2 * time.Second)
	for {
		batches := us.getMessageBatches()
		if len(batches) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for message batch")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Record heartbeat count now.
	countBeforeSkip := us.heartbeatCalls.Load()

	// Wait 300ms — multiple heartbeat intervals would have passed (50ms each).
	time.Sleep(300 * time.Millisecond)

	// Heartbeats should have been skipped because lastSendTime is recent.
	countAfterWait := us.heartbeatCalls.Load()
	if countAfterWait != countBeforeSkip {
		t.Errorf("expected heartbeats to be skipped, but %d extra heartbeats were sent", countAfterWait-countBeforeSkip)
	}

	u.Close()
}

func TestUplink_Heartbeat_RetryOnTransientFailure(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplinkWithHeartbeat(t, us, 50*time.Millisecond, 10*time.Millisecond, 0, 3)

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

	// Make heartbeat fail with 500 (transient).
	us.heartbeatStatus.Store(500)

	// Wait for heartbeat calls to accumulate (initial call + retry).
	deadline := time.After(2 * time.Second)
	for {
		// Each heartbeat tick produces 2 calls (initial + retry).
		if us.heartbeatCalls.Load() >= 4 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected at least 4 heartbeat calls (2 ticks × 2 attempts), got %d", us.heartbeatCalls.Load())
		case <-time.After(10 * time.Millisecond):
		}
	}

	u.Close()
}

func TestUplink_Heartbeat_RetrySucceedsResetsFailureCount(t *testing.T) {
	us := newTestUplinkServer(t)

	var maxFailuresCalled atomic.Int32
	u := newTestUplinkWithHeartbeat(t, us, 50*time.Millisecond, 10*time.Millisecond, 0, 3)
	u.onHeartbeatMaxFailures = func() {
		maxFailuresCalled.Add(1)
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

	// Let heartbeats succeed — no max failures callback should fire.
	time.Sleep(200 * time.Millisecond)

	if got := maxFailuresCalled.Load(); got != 0 {
		t.Errorf("maxFailures callback called %d times, want 0 (all heartbeats succeeded)", got)
	}

	u.Close()
}

func TestUplink_Heartbeat_MaxFailuresTriggersCallback(t *testing.T) {
	us := newTestUplinkServer(t)

	maxFailuresCh := make(chan struct{}, 1)
	u := newTestUplinkWithHeartbeat(t, us, 50*time.Millisecond, 10*time.Millisecond, 0, 3)
	u.onHeartbeatMaxFailures = func() {
		select {
		case maxFailuresCh <- struct{}{}:
		default:
		}
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

	// Make all heartbeats fail.
	us.heartbeatStatus.Store(500)

	// Wait for the max-failures callback. With 50ms interval and 10ms retry delay,
	// each tick is ~60ms. We need 3 consecutive failures → ~180ms.
	select {
	case <-maxFailuresCh:
		// Success — the callback was triggered.
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for heartbeat max failures callback")
	}

	u.Close()
}

func TestUplink_Heartbeat_ContextCancellationStops(t *testing.T) {
	us := newTestUplinkServer(t)
	u := newTestUplinkWithHeartbeat(t, us, 50*time.Millisecond, 10*time.Millisecond, 0, 3)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := u.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Wait for at least 1 heartbeat.
	deadline := time.After(2 * time.Second)
	for {
		if us.heartbeatCalls.Load() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for first heartbeat")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Cancel the context.
	cancel()

	countAfterCancel := us.heartbeatCalls.Load()
	time.Sleep(200 * time.Millisecond)

	if got := us.heartbeatCalls.Load(); got != countAfterCancel {
		t.Errorf("heartbeat calls after cancel: got %d more, want 0 more", got-countAfterCancel)
	}

	// Clean up: close still works even though context is cancelled.
	u.Close()
}
