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
	"sync/atomic"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
)

// serveWsURL converts an httptest.Server URL to a WebSocket URL.
// Kept for use by session_test.go and remote_update_test.go.
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

func TestRunServe_WorkspaceDefaultsToCwd(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	setupServeCredentials(t)

	// Empty workspace should default to "." and resolve to an absolute path.
	// Cancel immediately — we only care that workspace validation passes.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunServe(ServeOptions{Ctx: ctx})
	if err != nil && strings.Contains(err.Error(), "does not exist") {
		t.Errorf("empty workspace should default to cwd, got: %v", err)
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

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}
		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})

	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Check the connect body for expected metadata
	connectBody := ms.getConnectBody()
	if connectBody == nil {
		t.Fatal("connect request body was not received")
	}

	if connectBody["chief_version"] != "1.0.0" {
		t.Errorf("expected chief_version '1.0.0', got %v", connectBody["chief_version"])
	}
	if connectBody["device_name"] != "test-device" {
		t.Errorf("expected device_name 'test-device', got %v", connectBody["device_name"])
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

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}
		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace:  workspace,
		DeviceName: "my-custom-device",
		ServerURL:  ms.httpSrv.URL,
		Version:    "1.0.0",
		Ctx:        ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	connectBody := ms.getConnectBody()
	if connectBody == nil {
		t.Fatal("connect request body was not received")
	}

	if connectBody["device_name"] != "my-custom-device" {
		t.Errorf("expected device name 'my-custom-device', got %q", connectBody["device_name"])
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

	ms := newMockUplinkServer(t)
	// Set connect endpoint to return 401
	ms.connectStatus.Store(401)

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
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
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}
		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
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

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		// Wait for connection
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for initial state_snapshot
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
			cancel()
			return
		}

		// Send ping command via Pusher
		pingReq := map[string]string{
			"type":      "ping",
			"id":        "ping-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if err := ms.sendCommand(pingReq); err != nil {
			t.Logf("sendCommand(ping): %v", err)
			cancel()
			return
		}

		// Wait for pong response via HTTP messages
		if _, err := ms.waitForMessageType("pong", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(pong): %v", err)
		}

		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify pong was received
	pong, err := ms.waitForMessageType("pong", time.Second)
	if err != nil {
		t.Error("expected pong response to be received by server")
	} else {
		var msg map[string]interface{}
		json.Unmarshal(pong, &msg)
		if msg["type"] != "pong" {
			t.Errorf("expected type 'pong', got %v", msg["type"])
		}
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

	var tokenRefreshed bool
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())

	// Create a mock uplink server for the uplink endpoints
	ms := newMockUplinkServer(t)

	// Create a mux that combines token refresh with uplink server endpoints.
	// Token refresh goes to BaseURL, uplink goes to ServerURL. We use
	// a separate mux server as the BaseURL for token refresh.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			mu.Lock()
			tokenRefreshed = true
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "new-refreshed-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer tokenSrv.Close()

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}
		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
		BaseURL:   tokenSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !tokenRefreshed {
		t.Error("expected token refresh to be called for near-expiry credentials")
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

// serveTestHelper sets up a serve test with a mock uplink server.
// The serverFn receives the mock uplink server after the CLI has connected
// (HTTP connect + Pusher subscribe) and sent the initial state_snapshot.
// The serverFn should send commands via ms.sendCommand() and read responses
// via ms.waitForMessageType() or ms.getMessages().
func serveTestHelper(t *testing.T, workspacePath string, serverFn func(ms *mockUplinkServer)) error {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	// Wait for the CLI to connect and send state_snapshot, then run test logic.
	go func() {
		// Wait for Pusher subscription (indicates full connection).
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("serveTestHelper: %v", err)
			cancel()
			return
		}

		// Wait for initial state_snapshot to arrive.
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("serveTestHelper: %v", err)
			cancel()
			return
		}

		// Run test-specific server logic.
		serverFn(ms)

		// Cancel context to stop serve loop.
		cancel()
	}()

	return RunServe(ServeOptions{
		Workspace: workspacePath,
		ServerURL: ms.httpSrv.URL,
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

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for state_snapshot
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
		}

		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspaceDir,
		ServerURL: ms.httpSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Retrieve the state_snapshot message
	raw, err := ms.waitForMessageType("state_snapshot", time.Second)
	if err != nil {
		t.Fatal("state_snapshot was not received")
	}

	var snapshotReceived map[string]interface{}
	json.Unmarshal(raw, &snapshotReceived)

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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send list_projects request
		listReq := map[string]string{
			"type":      "list_projects",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		ms.sendCommand(listReq)

		// Read project_list response
		raw, err := ms.waitForMessageType("project_list", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &projectListReceived)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send get_project request
		getReq := map[string]string{
			"type":      "get_project",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
		}
		ms.sendCommand(getReq)

		// Read project_state response
		raw, err := ms.waitForMessageType("project_state", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &projectStateReceived)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send get_project for nonexistent project
		getReq := map[string]string{
			"type":      "get_project",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
		}
		ms.sendCommand(getReq)

		// Read error response
		raw, err := ms.waitForMessageType("error", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &errorReceived)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send get_prd request
		getReq := map[string]string{
			"type":      "get_prd",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		ms.sendCommand(getReq)

		// Read prd_content response
		raw, err := ms.waitForMessageType("prd_content", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &prdContentReceived)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send get_prd for nonexistent PRD
		getReq := map[string]string{
			"type":      "get_prd",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "nonexistent",
		}
		ms.sendCommand(getReq)

		// Read error response
		raw, err := ms.waitForMessageType("error", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &errorReceived)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send get_prd for nonexistent project
		getReq := map[string]string{
			"type":      "get_prd",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
			"prd_id":    "feature",
		}
		ms.sendCommand(getReq)

		// Read error response
		raw, err := ms.waitForMessageType("error", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &errorReceived)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send more than globalBurst (30) messages rapidly to trigger rate limiting
		for i := 0; i < 35; i++ {
			msg := map[string]string{
				"type":      "list_projects",
				"id":        fmt.Sprintf("req-%d", i),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			ms.sendCommand(msg)
		}

		// Wait for a RATE_LIMITED error response
		deadline := time.After(5 * time.Second)
		for {
			msgs := ms.getMessages()
			for _, raw := range msgs {
				var resp map[string]interface{}
				json.Unmarshal(raw, &resp)
				if resp["type"] == "error" && resp["code"] == "RATE_LIMITED" {
					mu.Lock()
					rateLimitReceived = true
					mu.Unlock()
					return
				}
			}
			select {
			case <-deadline:
				return
			case <-time.After(50 * time.Millisecond):
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Exhaust the global rate limit with normal messages
		for i := 0; i < 35; i++ {
			msg := map[string]string{
				"type":      "list_projects",
				"id":        fmt.Sprintf("req-%d", i),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			ms.sendCommand(msg)
		}

		// Now immediately send a ping — should bypass rate limiting
		ping := map[string]string{
			"type":      "ping",
			"id":        "ping-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		ms.sendCommand(ping)

		// Wait for both RATE_LIMITED and pong responses
		deadline := time.After(5 * time.Second)
		for {
			msgs := ms.getMessages()
			for _, raw := range msgs {
				var resp map[string]interface{}
				json.Unmarshal(raw, &resp)
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
			}
			mu.Lock()
			done := pongReceived && rateLimitSeen
			mu.Unlock()
			if done {
				return
			}
			select {
			case <-deadline:
				return
			case <-time.After(50 * time.Millisecond):
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send 3 start_run messages (limit is 2/minute)
		for i := 0; i < 3; i++ {
			msg := map[string]interface{}{
				"type":      "start_run",
				"id":        fmt.Sprintf("req-%d", i),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"project":   "nonexistent",
				"prd_id":    "test",
			}
			ms.sendCommand(msg)
		}

		// Wait for a RATE_LIMITED error response
		deadline := time.After(5 * time.Second)
		for {
			msgs := ms.getMessages()
			for _, raw := range msgs {
				var resp map[string]interface{}
				json.Unmarshal(raw, &resp)
				if resp["type"] == "error" && resp["code"] == "RATE_LIMITED" {
					mu.Lock()
					rateLimitReceived = true
					mu.Unlock()
					return
				}
			}
			select {
			case <-deadline:
				return
			case <-time.After(50 * time.Millisecond):
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

func TestRunServe_ShutdownLogsSequence(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	logFile := filepath.Join(home, "chief-shutdown.log")

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for state_snapshot to ensure connection is fully established
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
		}

		// Cancel to trigger shutdown
		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
		LogFile:   logFile,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify log file contains shutdown sequence
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "Shutting down...") {
		t.Error("log file missing 'Shutting down...' message")
	}
	if !strings.Contains(content, "Goodbye.") {
		t.Error("log file missing 'Goodbye.' message")
	}
}

func TestRunServe_ShutdownMarksInterruptedStories(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a git repo with a PRD
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdPath := filepath.Join(prdDir, "prd.json")
	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Test Story", "passes": false}, {"id": "US-002", "title": "Done Story", "passes": true}]}`
	if err := os.WriteFile(prdPath, []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a mock claude that hangs until killed (simulates in-progress work)
	mockDir := t.TempDir()
	mockScript := `#!/bin/sh
echo '{"type":"system","subtype":"init"}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Working on <ralph-status>US-001</ralph-status>"}]}}'
sleep 300
`
	if err := os.WriteFile(filepath.Join(mockDir, "claude"), []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", mockDir+":"+origPath)

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for state_snapshot
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
			cancel()
			return
		}

		// Send start_run to get a story in-progress
		startReq := map[string]string{
			"type":      "start_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		ms.sendCommand(startReq)

		// Wait for run_progress with story_started so we know US-001 is tracked
		deadline := time.After(10 * time.Second)
		for {
			msgs := ms.getMessages()
			for _, raw := range msgs {
				var msg map[string]interface{}
				json.Unmarshal(raw, &msg)
				if msg["type"] == "run_progress" {
					status, _ := msg["status"].(string)
					if status == "story_started" {
						// Now cancel to trigger shutdown while story is in-progress
						cancel()
						return
					}
				}
			}
			select {
			case <-deadline:
				t.Logf("timeout waiting for story_started")
				cancel()
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
	}()

	err := RunServe(ServeOptions{
		Workspace: workspaceDir,
		ServerURL: ms.httpSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify the PRD was updated with inProgress: true for US-001
	data, err := os.ReadFile(prdPath)
	if err != nil {
		t.Fatalf("failed to read PRD: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse PRD: %v", err)
	}

	stories := result["userStories"].([]interface{})
	story1 := stories[0].(map[string]interface{})
	if story1["inProgress"] != true {
		t.Errorf("expected US-001 to have inProgress=true after shutdown, got %v", story1["inProgress"])
	}

	// US-002 is already passing, should NOT be marked as inProgress
	story2 := stories[1].(map[string]interface{})
	if _, hasInProgress := story2["inProgress"]; hasInProgress && story2["inProgress"] == true {
		t.Error("expected US-002 to NOT have inProgress=true (already passes)")
	}
}

func TestRunServe_ShutdownLogFileFlush(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	logFile := filepath.Join(home, "chief-flush.log")

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for state_snapshot
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
		}

		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
		LogFile:   logFile,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify the log file was flushed and contains the final "Goodbye." message
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Goodbye.") {
		t.Error("log file missing 'Goodbye.' — may not have been flushed properly")
	}
}

func TestSessionManager_SessionCount(t *testing.T) {
	sm := &sessionManager{
		sessions:    make(map[string]*claudeSession),
		stopTimeout: make(chan struct{}),
	}
	// Manually close stopTimeout since we're not starting the timeout checker
	close(sm.stopTimeout)

	if sm.sessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", sm.sessionCount())
	}

	sm.sessions["sess1"] = &claudeSession{sessionID: "sess1"}
	sm.sessions["sess2"] = &claudeSession{sessionID: "sess2"}

	if sm.sessionCount() != 2 {
		t.Errorf("expected 2 sessions, got %d", sm.sessionCount())
	}
}

func TestRunServe_ServerURLFromEnvVar(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}
		cancel()
	}()

	// Set env var to point at our test server (no ServerURL in ServeOptions)
	t.Setenv("CHIEF_SERVER_URL", ms.httpSrv.URL)

	err := RunServe(ServeOptions{
		Workspace: workspace,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}
}

func TestRunServe_ServerURLPrecedence_FlagOverridesEnv(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}
		cancel()
	}()

	// Set env var to a bad URL — flag should override it
	t.Setenv("CHIEF_SERVER_URL", "http://bad-url-that-should-not-be-used:9999")

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL, // Flag value — should take precedence
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}
}

func TestRunServe_ServerURLLoggedOnStartup(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	logFile := filepath.Join(home, "serve.log")

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}
		cancel()
	}()

	serverURL := ms.httpSrv.URL
	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: serverURL,
		LogFile:   logFile,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Connecting to "+serverURL) {
		t.Errorf("expected log to contain 'Connecting to %s', got: %s", serverURL, content)
	}
}

// --- serveShutdown unit tests ---

// mockCloser implements uplinkCloser for testing serveShutdown directly.
type mockCloser struct {
	closeCalled  atomic.Int32
	closeDelay   time.Duration // Simulates a slow Close operation.
	closeErr     error
}

func (m *mockCloser) Close() error {
	m.closeCalled.Add(1)
	if m.closeDelay > 0 {
		time.Sleep(m.closeDelay)
	}
	return m.closeErr
}

func (m *mockCloser) CloseWithTimeout(timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- m.Close()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return nil
	}
}

func TestServeShutdown_CompletesWithinTimeout(t *testing.T) {
	closer := &mockCloser{}

	start := time.Now()
	err := serveShutdown(closer, nil, nil, nil, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("serveShutdown returned error: %v", err)
	}

	// Should complete quickly.
	if elapsed > 2*time.Second {
		t.Errorf("serveShutdown took %s, expected < 2s", elapsed)
	}

	// Close should have been called.
	if got := closer.closeCalled.Load(); got != 1 {
		t.Errorf("close called %d times, want 1", got)
	}
}

func TestServeShutdown_TimesOutWithHangingClose(t *testing.T) {
	// Create a closer that hangs longer than the shutdown timeout.
	closer := &mockCloser{closeDelay: 30 * time.Second}

	start := time.Now()
	err := serveShutdown(closer, nil, nil, nil, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("serveShutdown returned error: %v", err)
	}

	// Should complete within the shutdown timeout (10s) + small buffer,
	// not hang for the full 30s close delay.
	if elapsed > 15*time.Second {
		t.Errorf("serveShutdown took %s, expected < 15s (shutdown timeout is 10s)", elapsed)
	}

	t.Logf("serveShutdown completed in %s", elapsed.Round(time.Millisecond))
}

func TestServeShutdown_DisconnectFailureLoggedNotBlocking(t *testing.T) {
	closer := &mockCloser{closeErr: fmt.Errorf("connection refused")}

	err := serveShutdown(closer, nil, nil, nil, nil)

	// serveShutdown should not propagate the close error.
	if err != nil {
		t.Errorf("serveShutdown returned error: %v, want nil", err)
	}

	// Close should have been called.
	if got := closer.closeCalled.Load(); got != 1 {
		t.Errorf("close called %d times, want 1", got)
	}
}

func TestServeShutdown_CallsDisconnectOnServer(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspace := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		// Wait for state_snapshot to ensure connection is fully established.
		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
		}

		// Cancel to trigger shutdown.
		cancel()
	}()

	err := RunServe(ServeOptions{
		Workspace: workspace,
		ServerURL: ms.httpSrv.URL,
		Version:   "1.0.0",
		Ctx:       ctx,
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify disconnect was called during shutdown.
	if got := ms.disconnectCalls.Load(); got != 1 {
		t.Errorf("disconnect calls = %d, want 1", got)
	}
}
