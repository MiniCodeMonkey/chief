package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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

		// Read state_snapshot (sent after handshake)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()

		// Send ping
		conn.SetReadDeadline(time.Time{})
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

// createGitRepo creates a minimal git repository for testing.
func createGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Initialize git repo
	cmd := exec.Command("git", "init", dir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	// Configure user for the repo
	for _, args := range [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config failed: %v\n%s", err, out)
		}
	}
	// Create initial commit
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}
}

// serveTestHelper sets up a serve test with a WebSocket mock server.
// The serverFn receives the conn after hello/welcome handshake is done
// and the state_snapshot has been read. The test server reads hello,
// sends welcome, reads state_snapshot, then calls serverFn.
func serveTestHelper(t *testing.T, workspacePath string, serverFn func(conn *websocket.Conn)) error {
	t.Helper()

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

		// Read state_snapshot (always sent after handshake)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()
		conn.SetReadDeadline(time.Time{})

		// Run test-specific server logic
		serverFn(conn)

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

	return RunServe(ServeOptions{
		Workspace: workspacePath,
		WSURL:     serveWsURL(srv),
		Version:   "1.0.0",
		Ctx:       ctx,
	})
}

func TestRunServe_StateSnapshotOnConnect(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a git repo in the workspace
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	var snapshotReceived map[string]interface{}
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

		// Read state_snapshot
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &snapshotReceived)
			mu.Unlock()
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
		Workspace: workspaceDir,
		WSURL:     serveWsURL(srv),
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if snapshotReceived == nil {
		t.Fatal("state_snapshot was not received")
	}
	if snapshotReceived["type"] != "state_snapshot" {
		t.Errorf("expected type 'state_snapshot', got %v", snapshotReceived["type"])
	}

	// Verify projects are included
	projects, ok := snapshotReceived["projects"].([]interface{})
	if !ok {
		t.Fatal("expected projects array in state_snapshot")
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	proj := projects[0].(map[string]interface{})
	if proj["name"] != "myproject" {
		t.Errorf("expected project name 'myproject', got %v", proj["name"])
	}

	// Verify runs and sessions are empty arrays
	runs, ok := snapshotReceived["runs"].([]interface{})
	if !ok {
		t.Fatal("expected runs array in state_snapshot")
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
	sessions, ok := snapshotReceived["sessions"].([]interface{})
	if !ok {
		t.Fatal("expected sessions array in state_snapshot")
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestRunServe_ListProjects(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create two git repos
	createGitRepo(t, filepath.Join(workspaceDir, "alpha"))
	createGitRepo(t, filepath.Join(workspaceDir, "beta"))

	var projectListReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send list_projects request
		listReq := map[string]string{
			"type":      "list_projects",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(listReq)

		// Read project_list response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &projectListReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if projectListReceived == nil {
		t.Fatal("project_list was not received")
	}
	if projectListReceived["type"] != "project_list" {
		t.Errorf("expected type 'project_list', got %v", projectListReceived["type"])
	}

	projects, ok := projectListReceived["projects"].([]interface{})
	if !ok {
		t.Fatal("expected projects array")
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	// Collect project names
	names := make(map[string]bool)
	for _, p := range projects {
		proj := p.(map[string]interface{})
		names[proj["name"].(string)] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("expected projects alpha and beta, got %v", names)
	}
}

func TestRunServe_GetProject(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	var projectStateReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send get_project request
		getReq := map[string]string{
			"type":      "get_project",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
		}
		conn.WriteJSON(getReq)

		// Read project_state response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &projectStateReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if projectStateReceived == nil {
		t.Fatal("project_state was not received")
	}
	if projectStateReceived["type"] != "project_state" {
		t.Errorf("expected type 'project_state', got %v", projectStateReceived["type"])
	}

	project, ok := projectStateReceived["project"].(map[string]interface{})
	if !ok {
		t.Fatal("expected project object in project_state")
	}
	if project["name"] != "myproject" {
		t.Errorf("expected project name 'myproject', got %v", project["name"])
	}
}

func TestRunServe_GetProjectNotFound(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send get_project for nonexistent project
		getReq := map[string]string{
			"type":      "get_project",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
		}
		conn.WriteJSON(getReq)

		// Read error response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &errorReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if errorReceived == nil {
		t.Fatal("error message was not received")
	}
	if errorReceived["type"] != "error" {
		t.Errorf("expected type 'error', got %v", errorReceived["type"])
	}
	if errorReceived["code"] != "PROJECT_NOT_FOUND" {
		t.Errorf("expected code 'PROJECT_NOT_FOUND', got %v", errorReceived["code"])
	}
	if errorReceived["request_id"] != "req-1" {
		t.Errorf("expected request_id 'req-1', got %v", errorReceived["request_id"])
	}
}

func TestRunServe_GetPRD(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a git repo with .chief/prds/feature/
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write prd.md
	prdMD := "# My Feature\nThis is a feature PRD."
	if err := os.WriteFile(filepath.Join(prdDir, "prd.md"), []byte(prdMD), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write prd.json
	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "passes": true}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	var prdContentReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send get_prd request
		getReq := map[string]string{
			"type":      "get_prd",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		conn.WriteJSON(getReq)

		// Read prd_content response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &prdContentReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if prdContentReceived == nil {
		t.Fatal("prd_content was not received")
	}
	if prdContentReceived["type"] != "prd_content" {
		t.Errorf("expected type 'prd_content', got %v", prdContentReceived["type"])
	}
	if prdContentReceived["project"] != "myproject" {
		t.Errorf("expected project 'myproject', got %v", prdContentReceived["project"])
	}
	if prdContentReceived["prd_id"] != "feature" {
		t.Errorf("expected prd_id 'feature', got %v", prdContentReceived["prd_id"])
	}
	if prdContentReceived["content"] != prdMD {
		t.Errorf("expected content %q, got %v", prdMD, prdContentReceived["content"])
	}

	// Verify state is present and contains expected data
	state, ok := prdContentReceived["state"].(map[string]interface{})
	if !ok {
		t.Fatal("expected state object in prd_content")
	}
	if state["project"] != "My Feature" {
		t.Errorf("expected state.project 'My Feature', got %v", state["project"])
	}
}

func TestRunServe_GetPRDNotFound(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a git repo without any PRDs
	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send get_prd for nonexistent PRD
		getReq := map[string]string{
			"type":      "get_prd",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "nonexistent",
		}
		conn.WriteJSON(getReq)

		// Read error response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &errorReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if errorReceived == nil {
		t.Fatal("error message was not received")
	}
	if errorReceived["type"] != "error" {
		t.Errorf("expected type 'error', got %v", errorReceived["type"])
	}
	if errorReceived["code"] != "PRD_NOT_FOUND" {
		t.Errorf("expected code 'PRD_NOT_FOUND', got %v", errorReceived["code"])
	}
}

func TestRunServe_GetPRDProjectNotFound(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send get_prd for nonexistent project
		getReq := map[string]string{
			"type":      "get_prd",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
			"prd_id":    "feature",
		}
		conn.WriteJSON(getReq)

		// Read error response
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &errorReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if errorReceived == nil {
		t.Fatal("error message was not received")
	}
	if errorReceived["type"] != "error" {
		t.Errorf("expected type 'error', got %v", errorReceived["type"])
	}
	if errorReceived["code"] != "PROJECT_NOT_FOUND" {
		t.Errorf("expected code 'PROJECT_NOT_FOUND', got %v", errorReceived["code"])
	}
}

func TestRunServe_RateLimitGlobal(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var rateLimitReceived bool
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send more than globalBurst (30) messages rapidly to trigger rate limiting
		for i := 0; i < 35; i++ {
			msg := map[string]string{
				"type":      "list_projects",
				"id":        fmt.Sprintf("req-%d", i),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			conn.WriteJSON(msg)
		}

		// Read all responses — at least one should be a RATE_LIMITED error
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var resp map[string]interface{}
			json.Unmarshal(data, &resp)
			if resp["type"] == "error" && resp["code"] == "RATE_LIMITED" {
				mu.Lock()
				rateLimitReceived = true
				mu.Unlock()
				break
			}
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !rateLimitReceived {
		t.Error("expected RATE_LIMITED error after burst exhaustion")
	}
}

func TestRunServe_RateLimitPingExempt(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var pongReceived bool
	var rateLimitSeen bool
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Exhaust the global rate limit with normal messages, interleaved with reading responses
		for i := 0; i < 35; i++ {
			msg := map[string]string{
				"type":      "list_projects",
				"id":        fmt.Sprintf("req-%d", i),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			conn.WriteJSON(msg)
		}

		// Now immediately send a ping — should bypass rate limiting
		ping := map[string]string{
			"type":      "ping",
			"id":        "ping-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(ping)

		// Read all responses — look for both RATE_LIMITED and pong
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var resp map[string]interface{}
			json.Unmarshal(data, &resp)
			if resp["type"] == "pong" {
				mu.Lock()
				pongReceived = true
				mu.Unlock()
			}
			if resp["type"] == "error" && resp["code"] == "RATE_LIMITED" {
				mu.Lock()
				rateLimitSeen = true
				mu.Unlock()
			}
			// Stop once we've seen both
			mu.Lock()
			done := pongReceived && rateLimitSeen
			mu.Unlock()
			if done {
				break
			}
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !rateLimitSeen {
		t.Error("expected RATE_LIMITED error to confirm rate limiting was active")
	}
	if !pongReceived {
		t.Error("expected pong response even after rate limit exhaustion — ping should be exempt")
	}
}

func TestRunServe_RateLimitExpensiveOps(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var rateLimitReceived bool
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send 3 start_run messages (limit is 2/minute)
		for i := 0; i < 3; i++ {
			msg := map[string]interface{}{
				"type":      "start_run",
				"id":        fmt.Sprintf("req-%d", i),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"project":   "nonexistent",
				"prd_id":    "test",
			}
			conn.WriteJSON(msg)
		}

		// Read responses — the third should be RATE_LIMITED
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var resp map[string]interface{}
			json.Unmarshal(data, &resp)
			if resp["type"] == "error" && resp["code"] == "RATE_LIMITED" {
				mu.Lock()
				rateLimitReceived = true
				mu.Unlock()
				break
			}
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !rateLimitReceived {
		t.Error("expected RATE_LIMITED error for excessive expensive operations")
	}
}
