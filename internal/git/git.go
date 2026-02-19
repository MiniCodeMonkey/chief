// Package git provides Git utility functions for Chief.
package git

import (
	"os/exec"
	"strconv"
	"strings"
)

// GetCurrentBranch returns the current git branch name for a directory.
func GetCurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// IsProtectedBranch returns true if the branch name is main or master.
func IsProtectedBranch(branch string) bool {
	return branch == "main" || branch == "master"
}

// CreateBranch creates a new branch and switches to it.
func CreateBranch(dir, branchName string) error {
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = dir
	return cmd.Run()
}

// BranchExists returns true if a branch with the given name exists.
func BranchExists(dir, branchName string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		// Branch doesn't exist
		return false, nil
	}
	return true, nil
}

// IsGitRepo returns true if the directory is inside a git repository.
func IsGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// CommitCount returns the number of commits on branch that are not on the default branch.
// Returns 0 if the count cannot be determined.
func CommitCount(repoDir, branch string) int {
	defaultBranch, err := GetDefaultBranch(repoDir)
	if err != nil {
		return 0
	}
	cmd := exec.Command("git", "rev-list", "--count", defaultBranch+".."+branch)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return count
}

// GetDiff returns the git diff output for the working directory.
// It shows the diff between the current branch and its merge base with the default branch.
// If on main/master or if merge-base fails, it shows the last few commits' diff.
func GetDiff(dir string) (string, error) {
	branch, err := GetCurrentBranch(dir)
	if err != nil {
		return "", err
	}

	// If on a feature branch, diff against merge-base with main/master
	if !IsProtectedBranch(branch) {
		baseBranch, err := GetDefaultBranch(dir)
		if err == nil && baseBranch != "" {
			mergeBase, err := getMergeBase(dir, baseBranch, "HEAD")
			if err == nil && mergeBase != "" {
				return getDiffOutput(dir, mergeBase, "HEAD")
			}
		}
	}

	// Fallback: show diff of recent commits (last 10)
	return getDiffOutput(dir, "HEAD~10", "HEAD")
}

// GetDiffStats returns a short diffstat summary.
func GetDiffStats(dir string) (string, error) {
	branch, err := GetCurrentBranch(dir)
	if err != nil {
		return "", err
	}

	if !IsProtectedBranch(branch) {
		baseBranch, err := GetDefaultBranch(dir)
		if err == nil && baseBranch != "" {
			mergeBase, err := getMergeBase(dir, baseBranch, "HEAD")
			if err == nil && mergeBase != "" {
				cmd := exec.Command("git", "diff", "--stat", mergeBase, "HEAD")
				cmd.Dir = dir
				output, err := cmd.Output()
				if err != nil {
					return "", err
				}
				return strings.TrimSpace(string(output)), nil
			}
		}
	}

	cmd := exec.Command("git", "diff", "--stat", "HEAD~10", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetDiffForCommit returns the diff for a single commit.
func GetDiffForCommit(dir, commitHash string) (string, error) {
	cmd := exec.Command("git", "diff", commitHash+"~1", commitHash)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		// If commitHash~1 doesn't exist (first commit), diff against empty tree
		cmd = exec.Command("git", "diff", "4b825dc642cb6eb9a060e54bf899d69f82049674", commitHash)
		cmd.Dir = dir
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return string(output), nil
}

// GetDiffStatsForCommit returns the diffstat for a single commit.
func GetDiffStatsForCommit(dir, commitHash string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", commitHash+"~1", commitHash)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("git", "diff", "--stat", "4b825dc642cb6eb9a060e54bf899d69f82049674", commitHash)
		cmd.Dir = dir
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(string(output)), nil
}

// FindCommitForStory searches the git log for a commit matching the given story ID.
// Returns the commit hash if found, empty string otherwise.
func FindCommitForStory(dir, storyID string) (string, error) {
	cmd := exec.Command("git", "log", "--grep="+storyID, "--format=%H", "-1")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(string(output))
	return hash, nil
}

// getMergeBase returns the merge base commit between two refs.
func getMergeBase(dir, ref1, ref2 string) (string, error) {
	cmd := exec.Command("git", "merge-base", ref1, ref2)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getDiffOutput returns the full diff between two refs.
func getDiffOutput(dir, from, to string) (string, error) {
	cmd := exec.Command("git", "diff", from, to)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
