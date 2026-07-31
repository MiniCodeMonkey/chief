package agent

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/internal/loop"
)

// CopilotProvider implements loop.Provider for GitHub Copilot CLI.
type CopilotProvider struct {
	cliPath string
}

// NewCopilotProvider returns a Provider for GitHub Copilot CLI.
// If cliPath is empty, "copilot" is used.
func NewCopilotProvider(cliPath string) *CopilotProvider {
	if cliPath == "" {
		cliPath = "copilot"
	}
	return &CopilotProvider{cliPath: cliPath}
}

// Name implements loop.Provider.
func (p *CopilotProvider) Name() string { return "Copilot" }

// CLIPath implements loop.Provider.
func (p *CopilotProvider) CLIPath() string { return p.cliPath }

// LoopCommand implements loop.Provider.
func (p *CopilotProvider) LoopCommand(ctx context.Context, prompt, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.cliPath,
		"-p", prompt,
		"--output-format", "json",
		"--allow-all",
		"--no-ask-user",
		"--no-color",
		"--no-auto-update",
		"--no-remote",
		"--no-remote-export",
	)
	cmd.Dir = workDir
	return cmd
}

// InteractiveCommand implements loop.Provider.
func (p *CopilotProvider) InteractiveCommand(workDir, prompt string) *exec.Cmd {
	cmd := exec.Command(p.cliPath, "-i", prompt)
	cmd.Dir = workDir
	return cmd
}

// ParseLine implements loop.Provider.
func (p *CopilotProvider) ParseLine(line string) *loop.Event {
	return loop.ParseLineCopilot(line)
}

// LogFileName implements loop.Provider.
func (p *CopilotProvider) LogFileName() string { return "copilot.log" }

// CleanOutput extracts complete assistant messages from Copilot's JSONL output.
func (p *CopilotProvider) CleanOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return output
	}

	var messages []string
	for _, line := range strings.Split(output, "\n") {
		var event struct {
			Type string `json:"type"`
			Data struct {
				Content string `json:"content"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(line), &event) == nil &&
			event.Type == "assistant.message" &&
			event.Data.Content != "" {
			messages = append(messages, event.Data.Content)
		}
	}
	if len(messages) > 0 {
		return strings.Join(messages, "\n")
	}
	return output
}
