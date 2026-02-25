package loop

import (
	"context"
	"os/exec"
)

// OutputMode indicates how to capture the result of a one-shot agent command.
type OutputMode int

const (
	// OutputStdout means the result is read from stdout.
	OutputStdout OutputMode = iota
	// OutputFromFile means the result is written to a file; use the path returned by ConvertCommand/FixJSONCommand.
	OutputFromFile
)

// Provider is the interface for an agent CLI (e.g. Claude, Codex).
// Implementations live in internal/agent to avoid import cycles.
type Provider interface {
	Name() string
	CLIPath() string
	LoopCommand(ctx context.Context, prompt, workDir string) *exec.Cmd
	InteractiveCommand(workDir, prompt string) *exec.Cmd
	ConvertCommand(workDir, prompt string) (cmd *exec.Cmd, mode OutputMode, outPath string, err error)
	FixJSONCommand(prompt string) (cmd *exec.Cmd, mode OutputMode, outPath string, err error)
	ParseLine(line string) *Event
	LogFileName() string
}
