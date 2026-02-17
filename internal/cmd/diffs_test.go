package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/engine"
)

// gitCmd runs a git command in the given directory with test-safe env.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// createGitRepoWithStoryCommit creates a git repo with an initial commit
// and a story commit matching the "feat: <storyID> - <title>" pattern.
func createGitRepoWithStoryCommit(t *testing.T, dir, storyID, title string) {
	t.Helper()
	createGitRepo(t, dir)

	// Create a file and commit it with the story commit message
	filePath := filepath.Join(dir, "feature.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc feature() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "feature.go")
	gitCmd(t, dir, "commit", "-m", "feat: "+storyID+" - "+title)
}

func TestGetStoryDiff_Success(t *testing.T) {
	dir := t.TempDir()
	createGitRepoWithStoryCommit(t, dir, "US-001", "Add feature")

	diffText, files, err := getStoryDiff(dir, "US-001")
	if err != nil {
		t.Fatalf("getStoryDiff failed: %v", err)
	}

	if diffText == "" {
		t.Error("expected non-empty diff text")
	}
	if !strings.Contains(diffText, "feature.go") {
		t.Errorf("expected diff to contain 'feature.go', got: %s", diffText)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 changed file, got %d: %v", len(files), files)
	}
	if len(files) > 0 && files[0] != "feature.go" {
		t.Errorf("expected file 'feature.go', got %q", files[0])
	}
}

func TestGetStoryDiff_NoCommitFound(t *testing.T) {
	dir := t.TempDir()
	createGitRepo(t, dir)

	_, _, err := getStoryDiff(dir, "US-999")
	if err == nil {
		t.Fatal("expected error for missing story commit")
	}
	if !strings.Contains(err.Error(), "no commit found") {
		t.Errorf("expected 'no commit found' error, got: %v", err)
	}
}

func TestGetStoryDiff_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	createGitRepo(t, dir)

	// Create multiple files and commit
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "feat: US-002 - Add multiple files")

	diffText, files, err := getStoryDiff(dir, "US-002")
	if err != nil {
		t.Fatalf("getStoryDiff failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 changed files, got %d: %v", len(files), files)
	}

	if !strings.Contains(diffText, "a.go") || !strings.Contains(diffText, "b.go") || !strings.Contains(diffText, "c.go") {
		t.Errorf("expected diff to contain all files, got: %s", diffText)
	}
}

func TestFindStoryCommit_FindsMostRecent(t *testing.T) {
	dir := t.TempDir()
	createGitRepo(t, dir)

	// Create first commit for the story
	if err := os.WriteFile(filepath.Join(dir, "v1.go"), []byte("package v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "feat: US-003 - Initial attempt")

	// Create second commit for the same story (more recent)
	if err := os.WriteFile(filepath.Join(dir, "v2.go"), []byte("package v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "feat: US-003 - Fixed version")

	hash, err := findStoryCommit(dir, "US-003")
	if err != nil {
		t.Fatalf("findStoryCommit failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty commit hash")
	}

	// The most recent commit should be the "Fixed version" one
	// Verify by checking the commit message
	cmd := exec.Command("git", "log", "--format=%s", "-1", hash)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	msg := strings.TrimSpace(string(out))
	if msg != "feat: US-003 - Fixed version" {
		t.Errorf("expected most recent commit, got: %q", msg)
	}
}

func TestSendDiffMessage(t *testing.T) {
	// sendDiffMessage with nil client should not panic
	sendDiffMessage(nil, "project", "prd", "US-001", []string{"a.go"}, "diff text")
}

func TestSendDiffMessage_NilFiles(t *testing.T) {
	// sendDiffMessage with nil files should not panic
	sendDiffMessage(nil, "project", "prd", "US-001", nil, "diff text")
}

func TestRunManager_SendStoryDiff(t *testing.T) {
	eng := engine.New(5)
	defer eng.Shutdown()

	rm := newRunManager(eng, nil) // nil client — just verify no panic

	// Create a temp project with a git repo and story commit
	projectDir := t.TempDir()
	createGitRepoWithStoryCommit(t, projectDir, "US-001", "Test Story")

	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rm.mu.Lock()
	rm.runs["myproject/feature"] = &runInfo{
		project:   "myproject",
		prdID:     "feature",
		prdPath:   prdPath,
		startTime: time.Now(),
		storyID:   "US-001",
	}
	rm.mu.Unlock()

	// Call sendStoryDiff with nil client — should not panic, just log
	info := rm.runs["myproject/feature"]
	rm.sendStoryDiff(info, engine.ManagerEvent{}.Event)
}

func TestRunServe_GetDiff(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a git repo with a story commit
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepoWithStoryCommit(t, projectDir, "US-001", "Add feature")

	prdDir := filepath.Join(projectDir, ".chief", "prds", "feature")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Add feature", "passes": true}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		getDiffReq := map[string]interface{}{
			"type":      "get_diff",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
			"story_id":  "US-001",
		}
		if err := ms.sendCommand(getDiffReq); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

		raw, err := ms.waitForMessageType("diff", 5*time.Second)
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
	if response["type"] != "diff" {
		t.Errorf("expected type 'diff', got %v", response["type"])
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

	// Verify files array
	files, ok := response["files"].([]interface{})
	if !ok {
		t.Fatal("expected files to be an array")
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if len(files) > 0 && files[0] != "feature.go" {
		t.Errorf("expected file 'feature.go', got %v", files[0])
	}

	// Verify diff_text is non-empty and contains the file
	diffText, ok := response["diff_text"].(string)
	if !ok || diffText == "" {
		t.Error("expected non-empty diff_text")
	}
	if !strings.Contains(diffText, "feature.go") {
		t.Errorf("expected diff_text to contain 'feature.go'")
	}
}

func TestRunServe_GetDiffProjectNotFound(t *testing.T) {
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
		getDiffReq := map[string]interface{}{
			"type":      "get_diff",
			"id":        "req-2",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
			"prd_id":    "feature",
			"story_id":  "US-001",
		}
		if err := ms.sendCommand(getDiffReq); err != nil {
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
		t.Fatal("response was not received")
	}
	if response["type"] != "error" {
		t.Errorf("expected type 'error', got %v", response["type"])
	}
	if response["code"] != "PROJECT_NOT_FOUND" {
		t.Errorf("expected code 'PROJECT_NOT_FOUND', got %v", response["code"])
	}
}

func TestRunServe_GetDiffPRDNotFound(t *testing.T) {
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
		getDiffReq := map[string]interface{}{
			"type":      "get_diff",
			"id":        "req-3",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "nonexistent",
			"story_id":  "US-001",
		}
		if err := ms.sendCommand(getDiffReq); err != nil {
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
		t.Fatal("response was not received")
	}
	if response["type"] != "error" {
		t.Errorf("expected type 'error', got %v", response["type"])
	}
	if response["code"] != "PRD_NOT_FOUND" {
		t.Errorf("expected code 'PRD_NOT_FOUND', got %v", response["code"])
	}
}

func TestRunServe_GetDiffNoCommit(t *testing.T) {
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

	prdState := `{"project": "My Feature", "userStories": [{"id": "US-001", "title": "Test", "passes": false}]}`
	if err := os.WriteFile(filepath.Join(prdDir, "prd.json"), []byte(prdState), 0o644); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		getDiffReq := map[string]interface{}{
			"type":      "get_diff",
			"id":        "req-4",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"prd_id":    "feature",
			"story_id":  "US-001",
		}
		if err := ms.sendCommand(getDiffReq); err != nil {
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
		t.Fatal("response was not received")
	}
	if response["type"] != "error" {
		t.Errorf("expected type 'error', got %v", response["type"])
	}
	if response["code"] != "FILESYSTEM_ERROR" {
		t.Errorf("expected code 'FILESYSTEM_ERROR', got %v", response["code"])
	}
}

func TestRunServe_GetDiffs(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	setupServeCredentials(t)

	workspaceDir := filepath.Join(home, "projects")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a git repo with a story commit
	projectDir := filepath.Join(workspaceDir, "myproject")
	createGitRepoWithStoryCommit(t, projectDir, "US-001", "Add feature")

	var response map[string]interface{}
	var mu sync.Mutex

	err := serveTestHelper(t, workspaceDir, func(ms *mockUplinkServer) {
		// get_diffs does not require prd_id (unlike get_diff)
		req := map[string]interface{}{
			"type":      "get_diffs",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "myproject",
			"story_id":  "US-001",
		}
		if err := ms.sendCommand(req); err != nil {
			t.Errorf("sendCommand failed: %v", err)
			return
		}

		raw, err := ms.waitForMessageType("diffs_response", 5*time.Second)
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
		t.Fatal("diffs_response was not received")
	}
	if response["type"] != "diffs_response" {
		t.Errorf("expected type 'diffs_response', got %v", response["type"])
	}

	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		t.Fatal("expected payload to be an object")
	}
	if payload["project"] != "myproject" {
		t.Errorf("expected project 'myproject', got %v", payload["project"])
	}
	if payload["story_id"] != "US-001" {
		t.Errorf("expected story_id 'US-001', got %v", payload["story_id"])
	}

	files, ok := payload["files"].([]interface{})
	if !ok {
		t.Fatal("expected files to be an array")
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	file := files[0].(map[string]interface{})
	if file["filename"] != "feature.go" {
		t.Errorf("expected filename 'feature.go', got %v", file["filename"])
	}
	if int(file["additions"].(float64)) == 0 {
		t.Error("expected additions > 0")
	}
	if _, ok := file["patch"].(string); !ok || file["patch"] == "" {
		t.Error("expected non-empty patch string")
	}
}

func TestRunServe_GetDiffs_ProjectNotFound(t *testing.T) {
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
			"type":      "get_diffs",
			"id":        "req-1",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"project":   "nonexistent",
			"story_id":  "US-001",
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

func TestParseDiffFiles(t *testing.T) {
	diffText := `diff --git a/main.go b/main.go
index abc..def 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,5 @@
 package main
+import "fmt"
+func hello() { fmt.Println("hi") }
 func main() {}
diff --git a/util.go b/util.go
new file mode 100644
--- /dev/null
+++ b/util.go
@@ -0,0 +1,3 @@
+package main
+func helper() {}
+func other() {}
`

	files := parseDiffFiles(diffText)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	if files[0].Filename != "main.go" {
		t.Errorf("files[0].filename = %q, want %q", files[0].Filename, "main.go")
	}
	if files[0].Additions != 2 {
		t.Errorf("files[0].additions = %d, want 2", files[0].Additions)
	}
	if files[0].Deletions != 0 {
		t.Errorf("files[0].deletions = %d, want 0", files[0].Deletions)
	}

	if files[1].Filename != "util.go" {
		t.Errorf("files[1].filename = %q, want %q", files[1].Filename, "util.go")
	}
	if files[1].Additions != 3 {
		t.Errorf("files[1].additions = %d, want 3", files[1].Additions)
	}
}

func TestParseDiffFiles_Empty(t *testing.T) {
	files := parseDiffFiles("")
	if len(files) != 0 {
		t.Errorf("expected 0 files for empty diff, got %d", len(files))
	}
}
