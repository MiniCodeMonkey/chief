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
if [ "$1" != "exec" ] || [ "$2" != "--json" ] || [ "$3" != "--yolo" ] || [ "$4" != "-C" ] || [ "$6" != "-" ]; then
  echo "unexpected args: $*" >&2
  exit 91
fi

work_dir="$5"
cat > "$work_dir/.opencode_prompt"
printf '%s\n' '{"type":"thread.started"}'
printf '%s\n' '{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"working"}}'
`
	scriptPath := createExecutableScript(t, tmpDir, "mock-opencode-success", script)

	provider := NewOpenCodeProvider(scriptPath)
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

if [ "$1" != "exec" ] || [ "$2" != "--json" ] || [ "$3" != "--yolo" ] || [ "$4" != "-C" ] || [ "$6" != "-" ]; then
  echo "unexpected args: $*" >&2
  exit 91
fi

cat >/dev/null
echo 'mock failure from opencode' >&2
exit 23
`
	scriptPath := createExecutableScript(t, tmpDir, "mock-opencode-failure", script)

	provider := NewOpenCodeProvider(scriptPath)
	l := loop.NewLoopWithWorkDir(prdPath, tmpDir, "prompt", 1, provider)
	l.DisableRetry()

	err := l.Run(context.Background())
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "OpenCode exited with error") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "exit status 23") {
		t.Fatalf("expected exit status in error, got: %v", err)
	}

	events := drainEvents(l.Events())
	hasError := false
	for _, ev := range events {
		if ev.Type == loop.EventError {
			hasError = true
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

if [ "$1" != "exec" ] || [ "$2" != "--json" ] || [ "$3" != "--yolo" ] || [ "$4" != "-C" ] || [ "$6" != "-" ]; then
  echo "unexpected args: $*" >&2
  exit 91
fi

cat >/dev/null
printf '%s\n' '{"type":"thread.started"}'
sleep 5
`
	scriptPath := createExecutableScript(t, tmpDir, "mock-opencode-cancel", script)

	provider := NewOpenCodeProvider(scriptPath)
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
