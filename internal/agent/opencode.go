package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/internal/loop"
)

// OpenCodeProvider implements loop.Provider for the OpenCode CLI.
type OpenCodeProvider struct {
	cliPath string
}

// NewOpenCodeProvider returns a Provider for the OpenCode CLI.
// If cliPath is empty, "opencode" is used.
func NewOpenCodeProvider(cliPath string) *OpenCodeProvider {
	if cliPath == "" {
		cliPath = "opencode"
	}
	return &OpenCodeProvider{cliPath: cliPath}
}

// Name implements loop.Provider.
func (p *OpenCodeProvider) Name() string { return "OpenCode" }

// CLIPath implements loop.Provider.
func (p *OpenCodeProvider) CLIPath() string { return p.cliPath }

// LoopCommand implements loop.Provider.
func (p *OpenCodeProvider) LoopCommand(ctx context.Context, prompt, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.cliPath, "exec", "--json", "--yolo", "-C", workDir, "-")
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	return cmd
}

// InteractiveCommand implements loop.Provider.
func (p *OpenCodeProvider) InteractiveCommand(workDir, prompt string) *exec.Cmd {
	cmd := exec.Command(p.cliPath, prompt)
	cmd.Dir = workDir
	return cmd
}

// ConvertCommand implements loop.Provider.
func (p *OpenCodeProvider) ConvertCommand(workDir, prompt string) (*exec.Cmd, loop.OutputMode, string, error) {
	f, err := os.CreateTemp("", "chief-opencode-convert-*.txt")
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to create temp file for conversion output: %w", err)
	}
	outPath := f.Name()
	f.Close()
	cmd := exec.Command(p.cliPath, "exec", "--sandbox", "read-only", "-o", outPath, "-")
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	return cmd, loop.OutputFromFile, outPath, nil
}

// FixJSONCommand implements loop.Provider.
func (p *OpenCodeProvider) FixJSONCommand(prompt string) (*exec.Cmd, loop.OutputMode, string, error) {
	f, err := os.CreateTemp("", "chief-opencode-fixjson-*.txt")
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to create temp file for fix output: %w", err)
	}
	outPath := f.Name()
	f.Close()
	cmd := exec.Command(p.cliPath, "exec", "--sandbox", "read-only", "-o", outPath, "-")
	cmd.Stdin = strings.NewReader(prompt)
	return cmd, loop.OutputFromFile, outPath, nil
}

// ParseLine implements loop.Provider.
func (p *OpenCodeProvider) ParseLine(line string) *loop.Event {
	return loop.ParseLineCodex(line)
}

// LogFileName implements loop.Provider.
func (p *OpenCodeProvider) LogFileName() string { return "opencode.log" }
