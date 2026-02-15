package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temporary git repository with an initial commit and returns its path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "main"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup command %v failed: %s", args, string(out))
		}
	}

	// Create an initial commit so branches can be created
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s", string(out))
	}
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s", string(out))
	}

	return dir
}

func TestGetDefaultBranch(t *testing.T) {
	t.Run("detects main branch", func(t *testing.T) {
		dir := initTestRepo(t)
		branch, err := GetDefaultBranch(dir)
		if err != nil {
			t.Fatalf("GetDefaultBranch() error = %v", err)
		}
		if branch != "main" {
			t.Errorf("GetDefaultBranch() = %q, want %q", branch, "main")
		}
	})

	t.Run("detects master branch", func(t *testing.T) {
		dir := t.TempDir()
		cmds := [][]string{
			{"git", "init"},
			{"git", "config", "user.email", "test@test.com"},
			{"git", "config", "user.name", "Test"},
			{"git", "checkout", "-b", "master"},
		}
		for _, args := range cmds {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("setup command %v failed: %s", args, string(out))
			}
		}
		readme := filepath.Join(dir, "README.md")
		if err := os.WriteFile(readme, []byte("# Test\n"), 0644); err != nil {
			t.Fatalf("failed to create README: %v", err)
		}
		cmd := exec.Command("git", "add", ".")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git add failed: %s", string(out))
		}
		cmd = exec.Command("git", "commit", "-m", "initial commit")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit failed: %s", string(out))
		}

		branch, err := GetDefaultBranch(dir)
		if err != nil {
			t.Fatalf("GetDefaultBranch() error = %v", err)
		}
		if branch != "master" {
			t.Errorf("GetDefaultBranch() = %q, want %q", branch, "master")
		}
	})
}

func TestCreateWorktree(t *testing.T) {
	t.Run("creates worktree and branch", func(t *testing.T) {
		dir := initTestRepo(t)
		wtPath := filepath.Join(dir, "worktrees", "test-prd")

		err := CreateWorktree(dir, wtPath, "chief/test-prd")
		if err != nil {
			t.Fatalf("CreateWorktree() error = %v", err)
		}

		// Verify worktree exists and is on the right branch
		branch, err := GetCurrentBranch(wtPath)
		if err != nil {
			t.Fatalf("GetCurrentBranch() error = %v", err)
		}
		if branch != "chief/test-prd" {
			t.Errorf("branch = %q, want %q", branch, "chief/test-prd")
		}
	})

	t.Run("reuses existing valid worktree", func(t *testing.T) {
		dir := initTestRepo(t)
		wtPath := filepath.Join(dir, "worktrees", "test-prd")

		// Create worktree first time
		if err := CreateWorktree(dir, wtPath, "chief/test-prd"); err != nil {
			t.Fatalf("first CreateWorktree() error = %v", err)
		}

		// Create a file in the worktree to verify it's reused (not recreated)
		marker := filepath.Join(wtPath, "marker.txt")
		if err := os.WriteFile(marker, []byte("marker"), 0644); err != nil {
			t.Fatalf("failed to create marker: %v", err)
		}

		// Create again - should reuse
		if err := CreateWorktree(dir, wtPath, "chief/test-prd"); err != nil {
			t.Fatalf("second CreateWorktree() error = %v", err)
		}

		// Marker should still exist
		if _, err := os.Stat(marker); err != nil {
			t.Error("marker file was removed - worktree was not reused")
		}
	})

	t.Run("recreates stale worktree with wrong branch", func(t *testing.T) {
		dir := initTestRepo(t)
		wtPath := filepath.Join(dir, "worktrees", "test-prd")

		// Create worktree with one branch
		if err := CreateWorktree(dir, wtPath, "chief/branch-a"); err != nil {
			t.Fatalf("first CreateWorktree() error = %v", err)
		}

		// Create again with a different branch - should remove and recreate
		if err := CreateWorktree(dir, wtPath, "chief/branch-b"); err != nil {
			t.Fatalf("second CreateWorktree() error = %v", err)
		}

		branch, err := GetCurrentBranch(wtPath)
		if err != nil {
			t.Fatalf("GetCurrentBranch() error = %v", err)
		}
		if branch != "chief/branch-b" {
			t.Errorf("branch = %q, want %q", branch, "chief/branch-b")
		}
	})
}

func TestRemoveWorktree(t *testing.T) {
	t.Run("removes existing worktree", func(t *testing.T) {
		dir := initTestRepo(t)
		wtPath := filepath.Join(dir, "worktrees", "test-prd")

		if err := CreateWorktree(dir, wtPath, "chief/test-prd"); err != nil {
			t.Fatalf("CreateWorktree() error = %v", err)
		}

		err := RemoveWorktree(dir, wtPath)
		if err != nil {
			t.Fatalf("RemoveWorktree() error = %v", err)
		}

		// Verify the directory is gone
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Error("worktree directory still exists after removal")
		}
	})
}

func TestListWorktrees(t *testing.T) {
	t.Run("lists worktrees including main", func(t *testing.T) {
		dir := initTestRepo(t)
		wtPath := filepath.Join(dir, "worktrees", "test-prd")

		if err := CreateWorktree(dir, wtPath, "chief/test-prd"); err != nil {
			t.Fatalf("CreateWorktree() error = %v", err)
		}

		worktrees, err := ListWorktrees(dir)
		if err != nil {
			t.Fatalf("ListWorktrees() error = %v", err)
		}

		if len(worktrees) < 2 {
			t.Fatalf("expected at least 2 worktrees, got %d", len(worktrees))
		}

		// Find our worktree
		found := false
		for _, wt := range worktrees {
			if wt.Branch == "chief/test-prd" {
				found = true
				if wt.HEAD == "" {
					t.Error("worktree HEAD is empty")
				}
			}
		}
		if !found {
			t.Error("worktree with branch chief/test-prd not found in list")
		}
	})
}

func TestIsWorktree(t *testing.T) {
	t.Run("returns true for valid worktree", func(t *testing.T) {
		dir := initTestRepo(t)
		wtPath := filepath.Join(dir, "worktrees", "test-prd")

		if err := CreateWorktree(dir, wtPath, "chief/test-prd"); err != nil {
			t.Fatalf("CreateWorktree() error = %v", err)
		}

		if !IsWorktree(wtPath) {
			t.Error("IsWorktree() = false, want true")
		}
	})

	t.Run("returns false for non-existent path", func(t *testing.T) {
		if IsWorktree("/nonexistent/path") {
			t.Error("IsWorktree() = true for non-existent path")
		}
	})

	t.Run("returns false for plain directory", func(t *testing.T) {
		dir := t.TempDir()
		if IsWorktree(dir) {
			t.Error("IsWorktree() = true for plain directory")
		}
	})
}

func TestWorktreePathForPRD(t *testing.T) {
	result := WorktreePathForPRD("/home/user/project", "auth")
	expected := filepath.Join("/home/user/project", ".chief", "worktrees", "auth")
	if result != expected {
		t.Errorf("WorktreePathForPRD() = %q, want %q", result, expected)
	}
}

func TestPruneWorktrees(t *testing.T) {
	t.Run("prune succeeds on clean repo", func(t *testing.T) {
		dir := initTestRepo(t)
		err := PruneWorktrees(dir)
		if err != nil {
			t.Fatalf("PruneWorktrees() error = %v", err)
		}
	})
}

func TestMergeBranch(t *testing.T) {
	t.Run("fast-forward merge succeeds", func(t *testing.T) {
		dir := initTestRepo(t)

		// Create a branch with a commit
		cmd := exec.Command("git", "checkout", "-b", "feature")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("checkout failed: %s", string(out))
		}

		featureFile := filepath.Join(dir, "feature.txt")
		if err := os.WriteFile(featureFile, []byte("feature\n"), 0644); err != nil {
			t.Fatalf("failed to create feature file: %v", err)
		}
		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git add failed: %s", string(out))
		}
		cmd = exec.Command("git", "commit", "-m", "add feature")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit failed: %s", string(out))
		}

		// Switch back to main and merge
		cmd = exec.Command("git", "checkout", "main")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("checkout main failed: %s", string(out))
		}

		conflicts, err := MergeBranch(dir, "feature")
		if err != nil {
			t.Fatalf("MergeBranch() error = %v", err)
		}
		if len(conflicts) > 0 {
			t.Errorf("expected no conflicts, got %v", conflicts)
		}

		// Verify feature file exists on main
		if _, err := os.Stat(featureFile); err != nil {
			t.Error("feature.txt not present after merge")
		}
	})

	t.Run("merge conflict returns conflicting files", func(t *testing.T) {
		dir := initTestRepo(t)

		// Create conflicting changes on two branches
		conflictFile := filepath.Join(dir, "conflict.txt")
		if err := os.WriteFile(conflictFile, []byte("main content\n"), 0644); err != nil {
			t.Fatalf("failed to create conflict file: %v", err)
		}
		cmd := exec.Command("git", "add", ".")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git add failed: %s", string(out))
		}
		cmd = exec.Command("git", "commit", "-m", "main change")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit failed: %s", string(out))
		}

		// Create feature branch from parent commit
		cmd = exec.Command("git", "checkout", "-b", "feature", "HEAD~1")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("checkout failed: %s", string(out))
		}
		if err := os.WriteFile(conflictFile, []byte("feature content\n"), 0644); err != nil {
			t.Fatalf("failed to create conflict file: %v", err)
		}
		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git add failed: %s", string(out))
		}
		cmd = exec.Command("git", "commit", "-m", "feature change")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit failed: %s", string(out))
		}

		// Switch to main and try to merge
		cmd = exec.Command("git", "checkout", "main")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("checkout main failed: %s", string(out))
		}

		conflicts, err := MergeBranch(dir, "feature")
		if err == nil {
			t.Fatal("MergeBranch() expected error for conflict, got nil")
		}
		if len(conflicts) == 0 {
			t.Fatal("expected conflict files, got none")
		}

		foundConflict := false
		for _, f := range conflicts {
			if strings.Contains(f, "conflict.txt") {
				foundConflict = true
			}
		}
		if !foundConflict {
			t.Errorf("expected conflict.txt in conflicts, got %v", conflicts)
		}

		// Verify the merge was aborted (clean state)
		cmd = exec.Command("git", "status", "--porcelain")
		cmd.Dir = dir
		output, _ := cmd.Output()
		if strings.TrimSpace(string(output)) != "" {
			t.Errorf("expected clean working tree after merge abort, got: %s", string(output))
		}
	})
}

func TestDetectOrphanedWorktrees(t *testing.T) {
	t.Run("returns nil when worktrees directory does not exist", func(t *testing.T) {
		dir := t.TempDir()
		result := DetectOrphanedWorktrees(dir)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("returns empty map when worktrees directory is empty", func(t *testing.T) {
		dir := t.TempDir()
		worktreesDir := filepath.Join(dir, ".chief", "worktrees")
		if err := os.MkdirAll(worktreesDir, 0755); err != nil {
			t.Fatalf("failed to create worktrees dir: %v", err)
		}
		result := DetectOrphanedWorktrees(dir)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("detects worktree directories on disk", func(t *testing.T) {
		dir := t.TempDir()
		worktreesDir := filepath.Join(dir, ".chief", "worktrees")

		// Create some worktree directories
		for _, name := range []string{"auth", "payments"} {
			if err := os.MkdirAll(filepath.Join(worktreesDir, name), 0755); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}
		}

		result := DetectOrphanedWorktrees(dir)
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d: %v", len(result), result)
		}

		authPath, ok := result["auth"]
		if !ok {
			t.Error("expected 'auth' in result")
		}
		if authPath != filepath.Join(worktreesDir, "auth") {
			t.Errorf("expected auth path %q, got %q", filepath.Join(worktreesDir, "auth"), authPath)
		}

		paymentsPath, ok := result["payments"]
		if !ok {
			t.Error("expected 'payments' in result")
		}
		if paymentsPath != filepath.Join(worktreesDir, "payments") {
			t.Errorf("expected payments path %q, got %q", filepath.Join(worktreesDir, "payments"), paymentsPath)
		}
	})

	t.Run("ignores files in worktrees directory", func(t *testing.T) {
		dir := t.TempDir()
		worktreesDir := filepath.Join(dir, ".chief", "worktrees")
		if err := os.MkdirAll(worktreesDir, 0755); err != nil {
			t.Fatalf("failed to create worktrees dir: %v", err)
		}

		// Create a directory and a file
		if err := os.MkdirAll(filepath.Join(worktreesDir, "auth"), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(worktreesDir, "stale-file.txt"), []byte("junk"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		result := DetectOrphanedWorktrees(dir)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry (only dirs), got %d: %v", len(result), result)
		}
		if _, ok := result["auth"]; !ok {
			t.Error("expected 'auth' in result")
		}
	})
}
