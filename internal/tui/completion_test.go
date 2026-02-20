package tui

import (
	"strings"
	"testing"
)

func TestCompletionScreen_Configure(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 10, "chief/auth", 5, true, 0, nil)

	if cs.PRDName() != "auth" {
		t.Errorf("expected prdName 'auth', got '%s'", cs.PRDName())
	}
	if cs.Branch() != "chief/auth" {
		t.Errorf("expected branch 'chief/auth', got '%s'", cs.Branch())
	}
	if !cs.HasBranch() {
		t.Error("expected HasBranch() to be true")
	}
}

func TestCompletionScreen_NoBranch(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "", 0, false, 0, nil)

	if cs.HasBranch() {
		t.Error("expected HasBranch() to be false when branch is empty")
	}
}

func TestCompletionScreen_RenderHeader(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 10, "chief/auth", 5, true, 0, nil)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "PRD Complete!") {
		t.Error("expected 'PRD Complete!' in render output")
	}
	if !strings.Contains(rendered, "auth") {
		t.Error("expected PRD name 'auth' in render output")
	}
	if !strings.Contains(rendered, "8/10") {
		t.Error("expected '8/10' stories count in render output")
	}
}

func TestCompletionScreen_RenderBranchInfo(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "chief/auth") {
		t.Error("expected branch 'chief/auth' in render output")
	}
	if !strings.Contains(rendered, "5 commits") {
		t.Error("expected '5 commits' in render output")
	}
}

func TestCompletionScreen_RenderSingleCommit(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 1, 1, "chief/auth", 1, false, 0, nil)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "1 commit") {
		t.Error("expected '1 commit' (singular) in render output")
	}
}

func TestCompletionScreen_RenderNoBranch(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "", 0, false, 0, nil)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if strings.Contains(rendered, "Branch:") {
		t.Error("expected no 'Branch:' when no branch is set")
	}
	if strings.Contains(rendered, "commit") {
		t.Error("expected no commit info when no branch is set")
	}
}

func TestCompletionScreen_RenderNoAutoActions(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, false, 0, nil)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "Configure auto-push and PR in settings") {
		t.Error("expected auto-actions hint when hasAutoActions is false")
	}
}

func TestCompletionScreen_RenderWithAutoActions(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if strings.Contains(rendered, "Configure auto-push and PR in settings") {
		t.Error("expected no auto-actions hint when hasAutoActions is true")
	}
}

func TestCompletionScreen_RenderFooterWithBranch(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "m: merge") {
		t.Error("expected 'm: merge' in footer when branch is set")
	}
	if !strings.Contains(rendered, "c: clean") {
		t.Error("expected 'c: clean' in footer when branch is set")
	}
	if !strings.Contains(rendered, "l: switch PRD") {
		t.Error("expected 'l: switch PRD' in footer")
	}
	if !strings.Contains(rendered, "q: quit") {
		t.Error("expected 'q: quit' in footer")
	}
}

func TestCompletionScreen_RenderFooterNoBranch(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "", 0, false, 0, nil)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if strings.Contains(rendered, "m: merge") {
		t.Error("expected no 'm: merge' in footer when no branch is set")
	}
	if strings.Contains(rendered, "c: clean") {
		t.Error("expected no 'c: clean' in footer when no branch is set")
	}
	if !strings.Contains(rendered, "l: switch PRD") {
		t.Error("expected 'l: switch PRD' in footer")
	}
	if !strings.Contains(rendered, "q: quit") {
		t.Error("expected 'q: quit' in footer")
	}
}

func TestCompletionScreen_PushInProgress(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushInProgress()
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "Pushing branch to remote") {
		t.Error("expected 'Pushing branch to remote' when push is in progress")
	}
	if cs.pushState != AutoActionInProgress {
		t.Errorf("expected push state to be AutoActionInProgress, got %d", cs.pushState)
	}
	if !cs.IsAutoActionRunning() {
		t.Error("expected IsAutoActionRunning() to be true when push is in progress")
	}
}

func TestCompletionScreen_PushSuccess(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushSuccess()
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "Pushed branch to remote") {
		t.Error("expected 'Pushed branch to remote' when push succeeded")
	}
	// Should not show the "configure" hint when auto-actions are active
	if strings.Contains(rendered, "Configure auto-push") {
		t.Error("expected no auto-push hint when push is active")
	}
}

func TestCompletionScreen_PushError(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushError("authentication failed")
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "Push failed") {
		t.Error("expected 'Push failed' when push errored")
	}
	if !strings.Contains(rendered, "authentication failed") {
		t.Error("expected error message in render output")
	}
}

func TestCompletionScreen_PRInProgress(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushSuccess()
	cs.SetPRInProgress()
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "Creating pull request") {
		t.Error("expected 'Creating pull request' when PR is in progress")
	}
	if !cs.IsAutoActionRunning() {
		t.Error("expected IsAutoActionRunning() to be true when PR is in progress")
	}
}

func TestCompletionScreen_PRSuccess(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushSuccess()
	cs.SetPRSuccess("https://github.com/org/repo/pull/42", "feat(auth): Authentication")
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "Created PR") {
		t.Error("expected 'Created PR' when PR succeeded")
	}
	if !strings.Contains(rendered, "feat(auth): Authentication") {
		t.Error("expected PR title in render output")
	}
	if !strings.Contains(rendered, "https://github.com/org/repo/pull/42") {
		t.Error("expected PR URL in render output")
	}
	if cs.IsAutoActionRunning() {
		t.Error("expected IsAutoActionRunning() to be false when all actions complete")
	}
}

func TestCompletionScreen_PRError(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushSuccess()
	cs.SetPRError("gh not found, Install: https://cli.github.com")
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "PR creation failed") {
		t.Error("expected 'PR creation failed' when PR errored")
	}
	if !strings.Contains(rendered, "gh not found") {
		t.Error("expected error message in render output")
	}
}

func TestCompletionScreen_ConfigureResetsAutoActions(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushSuccess()
	cs.SetPRSuccess("https://example.com", "title")

	// Reconfigure should reset
	cs.Configure("payments", 3, 5, "chief/payments", 2, false, 0, nil)

	if cs.pushState != AutoActionIdle {
		t.Error("expected push state to be reset after Configure")
	}
	if cs.prState != AutoActionIdle {
		t.Error("expected PR state to be reset after Configure")
	}
	if cs.prURL != "" {
		t.Error("expected prURL to be empty after Configure")
	}
}

func TestCompletionScreen_Tick(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushInProgress()

	initial := cs.spinnerFrame
	cs.Tick()
	if cs.spinnerFrame != initial+1 {
		t.Error("expected spinner frame to advance on Tick()")
	}
}

func TestCompletionScreen_PushErrorNonBlocking(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true, 0, nil)
	cs.SetPushError("network error")
	cs.SetSize(80, 40)

	rendered := cs.Render()
	// Footer should still be present (keybindings remain usable)
	if !strings.Contains(rendered, "m: merge") {
		t.Error("expected footer keybindings to remain usable after push error")
	}
	if !strings.Contains(rendered, "q: quit") {
		t.Error("expected 'q: quit' in footer after error")
	}
}

func TestCenterModal(t *testing.T) {
	modal := "test modal content"
	result := centerModal(modal, 80, 40)

	// Should have top padding and left padding
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatal("expected centered modal to have multiple lines")
	}

	// First lines should be empty (top padding)
	hasTopPadding := false
	for _, line := range lines {
		if line == "" {
			hasTopPadding = true
			break
		}
	}
	if !hasTopPadding {
		t.Error("expected top padding in centered modal")
	}
}
