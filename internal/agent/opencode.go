package agent

import (
	"context"
	"os/exec"

	"github.com/minicodemonkey/chief/internal/loop"
)

// OpenCodeProvider implements loop.Provider for the OpenCode CLI.
type OpenCodeProvider struct {
	cliPath string
	model   string
}

// NewOpenCodeProvider returns a Provider for the OpenCode CLI.
// If cliPath is empty, "opencode" is used.
func NewOpenCodeProvider(cliPath, model string) *OpenCodeProvider {
	if cliPath == "" {
		cliPath = "opencode"
	}
	return &OpenCodeProvider{cliPath: cliPath, model: model}
}

// Name implements loop.Provider.
func (p *OpenCodeProvider) Name() string { return "OpenCode" }

// CLIPath implements loop.Provider.
func (p *OpenCodeProvider) CLIPath() string { return p.cliPath }

// LoopCommand implements loop.Provider.
func (p *OpenCodeProvider) LoopCommand(ctx context.Context, prompt, workDir string) *exec.Cmd {
	args := p.runArgs("json", workDir, prompt)
	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Dir = workDir
	return cmd
}

// InteractiveCommand implements loop.Provider.
func (p *OpenCodeProvider) InteractiveCommand(workDir, prompt string) *exec.Cmd {
	args := p.commonModelArgs()
	args = append(args, "--prompt", prompt)
	cmd := exec.Command(p.cliPath, args...)
	cmd.Dir = workDir
	return cmd
}

// ConvertCommand implements loop.Provider.
func (p *OpenCodeProvider) ConvertCommand(workDir, prompt string) (*exec.Cmd, loop.OutputMode, string, error) {
	cmd := exec.Command(p.cliPath, p.runArgs("default", workDir, prompt)...)
	cmd.Dir = workDir
	return cmd, loop.OutputStdout, "", nil
}

// FixJSONCommand implements loop.Provider.
func (p *OpenCodeProvider) FixJSONCommand(prompt string) (*exec.Cmd, loop.OutputMode, string, error) {
	args := p.commonModelArgs()
	args = append(args, "run", "--format", "default", prompt)
	cmd := exec.Command(p.cliPath, args...)
	return cmd, loop.OutputStdout, "", nil
}

// ParseLine implements loop.Provider.
func (p *OpenCodeProvider) ParseLine(line string) *loop.Event {
	if ev := loop.ParseLineOpenCode(line); ev != nil {
		return ev
	}
	return loop.ParseLineCodex(line)
}

// LogFileName implements loop.Provider.
func (p *OpenCodeProvider) LogFileName() string { return "opencode.log" }

func (p *OpenCodeProvider) commonModelArgs() []string {
	if p.model == "" {
		return nil
	}
	return []string{"--model", p.model}
}

func (p *OpenCodeProvider) runArgs(format, workDir, prompt string) []string {
	args := p.commonModelArgs()
	args = append(args, "run", "--format", format, "--dir", workDir, prompt)
	return args
}
