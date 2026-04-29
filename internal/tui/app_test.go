package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAppWithOptionsReturnsErrorOnInvalidConfig(t *testing.T) {
	tmp := t.TempDir()

	prdDir := filepath.Join(tmp, ".chief", "prds", "test")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("failed to create prd dir: %v", err)
	}

	prdPath := filepath.Join(prdDir, "prd.md")
	prdContent := "# Test\n\n### US-001: First Story\n- [ ] Works\n"
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write prd: %v", err)
	}

	configPath := filepath.Join(tmp, ".chief", "config.yaml")
	configContent := "worktree:\n  promptBranchPattern: \"[unclosed\"\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// If NewAppWithOptions ever calls os.Exit on config error, this test will fail by killing the test binary.
	app, err := NewAppWithOptions(prdPath, 5, nil)
	if err == nil {
		t.Fatal("expected NewAppWithOptions to return an error for invalid config")
	}
	if app != nil {
		t.Fatal("expected nil App when config load fails")
	}
	if !strings.Contains(err.Error(), "worktree.promptBranchPattern") {
		t.Fatalf("expected error to mention worktree.promptBranchPattern, got %q", err.Error())
	}
}
