package workspace

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/ws"
)

// testSender is a mock MessageSender that captures sent messages.
type testSender struct {
	mu       sync.Mutex
	messages []json.RawMessage
}

func (s *testSender) Send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.messages = append(s.messages, data)
	s.mu.Unlock()
	return nil
}

func (s *testSender) getMessages() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]json.RawMessage, len(s.messages))
	copy(cp, s.messages)
	return cp
}

func (s *testSender) waitForType(msgType string, timeout time.Duration) (json.RawMessage, bool) {
	deadline := time.After(timeout)
	for {
		msgs := s.getMessages()
		for _, raw := range msgs {
			var m struct{ Type string `json:"type"` }
			if json.Unmarshal(raw, &m) == nil && m.Type == msgType {
				return raw, true
			}
		}
		select {
		case <-deadline:
			return nil, false
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// initGitRepo initializes a git repo with an initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git command %v failed: %v\n%s", args, err, out)
		}
	}
}

func TestScan_DiscoversGitRepos(t *testing.T) {
	workspace := t.TempDir()

	// Create a git repo
	repoDir := filepath.Join(workspace, "my-project")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Create a non-git directory (should be ignored)
	nonGitDir := filepath.Join(workspace, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file (should be ignored)
	if err := os.WriteFile(filepath.Join(workspace, "some-file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := New(workspace, nil)
	projects := scanner.Scan()

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	p := projects[0]
	if p.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", p.Name)
	}
	if p.Path != repoDir {
		t.Errorf("expected path %q, got %q", repoDir, p.Path)
	}
	if p.HasChief {
		t.Error("expected has_chief to be false")
	}
	if p.Branch == "" {
		t.Error("expected branch to be set")
	}
	if p.Commit.Hash == "" {
		t.Error("expected commit hash to be set")
	}
	if p.Commit.Message != "initial commit" {
		t.Errorf("expected commit message 'initial commit', got %q", p.Commit.Message)
	}
	if p.Commit.Author != "Test User" {
		t.Errorf("expected commit author 'Test User', got %q", p.Commit.Author)
	}
	if p.Commit.Timestamp == "" {
		t.Error("expected commit timestamp to be set")
	}
}

func TestScan_DetectsChiefDirectory(t *testing.T) {
	workspace := t.TempDir()

	repoDir := filepath.Join(workspace, "chief-project")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Create .chief/ directory
	if err := os.MkdirAll(filepath.Join(repoDir, ".chief", "prds"), 0o755); err != nil {
		t.Fatal(err)
	}

	scanner := New(workspace, nil)
	projects := scanner.Scan()

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if !projects[0].HasChief {
		t.Error("expected has_chief to be true")
	}
}

func TestScan_DiscoversPRDs(t *testing.T) {
	workspace := t.TempDir()

	repoDir := filepath.Join(workspace, "prd-project")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Create .chief/prds/my-feature/prd.json
	prdDir := filepath.Join(repoDir, ".chief", "prds", "my-feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdData := map[string]interface{}{
		"project": "My Feature",
		"userStories": []map[string]interface{}{
			{"id": "US-001", "passes": true},
			{"id": "US-002", "passes": false},
			{"id": "US-003", "passes": true},
		},
	}
	data, _ := json.Marshal(prdData)
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := New(workspace, nil)
	projects := scanner.Scan()

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	p := projects[0]
	if len(p.PRDs) != 1 {
		t.Fatalf("expected 1 PRD, got %d", len(p.PRDs))
	}

	prd := p.PRDs[0]
	if prd.ID != "my-feature" {
		t.Errorf("expected PRD ID 'my-feature', got %q", prd.ID)
	}
	if prd.Name != "My Feature" {
		t.Errorf("expected PRD name 'My Feature', got %q", prd.Name)
	}
	if prd.StoryCount != 3 {
		t.Errorf("expected 3 stories, got %d", prd.StoryCount)
	}
	if prd.CompletionStatus != "2/3" {
		t.Errorf("expected completion '2/3', got %q", prd.CompletionStatus)
	}
}

func TestScan_MultipleProjects(t *testing.T) {
	workspace := t.TempDir()

	for _, name := range []string{"alpha", "beta", "gamma"} {
		dir := filepath.Join(workspace, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		initGitRepo(t, dir)
	}

	scanner := New(workspace, nil)
	projects := scanner.Scan()

	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}

	names := make(map[string]bool)
	for _, p := range projects {
		names[p.Name] = true
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !names[name] {
			t.Errorf("expected project %q to be discovered", name)
		}
	}
}

func TestScan_EmptyWorkspace(t *testing.T) {
	workspace := t.TempDir()

	scanner := New(workspace, nil)
	projects := scanner.Scan()

	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestScan_PermissionError(t *testing.T) {
	// Skip if running as root (permissions are not enforced)
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	workspace := t.TempDir()

	// Create a directory with .git inside, then remove traverse permission on parent
	// so os.Stat on .git fails with permission denied
	restrictedDir := filepath.Join(workspace, "restricted")
	if err := os.MkdirAll(filepath.Join(restrictedDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(restrictedDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(restrictedDir, 0o755)
	})

	// Create a normal git repo too
	goodDir := filepath.Join(workspace, "good-project")
	if err := os.MkdirAll(goodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, goodDir)

	scanner := New(workspace, nil)
	projects := scanner.Scan()

	// Should still discover the good project even if restricted one has issues
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "good-project" {
		t.Errorf("expected 'good-project', got %q", projects[0].Name)
	}
}

func TestScanAndUpdate_DetectsChanges(t *testing.T) {
	workspace := t.TempDir()

	scanner := New(workspace, nil)

	// First scan: empty
	changed := scanner.ScanAndUpdate()
	if changed {
		t.Error("expected no change on first scan of empty workspace")
	}

	// Add a project
	repoDir := filepath.Join(workspace, "new-project")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Second scan: should detect the new project
	changed = scanner.ScanAndUpdate()
	if !changed {
		t.Error("expected change after adding a project")
	}

	projects := scanner.Projects()
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	// Third scan: no changes
	changed = scanner.ScanAndUpdate()
	if changed {
		t.Error("expected no change on repeat scan")
	}
}

func TestScanAndUpdate_DetectsRemoval(t *testing.T) {
	workspace := t.TempDir()

	repoDir := filepath.Join(workspace, "removable")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	scanner := New(workspace, nil)
	scanner.ScanAndUpdate()

	if len(scanner.Projects()) != 1 {
		t.Fatal("expected 1 project initially")
	}

	// Remove the project
	if err := os.RemoveAll(repoDir); err != nil {
		t.Fatal(err)
	}

	changed := scanner.ScanAndUpdate()
	if !changed {
		t.Error("expected change after removing project")
	}
	if len(scanner.Projects()) != 0 {
		t.Error("expected 0 projects after removal")
	}
}

func TestRun_SendsProjectListOnChange(t *testing.T) {
	workspace := t.TempDir()

	sender := &testSender{}

	// Create a project before starting the scanner
	repoDir := filepath.Join(workspace, "starter")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	scanner := New(workspace, sender)
	scanner.interval = 100 * time.Millisecond // Speed up for testing

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run scanner in background
	go scanner.Run(ctx)

	// Wait for initial project_list message
	raw, ok := sender.waitForType(ws.TypeProjectList, 5*time.Second)
	if !ok {
		t.Fatal("timed out waiting for initial project_list message")
	}

	var first ws.ProjectListMessage
	if err := json.Unmarshal(raw, &first); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(first.Projects) != 1 {
		t.Errorf("expected 1 project in initial scan, got %d", len(first.Projects))
	} else if first.Projects[0].Name != "starter" {
		t.Errorf("expected project name 'starter', got %q", first.Projects[0].Name)
	}

	// Add another project
	newDir := filepath.Join(workspace, "newcomer")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, newDir)

	// Wait for periodic scan to detect the new project
	deadline := time.After(5 * time.Second)
	for {
		msgs := sender.getMessages()
		for _, raw := range msgs {
			var msg ws.ProjectListMessage
			if json.Unmarshal(raw, &msg) == nil && msg.Type == ws.TypeProjectList && len(msg.Projects) == 2 {
				return // Success
			}
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for updated project_list message")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	workspace := t.TempDir()

	scanner := New(workspace, nil)
	scanner.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scanner.Run(ctx)
		close(done)
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Good, it stopped
	case <-time.After(2 * time.Second):
		t.Fatal("scanner did not stop after context cancel")
	}
}

func TestProjectsEqual(t *testing.T) {
	a := []ws.ProjectSummary{
		{Name: "proj1", Path: "/a/proj1", Branch: "main", Commit: ws.CommitInfo{Hash: "abc"}},
		{Name: "proj2", Path: "/a/proj2", Branch: "dev", Commit: ws.CommitInfo{Hash: "def"}},
	}
	b := []ws.ProjectSummary{
		{Name: "proj1", Path: "/a/proj1", Branch: "main", Commit: ws.CommitInfo{Hash: "abc"}},
		{Name: "proj2", Path: "/a/proj2", Branch: "dev", Commit: ws.CommitInfo{Hash: "def"}},
	}

	if !projectsEqual(a, b) {
		t.Error("expected equal project lists to be equal")
	}

	// Change a commit hash
	b[1].Commit.Hash = "changed"
	if projectsEqual(a, b) {
		t.Error("expected project lists with different commit hashes to be unequal")
	}

	// Different lengths
	if projectsEqual(a, a[:1]) {
		t.Error("expected project lists of different lengths to be unequal")
	}

	// Both nil/empty
	if !projectsEqual(nil, nil) {
		t.Error("expected two nil lists to be equal")
	}
	if !projectsEqual(nil, []ws.ProjectSummary{}) {
		t.Error("expected nil and empty to be equal")
	}
}

func TestScan_GitBranch(t *testing.T) {
	workspace := t.TempDir()

	repoDir := filepath.Join(workspace, "branched")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Create and switch to a feature branch
	cmd := exec.Command("git", "checkout", "-b", "feature/test")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout failed: %v\n%s", err, out)
	}

	scanner := New(workspace, nil)
	projects := scanner.Scan()

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Branch != "feature/test" {
		t.Errorf("expected branch 'feature/test', got %q", projects[0].Branch)
	}
}
