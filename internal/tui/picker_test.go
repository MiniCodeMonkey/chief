package tui

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/minicodemonkey/chief/internal/loop"
)

func TestRenderEntryWithBranchAndWorktree(t *testing.T) {
	p := &PRDPicker{
		basePath:   "/project",
		currentPRD: "",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   8,
				Total:       8,
				LoopState:   loop.LoopStateComplete,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
	}

	result := p.renderEntry(p.entries[0], false, 80)
	if result == "" {
		t.Fatal("expected non-empty render result")
	}
	// Should contain branch name
	if !containsText(result, "chief/auth") {
		t.Errorf("expected branch 'chief/auth' in output, got: %s", result)
	}
	// Should contain worktree path
	if !containsText(result, ".chief/worktrees/auth/") {
		t.Errorf("expected worktree path in output, got: %s", result)
	}
}

func TestRenderEntryNoBranch(t *testing.T) {
	p := &PRDPicker{
		basePath:   "/project",
		currentPRD: "",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 3,
				Total:     8,
				LoopState: loop.LoopStateRunning,
				Iteration: 2,
				Branch:    "",
			},
		},
	}

	result := p.renderEntry(p.entries[0], false, 80)
	if result == "" {
		t.Fatal("expected non-empty render result")
	}
	// Should NOT contain branch brackets
	if containsText(result, "chief/") {
		t.Errorf("expected no branch in output when branch is empty, got: %s", result)
	}
}

func TestRenderEntryNoBranchOmitsCurrentDirectoryLabel(t *testing.T) {
	p := &PRDPicker{
		basePath:   "/project",
		currentPRD: "",
		entries: []PRDEntry{
			{
				Name:      "legacy",
				Completed: 5,
				Total:     5,
				LoopState: loop.LoopStateComplete,
				Branch:    "",
			},
			{
				Name:        "auth",
				Completed:   8,
				Total:       8,
				LoopState:   loop.LoopStateComplete,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
	}

	// Render the entry without a branch — should NOT show "(current directory)" to keep worktrees low-key
	result := p.renderEntry(p.entries[0], false, 80)
	if containsText(result, "(current directory)") {
		t.Errorf("expected no '(current directory)' label for branchless entry, got: %s", result)
	}
}

func TestRenderEntryNarrowTerminalOmitsBranchPath(t *testing.T) {
	p := &PRDPicker{
		basePath:   "/project",
		currentPRD: "",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   8,
				Total:       8,
				LoopState:   loop.LoopStateComplete,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
	}

	// Very narrow width — should not crash and should omit branch/path info
	result := p.renderEntry(p.entries[0], false, 35)
	if result == "" {
		t.Fatal("expected non-empty render result even at narrow width")
	}
	// At 35 chars wide, remaining space (35-32=3) is too small for branch info
	if containsText(result, "chief/auth") {
		t.Errorf("expected branch to be omitted at narrow width, got: %s", result)
	}
}

func TestFormatBranchPathFull(t *testing.T) {
	p := &PRDPicker{}

	result := p.formatBranchPath("chief/auth", ".chief/worktrees/auth/", 50)
	expected := "  chief/auth  .chief/worktrees/auth/"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatBranchPathTruncatesPath(t *testing.T) {
	p := &PRDPicker{}

	result := p.formatBranchPath("chief/auth", ".chief/worktrees/auth/", 30)
	// Should contain branch but path should be truncated with …
	if !containsSubstring(result, "chief/auth") {
		t.Errorf("expected branch in truncated output, got: %s", result)
	}
	runeCount := utf8.RuneCountInString(result)
	if runeCount > 30 {
		t.Errorf("expected result to fit within 30 display chars, got %d: %s", runeCount, result)
	}
}

func TestFormatBranchPathTruncatesBranch(t *testing.T) {
	p := &PRDPicker{}

	// Very small width — only room for branch (truncated)
	result := p.formatBranchPath("chief/very-long-branch-name", ".chief/worktrees/auth/", 15)
	runeCount := utf8.RuneCountInString(result)
	if runeCount > 15 {
		t.Errorf("expected result to fit within 15 display chars, got %d: %s", runeCount, result)
	}
}

func TestWorktreeDisplayPathWithWorktree(t *testing.T) {
	p := &PRDPicker{basePath: "/project"}

	entry := PRDEntry{WorktreeDir: "/project/.chief/worktrees/auth"}
	result := p.worktreeDisplayPath(entry)
	if result != ".chief/worktrees/auth/" {
		t.Errorf("expected '.chief/worktrees/auth/', got %q", result)
	}
}

func TestWorktreeDisplayPathWithoutWorktree(t *testing.T) {
	p := &PRDPicker{basePath: "/project"}

	entry := PRDEntry{WorktreeDir: ""}
	result := p.worktreeDisplayPath(entry)
	if result != "(current directory)" {
		t.Errorf("expected '(current directory)', got %q", result)
	}
}

func TestRenderEntryWithLoadError(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "broken",
				LoadError:   fmt.Errorf("parse error"),
				Branch:      "chief/broken",
				WorktreeDir: "/project/.chief/worktrees/broken",
			},
		},
	}

	result := p.renderEntry(p.entries[0], false, 80)
	// With load error, should show [error] but not branch/worktree info
	if !containsText(result, "error") {
		t.Errorf("expected [error] in output, got: %s", result)
	}
}

func TestCanMergeCompletedWithBranch(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 8,
				Total:     8,
				LoopState: loop.LoopStateComplete,
				Branch:    "chief/auth",
			},
		},
		selectedIndex: 0,
	}
	if !p.CanMerge() {
		t.Error("expected CanMerge() to return true for completed PRD with branch")
	}
}

func TestCanMergeNoBranch(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 8,
				Total:     8,
				LoopState: loop.LoopStateComplete,
				Branch:    "",
			},
		},
		selectedIndex: 0,
	}
	if p.CanMerge() {
		t.Error("expected CanMerge() to return false for completed PRD without branch")
	}
}

func TestCanMergeRunningPRD(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 3,
				Total:     8,
				LoopState: loop.LoopStateRunning,
				Branch:    "chief/auth",
			},
		},
		selectedIndex: 0,
	}
	if p.CanMerge() {
		t.Error("expected CanMerge() to return false for running PRD")
	}
}

func TestCanMergeAllPassedButNotCompleteState(t *testing.T) {
	// All stories pass but loop state is Ready (e.g., not started via loop)
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 5,
				Total:     5,
				LoopState: loop.LoopStateReady,
				Branch:    "chief/auth",
			},
		},
		selectedIndex: 0,
	}
	if !p.CanMerge() {
		t.Error("expected CanMerge() to return true when all stories pass, even if LoopState is Ready")
	}
}

func TestMergeResultSuccessRendering(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		width:    80,
		height:   24,
		entries: []PRDEntry{
			{Name: "auth", Branch: "chief/auth"},
		},
		mergeResult: &MergeResult{
			Success: true,
			Message: "Merged chief/auth into main",
			Branch:  "chief/auth",
		},
	}

	result := p.Render()
	if !containsText(result, "Merge Successful") {
		t.Errorf("expected 'Merge Successful' in success render, got: %s", stripAnsi(result))
	}
	if !containsText(result, "Merged chief/auth into main") {
		t.Errorf("expected merge message in output, got: %s", stripAnsi(result))
	}
	if !containsText(result, "Press any key to continue") {
		t.Errorf("expected dismiss hint in output, got: %s", stripAnsi(result))
	}
}

func TestMergeResultConflictRendering(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		width:    80,
		height:   24,
		entries: []PRDEntry{
			{Name: "auth", Branch: "chief/auth"},
		},
		mergeResult: &MergeResult{
			Success:   false,
			Message:   "Failed to merge chief/auth into current branch",
			Conflicts: []string{"src/auth.go", "src/handler.go"},
			Branch:    "chief/auth",
		},
	}

	result := p.Render()
	if !containsText(result, "Merge Conflict") {
		t.Errorf("expected 'Merge Conflict' in conflict render, got: %s", stripAnsi(result))
	}
	if !containsText(result, "src/auth.go") {
		t.Errorf("expected conflicting file in output, got: %s", stripAnsi(result))
	}
	if !containsText(result, "src/handler.go") {
		t.Errorf("expected conflicting file in output, got: %s", stripAnsi(result))
	}
	if !containsText(result, "git merge chief/auth") {
		t.Errorf("expected manual merge instruction in output, got: %s", stripAnsi(result))
	}
}

func TestMergeResultClearsOnDismiss(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		mergeResult: &MergeResult{
			Success: true,
			Message: "Merged",
			Branch:  "chief/auth",
		},
	}

	if !p.HasMergeResult() {
		t.Error("expected HasMergeResult() to return true")
	}

	p.ClearMergeResult()

	if p.HasMergeResult() {
		t.Error("expected HasMergeResult() to return false after clear")
	}
}

func TestFooterShowsMergeHintForCompletedPRD(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 8,
				Total:     8,
				LoopState: loop.LoopStateComplete,
				Branch:    "chief/auth",
			},
		},
		selectedIndex: 0,
	}

	shortcuts := p.buildFooterShortcuts()
	if !containsSubstring(shortcuts, "m: merge") {
		t.Errorf("expected 'm: merge' in footer for completed PRD with branch, got: %s", shortcuts)
	}
}

func TestFooterHidesMergeHintForRunningPRD(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 3,
				Total:     8,
				LoopState: loop.LoopStateRunning,
				Iteration: 2,
				Branch:    "chief/auth",
			},
		},
		selectedIndex: 0,
	}

	shortcuts := p.buildFooterShortcuts()
	if containsSubstring(shortcuts, "m: merge") {
		t.Errorf("expected no 'm: merge' in footer for running PRD, got: %s", shortcuts)
	}
}

// --- Clean Action Tests ---

func TestCanCleanNonRunningWithWorktree(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   8,
				Total:       8,
				LoopState:   loop.LoopStateComplete,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}
	if !p.CanClean() {
		t.Error("expected CanClean() to return true for completed non-running PRD with worktree")
	}
}

func TestCanCleanDisabledForRunningPRD(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   3,
				Total:       8,
				LoopState:   loop.LoopStateRunning,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}
	if p.CanClean() {
		t.Error("expected CanClean() to return false for running PRD")
	}
}

func TestCanCleanDisabledWithoutWorktree(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 8,
				Total:     8,
				LoopState: loop.LoopStateComplete,
				Branch:    "chief/auth",
			},
		},
		selectedIndex: 0,
	}
	if p.CanClean() {
		t.Error("expected CanClean() to return false for PRD without worktree")
	}
}

func TestCanCleanStoppedPRD(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   3,
				Total:       8,
				LoopState:   loop.LoopStateStopped,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}
	if !p.CanClean() {
		t.Error("expected CanClean() to return true for stopped PRD with worktree")
	}
}

func TestCleanConfirmationDialog(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   8,
				Total:       8,
				LoopState:   loop.LoopStateComplete,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}

	// Start clean confirmation
	p.StartCleanConfirmation()

	if !p.HasCleanConfirmation() {
		t.Fatal("expected HasCleanConfirmation() to return true after start")
	}

	cc := p.GetCleanConfirmation()
	if cc.EntryName != "auth" {
		t.Errorf("expected EntryName 'auth', got %q", cc.EntryName)
	}
	if cc.Branch != "chief/auth" {
		t.Errorf("expected Branch 'chief/auth', got %q", cc.Branch)
	}
	if cc.SelectedIdx != 0 {
		t.Errorf("expected SelectedIdx 0, got %d", cc.SelectedIdx)
	}

	// Default selection is RemoveAll
	if p.GetCleanOption() != CleanOptionRemoveAll {
		t.Errorf("expected CleanOptionRemoveAll by default, got %d", p.GetCleanOption())
	}
}

func TestCleanConfirmationNavigation(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}
	p.StartCleanConfirmation()

	// Move down to "Remove worktree only"
	p.CleanConfirmMoveDown()
	if p.GetCleanOption() != CleanOptionWorktreeOnly {
		t.Errorf("expected CleanOptionWorktreeOnly after move down, got %d", p.GetCleanOption())
	}

	// Move down to "Cancel"
	p.CleanConfirmMoveDown()
	if p.GetCleanOption() != CleanOptionCancel {
		t.Errorf("expected CleanOptionCancel after two moves down, got %d", p.GetCleanOption())
	}

	// Move down again - should stay at Cancel (index 2)
	p.CleanConfirmMoveDown()
	if p.GetCleanOption() != CleanOptionCancel {
		t.Errorf("expected CleanOptionCancel to remain after extra move down, got %d", p.GetCleanOption())
	}

	// Move back up
	p.CleanConfirmMoveUp()
	if p.GetCleanOption() != CleanOptionWorktreeOnly {
		t.Errorf("expected CleanOptionWorktreeOnly after move up, got %d", p.GetCleanOption())
	}
}

func TestCleanConfirmationCancel(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}
	p.StartCleanConfirmation()

	if !p.HasCleanConfirmation() {
		t.Fatal("expected confirmation to be active")
	}

	p.CancelCleanConfirmation()

	if p.HasCleanConfirmation() {
		t.Error("expected confirmation to be cancelled")
	}
}

func TestCleanConfirmationRendering(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		width:    80,
		height:   24,
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   8,
				Total:       8,
				LoopState:   loop.LoopStateComplete,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}
	p.StartCleanConfirmation()

	result := p.Render()
	stripped := stripAnsi(result)

	if !containsText(result, "Clean Worktree") {
		t.Errorf("expected 'Clean Worktree' in render, got: %s", stripped)
	}
	if !containsText(result, "auth") {
		t.Errorf("expected PRD name 'auth' in render, got: %s", stripped)
	}
	if !containsText(result, "chief/auth") {
		t.Errorf("expected branch 'chief/auth' in render, got: %s", stripped)
	}
	if !containsText(result, "Remove worktree + delete branch") {
		t.Errorf("expected option text in render, got: %s", stripped)
	}
	if !containsText(result, "Remove worktree only") {
		t.Errorf("expected option text in render, got: %s", stripped)
	}
	if !containsText(result, "Cancel") {
		t.Errorf("expected 'Cancel' option in render, got: %s", stripped)
	}
}

func TestCleanResultSuccessRendering(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		width:    80,
		height:   24,
		entries: []PRDEntry{
			{Name: "auth"},
		},
		cleanResult: &CleanResult{
			Success: true,
			Message: "Removed worktree and deleted branch chief/auth",
		},
	}

	result := p.Render()
	if !containsText(result, "Clean Successful") {
		t.Errorf("expected 'Clean Successful' in success render, got: %s", stripAnsi(result))
	}
	if !containsText(result, "Removed worktree and deleted branch chief/auth") {
		t.Errorf("expected clean message in output, got: %s", stripAnsi(result))
	}
	if !containsText(result, "Press any key to continue") {
		t.Errorf("expected dismiss hint in output, got: %s", stripAnsi(result))
	}
}

func TestCleanResultErrorRendering(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		width:    80,
		height:   24,
		entries: []PRDEntry{
			{Name: "auth"},
		},
		cleanResult: &CleanResult{
			Success: false,
			Message: "Failed to remove worktree: permission denied",
		},
	}

	result := p.Render()
	if !containsText(result, "Clean Failed") {
		t.Errorf("expected 'Clean Failed' in error render, got: %s", stripAnsi(result))
	}
	if !containsText(result, "permission denied") {
		t.Errorf("expected error message in output, got: %s", stripAnsi(result))
	}
}

func TestCleanResultClearsOnDismiss(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		cleanResult: &CleanResult{
			Success: true,
			Message: "Cleaned",
		},
	}

	if !p.HasCleanResult() {
		t.Error("expected HasCleanResult() to return true")
	}

	p.ClearCleanResult()

	if p.HasCleanResult() {
		t.Error("expected HasCleanResult() to return false after clear")
	}
}

func TestFooterShowsCleanHintForNonRunningPRDWithWorktree(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   8,
				Total:       8,
				LoopState:   loop.LoopStateComplete,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}

	shortcuts := p.buildFooterShortcuts()
	if !containsSubstring(shortcuts, "c: clean") {
		t.Errorf("expected 'c: clean' in footer for completed PRD with worktree, got: %s", shortcuts)
	}
}

func TestFooterHidesCleanHintForRunningPRD(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   3,
				Total:       8,
				LoopState:   loop.LoopStateRunning,
				Iteration:   2,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
			},
		},
		selectedIndex: 0,
	}

	shortcuts := p.buildFooterShortcuts()
	if containsSubstring(shortcuts, "c: clean") {
		t.Errorf("expected no 'c: clean' in footer for running PRD, got: %s", shortcuts)
	}
}

func TestFooterHidesCleanHintForPRDWithoutWorktree(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:      "auth",
				Completed: 8,
				Total:     8,
				LoopState: loop.LoopStateComplete,
				Branch:    "chief/auth",
			},
		},
		selectedIndex: 0,
	}

	shortcuts := p.buildFooterShortcuts()
	if containsSubstring(shortcuts, "c: clean") {
		t.Errorf("expected no 'c: clean' in footer for PRD without worktree, got: %s", shortcuts)
	}
}

// --- Orphaned Worktree Tests ---

func TestRenderEntryOrphanedWithPRD(t *testing.T) {
	p := &PRDPicker{
		basePath:   "/project",
		currentPRD: "",
		entries: []PRDEntry{
			{
				Name:        "auth",
				Completed:   8,
				Total:       8,
				LoopState:   loop.LoopStateComplete,
				Branch:      "chief/auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
				Orphaned:    true,
			},
		},
	}

	result := p.renderEntry(p.entries[0], false, 80)
	if !containsText(result, "[orphaned]") {
		t.Errorf("expected '[orphaned]' indicator for orphaned entry with PRD, got: %s", stripAnsi(result))
	}
	// Should still show progress since PRD is loaded
	if !containsText(result, "8/8") {
		t.Errorf("expected progress '8/8' for orphaned entry with PRD, got: %s", stripAnsi(result))
	}
}

func TestRenderEntryOrphanedWithoutPRD(t *testing.T) {
	p := &PRDPicker{
		basePath:   "/project",
		currentPRD: "",
		entries: []PRDEntry{
			{
				Name:        "stale-project",
				WorktreeDir: "/project/.chief/worktrees/stale-project",
				Orphaned:    true,
				LoadError:   fmt.Errorf("orphaned worktree (no prd.json)"),
			},
		},
	}

	result := p.renderEntry(p.entries[0], false, 80)
	if !containsText(result, "[orphaned worktree]") {
		t.Errorf("expected '[orphaned worktree]' for orphaned entry without PRD, got: %s", stripAnsi(result))
	}
}

func TestCanCleanOrphanedWorktree(t *testing.T) {
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "auth",
				WorktreeDir: "/project/.chief/worktrees/auth",
				Orphaned:    true,
				LoopState:   loop.LoopStateReady,
			},
		},
		selectedIndex: 0,
	}
	if !p.CanClean() {
		t.Error("expected CanClean() to return true for orphaned worktree")
	}
}

func TestOrphanedWorktreeNotTracked(t *testing.T) {
	// An orphaned entry with no PRD loaded should still be cleanable
	p := &PRDPicker{
		basePath: "/project",
		entries: []PRDEntry{
			{
				Name:        "stale",
				WorktreeDir: "/project/.chief/worktrees/stale",
				Orphaned:    true,
				LoopState:   loop.LoopStateReady,
				LoadError:   fmt.Errorf("orphaned worktree (no prd.json)"),
			},
		},
		selectedIndex: 0,
	}
	if !p.CanClean() {
		t.Error("expected CanClean() to return true for orphaned worktree without PRD")
	}
}

// containsText checks if rendered output contains a substring (ignoring ANSI codes).
func containsText(rendered, substr string) bool {
	// Strip ANSI escape sequences for comparison
	return containsSubstring(stripAnsi(rendered), substr)
}

// containsSubstring is a simple substring check.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// stripAnsi removes ANSI escape codes from a string.
func stripAnsi(s string) string {
	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip to end of escape sequence
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // skip the final letter
			}
			i = j
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}
