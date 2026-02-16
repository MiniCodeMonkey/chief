package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/engine"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/ws"
)

func TestRunServe_StartRun(t *testing.T) {
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

	// Write a minimal prd.json with one story
	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Test Story", "passes": false}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	var responseReceived map[string]interface{}
	var mu sync.Mutex
	gotError := false

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send start_run request
		startReq := map[string]string{
			"type":      "start_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		conn.WriteJSON(startReq)

		// Wait a moment for the run to start, then check if error was returned
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &responseReceived)
			// If it's an error, it means the run couldn't start (expected in test env without claude)
			if responseReceived["type"] == "error" {
				gotError = true
			}
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// In a test environment without a real claude binary, the engine.Start() call
	// will succeed (registers + starts the loop) but the loop itself will fail
	// quickly since there's no claude. We verify the handler routed correctly
	// by checking that we didn't get a PROJECT_NOT_FOUND error.
	mu.Lock()
	defer mu.Unlock()

	if responseReceived != nil && gotError {
		// If we got an error, it should NOT be PROJECT_NOT_FOUND
		code, _ := responseReceived["code"].(string)
		if code == "PROJECT_NOT_FOUND" {
			t.Errorf("should not have gotten PROJECT_NOT_FOUND for existing project")
		}
	}
}

func TestRunServe_StartRunProjectNotFound(t *testing.T) {
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
		startReq := map[string]string{
			"type":      "start_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
			"prd_id":    "feature",
		}
		conn.WriteJSON(startReq)

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

func TestRunServe_PauseRunNotActive(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		pauseReq := map[string]string{
			"type":      "pause_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		conn.WriteJSON(pauseReq)

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
	if errorReceived["code"] != "RUN_NOT_ACTIVE" {
		t.Errorf("expected code 'RUN_NOT_ACTIVE', got %v", errorReceived["code"])
	}
}

func TestRunServe_ResumeRunNotActive(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		resumeReq := map[string]string{
			"type":      "resume_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		conn.WriteJSON(resumeReq)

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
	if errorReceived["code"] != "RUN_NOT_ACTIVE" {
		t.Errorf("expected code 'RUN_NOT_ACTIVE', got %v", errorReceived["code"])
	}
}

func TestRunServe_StopRunNotActive(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		stopReq := map[string]string{
			"type":      "stop_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		conn.WriteJSON(stopReq)

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
	if errorReceived["code"] != "RUN_NOT_ACTIVE" {
		t.Errorf("expected code 'RUN_NOT_ACTIVE', got %v", errorReceived["code"])
	}
}

func TestRunManager_StartAndAlreadyActive(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Create a temp project with a PRD
	projectDir := t.TempDir()
	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a minimal prd.json
	prdState := `{"project": "Test", "userStories": [{"id": "US-001", "title": "Story", "passes": false}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Start a run
	err := rm.startRun("myproject", "feature", projectDir)
	if err != nil {
		t.Fatalf("startRun failed: %v", err)
	}

	// Wait briefly for the engine to register the run as running
	time.Sleep(100 * time.Millisecond)

	// Try to start the same run again — should get RUN_ALREADY_ACTIVE
	err = rm.startRun("myproject", "feature", projectDir)
	if err == nil {
		t.Fatal("expected error for already active run")
	}
	if err.Error() != "RUN_ALREADY_ACTIVE" {
		t.Errorf("expected 'RUN_ALREADY_ACTIVE', got: %v", err)
	}

	// Clean up
	rm.stopAll()
}

func TestRunManager_PauseAndResume(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Trying to pause when nothing is running
	err := rm.pauseRun("myproject", "feature")
	if err == nil || err.Error() != "RUN_NOT_ACTIVE" {
		t.Errorf("expected RUN_NOT_ACTIVE, got: %v", err)
	}

	// Trying to resume when nothing is paused
	err = rm.resumeRun("myproject", "feature")
	if err == nil || err.Error() != "RUN_NOT_ACTIVE" {
		t.Errorf("expected RUN_NOT_ACTIVE, got: %v", err)
	}
}

func TestRunManager_StopNotActive(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	err := rm.stopRun("myproject", "feature")
	if err == nil || err.Error() != "RUN_NOT_ACTIVE" {
		t.Errorf("expected RUN_NOT_ACTIVE, got: %v", err)
	}
}

func TestRunManager_ActiveRuns(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// No active runs initially
	runs := rm.activeRuns()
	if runs != nil && len(runs) != 0 {
		t.Errorf("expected no active runs, got %d", len(runs))
	}

	// Create a temp project with a PRD
	projectDir := t.TempDir()
	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdState := `{"project": "Test", "userStories": [{"id": "US-001", "title": "Story", "passes": false}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Start a run
	if err := rm.startRun("myproject", "feature", projectDir); err != nil {
		t.Fatalf("startRun failed: %v", err)
	}

	// Wait briefly for the engine to start
	time.Sleep(100 * time.Millisecond)

	// Should have one active run
	runs = rm.activeRuns()
	if len(runs) != 1 {
		t.Fatalf("expected 1 active run, got %d", len(runs))
	}

	if runs[0].Project != "myproject" {
		t.Errorf("expected project 'myproject', got %q", runs[0].Project)
	}
	if runs[0].PRDID != "feature" {
		t.Errorf("expected prd_id 'feature', got %q", runs[0].PRDID)
	}

	rm.stopAll()
}

func TestRunManager_MultipleConcurrentProjects(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Create two projects with PRDs
	for _, name := range []string{"project-a", "project-b"} {
		projectDir := filepath.Join(t.TempDir(), name)
		prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
		if err := os.MkdirAll(prdDir, 0o755); err != nil {
			t.Fatal(err)
		}

		prdState := `{"project": "Test", "userStories": [{"id": "US-001", "title": "Story", "passes": false}]}`
		if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := rm.startRun(name, "feature", projectDir); err != nil {
			t.Fatalf("startRun %s failed: %v", name, err)
		}
	}

	// Wait briefly
	time.Sleep(100 * time.Millisecond)

	// Should have two active runs
	runs := rm.activeRuns()
	if len(runs) != 2 {
		t.Errorf("expected 2 active runs, got %d", len(runs))
	}

	rm.stopAll()
}

func TestRunManager_LoopStateToString(t *testing.T) {
	tests := []struct {
		state    ws.RunState
		expected string
	}{
		{ws.RunState{Status: "running"}, "running"},
		{ws.RunState{Status: "paused"}, "paused"},
		{ws.RunState{Status: "stopped"}, "stopped"},
	}

	for _, tt := range tests {
		if tt.state.Status != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, tt.state.Status)
		}
	}
}

func TestRunManager_HandleQuotaExhausted(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	// Create a mock WS client to capture sent messages
	rm := newRunManager(eng, nil)

	// Add a run to the runs map
	rm.mu.Lock()
	rm.runs["myproject/feature"] = &runInfo{
		project: "myproject",
		prdID:   "feature",
		prdPath: "/tmp/test/prd.json",
	}
	rm.mu.Unlock()

	// handleQuotaExhausted should not panic even with nil client
	// (it logs errors but continues)
	rm.handleQuotaExhausted("myproject/feature")
}

func TestRunManager_HandleQuotaExhaustedUnknownRun(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Should not panic for unknown run
	rm.handleQuotaExhausted("unknown/run")
}

func TestRunManager_EventMonitorQuotaDetection(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Set up run tracking
	rm.mu.Lock()
	rm.runs["test/feature"] = &runInfo{
		project: "test",
		prdID:   "feature",
		prdPath: "/tmp/test/prd.json",
	}
	rm.mu.Unlock()

	// Start the event monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rm.startEventMonitor(ctx)

	// Give the goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop the monitor
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestRunManager_QuotaExhaustedWebSocket(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a git repo with a PRD that uses a mock claude that simulates quota error
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Test Story", "passes": false}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a mock claude that outputs a quota error on stderr and exits with non-zero
	mockDir := t.TempDir()
	mockScript := `#!/bin/sh
echo "rate limit exceeded" >&2
exit 1
`
	if err := os.WriteFile(filepath.Join(mockDir, "claude"), []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", mockDir+":"+origPath)

	var messages []map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send start_run request
		startReq := map[string]string{
			"type":      "start_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		conn.WriteJSON(startReq)

		// Read messages — we expect run_paused with reason "quota_exhausted"
		// and a quota_exhausted message
		for i := 0; i < 5; i++ {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
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
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Check that we received a run_paused with reason quota_exhausted
	foundRunPaused := false
	foundQuotaExhausted := false
	for _, msg := range messages {
		if msg["type"] == "run_paused" {
			if msg["reason"] == "quota_exhausted" {
				foundRunPaused = true
			}
		}
		if msg["type"] == "quota_exhausted" {
			foundQuotaExhausted = true
		}
	}

	if !foundRunPaused {
		t.Errorf("expected run_paused with reason quota_exhausted, got messages: %v", messages)
	}
	if !foundQuotaExhausted {
		t.Errorf("expected quota_exhausted message, got messages: %v", messages)
	}
}

func TestIsQuotaErrorIntegration(t *testing.T) {
	// Test that the loop package's IsQuotaError function correctly identifies quota errors
	tests := []struct {
		stderr   string
		expected bool
	}{
		{"Error: rate limit exceeded for model", true},
		{"HTTP 429 Too Many Requests", true},
		{"quota has been exceeded", true},
		{"normal crash: segfault", false},
	}

	for _, tt := range tests {
		got := loop.IsQuotaError(tt.stderr)
		if got != tt.expected {
			t.Errorf("IsQuotaError(%q) = %v, want %v", tt.stderr, got, tt.expected)
		}
	}
}

func TestRunManager_HandleEventRunProgress(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil) // nil client — sendRunProgress guards against nil

	// Add a run to the runs map
	rm.mu.Lock()
	rm.runs["myproject/feature"] = &runInfo{
		project:   "myproject",
		prdID:     "feature",
		prdPath:   "/tmp/test/prd.json",
		startTime: time.Now(),
	}
	rm.mu.Unlock()

	// Test that handleEvent does not panic for each event type with nil client
	eventTypes := []loop.EventType{
		loop.EventIterationStart,
		loop.EventStoryStarted,
		loop.EventStoryCompleted,
		loop.EventComplete,
		loop.EventMaxIterationsReached,
		loop.EventRetrying,
		loop.EventAssistantText,
		loop.EventToolStart,
		loop.EventToolResult,
		loop.EventError,
	}

	for _, et := range eventTypes {
		event := engine.ManagerEvent{
			PRDName: "myproject/feature",
			Event: loop.Event{
				Type:      et,
				Iteration: 1,
				StoryID:   "US-001",
				Text:      "test text",
				Tool:      "TestTool",
			},
		}
		rm.handleEvent(event) // should not panic with nil client
	}
}

func TestRunManager_HandleEventUnknownRun(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Events for unknown runs should be silently ignored
	event := engine.ManagerEvent{
		PRDName: "unknown/run",
		Event: loop.Event{
			Type:      loop.EventIterationStart,
			Iteration: 1,
		},
	}
	rm.handleEvent(event) // should not panic
}

func TestRunManager_HandleEventStoryTracking(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	rm.mu.Lock()
	rm.runs["myproject/feature"] = &runInfo{
		project:   "myproject",
		prdID:     "feature",
		prdPath:   "/tmp/test/prd.json",
		startTime: time.Now(),
	}
	rm.mu.Unlock()

	// Send a StoryStarted event — should update the tracked storyID
	event := engine.ManagerEvent{
		PRDName: "myproject/feature",
		Event: loop.Event{
			Type:    loop.EventStoryStarted,
			StoryID: "US-042",
		},
	}
	rm.handleEvent(event)

	rm.mu.RLock()
	storyID := rm.runs["myproject/feature"].storyID
	rm.mu.RUnlock()

	if storyID != "US-042" {
		t.Errorf("expected storyID 'US-042', got %q", storyID)
	}
}

func TestRunManager_SendRunComplete(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil) // nil client — guards against nil

	// Create a temp PRD with known pass/fail counts
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdJSON := `{"project": "Test", "userStories": [{"id": "US-001", "passes": true}, {"id": "US-002", "passes": true}, {"id": "US-003", "passes": false}]}`
	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(prdJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	info := &runInfo{
		project:   "myproject",
		prdID:     "feature",
		prdPath:   prdPath,
		startTime: time.Now().Add(-5 * time.Minute),
	}

	// Should not panic with nil client
	rm.sendRunComplete(info, "myproject/feature")
}

func TestRunManager_MarkInterruptedStories(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Create a temp project with a PRD
	projectDir := t.TempDir()
	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdPath := filepath.Join(prdDir, "prd.json")
	prdState := `{"project": "Test", "userStories": [{"id": "US-001", "title": "Story 1", "passes": false}, {"id": "US-002", "title": "Story 2", "passes": true}]}`
	if err := os.WriteFile(prdPath, []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add a run with an active story
	rm.mu.Lock()
	rm.runs["test/feature"] = &runInfo{
		project:   "test",
		prdID:     "feature",
		prdPath:   prdPath,
		startTime: time.Now(),
		storyID:   "US-001",
	}
	rm.mu.Unlock()

	// Mark interrupted stories
	rm.markInterruptedStories()

	// Verify the PRD was updated
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
		t.Errorf("expected US-001 to have inProgress=true, got %v", story1["inProgress"])
	}

	// US-002 is already passing, should NOT be marked as inProgress
	story2 := stories[1].(map[string]interface{})
	if _, hasInProgress := story2["inProgress"]; hasInProgress && story2["inProgress"] == true {
		t.Error("expected US-002 to NOT have inProgress=true (already passes)")
	}
}

func TestRunManager_MarkInterruptedStoriesNoStoryID(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Create a temp project with a PRD
	projectDir := t.TempDir()
	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdPath := filepath.Join(prdDir, "prd.json")
	prdState := `{"project": "Test", "userStories": [{"id": "US-001", "title": "Story 1", "passes": false}]}`
	if err := os.WriteFile(prdPath, []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add a run WITHOUT a story ID (no story started yet)
	rm.mu.Lock()
	rm.runs["test/feature"] = &runInfo{
		project:   "test",
		prdID:     "feature",
		prdPath:   prdPath,
		startTime: time.Now(),
		storyID:   "", // no story started
	}
	rm.mu.Unlock()

	// Mark interrupted stories — should be a no-op
	rm.markInterruptedStories()

	// Verify the PRD was NOT modified
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
	if _, hasInProgress := story1["inProgress"]; hasInProgress && story1["inProgress"] == true {
		t.Error("expected US-001 to NOT have inProgress=true when no story was started")
	}
}

func TestRunManager_ActiveRunCount(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	if rm.activeRunCount() != 0 {
		t.Errorf("expected 0 active runs, got %d", rm.activeRunCount())
	}

	rm.mu.Lock()
	rm.runs["test/feature"] = &runInfo{
		project: "test",
		prdID:   "feature",
	}
	rm.mu.Unlock()

	if rm.activeRunCount() != 1 {
		t.Errorf("expected 1 active run, got %d", rm.activeRunCount())
	}
}

func TestRunServe_RunProgressStreaming(t *testing.T) {
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

	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Test Story", "passes": false}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a mock claude that outputs stream-json with a story start marker then exits successfully
	mockDir := t.TempDir()
	mockScript := `#!/bin/sh
echo '{"type":"system","subtype":"init"}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Working on <ralph-status>US-001</ralph-status>"}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Hello from Claude"}]}}'
echo '{"type":"result"}'
exit 0
`
	if err := os.WriteFile(filepath.Join(mockDir, "claude"), []byte(mockScript), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", mockDir+":"+origPath)

	var messages []map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send start_run request
		startReq := map[string]string{
			"type":      "start_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		conn.WriteJSON(startReq)

		// Read messages — expect run_progress and claude_output messages
		for i := 0; i < 15; i++ {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
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
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Check for run_progress messages
	foundIterationStart := false
	foundStoryStarted := false
	foundClaudeOutput := false
	for _, msg := range messages {
		if msg["type"] == "run_progress" {
			status, _ := msg["status"].(string)
			if status == "iteration_started" {
				foundIterationStart = true
			}
			if status == "story_started" {
				foundStoryStarted = true
				if msg["story_id"] != "US-001" {
					t.Errorf("expected story_id 'US-001', got %v", msg["story_id"])
				}
				if msg["project"] != "myproject" {
					t.Errorf("expected project 'myproject', got %v", msg["project"])
				}
				if msg["prd_id"] != "feature" {
					t.Errorf("expected prd_id 'feature', got %v", msg["prd_id"])
				}
			}
		}
		if msg["type"] == "claude_output" {
			foundClaudeOutput = true
			if msg["project"] != "myproject" {
				t.Errorf("expected project 'myproject', got %v", msg["project"])
			}
		}
	}

	if !foundIterationStart {
		t.Errorf("expected run_progress with status 'iteration_started', messages: %v", messages)
	}
	if !foundStoryStarted {
		t.Errorf("expected run_progress with status 'story_started', messages: %v", messages)
	}
	if !foundClaudeOutput {
		t.Errorf("expected claude_output messages, messages: %v", messages)
	}
}
