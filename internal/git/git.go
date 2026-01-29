// Package git provides Git utility functions for Chief.
package git

import (
	"os/exec"
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
