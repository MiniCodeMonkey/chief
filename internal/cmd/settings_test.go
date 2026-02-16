package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/config"
)

func TestRunServe_GetSettings_Defaults(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	var settingsReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		req := map[string]string{
			"type":      "get_settings",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

		raw, err := ms.waitForMessageType("settings", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &settingsReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if settingsReceived == nil {
		t.Fatal("settings was not received")
	}
	if settingsReceived["type"] != "settings" {
		t.Errorf("expected type 'settings', got %v", settingsReceived["type"])
	}
	if settingsReceived["project"] != "myproject" {
		t.Errorf("expected project 'myproject', got %v", settingsReceived["project"])
	}
	// Default max_iterations should be 5
	if maxIter, ok := settingsReceived["max_iterations"].(float64); !ok || int(maxIter) != 5 {
		t.Errorf("expected max_iterations 5, got %v", settingsReceived["max_iterations"])
	}
	// Default auto_commit should be true
	if autoCommit, ok := settingsReceived["auto_commit"].(bool); !ok || !autoCommit {
		t.Errorf("expected auto_commit true, got %v", settingsReceived["auto_commit"])
	}
	// Other fields should be empty strings
	if settingsReceived["commit_prefix"] != "" {
		t.Errorf("expected empty commit_prefix, got %v", settingsReceived["commit_prefix"])
	}
	if settingsReceived["claude_model"] != "" {
		t.Errorf("expected empty claude_model, got %v", settingsReceived["claude_model"])
	}
	if settingsReceived["test_command"] != "" {
		t.Errorf("expected empty test_command, got %v", settingsReceived["test_command"])
	}
}

func TestRunServe_GetSettings_ProjectNotFound(t *testing.T) {
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
		req := map[string]string{
			"type":      "get_settings",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

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

func TestRunServe_GetSettings_WithExistingConfig(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	// Write existing config
	autoCommit := false
	cfg := &config.Config{
		MaxIterations: 10,
		AutoCommit:    &autoCommit,
		CommitPrefix:  "fix:",
		ClaudeModel:   "claude-sonnet-4-5-20250929",
		TestCommand:   "npm test",
	}
	if err := config.Save(projectDir, cfg); err != nil {
		t.Fatal(err)
	}

	var settingsReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		req := map[string]string{
			"type":      "get_settings",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

		raw, err := ms.waitForMessageType("settings", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &settingsReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if settingsReceived == nil {
		t.Fatal("settings was not received")
	}
	if maxIter, ok := settingsReceived["max_iterations"].(float64); !ok || int(maxIter) != 10 {
		t.Errorf("expected max_iterations 10, got %v", settingsReceived["max_iterations"])
	}
	if autoCommitVal, ok := settingsReceived["auto_commit"].(bool); !ok || autoCommitVal {
		t.Errorf("expected auto_commit false, got %v", settingsReceived["auto_commit"])
	}
	if settingsReceived["commit_prefix"] != "fix:" {
		t.Errorf("expected commit_prefix 'fix:', got %v", settingsReceived["commit_prefix"])
	}
	if settingsReceived["claude_model"] != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected claude_model 'claude-sonnet-4-5-20250929', got %v", settingsReceived["claude_model"])
	}
	if settingsReceived["test_command"] != "npm test" {
		t.Errorf("expected test_command 'npm test', got %v", settingsReceived["test_command"])
	}
}

func TestRunServe_UpdateSettings(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	var settingsReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		maxIter := 8
		autoCommit := false
		commitPrefix := "chore:"
		claudeModel := "claude-sonnet-4-5-20250929"
		testCommand := "go test ./..."

		req := map[string]interface{}{
			"type":           "update_settings",
			"id":             "req-1",
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
			"project":        "myproject",
			"max_iterations": maxIter,
			"auto_commit":    autoCommit,
			"commit_prefix":  commitPrefix,
			"claude_model":   claudeModel,
			"test_command":   testCommand,
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

		raw, err := ms.waitForMessageType("settings", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &settingsReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if settingsReceived == nil {
		t.Fatal("settings was not received")
	}
	if settingsReceived["type"] != "settings" {
		t.Errorf("expected type 'settings', got %v", settingsReceived["type"])
	}
	if maxIter, ok := settingsReceived["max_iterations"].(float64); !ok || int(maxIter) != 8 {
		t.Errorf("expected max_iterations 8, got %v", settingsReceived["max_iterations"])
	}
	if autoCommitVal, ok := settingsReceived["auto_commit"].(bool); !ok || autoCommitVal {
		t.Errorf("expected auto_commit false, got %v", settingsReceived["auto_commit"])
	}
	if settingsReceived["commit_prefix"] != "chore:" {
		t.Errorf("expected commit_prefix 'chore:', got %v", settingsReceived["commit_prefix"])
	}
	if settingsReceived["claude_model"] != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected claude_model 'claude-sonnet-4-5-20250929', got %v", settingsReceived["claude_model"])
	}
	if settingsReceived["test_command"] != "go test ./..." {
		t.Errorf("expected test_command 'go test ./...', got %v", settingsReceived["test_command"])
	}

	// Verify the config was persisted to disk
	cfg, err := config.Load(filepath.Join(workspaceDir, "myproject"))
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.MaxIterations != 8 {
		t.Errorf("expected saved max_iterations 8, got %d", cfg.MaxIterations)
	}
	if cfg.AutoCommit == nil || *cfg.AutoCommit != false {
		t.Errorf("expected saved auto_commit false, got %v", cfg.AutoCommit)
	}
}

func TestRunServe_UpdateSettings_PartialUpdate(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	// Set initial config
	autoCommit := false
	cfg := &config.Config{
		MaxIterations: 10,
		AutoCommit:    &autoCommit,
		CommitPrefix:  "fix:",
		TestCommand:   "npm test",
	}
	if err := config.Save(projectDir, cfg); err != nil {
		t.Fatal(err)
	}

	var settingsReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Only update test_command — other fields should be preserved
		req := map[string]interface{}{
			"type":         "update_settings",
			"id":           "req-1",
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
			"project":      "myproject",
			"test_command": "go test ./...",
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

		raw, err := ms.waitForMessageType("settings", 5*time.Second)
		if err == nil {
			mu.Lock()
			json.Unmarshal(raw, &settingsReceived)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if settingsReceived == nil {
		t.Fatal("settings was not received")
	}
	// Existing values should be preserved
	if maxIter, ok := settingsReceived["max_iterations"].(float64); !ok || int(maxIter) != 10 {
		t.Errorf("expected max_iterations 10 preserved, got %v", settingsReceived["max_iterations"])
	}
	if autoCommitVal, ok := settingsReceived["auto_commit"].(bool); !ok || autoCommitVal {
		t.Errorf("expected auto_commit false preserved, got %v", settingsReceived["auto_commit"])
	}
	if settingsReceived["commit_prefix"] != "fix:" {
		t.Errorf("expected commit_prefix 'fix:' preserved, got %v", settingsReceived["commit_prefix"])
	}
	// Updated value
	if settingsReceived["test_command"] != "go test ./..." {
		t.Errorf("expected test_command 'go test ./...', got %v", settingsReceived["test_command"])
	}
}

func TestRunServe_UpdateSettings_ProjectNotFound(t *testing.T) {
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
		req := map[string]interface{}{
			"type":           "update_settings",
			"id":             "req-1",
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
			"project":        "nonexistent",
			"max_iterations": 3,
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

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

func TestRunServe_UpdateSettings_InvalidMaxIterations(t *testing.T) {
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		req := map[string]interface{}{
			"type":           "update_settings",
			"id":             "req-1",
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
			"project":        "myproject",
			"max_iterations": 0,
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

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
	if errorReceived["code"] != "FILESYSTEM_ERROR" {
		t.Errorf("expected code 'FILESYSTEM_ERROR', got %v", errorReceived["code"])
	}
}
