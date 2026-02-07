package tui

import (
	"testing"
)

func TestBranchWarningProtectedBranch(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetSize(80, 24)
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogProtectedBranch)
	bw.Reset()

	// Should have 4 options: worktree+branch, branch only, continue on main, cancel
	if len(bw.options) != 4 {
		t.Fatalf("expected 4 options for protected branch, got %d", len(bw.options))
	}

	// First option should be "Create worktree + branch" (recommended)
	if bw.options[0].option != BranchOptionCreateWorktree {
		t.Errorf("expected first option to be CreateWorktree, got %v", bw.options[0].option)
	}
	if !bw.options[0].recommended {
		t.Error("expected first option to be recommended")
	}

	// Default selection should be first option
	if bw.GetSelectedOption() != BranchOptionCreateWorktree {
		t.Errorf("expected default selection to be CreateWorktree, got %v", bw.GetSelectedOption())
	}

	// Second option should be "Create branch only"
	if bw.options[1].option != BranchOptionCreateBranch {
		t.Errorf("expected second option to be CreateBranch, got %v", bw.options[1].option)
	}

	// Third option should be "Continue on main"
	if bw.options[2].option != BranchOptionContinue {
		t.Errorf("expected third option to be Continue, got %v", bw.options[2].option)
	}

	// Fourth option should be Cancel
	if bw.options[3].option != BranchOptionCancel {
		t.Errorf("expected fourth option to be Cancel, got %v", bw.options[3].option)
	}
}

func TestBranchWarningAnotherPRDRunning(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetSize(80, 24)
	bw.SetContext("feature/x", "payments", ".chief/worktrees/payments/")
	bw.SetDialogContext(DialogAnotherPRDRunning)
	bw.Reset()

	// Should have 3 options: create worktree, run in same dir, cancel
	if len(bw.options) != 3 {
		t.Fatalf("expected 3 options for another PRD running, got %d", len(bw.options))
	}

	if bw.options[0].option != BranchOptionCreateWorktree {
		t.Errorf("expected first option to be CreateWorktree, got %v", bw.options[0].option)
	}
	if !bw.options[0].recommended {
		t.Error("expected first option to be recommended")
	}

	if bw.options[1].option != BranchOptionContinue {
		t.Errorf("expected second option to be Continue, got %v", bw.options[1].option)
	}

	if bw.options[2].option != BranchOptionCancel {
		t.Errorf("expected third option to be Cancel, got %v", bw.options[2].option)
	}
}

func TestBranchWarningNoConflicts(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetSize(80, 24)
	bw.SetContext("feature/x", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogNoConflicts)
	bw.Reset()

	// Should have 3 options: run in current dir, create worktree+branch, cancel
	if len(bw.options) != 3 {
		t.Fatalf("expected 3 options for no conflicts, got %d", len(bw.options))
	}

	// First option should be "Run in current directory" (recommended)
	if bw.options[0].option != BranchOptionContinue {
		t.Errorf("expected first option to be Continue (current dir), got %v", bw.options[0].option)
	}
	if !bw.options[0].recommended {
		t.Error("expected first option to be recommended")
	}

	// Second option should be "Create worktree + branch"
	if bw.options[1].option != BranchOptionCreateWorktree {
		t.Errorf("expected second option to be CreateWorktree, got %v", bw.options[1].option)
	}

	// Third option should be Cancel
	if bw.options[2].option != BranchOptionCancel {
		t.Errorf("expected third option to be Cancel, got %v", bw.options[2].option)
	}
}

func TestBranchWarningNavigation(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetSize(80, 24)
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogProtectedBranch)
	bw.Reset()

	// Start at index 0
	if bw.selectedIndex != 0 {
		t.Fatalf("expected initial index 0, got %d", bw.selectedIndex)
	}

	// Move down
	bw.MoveDown()
	if bw.selectedIndex != 1 {
		t.Errorf("expected index 1 after MoveDown, got %d", bw.selectedIndex)
	}

	// Move down to the end
	bw.MoveDown()
	bw.MoveDown()
	if bw.selectedIndex != 3 {
		t.Errorf("expected index 3, got %d", bw.selectedIndex)
	}

	// Can't go past the end
	bw.MoveDown()
	if bw.selectedIndex != 3 {
		t.Errorf("expected index to stay at 3, got %d", bw.selectedIndex)
	}

	// Move up
	bw.MoveUp()
	if bw.selectedIndex != 2 {
		t.Errorf("expected index 2 after MoveUp, got %d", bw.selectedIndex)
	}

	// Move up to the start
	bw.MoveUp()
	bw.MoveUp()
	if bw.selectedIndex != 0 {
		t.Errorf("expected index 0, got %d", bw.selectedIndex)
	}

	// Can't go past the start
	bw.MoveUp()
	if bw.selectedIndex != 0 {
		t.Errorf("expected index to stay at 0, got %d", bw.selectedIndex)
	}
}

func TestBranchWarningBranchEdit(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetSize(80, 24)
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogProtectedBranch)
	bw.Reset()

	// Default branch name
	if bw.GetSuggestedBranch() != "chief/auth" {
		t.Errorf("expected branch 'chief/auth', got %q", bw.GetSuggestedBranch())
	}

	// Enter edit mode
	bw.StartEditMode()
	if !bw.IsEditMode() {
		t.Error("expected edit mode to be true")
	}

	// Delete and add chars
	bw.DeleteInputChar()
	bw.DeleteInputChar()
	bw.DeleteInputChar()
	bw.DeleteInputChar()
	bw.AddInputChar('m')
	bw.AddInputChar('y')
	bw.AddInputChar('-')
	bw.AddInputChar('p')
	bw.AddInputChar('r')
	bw.AddInputChar('d')
	if bw.GetSuggestedBranch() != "chief/my-prd" {
		t.Errorf("expected 'chief/my-prd', got %q", bw.GetSuggestedBranch())
	}

	// Invalid characters should be rejected
	bw.AddInputChar(' ')
	bw.AddInputChar('!')
	if bw.GetSuggestedBranch() != "chief/my-prd" {
		t.Errorf("expected 'chief/my-prd' (unchanged), got %q", bw.GetSuggestedBranch())
	}

	// Cancel edit mode
	bw.CancelEditMode()
	if bw.IsEditMode() {
		t.Error("expected edit mode to be false")
	}
}

func TestBranchWarningPathHints(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetSize(80, 24)
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")
	bw.SetDialogContext(DialogProtectedBranch)

	// Check that options have correct path hints
	if bw.options[0].hint != ".chief/worktrees/auth/" {
		t.Errorf("expected worktree path hint, got %q", bw.options[0].hint)
	}
	if bw.options[1].hint != "./ (current directory)" {
		t.Errorf("expected current dir hint for branch only, got %q", bw.options[1].hint)
	}
	if bw.options[2].hint != "./ (current directory)" {
		t.Errorf("expected current dir hint for continue, got %q", bw.options[2].hint)
	}
}

func TestBranchWarningRender(t *testing.T) {
	// Test that Render doesn't panic for each context
	contexts := []DialogContext{DialogProtectedBranch, DialogAnotherPRDRunning, DialogNoConflicts}
	for _, ctx := range contexts {
		bw := NewBranchWarning()
		bw.SetSize(80, 24)
		bw.SetContext("main", "auth", ".chief/worktrees/auth/")
		bw.SetDialogContext(ctx)
		bw.Reset()

		output := bw.Render()
		if output == "" {
			t.Errorf("expected non-empty render for context %d", ctx)
		}
	}
}

func TestBranchWarningGetDialogContext(t *testing.T) {
	bw := NewBranchWarning()
	bw.SetContext("main", "auth", ".chief/worktrees/auth/")

	bw.SetDialogContext(DialogProtectedBranch)
	if bw.GetDialogContext() != DialogProtectedBranch {
		t.Error("expected DialogProtectedBranch")
	}

	bw.SetDialogContext(DialogAnotherPRDRunning)
	if bw.GetDialogContext() != DialogAnotherPRDRunning {
		t.Error("expected DialogAnotherPRDRunning")
	}

	bw.SetDialogContext(DialogNoConflicts)
	if bw.GetDialogContext() != DialogNoConflicts {
		t.Error("expected DialogNoConflicts")
	}
}
