package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/ws"
)

// captureSender is a mock messageSender that captures sent messages.
type captureSender struct {
	mu       sync.Mutex
	messages []map[string]interface{}
}

func (c *captureSender) Send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	c.mu.Lock()
	c.messages = append(c.messages, m)
	c.mu.Unlock()
	return nil
}

func (c *captureSender) getMessages() []map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]map[string]interface{}, len(c.messages))
	copy(cp, c.messages)
	return cp
}

// discardSender is a mock messageSender that discards all messages.
type discardSender struct{}

func (d *discardSender) Send(msg interface{}) error { return nil }

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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send new_prd request
		newPRDReq := map[string]string{
			"type":       "new_prd",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"project":    "myproject",
			"session_id": "sess-123",
			"message":    "Build a todo app",
		}
		ms.sendCommand(newPRDReq)

		// We should receive prd_output messages.
		// Since we can't actually run claude in tests, expect an error response
		// (claude binary not available in test) — this tests the error path.
		// Wait for first message with 3 second timeout.
		raw, err := ms.waitForMessageType("error", 3*time.Second)
		if err != nil {
			// If not an error, might be prd_output
			raw, err = ms.waitForMessageType("prd_output", 3*time.Second)
			if err != nil {
				t.Fatal("expected error or prd_output message")
			}
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Check that we got some kind of response
		msgType := msg["type"].(string)
		if msgType != "error" && msgType != "prd_output" {
			t.Errorf("expected error or prd_output message, got %s", msgType)
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send new_prd for nonexistent project
		newPRDReq := map[string]string{
			"type":       "new_prd",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"project":    "nonexistent",
			"session_id": "sess-123",
			"message":    "Build a todo app",
		}
		ms.sendCommand(newPRDReq)

		// Read error response
		raw, err := ms.waitForMessageType("error", 2*time.Second)
		if err != nil {
			t.Fatalf("expected error message: %v", err)
		}

		var errorReceived map[string]interface{}
		if err := json.Unmarshal(raw, &errorReceived); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}

		if errorReceived["type"] != "error" {
			t.Errorf("expected type 'error', got %v", errorReceived["type"])
		}
		if errorReceived["code"] != "PROJECT_NOT_FOUND" {
			t.Errorf("expected code 'PROJECT_NOT_FOUND', got %v", errorReceived["code"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send prd_message for nonexistent session
		prdMsg := map[string]string{
			"type":       "prd_message",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "nonexistent-session",
			"message":    "hello",
		}
		ms.sendCommand(prdMsg)

		// Read error response
		raw, err := ms.waitForMessageType("error", 2*time.Second)
		if err != nil {
			t.Fatalf("expected error message: %v", err)
		}

		var errorReceived map[string]interface{}
		if err := json.Unmarshal(raw, &errorReceived); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}

		if errorReceived["type"] != "error" {
			t.Errorf("expected type 'error', got %v", errorReceived["type"])
		}
		if errorReceived["code"] != "SESSION_NOT_FOUND" {
			t.Errorf("expected code 'SESSION_NOT_FOUND', got %v", errorReceived["code"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send close_prd_session for nonexistent session
		closeMsg := map[string]interface{}{
			"type":       "close_prd_session",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "nonexistent-session",
			"save":       false,
		}
		ms.sendCommand(closeMsg)

		// Read error response
		raw, err := ms.waitForMessageType("error", 2*time.Second)
		if err != nil {
			t.Fatalf("expected error message: %v", err)
		}

		var errorReceived map[string]interface{}
		if err := json.Unmarshal(raw, &errorReceived); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}

		if errorReceived["type"] != "error" {
			t.Errorf("expected type 'error', got %v", errorReceived["type"])
		}
		if errorReceived["code"] != "SESSION_NOT_FOUND" {
			t.Errorf("expected code 'SESSION_NOT_FOUND', got %v", errorReceived["code"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
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

		// Send new_prd request via Pusher
		newPRDReq := map[string]string{
			"type":       "new_prd",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"project":    "myproject",
			"session_id": "sess-mock-1",
			"message":    "Build a todo app",
		}
		ms.sendCommand(newPRDReq)

		// Wait a bit for process to start and produce output
		time.Sleep(500 * time.Millisecond)

		// Send a prd_message
		prdMsg := map[string]string{
			"type":       "prd_message",
			"id":         "req-2",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "sess-mock-1",
			"message":    "Add user authentication",
		}
		ms.sendCommand(prdMsg)

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
		ms.sendCommand(closeMsg)

		// Wait for a prd_response_complete message
		deadline := time.After(5 * time.Second)
		for {
			msgs := ms.getMessages()
			for _, raw := range msgs {
				var msg map[string]interface{}
				json.Unmarshal(raw, &msg)
				if msg["type"] == "prd_response_complete" {
					cancel()
					return
				}
			}
			select {
			case <-deadline:
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

	// Collect all prd_output messages
	allMsgs := ms.getMessages()
	var prdOutputs []map[string]interface{}
	for _, raw := range allMsgs {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) == nil && msg["type"] == "prd_output" {
			prdOutputs = append(prdOutputs, msg)
		}
	}

	if len(prdOutputs) == 0 {
		t.Fatal("expected at least one prd_output message")
	}

	// Verify session_id is set on all prd_output messages
	for _, co := range prdOutputs {
		if co["session_id"] != "sess-mock-1" {
			t.Errorf("expected session_id 'sess-mock-1', got %v", co["session_id"])
		}
		if co["project"] != "myproject" {
			t.Errorf("expected project 'myproject', got %v", co["project"])
		}
	}

	// Verify we got a prd_response_complete message
	hasComplete := false
	for _, raw := range allMsgs {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) == nil && msg["type"] == "prd_response_complete" {
			hasComplete = true
			if msg["session_id"] != "sess-mock-1" {
				t.Errorf("expected session_id 'sess-mock-1' on prd_response_complete, got %v", msg["session_id"])
			}
			break
		}
	}
	if !hasComplete {
		t.Error("expected a prd_response_complete message")
	}

	// Verify we received some actual content
	hasContent := false
	for _, co := range prdOutputs {
		if text, ok := co["text"].(string); ok && strings.TrimSpace(text) != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("expected at least one prd_output with non-empty text")
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

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
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

		// Send new_prd via Pusher
		newPRDReq := map[string]string{
			"type":       "new_prd",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"project":    "myproject",
			"session_id": "sess-save-1",
			"message":    "Build an API",
		}
		ms.sendCommand(newPRDReq)

		time.Sleep(500 * time.Millisecond)

		// Close with save=true (waits for Claude to finish)
		closeMsg := map[string]interface{}{
			"type":       "close_prd_session",
			"id":         "req-2",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "sess-save-1",
			"save":       true,
		}
		ms.sendCommand(closeMsg)

		// Wait for a prd_response_complete message
		deadline := time.After(5 * time.Second)
		for {
			msgs := ms.getMessages()
			for _, raw := range msgs {
				var msg map[string]interface{}
				json.Unmarshal(raw, &msg)
				if msg["type"] == "prd_response_complete" {
					cancel()
					return
				}
			}
			select {
			case <-deadline:
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

	// Verify we received a prd_response_complete message
	allMsgs := ms.getMessages()
	hasComplete := false
	for _, raw := range allMsgs {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) == nil && msg["type"] == "prd_response_complete" {
			hasComplete = true
			break
		}
	}
	if !hasComplete {
		t.Error("expected a prd_response_complete message after save close")
	}
}

func TestSessionManager_ActiveSessions(t *testing.T) {
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

	sm := newSessionManager(&discardSender{})

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

	sender := &captureSender{}
	sm := newSessionManager(sender)

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

	// Check that we received the echoed message via captureSender
	msgs := sender.getMessages()
	hasEcho := false
	for _, msg := range msgs {
		if msg["type"] == "prd_output" {
			if text, ok := msg["text"].(string); ok && strings.Contains(text, "echo: hello world") {
				hasEcho = true
				break
			}
		}
	}
	if !hasEcho {
		t.Errorf("expected echoed message 'echo: hello world' in captured messages: %v", msgs)
	}

	// Clean up
	sm.killAll()
}

func TestSessionManager_CloseSession_Errors(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	sm := newSessionManager(&discardSender{})

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

	sm := newSessionManager(&discardSender{})

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

func TestSessionManager_RefinePRD(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	// Create existing PRD directory with prd.md
	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature-auth")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prdDir, "prd.md"), []byte("# Auth PRD\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send refine_prd request
		refinePRDReq := map[string]string{
			"type":       "refine_prd",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"project":    "myproject",
			"session_id": "sess-refine-1",
			"prd_id":     "feature-auth",
			"message":    "Add OAuth support",
		}
		ms.sendCommand(refinePRDReq)

		// Since we can't actually run claude in tests, expect an error response
		// (claude binary not available in test) — this tests the error path.
		raw, err := ms.waitForMessageType("error", 3*time.Second)
		if err != nil {
			raw, err = ms.waitForMessageType("prd_output", 3*time.Second)
			if err != nil {
				t.Fatal("expected error or prd_output message")
			}
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		msgType := msg["type"].(string)
		if msgType != "error" && msgType != "prd_output" {
			t.Errorf("expected error or prd_output message, got %s", msgType)
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}
}

func TestSessionManager_RefinePRD_ProjectNotFound(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		refinePRDReq := map[string]string{
			"type":       "refine_prd",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"project":    "nonexistent",
			"session_id": "sess-refine-1",
			"prd_id":     "feature-auth",
			"message":    "Add OAuth support",
		}
		ms.sendCommand(refinePRDReq)

		raw, err := ms.waitForMessageType("error", 2*time.Second)
		if err != nil {
			t.Fatalf("expected error message: %v", err)
		}

		var errorReceived map[string]interface{}
		if err := json.Unmarshal(raw, &errorReceived); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}

		if errorReceived["type"] != "error" {
			t.Errorf("expected type 'error', got %v", errorReceived["type"])
		}
		if errorReceived["code"] != "PROJECT_NOT_FOUND" {
			t.Errorf("expected code 'PROJECT_NOT_FOUND', got %v", errorReceived["code"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}
}

func TestSessionManager_RefinePRD_PRDNotFound(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		refinePRDReq := map[string]string{
			"type":       "refine_prd",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"project":    "myproject",
			"session_id": "sess-refine-1",
			"prd_id":     "nonexistent-prd",
			"message":    "Add OAuth support",
		}
		ms.sendCommand(refinePRDReq)

		raw, err := ms.waitForMessageType("error", 2*time.Second)
		if err != nil {
			t.Fatalf("expected error message: %v", err)
		}

		var errorReceived map[string]interface{}
		if err := json.Unmarshal(raw, &errorReceived); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}

		if errorReceived["type"] != "error" {
			t.Errorf("expected type 'error', got %v", errorReceived["type"])
		}
		if errorReceived["code"] != "CLAUDE_ERROR" {
			t.Errorf("expected code 'CLAUDE_ERROR', got %v", errorReceived["code"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}
}

func TestSessionManager_WithMockClaude_RefinePRD(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	// Create existing PRD directory with prd.md
	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature-auth")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prdDir, "prd.md"), []byte("# Auth PRD\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a mock "claude" script that echoes input
	mockClaudeBin := filepath.Join(home, "claude")
	mockScript := `#!/bin/sh
echo "Claude PRD edit session started"
echo "Processing: $1"
while IFS= read -r line; do
    echo "Received: $line"
done
echo "Edit complete"
`
	if err := os.WriteFile(mockClaudeBin, []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", home+":"+origPath)

	ctx, cancel := context.WithCancel(context.Background())
	ms := newMockUplinkServer(t)

	go func() {
		if err := ms.waitForPusherSubscribe(10 * time.Second); err != nil {
			t.Logf("waitForPusherSubscribe: %v", err)
			cancel()
			return
		}

		if _, err := ms.waitForMessageType("state_snapshot", 5*time.Second); err != nil {
			t.Logf("waitForMessageType(state_snapshot): %v", err)
			cancel()
			return
		}

		// Send refine_prd request via Pusher
		refinePRDReq := map[string]string{
			"type":       "refine_prd",
			"id":         "req-1",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"project":    "myproject",
			"session_id": "sess-refine-mock-1",
			"prd_id":     "feature-auth",
			"message":    "Add OAuth support",
		}
		ms.sendCommand(refinePRDReq)

		// Wait for output
		time.Sleep(500 * time.Millisecond)

		// Send a follow-up message
		prdMsg := map[string]string{
			"type":       "prd_message",
			"id":         "req-2",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "sess-refine-mock-1",
			"message":    "Also add RBAC",
		}
		ms.sendCommand(prdMsg)

		// Wait for output
		time.Sleep(500 * time.Millisecond)

		// Close the session
		closeMsg := map[string]interface{}{
			"type":       "close_prd_session",
			"id":         "req-3",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"session_id": "sess-refine-mock-1",
			"save":       false,
		}
		ms.sendCommand(closeMsg)

		// Wait for prd_response_complete
		deadline := time.After(5 * time.Second)
		for {
			msgs := ms.getMessages()
			for _, raw := range msgs {
				var msg map[string]interface{}
				json.Unmarshal(raw, &msg)
				if msg["type"] == "prd_response_complete" {
					cancel()
					return
				}
			}
			select {
			case <-deadline:
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

	// Collect all prd_output messages
	allMsgs := ms.getMessages()
	var prdOutputs []map[string]interface{}
	for _, raw := range allMsgs {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) == nil && msg["type"] == "prd_output" {
			prdOutputs = append(prdOutputs, msg)
		}
	}

	if len(prdOutputs) == 0 {
		t.Fatal("expected at least one prd_output message")
	}

	// Verify session_id is set on all prd_output messages
	for _, co := range prdOutputs {
		if co["session_id"] != "sess-refine-mock-1" {
			t.Errorf("expected session_id 'sess-refine-mock-1', got %v", co["session_id"])
		}
		if co["project"] != "myproject" {
			t.Errorf("expected project 'myproject', got %v", co["project"])
		}
	}

	// Verify we got a prd_response_complete message
	hasComplete := false
	for _, raw := range allMsgs {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) == nil && msg["type"] == "prd_response_complete" {
			hasComplete = true
			if msg["session_id"] != "sess-refine-mock-1" {
				t.Errorf("expected session_id 'sess-refine-mock-1' on prd_response_complete, got %v", msg["session_id"])
			}
			break
		}
	}
	if !hasComplete {
		t.Error("expected a prd_response_complete message")
	}

	// Verify the user's message was received by Claude (should appear in prd_output)
	hasUserMessage := false
	for _, co := range prdOutputs {
		if text, ok := co["text"].(string); ok && strings.Contains(text, "Add OAuth support") {
			hasUserMessage = true
			break
		}
	}
	if !hasUserMessage {
		t.Error("expected user's refine message 'Add OAuth support' to appear in prd_output")
	}
}

// newTestSessionManager creates a session manager with configurable timeouts for testing.
// It does NOT start the timeout checker goroutine automatically.
func newTestSessionManager(t *testing.T, timeout time.Duration, warningThresholds []int, checkInterval time.Duration) (*sessionManager, *captureSender, func()) {
	t.Helper()

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

	sender := &captureSender{}
	sm := &sessionManager{
		sessions:          make(map[string]*claudeSession),
		sender:            sender,
		timeout:           timeout,
		warningThresholds: warningThresholds,
		checkInterval:     checkInterval,
		stopTimeout:       make(chan struct{}),
	}

	cleanup := func() {
		sm.killAll()
	}

	return sm, sender, cleanup
}

func TestSessionManager_TimeoutExpiration(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Create mock claude
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

	sender := &captureSender{}
	sm := &sessionManager{
		sessions:          make(map[string]*claudeSession),
		sender:            sender,
		timeout:           200 * time.Millisecond, // Very short for testing
		warningThresholds: []int{},                // No warnings, just test expiry
		checkInterval:     50 * time.Millisecond,  // Check frequently
		stopTimeout:       make(chan struct{}),
	}
	go sm.runTimeoutChecker(sm.stopTimeout)

	projectDir := filepath.Join(home, "testproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := sm.newPRD(projectDir, "testproject", "sess-timeout-1", "test")
	if err != nil {
		t.Fatalf("newPRD failed: %v", err)
	}

	// Session should be active
	if len(sm.activeSessions()) != 1 {
		t.Fatal("expected 1 active session")
	}

	// Wait for timeout to expire + some buffer
	time.Sleep(500 * time.Millisecond)

	// Session should be expired and removed
	if len(sm.activeSessions()) != 0 {
		t.Errorf("expected 0 active sessions after timeout, got %d", len(sm.activeSessions()))
	}

	// Check that session_expired message was sent
	msgs := sender.getMessages()
	hasExpired := false
	for _, msg := range msgs {
		if msg["type"] == "session_expired" && msg["session_id"] == "sess-timeout-1" {
			hasExpired = true
			break
		}
	}
	if !hasExpired {
		t.Error("expected session_expired message to be sent")
	}

	sm.killAll()
}

func TestSessionManager_TimeoutWarnings(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Create mock claude
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

	sender := &captureSender{}

	// Use a 3-minute timeout with thresholds at 1 and 2 minutes.
	// We simulate time by setting lastActive in the past.
	sm := &sessionManager{
		sessions:          make(map[string]*claudeSession),
		sender:            sender,
		timeout:           3 * time.Minute,
		warningThresholds: []int{1, 2},           // Warn at 1min and 2min of inactivity
		checkInterval:     50 * time.Millisecond,  // Check frequently
		stopTimeout:       make(chan struct{}),
	}
	go sm.runTimeoutChecker(sm.stopTimeout)

	projectDir := filepath.Join(home, "testproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := sm.newPRD(projectDir, "testproject", "sess-warn-1", "test")
	if err != nil {
		t.Fatalf("newPRD failed: %v", err)
	}

	// Simulate 90 seconds of inactivity by setting lastActive in the past
	sess := sm.getSession("sess-warn-1")
	if sess == nil {
		t.Fatal("session not found")
	}
	sess.activeMu.Lock()
	sess.lastActive = time.Now().Add(-90 * time.Second)
	sess.activeMu.Unlock()

	// Wait for the checker to pick it up
	time.Sleep(200 * time.Millisecond)

	// Should have the 1-minute warning (2 remaining)
	msgs := sender.getMessages()
	var warningMessages []map[string]interface{}
	for _, msg := range msgs {
		if msg["type"] == "session_timeout_warning" {
			warningMessages = append(warningMessages, msg)
		}
	}

	if len(warningMessages) != 1 {
		t.Fatalf("expected 1 warning message, got %d", len(warningMessages))
	}

	// The warning at 1 min means 3-1 = 2 minutes remaining
	if warningMessages[0]["minutes_remaining"] != float64(2) {
		t.Errorf("expected minutes_remaining=2, got %v", warningMessages[0]["minutes_remaining"])
	}

	// Now simulate 2.5 minutes of inactivity
	sess.activeMu.Lock()
	sess.lastActive = time.Now().Add(-150 * time.Second)
	sess.activeMu.Unlock()

	time.Sleep(200 * time.Millisecond)

	msgs = sender.getMessages()
	warningMessages = nil
	for _, msg := range msgs {
		if msg["type"] == "session_timeout_warning" {
			warningMessages = append(warningMessages, msg)
		}
	}

	// Should now have 2 warnings (1 min and 2 min thresholds)
	if len(warningMessages) != 2 {
		t.Fatalf("expected 2 warning messages, got %d", len(warningMessages))
	}

	sm.killAll()
}

func TestSessionManager_TimeoutResetOnMessage(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Create mock claude
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

	sm := &sessionManager{
		sessions:          make(map[string]*claudeSession),
		sender:            &discardSender{},
		timeout:           300 * time.Millisecond,
		warningThresholds: []int{},
		checkInterval:     50 * time.Millisecond,
		stopTimeout:       make(chan struct{}),
	}
	go sm.runTimeoutChecker(sm.stopTimeout)

	projectDir := filepath.Join(home, "testproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := sm.newPRD(projectDir, "testproject", "sess-reset-1", "test")
	if err != nil {
		t.Fatalf("newPRD failed: %v", err)
	}

	// Wait 200ms (timeout is 300ms)
	time.Sleep(200 * time.Millisecond)

	// Session should still be active
	if len(sm.activeSessions()) != 1 {
		t.Fatal("expected session to still be active before timeout")
	}

	// Send a message to reset the timer
	if err := sm.sendMessage("sess-reset-1", "keep alive"); err != nil {
		t.Fatalf("sendMessage failed: %v", err)
	}

	// Wait another 200ms (total 400ms since start, but only 200ms since last activity)
	time.Sleep(200 * time.Millisecond)

	// Session should still be active because we reset the timer
	if len(sm.activeSessions()) != 1 {
		t.Error("expected session to still be active after timer reset")
	}

	// Wait for the full timeout from last activity (another 200ms)
	time.Sleep(200 * time.Millisecond)

	// Now it should have timed out
	if len(sm.activeSessions()) != 0 {
		t.Errorf("expected 0 active sessions after timeout, got %d", len(sm.activeSessions()))
	}

	sm.killAll()
}

func TestSessionManager_IndependentTimers(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Create mock claude
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

	sender := &captureSender{}
	sm := &sessionManager{
		sessions:          make(map[string]*claudeSession),
		sender:            sender,
		timeout:           300 * time.Millisecond,
		warningThresholds: []int{},
		checkInterval:     50 * time.Millisecond,
		stopTimeout:       make(chan struct{}),
	}
	go sm.runTimeoutChecker(sm.stopTimeout)

	projectDir1 := filepath.Join(home, "project1")
	projectDir2 := filepath.Join(home, "project2")
	os.MkdirAll(projectDir1, 0o755)
	os.MkdirAll(projectDir2, 0o755)

	// Start two sessions
	if err := sm.newPRD(projectDir1, "project1", "sess-a", "test"); err != nil {
		t.Fatalf("newPRD a failed: %v", err)
	}
	if err := sm.newPRD(projectDir2, "project2", "sess-b", "test"); err != nil {
		t.Fatalf("newPRD b failed: %v", err)
	}

	// Both should be active
	if len(sm.activeSessions()) != 2 {
		t.Fatalf("expected 2 active sessions, got %d", len(sm.activeSessions()))
	}

	// Keep session B alive by sending a message after 200ms
	time.Sleep(200 * time.Millisecond)
	if err := sm.sendMessage("sess-b", "keep alive"); err != nil {
		t.Fatalf("sendMessage failed: %v", err)
	}

	// Wait for session A to expire (another 200ms)
	time.Sleep(200 * time.Millisecond)

	// Session A should be expired, session B should still be active
	sessions := sm.activeSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(sessions))
	}
	if sessions[0].SessionID != "sess-b" {
		t.Errorf("expected session 'sess-b' to survive, got %q", sessions[0].SessionID)
	}

	// Verify session_expired was sent for sess-a
	msgs := sender.getMessages()
	hasExpiredA := false
	for _, msg := range msgs {
		if msg["type"] == "session_expired" && msg["session_id"] == "sess-a" {
			hasExpiredA = true
			break
		}
	}

	if !hasExpiredA {
		t.Error("expected session_expired for sess-a")
	}

	sm.killAll()
}

func TestSessionManager_TimeoutCheckerGoroutineSafe(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	// Create mock claude
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

	sm := &sessionManager{
		sessions:          make(map[string]*claudeSession),
		sender:            &discardSender{},
		timeout:           500 * time.Millisecond,
		warningThresholds: []int{},
		checkInterval:     50 * time.Millisecond,
		stopTimeout:       make(chan struct{}),
	}
	go sm.runTimeoutChecker(sm.stopTimeout)

	projectDir := filepath.Join(home, "testproject")
	os.MkdirAll(projectDir, 0o755)

	// Concurrently create sessions and send messages while timeout checker runs
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sid := fmt.Sprintf("sess-conc-%d", idx)
			dir := filepath.Join(home, fmt.Sprintf("proj-%d", idx))
			os.MkdirAll(dir, 0o755)

			if err := sm.newPRD(dir, fmt.Sprintf("proj-%d", idx), sid, "test"); err != nil {
				t.Errorf("newPRD %s failed: %v", sid, err)
				return
			}

			// Send some messages
			for j := 0; j < 3; j++ {
				time.Sleep(50 * time.Millisecond)
				sm.sendMessage(sid, fmt.Sprintf("msg-%d", j))
			}
		}(i)
	}
	wg.Wait()

	// No crash = goroutine-safe. Wait for all to expire.
	time.Sleep(700 * time.Millisecond)

	if len(sm.activeSessions()) != 0 {
		t.Errorf("expected all sessions to expire, got %d active", len(sm.activeSessions()))
	}

	sm.killAll()
}
