package tui

import (
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/loop"
)

func TestRenderTabWithBranch(t *testing.T) {
	tb := &TabBar{}

	entry := TabEntry{
		Name:      "auth",
		Branch:    "chief/auth",
		LoopState: loop.LoopStateRunning,
		Iteration: 3,
		Total:     8,
		Completed: 3,
	}

	result := tb.renderTab(entry, 1)
	if !strings.Contains(result, "[chief/auth]") {
		t.Errorf("expected tab to contain [chief/auth], got: %s", result)
	}
	if !strings.Contains(result, "auth") {
		t.Errorf("expected tab to contain name 'auth', got: %s", result)
	}
}

func TestRenderTabWithoutBranch(t *testing.T) {
	tb := &TabBar{}

	entry := TabEntry{
		Name:      "auth",
		LoopState: loop.LoopStateReady,
		Total:     8,
		Completed: 3,
	}

	result := tb.renderTab(entry, 1)
	// Should not contain a branch bracket like [chief/auth], but may contain [3/8] progress
	if strings.Contains(result, "[chief/") {
		t.Errorf("expected tab without branch to not contain branch brackets, got: %s", result)
	}
}

func TestRenderTabBranchTruncation(t *testing.T) {
	tb := &TabBar{}

	entry := TabEntry{
		Name:      "auth",
		Branch:    "chief/very-long-branch-name-that-is-too-long",
		LoopState: loop.LoopStateReady,
		Total:     5,
		Completed: 2,
	}

	result := tb.renderTab(entry, 1)
	// Branch should be truncated to 20 chars max (19 + "…")
	if strings.Contains(result, "chief/very-long-branch-name-that-is-too-long") {
		t.Errorf("expected long branch name to be truncated, got: %s", result)
	}
	// Should contain the truncated version
	if !strings.Contains(result, "chief/very-long-bra…") {
		t.Errorf("expected truncated branch name, got: %s", result)
	}
}

func TestRenderCompactTabOmitsBranch(t *testing.T) {
	tb := &TabBar{}

	entry := TabEntry{
		Name:      "auth",
		Branch:    "chief/auth",
		LoopState: loop.LoopStateRunning,
	}

	result := tb.renderCompactTab(entry, 1)
	if strings.Contains(result, "chief/auth") {
		t.Errorf("expected compact tab to omit branch, got: %s", result)
	}
}

func TestTabEntryBranchField(t *testing.T) {
	entry := TabEntry{
		Name:   "payments",
		Branch: "chief/payments",
	}

	if entry.Branch != "chief/payments" {
		t.Errorf("expected Branch to be 'chief/payments', got: %s", entry.Branch)
	}
}

func TestRenderTabBranchWithActiveIndicator(t *testing.T) {
	tb := &TabBar{}

	entry := TabEntry{
		Name:      "auth",
		Branch:    "chief/auth",
		LoopState: loop.LoopStateReady,
		IsActive:  true,
		Total:     8,
		Completed: 3,
	}

	result := tb.renderTab(entry, 1)
	if !strings.Contains(result, "[chief/auth]") {
		t.Errorf("expected active tab to contain [chief/auth], got: %s", result)
	}
	if !strings.Contains(result, "◉") {
		t.Errorf("expected active tab to contain active indicator, got: %s", result)
	}
}

func TestRenderTabEmptyBranch(t *testing.T) {
	tb := &TabBar{}

	entry := TabEntry{
		Name:      "auth",
		Branch:    "",
		LoopState: loop.LoopStateReady,
		Total:     5,
		Completed: 2,
	}

	result := tb.renderTab(entry, 1)
	if strings.Contains(result, "[]") {
		t.Errorf("expected empty branch to not show empty brackets, got: %s", result)
	}
}
