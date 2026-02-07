package tui

import (
	"strings"
	"testing"
)

func TestCompletionScreen_Configure(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 10, "chief/auth", 5, true)

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
	cs.Configure("auth", 8, 8, "", 0, false)

	if cs.HasBranch() {
		t.Error("expected HasBranch() to be false when branch is empty")
	}
}

func TestCompletionScreen_RenderHeader(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 10, "chief/auth", 5, true)
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
	cs.Configure("auth", 8, 8, "chief/auth", 5, true)
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
	cs.Configure("auth", 1, 1, "chief/auth", 1, false)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "1 commit on branch") {
		t.Error("expected '1 commit on branch' (singular) in render output")
	}
}

func TestCompletionScreen_RenderNoBranch(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "", 0, false)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if strings.Contains(rendered, "Branch:") {
		t.Error("expected no 'Branch:' when no branch is set")
	}
	if strings.Contains(rendered, "Commits:") {
		t.Error("expected no 'Commits:' when no branch is set")
	}
}

func TestCompletionScreen_RenderNoAutoActions(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, false)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if !strings.Contains(rendered, "Configure auto-push and PR in settings") {
		t.Error("expected auto-actions hint when hasAutoActions is false")
	}
}

func TestCompletionScreen_RenderWithAutoActions(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true)
	cs.SetSize(80, 40)

	rendered := cs.Render()
	if strings.Contains(rendered, "Configure auto-push and PR in settings") {
		t.Error("expected no auto-actions hint when hasAutoActions is true")
	}
}

func TestCompletionScreen_RenderFooterWithBranch(t *testing.T) {
	cs := NewCompletionScreen()
	cs.Configure("auth", 8, 8, "chief/auth", 5, true)
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
	cs.Configure("auth", 8, 8, "", 0, false)
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
