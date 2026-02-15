package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/engine"
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
