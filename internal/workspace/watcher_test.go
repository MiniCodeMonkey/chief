package workspace

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/ws"
)

func TestWatcher_WorkspaceRootChanges(t *testing.T) {
	workspace := t.TempDir()

	// Create initial project
	repoDir := filepath.Join(workspace, "existing")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	sender := &testSender{}

	scanner := New(workspace, sender)
	scanner.ScanAndUpdate()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := NewWatcher(workspace, scanner, sender)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	go watcher.Run(ctx)

	// Allow watcher to start
	time.Sleep(100 * time.Millisecond)

	// Create a new project directory with git
	newDir := filepath.Join(workspace, "new-project")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, newDir)

	// Wait for the project_list message with 2 projects
	deadline := time.After(5 * time.Second)
	for {
		msgs := sender.getMessages()
		for _, raw := range msgs {
			var msg struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &msg) == nil && msg.Type == ws.TypeProjectList {
				var plMsg ws.ProjectListMessage
				if err := json.Unmarshal(raw, &plMsg); err != nil {
					t.Fatalf("unmarshal project_list: %v", err)
				}
				if len(plMsg.Projects) == 2 {
					return // Success
				}
			}
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for project_list with new project")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestWatcher_ActivateProject(t *testing.T) {
	workspace := t.TempDir()

	// Create a project with .chief and .git
	repoDir := filepath.Join(workspace, "my-project")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)
	if err := os.MkdirAll(filepath.Join(repoDir, ".chief", "prds"), 0o755); err != nil {
		t.Fatal(err)
	}

	scanner := New(workspace, nil)
	scanner.ScanAndUpdate()

	watcher, err := NewWatcher(workspace, scanner, nil)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Run(ctx)

	// Initially no active projects
	watcher.mu.Lock()
	if len(watcher.activeProjects) != 0 {
		t.Error("expected 0 active projects initially")
	}
	watcher.mu.Unlock()

	// Activate a project
	watcher.Activate("my-project")

	watcher.mu.Lock()
	ap, exists := watcher.activeProjects["my-project"]
	watcher.mu.Unlock()

	if !exists {
		t.Fatal("expected my-project to be active")
	}
	if !ap.watching {
		t.Error("expected deep watchers to be set up")
	}
}

func TestWatcher_ActivateUnknownProject(t *testing.T) {
	workspace := t.TempDir()

	scanner := New(workspace, nil)
	scanner.ScanAndUpdate()

	watcher, err := NewWatcher(workspace, scanner, nil)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Activating unknown project should not panic or add to active list
	watcher.Activate("nonexistent")

	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	if len(watcher.activeProjects) != 0 {
		t.Error("expected 0 active projects for unknown project")
	}
}

func TestWatcher_ActivateRefreshesActivity(t *testing.T) {
	workspace := t.TempDir()

	repoDir := filepath.Join(workspace, "proj")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	scanner := New(workspace, nil)
	scanner.ScanAndUpdate()

	watcher, err := NewWatcher(workspace, scanner, nil)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	watcher.Activate("proj")

	watcher.mu.Lock()
	firstActive := watcher.activeProjects["proj"].lastActive
	watcher.mu.Unlock()

	time.Sleep(10 * time.Millisecond)

	watcher.Activate("proj")

	watcher.mu.Lock()
	secondActive := watcher.activeProjects["proj"].lastActive
	watcher.mu.Unlock()

	if !secondActive.After(firstActive) {
		t.Error("expected lastActive to be refreshed on re-activation")
	}
}

func TestWatcher_InactivityCleanup(t *testing.T) {
	workspace := t.TempDir()

	repoDir := filepath.Join(workspace, "proj")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	scanner := New(workspace, nil)
	scanner.ScanAndUpdate()

	watcher, err := NewWatcher(workspace, scanner, nil)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Use a very short timeout for testing
	watcher.inactiveTimeout = 50 * time.Millisecond

	watcher.Activate("proj")

	watcher.mu.Lock()
	if len(watcher.activeProjects) != 1 {
		t.Fatal("expected 1 active project")
	}
	watcher.mu.Unlock()

	// Wait for the project to become inactive
	time.Sleep(100 * time.Millisecond)

	watcher.cleanupInactive()

	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	if len(watcher.activeProjects) != 0 {
		t.Error("expected project to be cleaned up after inactivity timeout")
	}
}

func TestWatcher_ChiefPRDChangeSendsProjectState(t *testing.T) {
	workspace := t.TempDir()

	// Create project with .chief/prds
	repoDir := filepath.Join(workspace, "proj")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	prdDir := filepath.Join(repoDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdData := map[string]interface{}{
		"project": "Feature",
		"userStories": []map[string]interface{}{
			{"id": "US-001", "passes": false},
		},
	}
	data, _ := json.Marshal(prdData)
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	sender := &testSender{}

	scanner := New(workspace, sender)
	scanner.ScanAndUpdate()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := NewWatcher(workspace, scanner, sender)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	go watcher.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Activate the project to set up deep watchers
	watcher.Activate("proj")
	time.Sleep(100 * time.Millisecond)

	// Modify the PRD file
	prdData["userStories"] = []map[string]interface{}{
		{"id": "US-001", "passes": true},
	}
	data, _ = json.Marshal(prdData)
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for project_state message
	deadline := time.After(5 * time.Second)
	for {
		msgs := sender.getMessages()
		for _, raw := range msgs {
			var msg struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &msg) == nil && msg.Type == ws.TypeProjectState {
				var psMsg ws.ProjectStateMessage
				if err := json.Unmarshal(raw, &psMsg); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if psMsg.Project.Name == "proj" {
					return // Success
				}
			}
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for project_state message after PRD change")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestWatcher_GitHEADChangeSendsProjectState(t *testing.T) {
	workspace := t.TempDir()

	repoDir := filepath.Join(workspace, "proj")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	sender := &testSender{}

	scanner := New(workspace, sender)
	scanner.ScanAndUpdate()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := NewWatcher(workspace, scanner, sender)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	go watcher.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Activate the project
	watcher.Activate("proj")
	time.Sleep(100 * time.Millisecond)

	// Switch branch (changes .git/HEAD)
	cmd := exec.Command("git", "checkout", "-b", "feature/new-branch")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout failed: %v\n%s", err, out)
	}

	// Wait for project_state message
	deadline := time.After(5 * time.Second)
	for {
		msgs := sender.getMessages()
		for _, raw := range msgs {
			var msg struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &msg) == nil && msg.Type == ws.TypeProjectState {
				var psMsg ws.ProjectStateMessage
				if err := json.Unmarshal(raw, &psMsg); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if psMsg.Project.Name == "proj" {
					return // Success
				}
			}
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for project_state message after branch switch")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestWatcher_ContextCancellation(t *testing.T) {
	workspace := t.TempDir()

	scanner := New(workspace, nil)

	watcher, err := NewWatcher(workspace, scanner, nil)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- watcher.Run(ctx)
	}()

	// Let it start
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Good, it stopped
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after context cancel")
	}
}

func TestWatcher_NoDeepWatchersForInactiveProjects(t *testing.T) {
	workspace := t.TempDir()

	// Create two projects
	for _, name := range []string{"active-proj", "inactive-proj"} {
		dir := filepath.Join(workspace, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		initGitRepo(t, dir)
		if err := os.MkdirAll(filepath.Join(dir, ".chief", "prds"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	scanner := New(workspace, nil)
	scanner.ScanAndUpdate()

	watcher, err := NewWatcher(workspace, scanner, nil)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Only activate one project
	watcher.Activate("active-proj")

	watcher.mu.Lock()
	defer watcher.mu.Unlock()

	if _, exists := watcher.activeProjects["active-proj"]; !exists {
		t.Error("expected active-proj to be in active projects")
	}
	if _, exists := watcher.activeProjects["inactive-proj"]; exists {
		t.Error("expected inactive-proj to NOT be in active projects")
	}
}
