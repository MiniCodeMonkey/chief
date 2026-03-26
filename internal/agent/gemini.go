package agent

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/internal/loop"
)

// GeminiProvider implements loop.Provider for the Gemini CLI.
type GeminiProvider struct {
	cliPath string
}

// NewGeminiProvider returns a Provider for the Gemini CLI.
// If cliPath is empty, "gemini" is used.
func NewGeminiProvider(cliPath string) *GeminiProvider {
	if cliPath == "" {
		cliPath = "gemini"
	}
	return &GeminiProvider{cliPath: cliPath}
}

// Name implements loop.Provider.
func (p *GeminiProvider) Name() string { return "Gemini" }

// CLIPath implements loop.Provider.
func (p *GeminiProvider) CLIPath() string { return p.cliPath }

// LoopCommand implements loop.Provider.
// Runs Gemini in non-interactive (headless) mode with streaming JSON output
// and YOLO approval mode so all tool calls are auto-approved.
func (p *GeminiProvider) LoopCommand(ctx context.Context, prompt, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.cliPath,
		"-p", prompt,
		"--output-format", "stream-json",
		"--yolo",
	)
	cmd.Dir = workDir
	return cmd
}

// InteractiveCommand implements loop.Provider.
func (p *GeminiProvider) InteractiveCommand(workDir, prompt string) *exec.Cmd {
	cmd := exec.Command(p.cliPath, prompt)
	cmd.Dir = workDir
	return cmd
}

// ParseLine implements loop.Provider.
func (p *GeminiProvider) ParseLine(line string) *loop.Event {
	return loop.ParseLineGemini(line)
}

// LogFileName implements loop.Provider.
func (p *GeminiProvider) LogFileName() string { return "gemini.log" }

// geminiAssistantMessage is used by CleanOutput to extract assistant text deltas.
type geminiAssistantMessage struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CleanOutput concatenates all assistant message delta chunks from Gemini's
// stream-json NDJSON output and returns the full assistant response.
// Falls back to the raw output if no assistant messages are found.
func (p *GeminiProvider) CleanOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return output
	}

	var sb strings.Builder
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg geminiAssistantMessage
		if json.Unmarshal([]byte(line), &msg) == nil && msg.Type == "message" && msg.Role == "assistant" && msg.Content != "" {
			sb.WriteString(msg.Content)
		}
	}
	if sb.Len() > 0 {
		return sb.String()
	}
	return output
}
