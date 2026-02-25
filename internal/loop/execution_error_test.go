package loop

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

type staticProvider struct {
	name string
	path string
}

func (p staticProvider) Name() string    { return p.name }
func (p staticProvider) CLIPath() string { return p.path }
func (p staticProvider) LoopCommand(context.Context, string, string) *exec.Cmd {
	return nil
}
func (p staticProvider) InteractiveCommand(string, string) *exec.Cmd { return nil }
func (p staticProvider) ConvertCommand(string, string) (*exec.Cmd, OutputMode, string, error) {
	return nil, OutputStdout, "", nil
}
func (p staticProvider) FixJSONCommand(string) (*exec.Cmd, OutputMode, string, error) {
	return nil, OutputStdout, "", nil
}
func (p staticProvider) ParseLine(string) *Event { return nil }
func (p staticProvider) LogFileName() string     { return "test.log" }

func TestNewExecutionError_MissingBinaryOpenCode(t *testing.T) {
	provider := staticProvider{name: "OpenCode", path: "opencode"}
	got := newExecutionError(provider, exec.ErrNotFound, "")

	if got.Kind != ExecutionErrorKindMissingBinary {
		t.Fatalf("kind = %s, want %s", got.Kind, ExecutionErrorKindMissingBinary)
	}
	if !strings.Contains(got.Remediation, "agent.opencode.cliPath") {
		t.Fatalf("expected OpenCode remediation hint, got: %q", got.Remediation)
	}
	if !strings.Contains(got.Error(), "missing_binary") {
		t.Fatalf("expected kind label in error, got: %q", got.Error())
	}
}

func TestNewExecutionError_MissingBinaryGenericProvider(t *testing.T) {
	provider := staticProvider{name: "Codex", path: "codex"}
	got := newExecutionError(provider, exec.ErrNotFound, "")

	if got.Kind != ExecutionErrorKindMissingBinary {
		t.Fatalf("kind = %s, want %s", got.Kind, ExecutionErrorKindMissingBinary)
	}
	if !strings.Contains(got.Remediation, "agent.cliPath") {
		t.Fatalf("expected generic remediation hint, got: %q", got.Remediation)
	}
	if strings.Contains(got.Remediation, "agent.opencode.cliPath") {
		t.Fatalf("did not expect OpenCode-specific remediation for generic provider: %q", got.Remediation)
	}
}

func TestNewExecutionError_Timeout(t *testing.T) {
	provider := staticProvider{name: "Codex", path: "codex"}
	got := newExecutionError(provider, context.DeadlineExceeded, "")

	if got.Kind != ExecutionErrorKindTimeout {
		t.Fatalf("kind = %s, want %s", got.Kind, ExecutionErrorKindTimeout)
	}
	if !strings.Contains(got.Remediation, "Increase your run timeout") {
		t.Fatalf("expected timeout remediation, got: %q", got.Remediation)
	}
}

func TestNewExecutionError_NonZeroExit(t *testing.T) {
	provider := staticProvider{name: "OpenCode", path: "opencode"}
	_, runErr := exec.Command("go", "tool", "definitely-not-a-real-tool").CombinedOutput()
	if runErr == nil {
		t.Fatal("expected go tool command to fail")
	}

	got := newExecutionError(provider, runErr, "line one\nline two")
	if got.Kind != ExecutionErrorKindNonZeroExit {
		t.Fatalf("kind = %s, want %s (err=%v)", got.Kind, ExecutionErrorKindNonZeroExit, runErr)
	}
	if got.ExitCode <= 0 {
		t.Fatalf("expected positive exit code, got %d", got.ExitCode)
	}
	if got.Stderr != "line one | line two" {
		t.Fatalf("stderr summary = %q, want %q", got.Stderr, "line one | line two")
	}
	if !strings.Contains(got.Remediation, "agent.opencode.requiredEnv") {
		t.Fatalf("expected OpenCode non-zero remediation, got: %q", got.Remediation)
	}
}

func TestNewExecutionError_ProcessFailureIncludesCause(t *testing.T) {
	provider := staticProvider{name: "Codex", path: "codex"}
	got := newExecutionError(provider, errors.New("boom"), "oops")

	if got.Kind != ExecutionErrorKindProcessFailed {
		t.Fatalf("kind = %s, want %s", got.Kind, ExecutionErrorKindProcessFailed)
	}
	if !strings.Contains(got.Error(), "cause: boom") {
		t.Fatalf("expected process failure cause in error string, got: %q", got.Error())
	}
}

func TestCompactStderr_TruncatesLines(t *testing.T) {
	got := compactStderr("a\nb\nc\nd\ne")
	if got != "a | b | c | d | ..." {
		t.Fatalf("compactStderr = %q, want %q", got, "a | b | c | d | ...")
	}
}
