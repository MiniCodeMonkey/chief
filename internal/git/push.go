package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/internal/prd"
)

// CheckGHCLI validates that the GitHub CLI is installed and authenticated.
func CheckGHCLI() (installed bool, authenticated bool, err error) {
	// Check if gh is installed
	_, err = exec.LookPath("gh")
	if err != nil {
		return false, false, nil
	}

	// Check if gh is authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return true, false, nil
	}

	return true, true, nil
}

// PushBranch pushes the branch to origin.
func PushBranch(dir, branch string) error {
	cmd := exec.Command("git", "push", "-u", "origin", branch)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push branch: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// CreatePR creates a pull request via `gh pr create` and returns the PR URL.
func CreatePR(dir, branch, title, body string) (string, error) {
	cmd := exec.Command("gh", "pr", "create",
		"--head", branch,
		"--title", title,
		"--body", body,
	)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create PR: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// PRTitleFromPRD generates a conventional-commits title for a PR.
// Format: feat(<prd-name>): <project name>
func PRTitleFromPRD(prdName string, p *prd.PRD) string {
	return fmt.Sprintf("feat(%s): %s", prdName, p.Project)
}

// PRBodyFromPRD generates a PR body with a summary and list of completed stories.
func PRBodyFromPRD(p *prd.PRD) string {
	var b strings.Builder

	b.WriteString("## Summary\n\n")
	b.WriteString(p.Description)
	b.WriteString("\n\n")

	b.WriteString("## Changes\n\n")
	for _, story := range p.UserStories {
		if story.Passes {
			b.WriteString(fmt.Sprintf("- %s: %s\n", story.ID, story.Title))
		}
	}

	return b.String()
}

// DeleteBranch deletes a local branch.
func DeleteBranch(repoDir, branch string) error {
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete branch: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
