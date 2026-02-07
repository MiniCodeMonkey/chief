package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree entry.
type Worktree struct {
	Path     string
	Branch   string
	HEAD     string
	Prunable bool
}

// GetDefaultBranch detects the default branch (main or master) for a repository.
func GetDefaultBranch(repoDir string) (string, error) {
	// Try symbolic-ref first (works for repos with remotes)
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		// refs/remotes/origin/main -> main
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fallback: check if main or master branch exists
	for _, branch := range []string{"main", "master"} {
		exists, err := BranchExists(repoDir, branch)
		if err != nil {
			continue
		}
		if exists {
			return branch, nil
		}
	}

	return "", fmt.Errorf("could not detect default branch (tried main, master)")
}

// CreateWorktree creates a branch from the default branch and adds a worktree at the given path.
// If the worktree path already exists and is a valid worktree on the expected branch, it is reused.
// If the worktree path exists but is stale (wrong branch or invalid), it is removed and recreated.
func CreateWorktree(repoDir, worktreePath, branch string) error {
	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to resolve worktree path: %w", err)
	}

	// Check if the path already exists as a worktree
	if IsWorktree(absWorktreePath) {
		// Check if it's on the expected branch
		currentBranch, err := GetCurrentBranch(absWorktreePath)
		if err == nil && currentBranch == branch {
			// Valid worktree on the expected branch, reuse it
			return nil
		}
		// Stale worktree (wrong branch or invalid), remove and recreate
		if err := RemoveWorktree(repoDir, absWorktreePath); err != nil {
			return fmt.Errorf("failed to remove stale worktree: %w", err)
		}
	}

	defaultBranch, err := GetDefaultBranch(repoDir)
	if err != nil {
		return fmt.Errorf("failed to detect default branch: %w", err)
	}

	// Create the branch from the default branch if it doesn't exist
	exists, err := BranchExists(repoDir, branch)
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}
	if !exists {
		cmd := exec.Command("git", "branch", branch, defaultBranch)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create branch %s: %s", branch, strings.TrimSpace(string(out)))
		}
	}

	// Add the worktree
	cmd := exec.Command("git", "worktree", "add", absWorktreePath, branch)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add worktree: %s", strings.TrimSpace(string(out)))
	}

	return nil
}

// RemoveWorktree removes a git worktree at the given path.
func RemoveWorktree(repoDir, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ListWorktrees parses `git worktree list --porcelain` and returns all worktrees.
func ListWorktrees(repoDir string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			// refs/heads/branch-name -> branch-name
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "prunable":
			current.Prunable = true
		case line == "":
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
		}
	}
	// Append last entry if not empty-line terminated
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// IsWorktree checks if a path is a valid git worktree.
func IsWorktree(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	if strings.TrimSpace(string(output)) != "true" {
		return false
	}

	// Verify it's actually a worktree (not the main repo) by checking for .git file
	cmd = exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = path
	commonDir, err := cmd.Output()
	if err != nil {
		return false
	}

	cmd = exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	gitDir, err := cmd.Output()
	if err != nil {
		return false
	}

	// In a worktree, --git-dir differs from --git-common-dir
	// But we also consider the main worktree valid
	_ = commonDir
	_ = gitDir
	return true
}

// WorktreePathForPRD returns the worktree path for a given PRD name.
func WorktreePathForPRD(baseDir, prdName string) string {
	return filepath.Join(baseDir, ".chief", "worktrees", prdName)
}

// PruneWorktrees runs `git worktree prune` to clean up stale worktree tracking.
func PruneWorktrees(repoDir string) error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to prune worktrees: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DetectOrphanedWorktrees scans .chief/worktrees/ and returns a map of PRD name -> absolute worktree path
// for worktrees that exist on disk. The caller is responsible for determining which are orphaned
// (i.e., have no corresponding registered/running PRD).
func DetectOrphanedWorktrees(baseDir string) map[string]string {
	worktreesDir := filepath.Join(baseDir, ".chief", "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return nil
	}

	result := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		absPath := filepath.Join(worktreesDir, entry.Name())
		result[entry.Name()] = absPath
	}
	return result
}

// MergeBranch merges a branch into the current branch, returning conflicting file list on failure.
func MergeBranch(repoDir, branch string) ([]string, error) {
	cmd := exec.Command("git", "merge", branch)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Parse conflicting files from merge output
		conflicts := parseConflicts(repoDir)
		if len(conflicts) > 0 {
			// Abort the merge to leave a clean state
			abortCmd := exec.Command("git", "merge", "--abort")
			abortCmd.Dir = repoDir
			_ = abortCmd.Run()
			return conflicts, fmt.Errorf("merge conflict: %s", strings.TrimSpace(string(out)))
		}
		return nil, fmt.Errorf("merge failed: %s", strings.TrimSpace(string(out)))
	}
	return nil, nil
}

// parseConflicts uses `git diff --name-only --diff-filter=U` to find conflicting files.
func parseConflicts(repoDir string) []string {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var conflicts []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			conflicts = append(conflicts, line)
		}
	}
	return conflicts
}
