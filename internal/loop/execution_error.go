package loop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strings"
)

// ExecutionErrorKind categorizes user-facing execution failures.
type ExecutionErrorKind string

const (
	ExecutionErrorKindMissingBinary ExecutionErrorKind = "missing_binary"
	ExecutionErrorKindTimeout       ExecutionErrorKind = "timeout"
	ExecutionErrorKindNonZeroExit   ExecutionErrorKind = "non_zero_exit"
	ExecutionErrorKindProcessFailed ExecutionErrorKind = "process_failure"
)

// ExecutionError describes a provider execution failure with remediation guidance.
type ExecutionError struct {
	Provider    string
	CLIPath     string
	Kind        ExecutionErrorKind
	ExitCode    int
	Stderr      string
	Remediation string
	Cause       error
}

func (e *ExecutionError) Error() string {
	var detail string
	switch e.Kind {
	case ExecutionErrorKindMissingBinary:
		detail = fmt.Sprintf("CLI binary %q was not found", e.CLIPath)
	case ExecutionErrorKindTimeout:
		detail = "execution timed out before completion"
	case ExecutionErrorKindNonZeroExit:
		detail = fmt.Sprintf("process exited with status %d", e.ExitCode)
	default:
		detail = "process failed during execution"
	}

	msg := fmt.Sprintf("%s execution failed (%s): %s", e.Provider, e.Kind, detail)
	if e.Stderr != "" {
		msg += "; stderr: " + e.Stderr
	}
	if e.Remediation != "" {
		msg += "; remediation: " + e.Remediation
	}
	if e.Cause != nil && e.Kind == ExecutionErrorKindProcessFailed {
		msg += "; cause: " + e.Cause.Error()
	}
	return msg
}

// Unwrap returns the underlying process error.
func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

func newExecutionError(provider Provider, err error, stderr string) *ExecutionError {
	e := &ExecutionError{
		Provider: "Agent",
		Kind:     ExecutionErrorKindProcessFailed,
		ExitCode: -1,
		Stderr:   compactStderr(stderr),
		Cause:    err,
	}
	if provider != nil {
		e.Provider = provider.Name()
		e.CLIPath = provider.CLIPath()
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		e.Kind = ExecutionErrorKindTimeout
		e.Remediation = "Increase your run timeout (if configured) or retry with a smaller task scope."
	case isMissingBinaryError(err):
		e.Kind = ExecutionErrorKindMissingBinary
		e.Remediation = missingBinaryRemediation(e.Provider)
	case isNonZeroExitError(err, &e.ExitCode):
		e.Kind = ExecutionErrorKindNonZeroExit
		e.Remediation = nonZeroExitRemediation(e.Provider)
	default:
		e.Kind = ExecutionErrorKindProcessFailed
		e.Remediation = "Check the provider CLI installation and rerun after resolving the underlying process error."
	}

	return e
}

func asExecutionError(err error) (*ExecutionError, bool) {
	var execErr *ExecutionError
	if errors.As(err, &execErr) {
		return execErr, true
	}
	return nil, false
}

func isNonZeroExitError(err error, exitCode *int) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	if exitCode != nil {
		*exitCode = exitErr.ExitCode()
	}
	return true
}

func isMissingBinaryError(err error) bool {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return true
	}

	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return true
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "executable file not found") ||
		strings.Contains(lower, "no such file or directory")
}

func missingBinaryRemediation(providerName string) string {
	if strings.EqualFold(providerName, "OpenCode") {
		return "Install OpenCode and verify `opencode --version`, or set `agent.opencode.cliPath` (or `agent.cliPath`) in `.chief/config.yaml`."
	}
	return "Install the provider CLI and verify it is on PATH, or set `agent.cliPath` in `.chief/config.yaml`."
}

func nonZeroExitRemediation(providerName string) string {
	if strings.EqualFold(providerName, "OpenCode") {
		return "Review stderr output and verify auth/env configuration (including `agent.opencode.requiredEnv`) before retrying."
	}
	return "Review stderr output and retry after fixing the reported issue."
}

func compactStderr(stderr string) string {
	text := strings.TrimSpace(stderr)
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	const maxLines = 4
	truncated := false
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	joined := strings.Join(lines, " | ")
	const maxLen = 320
	if len(joined) > maxLen {
		joined = joined[:maxLen-3] + "..."
		truncated = true
	}

	if truncated {
		return joined + " | ..."
	}
	return joined
}
