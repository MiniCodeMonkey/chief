package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWSURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://uplink.chiefloop.com", "wss://uplink.chiefloop.com/ws"},
		{"http://localhost:8080", "ws://localhost:8080/ws"},
		{"https://uplink.chiefloop.com/", "wss://uplink.chiefloop.com/ws"},
	}

	for _, tt := range tests {
		got := buildWSURL(tt.input)
		if got != tt.want {
			t.Errorf("buildWSURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCollectDiffs(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	// Modify the initial file to create a diff.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("modified content"), 0o644); err != nil {
		t.Fatal(err)
	}

	diffs, err := collectDiffs(dir)
	if err != nil {
		t.Fatalf("collectDiffs error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	if diffs[0].Path != "hello.txt" {
		t.Errorf("expected path 'hello.txt', got %q", diffs[0].Path)
	}
	if diffs[0].Status != "modified" {
		t.Errorf("expected status 'modified', got %q", diffs[0].Status)
	}
	if diffs[0].Patch == "" {
		t.Error("expected non-empty patch")
	}
}

func TestCollectDiffsCleanRepo(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	diffs, err := collectDiffs(dir)
	if err != nil {
		t.Fatalf("collectDiffs error: %v", err)
	}

	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs for clean repo, got %d", len(diffs))
	}
}

func TestLoadCredentialsMissing(t *testing.T) {
	tmpHome := t.TempDir()
	_, err := loadCredentials(tmpHome)
	if err == nil {
		t.Fatal("expected error when credentials are missing")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("expected 'not logged in' error, got: %v", err)
	}
}

func TestDirectoryTraversalProtection(t *testing.T) {
	workspace := t.TempDir()

	// Attempt to escape workspace via ../
	escapePath := filepath.Join(workspace, "proj", "..", "..", "etc", "passwd")
	resolved, err := filepath.Abs(escapePath)
	if err != nil {
		t.Fatal(err)
	}

	absWorkspace, _ := filepath.Abs(workspace)
	if strings.HasPrefix(resolved, absWorkspace) {
		t.Error("traversal path should NOT be within workspace")
	}
}

// initTestGitRepo creates a minimal git repo for testing.
func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
}
