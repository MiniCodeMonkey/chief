package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// Send clone_repo request
		cloneReq := map[string]interface{}{
			"type":      "clone_repo",
			"id":        "req-clone-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"url":       bareRepo,
		}
		if err := ms.sendCommand(cloneReq); err != nil {
			t.Fatalf("failed to send clone command: %v", err)
		}

		// Wait for clone_complete message
		raw, err := ms.waitForMessageType("clone_complete", 5*time.Second)
		if err != nil {
			t.Fatalf("failed to receive clone_complete: %v", err)
		}

		var cloneComplete map[string]interface{}
		if err := json.Unmarshal(raw, &cloneComplete); err != nil {
			t.Fatalf("failed to unmarshal clone_complete: %v", err)
		}

		if cloneComplete["success"] != true {
			t.Errorf("expected success=true, got %v (error: %v)", cloneComplete["success"], cloneComplete["error"])
		}
		if cloneComplete["project"] != "bare-repo" {
			t.Errorf("expected project 'bare-repo', got %v", cloneComplete["project"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		cloneReq := map[string]interface{}{
			"type":           "clone_repo",
			"id":             "req-clone-2",
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
			"url":            bareRepo,
			"directory_name": "my-custom-name",
		}
		if err := ms.sendCommand(cloneReq); err != nil {
			t.Fatalf("failed to send clone command: %v", err)
		}

		// Wait for clone_complete message
		raw, err := ms.waitForMessageType("clone_complete", 5*time.Second)
		if err != nil {
			t.Fatalf("failed to receive clone_complete: %v", err)
		}

		var cloneComplete map[string]interface{}
		if err := json.Unmarshal(raw, &cloneComplete); err != nil {
			t.Fatalf("failed to unmarshal clone_complete: %v", err)
		}

		if cloneComplete["success"] != true {
			t.Errorf("expected success=true, got %v", cloneComplete["success"])
		}
		if cloneComplete["project"] != "my-custom-name" {
			t.Errorf("expected project 'my-custom-name', got %v", cloneComplete["project"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		cloneReq := map[string]interface{}{
			"type":      "clone_repo",
			"id":        "req-clone-3",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"url":       "https://github.com/user/existing-repo.git",
		}
		if err := ms.sendCommand(cloneReq); err != nil {
			t.Fatalf("failed to send clone command: %v", err)
		}

		// Wait for error message
		raw, err := ms.waitForMessageType("error", 2*time.Second)
		if err != nil {
			t.Fatalf("failed to receive error message: %v", err)
		}

		var errorReceived map[string]interface{}
		if err := json.Unmarshal(raw, &errorReceived); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}

		if errorReceived["code"] != "CLONE_FAILED" {
			t.Errorf("expected code CLONE_FAILED, got %v", errorReceived["code"])
		}
		if !strings.Contains(errorReceived["message"].(string), "already exists") {
			t.Errorf("expected 'already exists' in message, got %v", errorReceived["message"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		cloneReq := map[string]interface{}{
			"type":      "clone_repo",
			"id":        "req-clone-4",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"url":       "/nonexistent/invalid-repo",
		}
		if err := ms.sendCommand(cloneReq); err != nil {
			t.Fatalf("failed to send clone command: %v", err)
		}

		// Wait for clone_complete message
		raw, err := ms.waitForMessageType("clone_complete", 5*time.Second)
		if err != nil {
			t.Fatalf("failed to receive clone_complete: %v", err)
		}

		var cloneComplete map[string]interface{}
		if err := json.Unmarshal(raw, &cloneComplete); err != nil {
			t.Fatalf("failed to unmarshal clone_complete: %v", err)
		}

		if cloneComplete["success"] != false {
			t.Errorf("expected success=false, got %v", cloneComplete["success"])
		}
		errMsg, ok := cloneComplete["error"].(string)
		if !ok || errMsg == "" {
			t.Error("expected non-empty error message for failed clone")
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		createReq := map[string]interface{}{
			"type":      "create_project",
			"id":        "req-create-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"name":      "new-project",
			"git_init":  false,
		}
		if err := ms.sendCommand(createReq); err != nil {
			t.Fatalf("failed to send create command: %v", err)
		}

		// Wait for project_list message (without git_init, project won't show up in scanner)
		raw, err := ms.waitForMessageType("project_list", 2*time.Second)
		if err != nil {
			t.Fatalf("failed to receive project_list: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatalf("failed to unmarshal project_list: %v", err)
		}

		if response["type"] != "project_list" {
			t.Errorf("expected type 'project_list', got %v", response["type"])
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
}

func TestHandleCreateProject_WithGitInit(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		createReq := map[string]interface{}{
			"type":      "create_project",
			"id":        "req-create-2",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"name":      "git-project",
			"git_init":  true,
		}
		if err := ms.sendCommand(createReq); err != nil {
			t.Fatalf("failed to send create command: %v", err)
		}

		// Wait for project_state message (with git_init, scanner finds the project)
		raw, err := ms.waitForMessageType("project_state", 2*time.Second)
		if err != nil {
			t.Fatalf("failed to receive project_state: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatalf("failed to unmarshal project_state: %v", err)
		}

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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		createReq := map[string]interface{}{
			"type":      "create_project",
			"id":        "req-create-3",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"name":      "existing",
			"git_init":  false,
		}
		if err := ms.sendCommand(createReq); err != nil {
			t.Fatalf("failed to send create command: %v", err)
		}

		// Wait for error message
		raw, err := ms.waitForMessageType("error", 2*time.Second)
		if err != nil {
			t.Fatalf("failed to receive error message: %v", err)
		}

		var errorReceived map[string]interface{}
		if err := json.Unmarshal(raw, &errorReceived); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}

		if errorReceived["type"] != "error" {
			t.Errorf("expected type 'error', got %v", errorReceived["type"])
		}
		if errorReceived["code"] != "FILESYSTEM_ERROR" {
			t.Errorf("expected code FILESYSTEM_ERROR, got %v", errorReceived["code"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		createReq := map[string]interface{}{
			"type":      "create_project",
			"id":        "req-create-4",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"name":      "",
			"git_init":  false,
		}
		if err := ms.sendCommand(createReq); err != nil {
			t.Fatalf("failed to send create command: %v", err)
		}

		// Wait for error message
		raw, err := ms.waitForMessageType("error", 2*time.Second)
		if err != nil {
			t.Fatalf("failed to receive error message: %v", err)
		}

		var errorReceived map[string]interface{}
		if err := json.Unmarshal(raw, &errorReceived); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}

		if errorReceived["code"] != "FILESYSTEM_ERROR" {
			t.Errorf("expected code FILESYSTEM_ERROR, got %v", errorReceived["code"])
		}
	})
	if err != nil {
		t.Fatalf("RunServe returned error: %v", err)
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
