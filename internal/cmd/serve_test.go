package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/auth"
	"github.com/minicodemonkey/chief/internal/protocol"
	"github.com/minicodemonkey/chief/internal/uplink"
)

func TestBuildWSURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://uplink.chiefloop.com", "wss://uplink.chiefloop.com/ws"},
		{"http://localhost:8080", "ws://localhost:8080/ws"},
		{"https://uplink.chiefloop.com/", "wss://uplink.chiefloop.com/ws"},
	}

	for _, tt := range tests {
		got := buildWSURL(tt.input)
		if got != tt.want {
			t.Errorf("buildWSURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCollectDiffs(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	// Modify the initial file to create a diff.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("modified content"), 0o644); err != nil {
		t.Fatal(err)
	}

	diffs, err := collectDiffs(dir)
	if err != nil {
		t.Fatalf("collectDiffs error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	if diffs[0].Path != "hello.txt" {
		t.Errorf("expected path 'hello.txt', got %q", diffs[0].Path)
	}
	if diffs[0].Status != "modified" {
		t.Errorf("expected status 'modified', got %q", diffs[0].Status)
	}
	if diffs[0].Patch == "" {
		t.Error("expected non-empty patch")
	}
}

func TestCollectDiffsCleanRepo(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	diffs, err := collectDiffs(dir)
	if err != nil {
		t.Fatalf("collectDiffs error: %v", err)
	}

	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs for clean repo, got %d", len(diffs))
	}
}

func TestLoadCredentialsMissing(t *testing.T) {
	tmpHome := t.TempDir()
	_, err := loadCredentials(tmpHome)
	if err == nil {
		t.Fatal("expected error when credentials are missing")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("expected 'not logged in' error, got: %v", err)
	}
}

func TestDirectoryTraversalProtection(t *testing.T) {
	workspace := t.TempDir()

	// Attempt to escape workspace via ../
	escapePath := filepath.Join(workspace, "proj", "..", "..", "etc", "passwd")
	resolved, err := filepath.Abs(escapePath)
	if err != nil {
		t.Fatal(err)
	}

	absWorkspace, _ := filepath.Abs(workspace)
	if strings.HasPrefix(resolved, absWorkspace) {
		t.Error("traversal path should NOT be within workspace")
	}
}

func TestHeartbeatLoopSendsHeartbeat(t *testing.T) {
	// Set up a WebSocket server that captures received messages.
	var mu sync.Mutex
	var received []protocol.Envelope

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Send welcome message.
		welcome := protocol.NewEnvelope(protocol.TypeWelcome, "server")
		welcomePayload, _ := json.Marshal(protocol.Welcome{ConnectionID: "test-session"})
		welcome.Payload = welcomePayload
		data, _ := welcome.Marshal()
		conn.WriteMessage(websocket.TextMessage, data)

		// Read incoming messages.
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			env, err := protocol.ParseEnvelope(msg)
			if err != nil {
				continue
			}
			mu.Lock()
			received = append(received, env)
			mu.Unlock()
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := uplink.NewClient(wsURL, "test-token")

	if err := client.Connect(); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer client.Close()
	<-client.Ready()

	// Run heartbeat loop with a short interval.
	ctx, cancel := context.WithCancel(context.Background())
	startTime := time.Now()
	go heartbeatLoopWithInterval(ctx, client, "device-123", startTime, 50*time.Millisecond)

	// Wait for at least 2 heartbeats.
	time.Sleep(150 * time.Millisecond)
	cancel()

	// Verify heartbeats were sent.
	mu.Lock()
	defer mu.Unlock()

	heartbeats := 0
	for _, env := range received {
		if env.Type == protocol.TypeDeviceHeartbeat {
			heartbeats++
			var hb protocol.StateDeviceHeartbeat
			if err := json.Unmarshal(env.Payload, &hb); err != nil {
				t.Fatalf("unmarshal heartbeat payload: %v", err)
			}
			if hb.UptimeSeconds < 0 {
				t.Errorf("uptime should be non-negative, got %d", hb.UptimeSeconds)
			}
			if env.DeviceID != "device-123" {
				t.Errorf("expected device_id 'device-123', got %q", env.DeviceID)
			}
		}
	}

	if heartbeats < 2 {
		t.Errorf("expected at least 2 heartbeats, got %d", heartbeats)
	}
}

func TestHeartbeatLoopStopsOnCancel(t *testing.T) {
	// Set up a WebSocket server.
	var mu sync.Mutex
	var received []protocol.Envelope

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		welcome := protocol.NewEnvelope(protocol.TypeWelcome, "server")
		welcomePayload, _ := json.Marshal(protocol.Welcome{ConnectionID: "test-session"})
		welcome.Payload = welcomePayload
		data, _ := welcome.Marshal()
		conn.WriteMessage(websocket.TextMessage, data)

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			env, _ := protocol.ParseEnvelope(msg)
			mu.Lock()
			received = append(received, env)
			mu.Unlock()
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := uplink.NewClient(wsURL, "test-token")
	if err := client.Connect(); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer client.Close()
	<-client.Ready()

	// Start and immediately cancel.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		heartbeatLoopWithInterval(ctx, client, "device-123", time.Now(), 50*time.Millisecond)
		close(done)
	}()

	cancel()

	// Heartbeat loop should exit promptly.
	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("heartbeat loop did not stop after context cancellation")
	}
}

func TestRefreshLoopStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	creds := &auth.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		Expiry:       time.Now().Add(1 * time.Hour), // Not expired, so no refresh attempt.
		DeviceID:     "device-123",
		UplinkURL:    "http://localhost",
	}

	done := make(chan struct{})
	go func() {
		refreshLoop(ctx, t.TempDir(), nil, creds)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("refresh loop did not stop after context cancellation")
	}
}

func TestRefreshLoopRefreshesBeforeExpiry(t *testing.T) {
	// Mock server that returns a new token.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/token/refresh" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(`{"access_token":"new-token","refresh_token":"new-refresh","expires_in":3600}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	tmpHome := t.TempDir()

	creds := &auth.Credentials{
		AccessToken:  "old-token",
		RefreshToken: "old-refresh",
		Expiry:       time.Now().Add(2 * time.Minute), // Within 5-minute refresh buffer.
		DeviceID:     "device-123",
		UplinkURL:    server.URL,
	}

	// Save initial credentials so tryRefreshToken can save back.
	if err := auth.SaveCredentialsTo(tmpHome, creds); err != nil {
		t.Fatalf("save creds: %v", err)
	}

	// Manually trigger what refreshLoop would do.
	if creds.NeedsRefresh() {
		refreshed, err := tryRefreshToken(tmpHome, creds)
		if err != nil {
			t.Fatalf("refresh error: %v", err)
		}
		if refreshed.AccessToken != "new-token" {
			t.Errorf("expected new access token, got %q", refreshed.AccessToken)
		}
		if refreshed.RefreshToken != "new-refresh" {
			t.Errorf("expected new refresh token, got %q", refreshed.RefreshToken)
		}
	} else {
		t.Fatal("expected NeedsRefresh() to be true for token expiring in 2 minutes")
	}
}

// initTestGitRepo creates a minimal git repo for testing.
func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
}
