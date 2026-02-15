package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/minicodemonkey/chief/internal/ws"
)

func TestInferDirName(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/user/repo.git", "repo"},
		{"https://github.com/user/repo", "repo"},
		{"git@github.com:user/repo.git", "repo"},
		{"https://github.com/user/repo/", "repo"},
		{"https://github.com/user/my-project.git", "my-project"},
		{"git@github.com:org/my-lib.git", "my-lib"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := inferDirName(tt.url)
			if got != tt.expected {
				t.Errorf("inferDirName(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

func TestHandleCloneRepo_Success(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a bare git repo to clone from
	bareRepo := filepath.Join(home, "bare-repo.git")
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}

	var messages []map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		// Send clone_repo request
		cloneReq := map[string]interface{}{
			"type":      "clone_repo",
			"id":        "req-clone-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"url":       bareRepo,
		}
		conn.WriteJSON(cloneReq)

		// Read messages until we get clone_complete
		for i := 0; i < 20; i++ {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg map[string]interface{}
			json.Unmarshal(data, &msg)
			mu.Lock()
			messages = append(messages, msg)
			mu.Unlock()
			if msg["type"] == "clone_complete" {
				break
			}
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Find clone_complete message
	var cloneComplete map[string]interface{}
	for _, msg := range messages {
		if msg["type"] == "clone_complete" {
			cloneComplete = msg
			break
		}
	}

	if cloneComplete == nil {
		t.Fatal("clone_complete message was not received")
	}
	if cloneComplete["success"] != true {
		t.Errorf("expected success=true, got %v (error: %v)", cloneComplete["success"], cloneComplete["error"])
	}
	if cloneComplete["project"] != "bare-repo" {
		t.Errorf("expected project 'bare-repo', got %v", cloneComplete["project"])
	}

	// Verify the directory was created
	clonedDir := filepath.Join(workspaceDir, "bare-repo")
	if _, err := os.Stat(filepath.Join(clonedDir, ".git")); os.IsNotExist(err) {
		t.Error("cloned repository directory does not have .git")
	}
}

func TestHandleCloneRepo_CustomDirectoryName(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a bare git repo to clone from
	bareRepo := filepath.Join(home, "source.git")
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}

	var cloneComplete map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		cloneReq := map[string]interface{}{
			"type":           "clone_repo",
			"id":             "req-clone-2",
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
			"url":            bareRepo,
			"directory_name": "my-custom-name",
		}
		conn.WriteJSON(cloneReq)

		for i := 0; i < 20; i++ {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg map[string]interface{}
			json.Unmarshal(data, &msg)
			if msg["type"] == "clone_complete" {
				mu.Lock()
				cloneComplete = msg
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

	if cloneComplete == nil {
		t.Fatal("clone_complete not received")
	}
	if cloneComplete["success"] != true {
		t.Errorf("expected success=true, got %v", cloneComplete["success"])
	}
	if cloneComplete["project"] != "my-custom-name" {
		t.Errorf("expected project 'my-custom-name', got %v", cloneComplete["project"])
	}

	// Verify directory exists under custom name
	if _, err := os.Stat(filepath.Join(workspaceDir, "my-custom-name", ".git")); os.IsNotExist(err) {
		t.Error("cloned repo not found at custom directory name")
	}
}

func TestHandleCloneRepo_DirectoryExists(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create the target directory ahead of time
	if err := os.MkdirAll(filepath.Join(workspaceDir, "existing-repo"), 0o755); err != nil {
		t.Fatal(err)
	}

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		cloneReq := map[string]interface{}{
			"type":      "clone_repo",
			"id":        "req-clone-3",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"url":       "https://github.com/user/existing-repo.git",
		}
		conn.WriteJSON(cloneReq)

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
		t.Fatal("error message not received")
	}
	if errorReceived["code"] != "CLONE_FAILED" {
		t.Errorf("expected code CLONE_FAILED, got %v", errorReceived["code"])
	}
	if !strings.Contains(errorReceived["message"].(string), "already exists") {
		t.Errorf("expected 'already exists' in message, got %v", errorReceived["message"])
	}
}

func TestHandleCloneRepo_InvalidURL(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var cloneComplete map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		cloneReq := map[string]interface{}{
			"type":      "clone_repo",
			"id":        "req-clone-4",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"url":       "/nonexistent/invalid-repo",
		}
		conn.WriteJSON(cloneReq)

		for i := 0; i < 20; i++ {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg map[string]interface{}
			json.Unmarshal(data, &msg)
			if msg["type"] == "clone_complete" {
				mu.Lock()
				cloneComplete = msg
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

	if cloneComplete == nil {
		t.Fatal("clone_complete not received")
	}
	if cloneComplete["success"] != false {
		t.Errorf("expected success=false, got %v", cloneComplete["success"])
	}
	errMsg, ok := cloneComplete["error"].(string)
	if !ok || errMsg == "" {
		t.Error("expected non-empty error message for failed clone")
	}
}

func TestHandleCreateProject_Success(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		createReq := map[string]interface{}{
			"type":      "create_project",
			"id":        "req-create-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"name":      "new-project",
			"git_init":  false,
		}
		conn.WriteJSON(createReq)

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &response)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify directory was created
	projectDir := filepath.Join(workspaceDir, "new-project")
	info, err := os.Stat(projectDir)
	if err != nil {
		t.Fatalf("project directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected project path to be a directory")
	}

	mu.Lock()
	defer mu.Unlock()

	if response == nil {
		t.Fatal("no response received")
	}
	// Without git_init, project won't show up in scanner (no .git),
	// so we get a project_list response
	if response["type"] != "project_list" {
		t.Errorf("expected type 'project_list', got %v", response["type"])
	}
}

func TestHandleCreateProject_WithGitInit(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		createReq := map[string]interface{}{
			"type":      "create_project",
			"id":        "req-create-2",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"name":      "git-project",
			"git_init":  true,
		}
		conn.WriteJSON(createReq)

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err == nil {
			mu.Lock()
			json.Unmarshal(data, &response)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
	}

	// Verify git repo was initialized
	projectDir := filepath.Join(workspaceDir, "git-project")
	gitDir := filepath.Join(projectDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("expected .git directory to be created")
	}

	mu.Lock()
	defer mu.Unlock()

	if response == nil {
		t.Fatal("no response received")
	}
	// With git_init, scanner finds the project, so we get project_state
	if response["type"] != "project_state" {
		t.Errorf("expected type 'project_state', got %v", response["type"])
	}
	project, ok := response["project"].(map[string]interface{})
	if !ok {
		t.Fatal("expected project object in response")
	}
	if project["name"] != "git-project" {
		t.Errorf("expected project name 'git-project', got %v", project["name"])
	}
}

func TestHandleCreateProject_AlreadyExists(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create directory ahead of time
	if err := os.MkdirAll(filepath.Join(workspaceDir, "existing"), 0o755); err != nil {
		t.Fatal(err)
	}

	var errorReceived map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(conn *websocket.Conn) {
		createReq := map[string]interface{}{
			"type":      "create_project",
			"id":        "req-create-3",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"name":      "existing",
			"git_init":  false,
		}
		conn.WriteJSON(createReq)

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
		t.Fatal("error message not received")
	}
	if errorReceived["type"] != "error" {
		t.Errorf("expected type 'error', got %v", errorReceived["type"])
	}
	if errorReceived["code"] != "FILESYSTEM_ERROR" {
		t.Errorf("expected code FILESYSTEM_ERROR, got %v", errorReceived["code"])
	}
}

func TestHandleCreateProject_EmptyName(t *testing.T) {
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
		createReq := map[string]interface{}{
			"type":      "create_project",
			"id":        "req-create-4",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"name":      "",
			"git_init":  false,
		}
		conn.WriteJSON(createReq)

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
		t.Fatal("error message not received")
	}
	if errorReceived["code"] != "FILESYSTEM_ERROR" {
		t.Errorf("expected code FILESYSTEM_ERROR, got %v", errorReceived["code"])
	}
}

func TestScanGitProgress(t *testing.T) {
	input := "Cloning into 'repo'...\rReceiving objects:  50%\rReceiving objects: 100%\nDone.\n"
	var tokens []string
	data := []byte(input)
	for len(data) > 0 {
		advance, token, err := scanGitProgress(data, false)
		if err != nil {
			t.Fatal(err)
		}
		if advance == 0 {
			// Process remaining at EOF
			_, token, _ = scanGitProgress(data, true)
			if token != nil {
				tokens = append(tokens, string(token))
			}
			break
		}
		if token != nil {
			tokens = append(tokens, string(token))
		}
		data = data[advance:]
	}

	expected := []string{"Cloning into 'repo'...", "Receiving objects:  50%", "Receiving objects: 100%", "Done."}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("token[%d] = %q, want %q", i, tok, expected[i])
		}
	}
}

// Unit tests for clone/create functions with mock projectFinder

type mockScanner struct {
	workspacePath string
	projects      []ws.ProjectSummary
}

func (m *mockScanner) FindProject(name string) (ws.ProjectSummary, bool) {
	for _, p := range m.projects {
		if p.Name == name {
			return p, true
		}
	}
	return ws.ProjectSummary{}, false
}

func TestCloneProgressParsing(t *testing.T) {
	tests := []struct {
		input   string
		percent int
	}{
		{"Receiving objects:  50% (1/2)", 50},
		{"Resolving deltas: 100% (10/10)", 100},
		{"Cloning into 'repo'...", 0},
		{"Receiving objects:   3% (1/33)", 3},
	}

	for _, tt := range tests {
		matches := percentPattern.FindStringSubmatch(tt.input)
		got := 0
		if len(matches) > 1 {
			got, _ = strconv.Atoi(matches[1])
		}
		if got != tt.percent {
			t.Errorf("percent for %q: got %d, want %d", tt.input, got, tt.percent)
		}
	}
}

func TestSendCloneComplete_NilClient(t *testing.T) {
	// Should not panic
	sendCloneComplete(nil, "https://example.com/repo.git", "repo", true, "")
}

func TestSendCloneProgress_NilClient(t *testing.T) {
	// Should not panic
	sendCloneProgress(nil, "https://example.com/repo.git", "progress", 50)
}
