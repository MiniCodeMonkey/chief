package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/prd"
)

func createExecutableScript(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to write mock script: %v", err)
	}
	return path
}

func createPRDFile(t *testing.T, dir string, complete bool) string {
	t.Helper()

	p := &prd.PRD{
		Project:     "OpenCode Test",
		Description: "integration",
		UserStories: []prd.UserStory{{
			ID:          "US-001",
			Title:       "Story",
			Description: "Test",
			Priority:    1,
			Passes:      complete,
		}},
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal prd: %v", err)
	}

	prdPath := filepath.Join(dir, "prd.json")
	if err := os.WriteFile(prdPath, data, 0o644); err != nil {
		t.Fatalf("failed to write prd: %v", err)
	}
	return prdPath
}

func drainEvents(ch <-chan loop.Event) []loop.Event {
	var events []loop.Event
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestOpenCodeProvider_RunIntegration_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock shell scripts are unix-only")
	}

	tmpDir := t.TempDir()
	prdPath := createPRDFile(t, tmpDir, true)
	prompt := "implement story US-003"

	script := `#!/bin/sh
set -eu

if [ "$#" -ne 6 ]; then
  echo "unexpected arg count: $#" >&2
  exit 90
fi
if [ "$1" != "run" ] || [ "$2" != "--format" ] || [ "$3" != "json" ] || [ "$4" != "--dir" ]; then
  echo "unexpected args: $*" >&2
  exit 91
fi

work_dir="$5"
printf '%s' "$6" > "$work_dir/.opencode_prompt"
printf '%s\n' '{"type":"thread.started"}'
printf '%s\n' '{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"working"}}'
`
	scriptPath := createExecutableScript(t, tmpDir, "mock-opencode-success", script)

	provider := NewOpenCodeProvider(scriptPath, "")
	l := loop.NewLoopWithWorkDir(prdPath, tmpDir, prompt, 1, provider)
	l.DisableRetry()

	err := l.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	capturedPrompt, err := os.ReadFile(filepath.Join(tmpDir, ".opencode_prompt"))
	if err != nil {
		t.Fatalf("failed reading captured prompt: %v", err)
	}
	if string(capturedPrompt) != prompt {
		t.Fatalf("prompt mismatch: got %q want %q", string(capturedPrompt), prompt)
	}

	events := drainEvents(l.Events())
	if len(events) == 0 {
		t.Fatal("expected loop events, got none")
	}

	hasComplete := false
	for _, ev := range events {
		if ev.Type == loop.EventComplete {
			hasComplete = true
			break
		}
	}
	if !hasComplete {
		t.Fatalf("expected EventComplete, got events: %+v", events)
	}
}

func TestOpenCodeProvider_RunIntegration_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock shell scripts are unix-only")
	}

	tmpDir := t.TempDir()
	prdPath := createPRDFile(t, tmpDir, false)

	script := `#!/bin/sh
set -eu

if [ "$1" != "run" ] || [ "$2" != "--format" ] || [ "$3" != "json" ] || [ "$4" != "--dir" ]; then
  echo "unexpected args: $*" >&2
  exit 91
fi

echo 'mock failure from opencode' >&2
exit 23
`
	scriptPath := createExecutableScript(t, tmpDir, "mock-opencode-failure", script)

	provider := NewOpenCodeProvider(scriptPath, "")
	l := loop.NewLoopWithWorkDir(prdPath, tmpDir, "prompt", 1, provider)
	l.DisableRetry()

	err := l.Run(context.Background())
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}

	var execErr *loop.ExecutionError
	if !errors.As(err, &execErr) {
		t.Fatalf("expected loop.ExecutionError, got %T: %v", err, err)
	}
	if execErr.Kind != loop.ExecutionErrorKindNonZeroExit {
		t.Fatalf("expected non-zero exit kind, got %s", execErr.Kind)
	}
	if execErr.ExitCode != 23 {
		t.Fatalf("expected exit code 23, got %d", execErr.ExitCode)
	}
	if !strings.Contains(execErr.Stderr, "mock failure from opencode") {
		t.Fatalf("expected stderr summary to include script output, got: %q", execErr.Stderr)
	}
	if !strings.Contains(err.Error(), "stderr:") {
		t.Fatalf("expected labeled stderr in error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "remediation:") {
		t.Fatalf("expected remediation guidance in error message, got: %v", err)
	}

	logData, readErr := os.ReadFile(filepath.Join(tmpDir, "opencode.log"))
	if readErr != nil {
		t.Fatalf("failed to read log file: %v", readErr)
	}
	if !strings.Contains(string(logData), "[stderr] mock failure from opencode") {
		t.Fatalf("expected stderr log label, log contents: %s", string(logData))
	}

	events := drainEvents(l.Events())
	hasError := false
	for _, ev := range events {
		if ev.Type == loop.EventError {
			hasError = true
			if ev.Text == "" || !strings.Contains(ev.Text, "non_zero_exit") {
				t.Fatalf("expected EventError text with explicit state, got: %+v", ev)
			}
			break
		}
	}
	if !hasError {
		t.Fatalf("expected EventError, got events: %+v", events)
	}
}

func TestOpenCodeProvider_RunIntegration_MissingBinary(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createPRDFile(t, tmpDir, false)

	provider := NewOpenCodeProvider(filepath.Join(tmpDir, "missing-opencode"), "")
	l := loop.NewLoopWithWorkDir(prdPath, tmpDir, "prompt", 1, provider)
	l.DisableRetry()

	err := l.Run(context.Background())
	if err == nil {
		t.Fatal("Run() expected missing binary error, got nil")
	}

	var execErr *loop.ExecutionError
	if !errors.As(err, &execErr) {
		t.Fatalf("expected loop.ExecutionError, got %T: %v", err, err)
	}
	if execErr.Kind != loop.ExecutionErrorKindMissingBinary {
		t.Fatalf("expected missing_binary kind, got %s", execErr.Kind)
	}
	if !strings.Contains(err.Error(), "agent.opencode.cliPath") {
		t.Fatalf("expected opencode remediation hint, got: %v", err)
	}

	events := drainEvents(l.Events())
	hasError := false
	for _, ev := range events {
		if ev.Type == loop.EventError {
			hasError = true
			if !strings.Contains(ev.Text, "missing_binary") {
				t.Fatalf("expected explicit missing_binary state in event text, got: %s", ev.Text)
			}
			break
		}
	}
	if !hasError {
		t.Fatalf("expected EventError, got events: %+v", events)
	}
}

func TestOpenCodeProvider_RunIntegration_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock shell scripts are unix-only")
	}

	tmpDir := t.TempDir()
	prdPath := createPRDFile(t, tmpDir, false)

	script := `#!/bin/sh
set -eu
sleep 5
`
	scriptPath := createExecutableScript(t, tmpDir, "mock-opencode-timeout", script)

	provider := NewOpenCodeProvider(scriptPath, "")
	l := loop.NewLoopWithWorkDir(prdPath, tmpDir, "prompt", 1, provider)
	l.DisableRetry()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := l.Run(ctx)
	if err == nil {
		t.Fatal("Run() expected timeout error, got nil")
	}

	var execErr *loop.ExecutionError
	if !errors.As(err, &execErr) {
		t.Fatalf("expected loop.ExecutionError, got %T: %v", err, err)
	}
	if execErr.Kind != loop.ExecutionErrorKindTimeout {
		t.Fatalf("expected timeout kind, got %s", execErr.Kind)
	}
	if !strings.Contains(err.Error(), "remediation:") {
		t.Fatalf("expected remediation guidance in timeout error, got: %v", err)
	}

	events := drainEvents(l.Events())
	hasError := false
	for _, ev := range events {
		if ev.Type == loop.EventError {
			hasError = true
			if !strings.Contains(ev.Text, "timeout") {
				t.Fatalf("expected timeout in event text, got: %s", ev.Text)
			}
			break
		}
	}
	if !hasError {
		t.Fatalf("expected EventError, got events: %+v", events)
	}
}

func TestOpenCodeProvider_RunIntegration_Canceled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock shell scripts are unix-only")
	}

	tmpDir := t.TempDir()
	prdPath := createPRDFile(t, tmpDir, false)

	script := `#!/bin/sh
set -eu

if [ "$1" != "run" ] || [ "$2" != "--format" ] || [ "$3" != "json" ] || [ "$4" != "--dir" ]; then
  echo "unexpected args: $*" >&2
  exit 91
fi

printf '%s\n' '{"type":"thread.started"}'
sleep 5
`
	scriptPath := createExecutableScript(t, tmpDir, "mock-opencode-cancel", script)

	provider := NewOpenCodeProvider(scriptPath, "")
	l := loop.NewLoopWithWorkDir(prdPath, tmpDir, "prompt", 1, provider)
	l.DisableRetry()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	err := l.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}

	events := drainEvents(l.Events())
	if len(events) == 0 {
		t.Fatal("expected at least one event before cancel")
	}
	if events[0].Type != loop.EventIterationStart {
		t.Fatalf("expected first event to be iteration start, got %s", events[0].Type.String())
	}
}
