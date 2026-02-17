package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRunServe_GetPRDs(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepo(t, projectDir)

	// Create .chief/prds with two PRDs
	prd1Dir := filepath.Join(projectDir, ".chief", "prds", "feature-auth")
	prd2Dir := filepath.Join(projectDir, ".chief", "prds", "feature-dashboard")
	if err := os.MkdirAll(prd1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(prd2Dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// PRD with 2/3 stories passing → "active"
	prd1JSON := `{"project": "Auth System", "userStories": [
		{"id": "US-001", "passes": true},
		{"id": "US-002", "passes": true},
		{"id": "US-003", "passes": false}
	]}`
	if err := os.WriteFile(filepath.Join(prd1Dir, "prd.json"), []byte(prd1JSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// PRD with 2/2 stories passing → "done"
	prd2JSON := `{"project": "Dashboard", "userStories": [
		{"id": "US-010", "passes": true},
		{"id": "US-011", "passes": true}
	]}`
	if err := os.WriteFile(filepath.Join(prd2Dir, "prd.json"), []byte(prd2JSON), 0o644); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		req := map[string]interface{}{
			"type":      "get_prds",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

		raw, err := ms.waitForMessageType("prds_response", 5*time.Second)
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
		t.Fatal("prds_response was not received")
	}
	if response["type"] != "prds_response" {
		t.Errorf("expected type 'prds_response', got %v", response["type"])
	}

	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		t.Fatal("expected payload to be an object")
	}
	if payload["project"] != "myproject" {
		t.Errorf("expected project 'myproject', got %v", payload["project"])
	}

	prds, ok := payload["prds"].([]interface{})
	if !ok {
		t.Fatal("expected prds to be an array")
	}
	if len(prds) != 2 {
		t.Fatalf("expected 2 PRDs, got %d", len(prds))
	}

	// Build a map by ID for easier assertions
	prdMap := make(map[string]map[string]interface{})
	for _, p := range prds {
		prd := p.(map[string]interface{})
		prdMap[prd["id"].(string)] = prd
	}

	// feature-auth: 2/3 passing → "active"
	auth := prdMap["feature-auth"]
	if auth == nil {
		t.Fatal("expected feature-auth PRD")
	}
	if auth["name"] != "Auth System" {
		t.Errorf("expected name 'Auth System', got %v", auth["name"])
	}
	if int(auth["story_count"].(float64)) != 3 {
		t.Errorf("expected story_count 3, got %v", auth["story_count"])
	}
	if auth["status"] != "active" {
		t.Errorf("expected status 'active', got %v", auth["status"])
	}

	// feature-dashboard: 2/2 passing → "done"
	dash := prdMap["feature-dashboard"]
	if dash == nil {
		t.Fatal("expected feature-dashboard PRD")
	}
	if dash["status"] != "done" {
		t.Errorf("expected status 'done', got %v", dash["status"])
	}
}

func TestRunServe_GetPRDs_ProjectNotFound(t *testing.T) {
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
		req := map[string]interface{}{
			"type":      "get_prds",
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
		t.Fatal("error message was not received")
	}
	if response["code"] != "PROJECT_NOT_FOUND" {
		t.Errorf("expected code 'PROJECT_NOT_FOUND', got %v", response["code"])
	}
}

func TestRunServe_GetPRDs_EmptyProject(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Project with no .chief directory → empty PRD list
	createGitRepo(t, filepath.Join(workspaceDir, "myproject"))

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		req := map[string]interface{}{
			"type":      "get_prds",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

		raw, err := ms.waitForMessageType("prds_response", 5*time.Second)
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
		t.Fatal("prds_response was not received")
	}

	payload := response["payload"].(map[string]interface{})
	prds, ok := payload["prds"].([]interface{})
	if !ok {
		t.Fatal("expected prds to be an array")
	}
	if len(prds) != 0 {
		t.Errorf("expected 0 PRDs for project without .chief, got %d", len(prds))
	}
}

func TestMapCompletionStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0/0", "draft"},
		{"0/5", "active"},
		{"3/5", "active"},
		{"5/5", "done"},
		{"", "draft"},
		{"invalid", "draft"},
	}

	for _, tc := range tests {
		result := mapCompletionStatus(tc.input)
		if result != tc.expected {
			t.Errorf("mapCompletionStatus(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
