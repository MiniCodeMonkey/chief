package agent

import (
	"context"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/internal/loop"
)

// ClaudeProvider implements loop.Provider for the Claude Code CLI.
type ClaudeProvider struct {
	cliPath string
}

// NewClaudeProvider returns a Provider for the Claude CLI.
// If cliPath is empty, "claude" is used.
func NewClaudeProvider(cliPath string) *ClaudeProvider {
	if cliPath == "" {
		cliPath = "claude"
	}
	return &ClaudeProvider{cliPath: cliPath}
}

// Name implements loop.Provider.
func (p *ClaudeProvider) Name() string { return "Claude" }

// CLIPath implements loop.Provider.
func (p *ClaudeProvider) CLIPath() string { return p.cliPath }

// LoopCommand implements loop.Provider.
func (p *ClaudeProvider) LoopCommand(ctx context.Context, prompt, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.cliPath,
		"--dangerously-skip-permissions",
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
	)
	cmd.Dir = workDir
	return cmd
}

// InteractiveCommand implements loop.Provider.
func (p *ClaudeProvider) InteractiveCommand(workDir, prompt string) *exec.Cmd {
	cmd := exec.Command(p.cliPath, prompt)
	cmd.Dir = workDir
	return cmd
}

// ConvertCommand implements loop.Provider.
func (p *ClaudeProvider) ConvertCommand(workDir, prompt string) (*exec.Cmd, loop.OutputMode, string, error) {
	cmd := exec.Command(p.cliPath, "-p", "--tools", "")
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	return cmd, loop.OutputStdout, "", nil
}

// FixJSONCommand implements loop.Provider.
func (p *ClaudeProvider) FixJSONCommand(prompt string) (*exec.Cmd, loop.OutputMode, string, error) {
	cmd := exec.Command(p.cliPath, "-p", prompt)
	return cmd, loop.OutputStdout, "", nil
}

// ParseLine implements loop.Provider.
func (p *ClaudeProvider) ParseLine(line string) *loop.Event {
	return loop.ParseLine(line)
}

// LogFileName implements loop.Provider.
func (p *ClaudeProvider) LogFileName() string { return "claude.log" }
