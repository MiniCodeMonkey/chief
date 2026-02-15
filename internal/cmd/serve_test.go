package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/auth"
)

// serveWsURL converts an httptest.Server URL to a WebSocket URL.
func serveWsURL(s *httptest.Server) string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

func setupServeCredentials(t *testing.T) {
	t.Helper()
	creds := &auth.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		DeviceName:   "test-device",
		User:         "user@example.com",
	}
	if err := auth.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}
}

func TestRunServe_WorkspaceDoesNotExist(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	setupServeCredentials(t)

	err := RunServe(ServeOptions{
		Workspace: "/nonexistent/path",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent workspace")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestRunServe_WorkspaceIsFile(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	setupServeCredentials(t)

	// Create a file instead of directory
	filePath := filepath.Join(home, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunServe(ServeOptions{
		Workspace: filePath,
	})
	if err == nil {
		t.Fatal("expected error for file workspace")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected 'not a directory' error, got: %v", err)
	}
}

func TestRunServe_NotLoggedIn(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	err := RunServe(ServeOptions{
		Workspace: workspace,
	})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
	if !strings.Contains(err.Error(), "Not logged in") {
		t.Errorf("expected 'Not logged in' error, got: %v", err)
	}
}

func TestRunServe_ConnectsAndHandshakes(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	var helloReceived map[string]interface{}
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())

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

		mu.Lock()
		json.Unmarshal(data, &helloReceived)
		mu.Unlock()

		welcome := map[string]string{
			"type":      "welcome",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(welcome)

		// Cancel context to stop serve loop
		cancel()

		// Keep alive until client disconnects
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		WSURL:     serveWsURL(srv),
		Version:   "1.0.0",
		Ctx:       ctx,
	})

	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if helloReceived == nil {
		t.Fatal("hello message was not received")
	}

	if helloReceived["type"] != "hello" {
		t.Errorf("expected type 'hello', got %v", helloReceived["type"])
	}
	if helloReceived["access_token"] != "test-token" {
		t.Errorf("expected access_token 'test-token', got %v", helloReceived["access_token"])
	}
	if helloReceived["chief_version"] != "1.0.0" {
		t.Errorf("expected chief_version '1.0.0', got %v", helloReceived["chief_version"])
	}
	if helloReceived["device_name"] != "test-device" {
		t.Errorf("expected device_name 'test-device', got %v", helloReceived["device_name"])
	}
}

func TestRunServe_DeviceNameOverride(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	var receivedDeviceName string
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())

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

		var hello map[string]interface{}
		json.Unmarshal(data, &hello)
		mu.Lock()
		if dn, ok := hello["device_name"].(string); ok {
			receivedDeviceName = dn
		}
		mu.Unlock()

		welcome := map[string]string{
			"type":      "welcome",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(welcome)

		cancel()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	err := RunServe(ServeOptions{
		Workspace:  workspace,
		DeviceName: "my-custom-device",
		WSURL:      serveWsURL(srv),
		Version:    "1.0.0",
		Ctx:        ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedDeviceName != "my-custom-device" {
		t.Errorf("expected device name 'my-custom-device', got %q", receivedDeviceName)
	}
}

func TestRunServe_IncompatibleVersion(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	upgrader := websocket.Upgrader{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read hello
		conn.ReadMessage()

		// Respond with incompatible
		incompatible := map[string]string{
			"type":      "incompatible",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"message":   "Please update to v2.0.0",
		}
		conn.WriteJSON(incompatible)

		// Close the server listener so reconnection fails
		srv.CloseClientConnections()

		// Keep connection alive briefly for the client to read the message
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		WSURL:     serveWsURL(srv),
		Version:   "1.0.0",
	})
	if err == nil {
		t.Fatal("expected error for incompatible version")
	}
	if !strings.Contains(err.Error(), "incompatible version") {
		t.Errorf("expected 'incompatible version' error, got: %v", err)
	}
}

func TestRunServe_AuthFailed(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	upgrader := websocket.Upgrader{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read hello
		conn.ReadMessage()

		// Respond with auth_failed
		authFailed := map[string]string{
			"type":      "auth_failed",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(authFailed)

		// Close server listener so reconnection fails
		srv.CloseClientConnections()

		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		WSURL:     serveWsURL(srv),
		Version:   "1.0.0",
	})
	if err == nil {
		t.Fatal("expected error for auth failure")
	}
	if !strings.Contains(err.Error(), "deauthorized") {
		t.Errorf("expected 'deauthorized' error, got: %v", err)
	}
}

func TestRunServe_LogFile(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	logFile := filepath.Join(home, "chief.log")

	ctx, cancel := context.WithCancel(context.Background())

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.ReadMessage()

		welcome := map[string]string{
			"type":      "welcome",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(welcome)

		cancel()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		WSURL:     serveWsURL(srv),
		LogFile:   logFile,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify log file was created and has content
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Error("log file is empty")
	}
	content := string(data)
	if !strings.Contains(content, "Starting chief serve") {
		t.Errorf("log file missing startup message, got: %s", content)
	}
}

func TestRunServe_PingPong(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	var pongReceived bool
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read hello
		conn.ReadMessage()

		// Send welcome
		welcome := map[string]string{
			"type":      "welcome",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(welcome)

		// Send ping
		ping := map[string]string{
			"type":      "ping",
			"id":        "ping-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(ping)

		// Wait for pong response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			var msg map[string]interface{}
			if json.Unmarshal(data, &msg) == nil && msg["type"] == "pong" {
				mu.Lock()
				pongReceived = true
				mu.Unlock()
			}
		}

		cancel()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		WSURL:     serveWsURL(srv),
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !pongReceived {
		t.Error("expected pong response to be received by server")
	}
}

func TestRunServe_TokenRefresh(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Save credentials that are near expiry
	creds := &auth.Credentials{
		AccessToken:  "old-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(2 * time.Minute), // Within 5 min threshold
		DeviceName:   "test-device",
		User:         "user@example.com",
	}
	if err := auth.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	var receivedToken string
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())

	// Mock both HTTP (for token refresh) and WebSocket (for serve)
	mux := http.NewServeMux()
	upgrader := websocket.Upgrader{}

	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-refreshed-token",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
		})
	})

	mux.HandleFunc("/ws/server", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var hello map[string]interface{}
		json.Unmarshal(data, &hello)
		mu.Lock()
		if tok, ok := hello["access_token"].(string); ok {
			receivedToken = tok
		}
		mu.Unlock()

		welcome := map[string]string{
			"type":      "welcome",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(welcome)

		cancel()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		WSURL:     serveWsURL(srv) + "/ws/server",
		BaseURL:   srv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedToken != "new-refreshed-token" {
		t.Errorf("expected refreshed token 'new-refreshed-token', got %q", receivedToken)
	}
}
