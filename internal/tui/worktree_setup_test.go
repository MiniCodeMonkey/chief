//go:build !windows

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunSetupCommand_Success(t *testing.T) {
	dir := t.TempDir()
	if err := runSetupCommand("touch marker", dir, 0, ""); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); err != nil {
		t.Errorf("expected marker file: %v", err)
	}
}

func TestRunSetupCommand_NonZeroExit(t *testing.T) {
	err := runSetupCommand("exit 7", t.TempDir(), 0, "")
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestRunSetupCommand_TimeoutKillsCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in -short mode")
	}
	start := time.Now()
	err := runSetupCommand("sleep 5", t.TempDir(), 100*time.Millisecond, "100ms")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected error to mention timeout, got: %v", err)
	}
	if !strings.Contains(err.Error(), "100ms") {
		t.Errorf("expected error to echo configured label '100ms', got: %v", err)
	}
	// Sanity: should kill well before the 5s sleep completes. Allow generous
	// slack for slow CI but still much less than 5s.
	if elapsed > 3*time.Second {
		t.Errorf("expected timeout to kill quickly, took %v", elapsed)
	}
}

func TestRunSetupCommand_TimeoutKillsChildProcessGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process-group test in -short mode")
	}
	dir := t.TempDir()
	// Spawn a long-running grandchild via `sh -c "sleep 5 & wait"`. The
	// outer sh forks sleep into the same group; configureProcessGroupKill
	// should SIGKILL the whole group, so the call returns promptly.
	start := time.Now()
	err := runSetupCommand("sleep 5 & wait", dir, 150*time.Millisecond, "150ms")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 3*time.Second {
		t.Errorf("group kill did not propagate; took %v", elapsed)
	}
}

func TestRunSetupCommand_FallsBackToDurationStringWhenLabelEmpty(t *testing.T) {
	err := runSetupCommand("sleep 5", t.TempDir(), 50*time.Millisecond, "")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "50ms") {
		t.Errorf("expected fallback label '50ms' in error, got: %v", err)
	}
}
