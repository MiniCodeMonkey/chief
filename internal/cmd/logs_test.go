package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/engine"
)

func TestStoryLogger_WriteAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	sl, err := newStoryLogger(prdPath)
	if err != nil {
		t.Fatalf("newStoryLogger failed: %v", err)
	}
	defer sl.Close()

	// Write some log lines
	sl.WriteLog("US-001", "Starting story US-001")
	sl.WriteLog("US-001", "Working on implementation")
	sl.WriteLog("US-001", "Story complete")

	sl.WriteLog("US-002", "Starting story US-002")
	sl.WriteLog("US-002", "Done")

	// Close to flush
	sl.Close()

	// Read the log files
	logDir := filepath.Join(prdDir, "logs")

	lines, err := readLogFile(filepath.Join(logDir, "US-001.log"), 0)
	if err != nil {
		t.Fatalf("readLogFile failed: %v", err)
	}
	if len(lines) != 3 {
		t.Errorf("expected 3 lines for US-001, got %d", len(lines))
	}
	if lines[0] != "Starting story US-001" {
		t.Errorf("expected first line 'Starting story US-001', got %q", lines[0])
	}

	lines, err = readLogFile(filepath.Join(logDir, "US-002.log"), 0)
	if err != nil {
		t.Fatalf("readLogFile failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 lines for US-002, got %d", len(lines))
	}
}

func TestStoryLogger_WriteEmptyStoryID(t *testing.T) {
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	sl, err := newStoryLogger(prdPath)
	if err != nil {
		t.Fatalf("newStoryLogger failed: %v", err)
	}
	defer sl.Close()

	// Writing with empty story ID should be a no-op
	sl.WriteLog("", "This should not be written")
	sl.Close()

	// Verify no files were created
	logDir := filepath.Join(prdDir, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no log files, got %d", len(entries))
	}
}

func TestStoryLogger_OverwriteOnNewRun(t *testing.T) {
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create first logger and write some logs
	sl1, err := newStoryLogger(prdPath)
	if err != nil {
		t.Fatal(err)
	}
	sl1.WriteLog("US-001", "First run output")
	sl1.Close()

	// Create second logger — should overwrite previous logs
	sl2, err := newStoryLogger(prdPath)
	if err != nil {
		t.Fatal(err)
	}
	sl2.WriteLog("US-001", "Second run output")
	sl2.Close()

	// Read the log — should only have second run's content
	logDir := filepath.Join(prdDir, "logs")
	lines, err := readLogFile(filepath.Join(logDir, "US-001.log"), 0)
	if err != nil {
		t.Fatalf("readLogFile failed: %v", err)
	}
	if len(lines) != 1 {
		t.Errorf("expected 1 line (overwritten), got %d: %v", len(lines), lines)
	}
	if len(lines) > 0 && lines[0] != "Second run output" {
		t.Errorf("expected 'Second run output', got %q", lines[0])
	}
}

func TestReadLogFile_WithLineLimit(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Write 10 lines
	var content string
	for i := 1; i <= 10; i++ {
		content += "Line " + string(rune('0'+i)) + "\n"
	}
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Read with limit of 3 — should get last 3 lines
	lines, err := readLogFile(logPath, 3)
	if err != nil {
		t.Fatalf("readLogFile failed: %v", err)
	}
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestReadLogFile_Nonexistent(t *testing.T) {
	lines, err := readLogFile("/nonexistent/path/test.log", 0)
	if err != nil {
		t.Fatalf("expected no error for nonexistent file, got: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected empty lines for nonexistent file, got %d", len(lines))
	}
}

func TestReadMostRecentLog(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write two log files with different mod times
	if err := os.WriteFile(filepath.Join(logDir, "US-001.log"), []byte("old log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ensure US-002 is newer
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(logDir, "US-002.log"), []byte("new log line 1\nnew log line 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	storyID, lines, err := readMostRecentLog(logDir, 0)
	if err != nil {
		t.Fatalf("readMostRecentLog failed: %v", err)
	}
	if storyID != "US-002" {
		t.Errorf("expected most recent story 'US-002', got %q", storyID)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestReadMostRecentLog_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	storyID, lines, err := readMostRecentLog(logDir, 0)
	if err != nil {
		t.Fatalf("readMostRecentLog failed: %v", err)
	}
	if storyID != "" {
		t.Errorf("expected empty story ID, got %q", storyID)
	}
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %d", len(lines))
	}
}

func TestReadMostRecentLog_NonexistentDir(t *testing.T) {
	storyID, lines, err := readMostRecentLog("/nonexistent/logs", 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if storyID != "" || len(lines) != 0 {
		t.Errorf("expected empty results, got storyID=%q lines=%v", storyID, lines)
	}
}

func TestRunManager_StoryLogWriting(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	// Create a temp project with a PRD
	projectDir := t.TempDir()
	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdState := `{"project": "Test", "userStories": [{"id": "US-001", "title": "Story", "passes": false}]}`
	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Start a run (creates logger)
	err := rm.startRun("myproject", "feature", projectDir)
	if err != nil {
		t.Fatalf("startRun failed: %v", err)
	}

	// Verify logger was created
	rm.mu.RLock()
	_, hasLogger := rm.loggers["myproject/feature"]
	rm.mu.RUnlock()
	if !hasLogger {
		t.Fatal("expected logger to be created for the run")
	}

	// Write some story logs
	rm.writeStoryLog("myproject/feature", "US-001", "Hello from story log")
	rm.writeStoryLog("myproject/feature", "US-001", "Another line")

	// Stop and cleanup
	rm.stopAll()

	// Read the log file
	logPath := filepath.Join(prdDir, "logs", "US-001.log")
	lines, err := readLogFile(logPath, 0)
	if err != nil {
		t.Fatalf("readLogFile failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	if len(lines) > 0 && lines[0] != "Hello from story log" {
		t.Errorf("expected 'Hello from story log', got %q", lines[0])
	}
}

func TestRunServe_GetLogs(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a git repo with a PRD that has log files
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	logDir := filepath.Join(prdDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Test Story", "passes": true}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a log file for US-001
	logContent := "Starting story US-001\nWorking on implementation\nStory complete\n"
	if err := os.WriteFile(filepath.Join(logDir, "US-001.log"), []byte(logContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send get_logs request with story_id
		getLogsReq := map[string]interface{}{
			"type":      "get_logs",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
			"story_id":  "US-001",
		}
		ms.sendCommand(getLogsReq)

		raw, err := ms.waitForMessageType("log_lines", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &response)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if response == nil {
		t.Fatal("response was not received")
	}
	if response["type"] != "log_lines" {
		t.Errorf("expected type 'log_lines', got %v", response["type"])
	}
	if response["project"] != "myproject" {
		t.Errorf("expected project 'myproject', got %v", response["project"])
	}
	if response["prd_id"] != "feature" {
		t.Errorf("expected prd_id 'feature', got %v", response["prd_id"])
	}
	if response["story_id"] != "US-001" {
		t.Errorf("expected story_id 'US-001', got %v", response["story_id"])
	}

	lines, ok := response["lines"].([]interface{})
	if !ok {
		t.Fatal("expected lines to be an array")
	}
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestRunServe_GetLogsNoStoryID(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	logDir := filepath.Join(prdDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Test Story", "passes": true}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a log file — when no story_id provided, should return most recent
	if err := os.WriteFile(filepath.Join(logDir, "US-001.log"), []byte("recent log line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		getLogsReq := map[string]interface{}{
			"type":      "get_logs",
			"id":        "req-2",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		ms.sendCommand(getLogsReq)

		raw, err := ms.waitForMessageType("log_lines", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &response)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if response == nil {
		t.Fatal("response was not received")
	}
	if response["type"] != "log_lines" {
		t.Errorf("expected type 'log_lines', got %v", response["type"])
	}
	if response["story_id"] != "US-001" {
		t.Errorf("expected story_id 'US-001', got %v", response["story_id"])
	}
}

func TestRunServe_GetLogsProjectNotFound(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		getLogsReq := map[string]interface{}{
			"type":      "get_logs",
			"id":        "req-3",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
			"prd_id":    "feature",
			"story_id":  "US-001",
		}
		ms.sendCommand(getLogsReq)

		raw, err := ms.waitForMessageType("error", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &response)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if response == nil {
		t.Fatal("response was not received")
	}
	if response["type"] != "error" {
		t.Errorf("expected type 'error', got %v", response["type"])
	}
	if response["code"] != "PROJECT_NOT_FOUND" {
		t.Errorf("expected code 'PROJECT_NOT_FOUND', got %v", response["code"])
	}
}

func TestRunServe_GetLogsPRDNotFound(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		getLogsReq := map[string]interface{}{
			"type":      "get_logs",
			"id":        "req-4",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "nonexistent",
			"story_id":  "US-001",
		}
		ms.sendCommand(getLogsReq)

		raw, err := ms.waitForMessageType("error", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &response)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if response == nil {
		t.Fatal("response was not received")
	}
	if response["type"] != "error" {
		t.Errorf("expected type 'error', got %v", response["type"])
	}
	if response["code"] != "PRD_NOT_FOUND" {
		t.Errorf("expected code 'PRD_NOT_FOUND', got %v", response["code"])
	}
}

func TestRunServe_GetLogsWithLineLimit(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	logDir := filepath.Join(prdDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Test Story", "passes": true}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write 5 log lines
	logContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"
	if err := os.WriteFile(filepath.Join(logDir, "US-001.log"), []byte(logContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Request only 2 lines
		getLogsReq := map[string]interface{}{
			"type":      "get_logs",
			"id":        "req-5",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
			"story_id":  "US-001",
			"lines":     2,
		}
		ms.sendCommand(getLogsReq)

		raw, err := ms.waitForMessageType("log_lines", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &response)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if response == nil {
		t.Fatal("response was not received")
	}
	if response["type"] != "log_lines" {
		t.Errorf("expected type 'log_lines', got %v", response["type"])
	}

	lines, ok := response["lines"].([]interface{})
	if !ok {
		t.Fatal("expected lines to be an array")
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (limited), got %d", len(lines))
	}
	// Should return the last 2 lines
	if len(lines) >= 2 {
		if lines[0] != "Line 4" {
			t.Errorf("expected 'Line 4', got %v", lines[0])
		}
		if lines[1] != "Line 5" {
			t.Errorf("expected 'Line 5', got %v", lines[1])
		}
	}
}

func TestRunServe_LoggingIntegration(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

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

	// Create a mock claude that outputs stream-json
	mockDir := t.TempDir()
	mockScript := `#!/bin/sh
echo '{"type":"system","subtype":"init"}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Working on <ralph-status>US-001</ralph-status>"}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Implementing feature"}]}}'
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send start_run request
		startReq := map[string]string{
			"type":      "start_run",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
		}
		ms.sendCommand(startReq)

		// Wait for multiple messages (expecting at least some stream messages)
		rawMessages, err := ms.waitForMessages(15, 5*time.Second)
		if err == nil {
			mu.Lock()
			for _, raw := range rawMessages {
				var msg map[string]interface{}
				if json.Unmarshal(raw, &msg) == nil {
					messages = append(messages, msg)
				}
			}
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify that log files were created in .chief/prds/feature/logs/
	logDir := filepath.Join(prdDir, "logs")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Error("expected logs directory to be created")
	}

	// The log file should exist for US-001 (the story that was started)
	logFile := filepath.Join(logDir, "US-001.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("expected US-001.log to be created")
	} else {
		lines, err := readLogFile(logFile, 0)
		if err != nil {
			t.Fatalf("readLogFile failed: %v", err)
		}
		// Should have at least some log content
		if len(lines) == 0 {
			t.Error("expected log file to have content")
		}
	}
}

func TestRunManager_CleanupClosesLogger(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil)

	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdPath := filepath.Join(prdDir, "prd.json")
	prdState := `{"project": "Test", "userStories": [{"id": "US-001", "title": "Story", "passes": false}]}`
	if err := os.WriteFile(prdPath, []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := rm.startRun("myproject", "feature", tmpDir); err != nil {
		t.Fatalf("startRun failed: %v", err)
	}

	key := "myproject/feature"

	// Verify logger exists
	rm.mu.RLock()
	_, hasLogger := rm.loggers[key]
	rm.mu.RUnlock()
	if !hasLogger {
		t.Fatal("expected logger to be created")
	}

	// Cleanup should remove the logger
	rm.cleanup(key)

	rm.mu.RLock()
	_, hasLogger = rm.loggers[key]
	rm.mu.RUnlock()
	if hasLogger {
		t.Error("expected logger to be removed after cleanup")
	}
}
