package agent

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/internal/loop"
)

// CodexProvider implements loop.Provider for the Codex CLI.
type CodexProvider struct {
	cliPath string
}

// NewCodexProvider returns a Provider for the Codex CLI.
// If cliPath is empty, "codex" is used.
func NewCodexProvider(cliPath string) *CodexProvider {
	if cliPath == "" {
		cliPath = "codex"
	}
	return &CodexProvider{cliPath: cliPath}
}

// Name implements loop.Provider.
func (p *CodexProvider) Name() string { return "Codex" }

// CLIPath implements loop.Provider.
func (p *CodexProvider) CLIPath() string { return p.cliPath }

// LoopCommand implements loop.Provider.
func (p *CodexProvider) LoopCommand(ctx context.Context, prompt, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.cliPath, "exec", "--json", "--yolo", "-C", workDir, "-")
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	return cmd
}

// InteractiveCommand implements loop.Provider.
func (p *CodexProvider) InteractiveCommand(workDir, prompt string) *exec.Cmd {
	cmd := exec.Command(p.cliPath, prompt)
	cmd.Dir = workDir
	return cmd
}

// ConvertCommand implements loop.Provider.
func (p *CodexProvider) ConvertCommand(workDir, prompt string) (*exec.Cmd, loop.OutputMode, string) {
	f, err := os.CreateTemp("", "chief-codex-convert-*.txt")
	if err != nil {
		// Caller will fail when running cmd; return empty path
		cmd := exec.Command(p.cliPath, "exec", "--sandbox", "read-only", "--output-last-message", "-o", "", "-")
		cmd.Dir = workDir
		cmd.Stdin = strings.NewReader(prompt)
		return cmd, loop.OutputFromFile, ""
	}
	outPath := f.Name()
	f.Close()
	cmd := exec.Command(p.cliPath, "exec", "--sandbox", "read-only", "--output-last-message", "-o", outPath, "-")
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	return cmd, loop.OutputFromFile, outPath
}

// FixJSONCommand implements loop.Provider.
func (p *CodexProvider) FixJSONCommand(prompt string) (*exec.Cmd, loop.OutputMode, string) {
	f, err := os.CreateTemp("", "chief-codex-fixjson-*.txt")
	if err != nil {
		cmd := exec.Command(p.cliPath, "exec", "--sandbox", "read-only", "--output-last-message", "-o", "", "-")
		cmd.Stdin = strings.NewReader(prompt)
		return cmd, loop.OutputFromFile, ""
	}
	outPath := f.Name()
	f.Close()
	cmd := exec.Command(p.cliPath, "exec", "--sandbox", "read-only", "--output-last-message", "-o", outPath, "-")
	cmd.Stdin = strings.NewReader(prompt)
	return cmd, loop.OutputFromFile, outPath
}

// ParseLine implements loop.Provider.
func (p *CodexProvider) ParseLine(line string) *loop.Event {
	return loop.ParseLineCodex(line)
}

// LogFileName implements loop.Provider.
func (p *CodexProvider) LogFileName() string { return "codex.log" }
