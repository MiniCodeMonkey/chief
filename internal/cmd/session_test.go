package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/ws"
)

// mockProjectFinder implements projectFinder for tests.
type mockProjectFinder struct {
	projects map[string]ws.ProjectSummary
}

func (m *mockProjectFinder) FindProject(name string) (ws.ProjectSummary, bool) {
	p, ok := m.projects[name]
	return p, ok
}

func TestSessionManager_NewPRD(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	var messages []map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send new_prd request
		newPRDReq := map[string]string{
			"type":            "new_prd",
			"id":              "req-1",
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"project":         "myproject",
			"session_id":      "sess-123",
			"initial_message": "Build a todo app",
		}
		conn.WriteJSON(newPRDReq)

		// We should receive claude_output messages.
		// Since we can't actually run claude in tests, expect an error response
		// (claude binary not available in test) — this tests the error path.
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg map[string]interface{}
			if json.Unmarshal(data, &msg) == nil {
				mu.Lock()
				messages = append(messages, msg)
				mu.Unlock()
				// If we get an error or claude_output done, stop
				if msg["type"] == "error" || (msg["type"] == "claude_output" && msg["done"] == true) {
					break
				}
			}
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Since claude binary isn't available in test, we expect either:
	// 1. An error message about failing to start Claude, OR
	// 2. A claude_output done=true message if the process started but exited immediately
	if len(messages) == 0 {
		t.Fatal("expected at least one response message")
	}

	// Check that we got some kind of response
	lastMsg := messages[len(messages)-1]
	msgType := lastMsg["type"].(string)
	if msgType != "error" && msgType != "claude_output" {
		t.Errorf("expected error or claude_output message, got %s", msgType)
	}
}

func TestSessionManager_NewPRD_ProjectNotFound(t *testing.T) {
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
		// Send new_prd for nonexistent project
		newPRDReq := map[string]string{
			"type":            "new_prd",
			"id":              "req-1",
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"project":         "nonexistent",
			"session_id":      "sess-123",
			"initial_message": "Build a todo app",
		}
		conn.WriteJSON(newPRDReq)

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

func TestSessionManager_PRDMessage_SessionNotFound(t *testing.T) {
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
		// Send prd_message for nonexistent session
		prdMsg := map[string]string{
			"type":       "prd_message",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "nonexistent-session",
			"content":    "hello",
		}
		conn.WriteJSON(prdMsg)

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
	if errorReceived["code"] != "SESSION_NOT_FOUND" {
		t.Errorf("expected code 'SESSION_NOT_FOUND', got %v", errorReceived["code"])
	}
}

func TestSessionManager_ClosePRDSession_SessionNotFound(t *testing.T) {
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
		// Send close_prd_session for nonexistent session
		closeMsg := map[string]interface{}{
			"type":       "close_prd_session",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "nonexistent-session",
			"save":       false,
		}
		conn.WriteJSON(closeMsg)

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
	if errorReceived["code"] != "SESSION_NOT_FOUND" {
		t.Errorf("expected code 'SESSION_NOT_FOUND', got %v", errorReceived["code"])
	}
}

// TestSessionManager_WithMockClaude uses a shell script to simulate Claude,
// testing the full session lifecycle: spawn, stream output, send message, close.
func TestSessionManager_WithMockClaude(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	// Create a mock "claude" script that echoes input
	mockClaudeBin := filepath.Join(home, "claude")
	mockScript := `#!/bin/sh
echo "Claude PRD session started"
echo "Processing: $1"
# Read from stdin and echo back
while IFS= read -r line; do
    echo "Received: $line"
done
echo "Session complete"
`
	if err := os.WriteFile(mockClaudeBin, []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}

	// Add mock claude to PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", home+":"+origPath)

	var messages []map[string]interface{}
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Handshake
		conn.ReadMessage()
		welcome := map[string]string{
			"type":      "welcome",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(welcome)

		// Read state_snapshot
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()
		conn.SetReadDeadline(time.Time{})

		// Send new_prd request
		newPRDReq := map[string]string{
			"type":            "new_prd",
			"id":              "req-1",
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"project":         "myproject",
			"session_id":      "sess-mock-1",
			"initial_message": "Build a todo app",
		}
		conn.WriteJSON(newPRDReq)

		// Wait a bit for process to start and produce output
		time.Sleep(500 * time.Millisecond)

		// Send a prd_message
		prdMsg := map[string]string{
			"type":       "prd_message",
			"id":         "req-2",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "sess-mock-1",
			"content":    "Add user authentication",
		}
		conn.WriteJSON(prdMsg)

		// Wait for output
		time.Sleep(500 * time.Millisecond)

		// Close the session (save=false, kill immediately)
		closeMsg := map[string]interface{}{
			"type":       "close_prd_session",
			"id":         "req-3",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "sess-mock-1",
			"save":       false,
		}
		conn.WriteJSON(closeMsg)

		// Collect all claude_output messages
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg map[string]interface{}
			if json.Unmarshal(data, &msg) == nil {
				mu.Lock()
				messages = append(messages, msg)
				mu.Unlock()
			}
		}

		cancel()
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

	// Verify we received claude_output messages
	var claudeOutputs []map[string]interface{}
	for _, msg := range messages {
		if msg["type"] == "claude_output" {
			claudeOutputs = append(claudeOutputs, msg)
		}
	}

	if len(claudeOutputs) == 0 {
		t.Fatal("expected at least one claude_output message")
	}

	// Verify session_id is set on all claude_output messages
	for _, co := range claudeOutputs {
		if co["session_id"] != "sess-mock-1" {
			t.Errorf("expected session_id 'sess-mock-1', got %v", co["session_id"])
		}
		if co["project"] != "myproject" {
			t.Errorf("expected project 'myproject', got %v", co["project"])
		}
	}

	// Verify we got a done=true message
	lastOutput := claudeOutputs[len(claudeOutputs)-1]
	if lastOutput["done"] != true {
		t.Error("expected last claude_output to have done=true")
	}

	// Verify we received some actual content
	hasContent := false
	for _, co := range claudeOutputs {
		if data, ok := co["data"].(string); ok && strings.TrimSpace(data) != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("expected at least one claude_output with non-empty data")
	}
}

// TestSessionManager_WithMockClaude_SaveClose tests save=true close behavior.
func TestSessionManager_WithMockClaude_SaveClose(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	// Create a mock "claude" script that exits on EOF
	mockClaudeBin := filepath.Join(home, "claude")
	mockScript := `#!/bin/sh
echo "Session started"
# Read until EOF (stdin closed)
while IFS= read -r line; do
    echo "Got: $line"
done
echo "Saving PRD..."
exit 0
`
	if err := os.WriteFile(mockClaudeBin, []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", home+":"+origPath)

	var messages []map[string]interface{}
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Handshake
		conn.ReadMessage()
		welcome := map[string]string{
			"type":      "welcome",
			"id":        "test-id",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		conn.WriteJSON(welcome)

		// Read state_snapshot
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()
		conn.SetReadDeadline(time.Time{})

		// Send new_prd
		newPRDReq := map[string]string{
			"type":            "new_prd",
			"id":              "req-1",
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"project":         "myproject",
			"session_id":      "sess-save-1",
			"initial_message": "Build an API",
		}
		conn.WriteJSON(newPRDReq)

		time.Sleep(500 * time.Millisecond)

		// Close with save=true (waits for Claude to finish)
		closeMsg := map[string]interface{}{
			"type":       "close_prd_session",
			"id":         "req-2",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "sess-save-1",
			"save":       true,
		}
		conn.WriteJSON(closeMsg)

		// Collect messages
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg map[string]interface{}
			if json.Unmarshal(data, &msg) == nil {
				mu.Lock()
				messages = append(messages, msg)
				mu.Unlock()
			}
		}

		cancel()
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

	// Verify we received a done=true message
	hasDone := false
	for _, msg := range messages {
		if msg["type"] == "claude_output" && msg["done"] == true {
			hasDone = true
			break
		}
	}
	if !hasDone {
		t.Error("expected a claude_output message with done=true after save close")
	}
}

func TestSessionManager_ActiveSessions(t *testing.T) {
	// Create a mock ws client for session manager
	home := t.TempDir()
	setTestHome(t, home)

	// Create a mock "claude" script that stays alive
	mockClaudeBin := filepath.Join(home, "claude")
	mockScript := `#!/bin/sh
while IFS= read -r line; do
    echo "$line"
done
`
	if err := os.WriteFile(mockClaudeBin, []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", home+":"+origPath)

	// Create a simple WebSocket server for the client
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := ws.New(serveWsURL(srv))
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		client.Close()
	}()

	sm := newSessionManager(client)

	// Initially no active sessions
	sessions := sm.activeSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 active sessions, got %d", len(sessions))
	}

	// Create a project dir for the session
	projectDir := filepath.Join(home, "testproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Start a session
	err := sm.newPRD(projectDir, "testproject", "sess-1", "test message")
	if err != nil {
		t.Fatalf("newPRD failed: %v", err)
	}

	// Now should have 1 active session
	sessions = sm.activeSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(sessions))
	}
	if sessions[0].SessionID != "sess-1" {
		t.Errorf("expected session_id 'sess-1', got %q", sessions[0].SessionID)
	}
	if sessions[0].Project != "testproject" {
		t.Errorf("expected project 'testproject', got %q", sessions[0].Project)
	}

	// Kill all sessions
	sm.killAll()

	// Now should have 0 active sessions
	sessions = sm.activeSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 active sessions after killAll, got %d", len(sessions))
	}
}

func TestSessionManager_SendMessage(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Create a mock "claude" script that echoes input
	mockClaudeBin := filepath.Join(home, "claude")
	mockScript := `#!/bin/sh
echo "ready"
while IFS= read -r line; do
    echo "echo: $line"
done
`
	if err := os.WriteFile(mockClaudeBin, []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", home+":"+origPath)

	upgrader := websocket.Upgrader{}
	var receivedMessages []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg map[string]interface{}
			if json.Unmarshal(data, &msg) == nil {
				if msg["type"] == "claude_output" {
					if d, ok := msg["data"].(string); ok {
						mu.Lock()
						receivedMessages = append(receivedMessages, d)
						mu.Unlock()
					}
				}
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := ws.New(serveWsURL(srv))
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		client.Close()
	}()

	sm := newSessionManager(client)

	projectDir := filepath.Join(home, "testproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := sm.newPRD(projectDir, "testproject", "sess-msg-1", "test")
	if err != nil {
		t.Fatalf("newPRD failed: %v", err)
	}

	// Wait for process to start
	time.Sleep(300 * time.Millisecond)

	// Send a message
	if err := sm.sendMessage("sess-msg-1", "hello world"); err != nil {
		t.Fatalf("sendMessage failed: %v", err)
	}

	// Wait for echo
	time.Sleep(500 * time.Millisecond)

	// Verify error on nonexistent session
	if err := sm.sendMessage("nonexistent", "test"); err == nil {
		t.Error("expected error for nonexistent session")
	}

	// Check that we received the echoed message
	mu.Lock()
	defer mu.Unlock()

	hasEcho := false
	for _, msg := range receivedMessages {
		if strings.Contains(msg, "echo: hello world") {
			hasEcho = true
			break
		}
	}
	if !hasEcho {
		t.Errorf("expected echoed message 'echo: hello world' in received messages: %v", receivedMessages)
	}

	// Clean up
	sm.killAll()
}

func TestSessionManager_CloseSession_Errors(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := ws.New(serveWsURL(srv))
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		client.Close()
	}()

	sm := newSessionManager(client)

	// Close nonexistent session
	err := sm.closeSession("nonexistent", false)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "session not found") {
		t.Errorf("expected 'session not found' error, got: %v", err)
	}
}

func TestSessionManager_DuplicateSession(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Create a mock "claude" script
	mockClaudeBin := filepath.Join(home, "claude")
	mockScript := fmt.Sprintf("#!/bin/sh\nwhile IFS= read -r line; do echo \"$line\"; done")
	if err := os.WriteFile(mockClaudeBin, []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", home+":"+origPath)

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := ws.New(serveWsURL(srv))
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		client.Close()
	}()

	sm := newSessionManager(client)

	projectDir := filepath.Join(home, "testproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Start first session
	err := sm.newPRD(projectDir, "testproject", "sess-dup", "test")
	if err != nil {
		t.Fatalf("first newPRD failed: %v", err)
	}

	// Try to start duplicate session
	err = sm.newPRD(projectDir, "testproject", "sess-dup", "test")
	if err == nil {
		t.Error("expected error for duplicate session_id")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	sm.killAll()
}
