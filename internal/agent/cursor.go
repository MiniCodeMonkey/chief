package agent

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/internal/loop"
)

// CursorProvider implements loop.Provider for the Cursor CLI (agent).
type CursorProvider struct {
	cliPath string
}

// NewCursorProvider returns a Provider for the Cursor CLI.
// If cliPath is empty, "agent" is used.
func NewCursorProvider(cliPath string) *CursorProvider {
	if cliPath == "" {
		cliPath = "agent"
	}
	return &CursorProvider{cliPath: cliPath}
}

// Name implements loop.Provider.
func (p *CursorProvider) Name() string { return "Cursor" }

// CLIPath implements loop.Provider.
func (p *CursorProvider) CLIPath() string { return p.cliPath }

// LoopCommand implements loop.Provider.
// Prompt is supplied via stdin; Cursor CLI reads it when -p has no argument.
func (p *CursorProvider) LoopCommand(ctx context.Context, prompt, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.cliPath,
		"-p",
		"--output-format", "stream-json",
		"--force",
		"--workspace", workDir,
		"--trust",
	)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	return cmd
}

// InteractiveCommand implements loop.Provider.
func (p *CursorProvider) InteractiveCommand(workDir, prompt string) *exec.Cmd {
	cmd := exec.Command(p.cliPath, prompt)
	cmd.Dir = workDir
	return cmd
}

// ConvertCommand implements loop.Provider.
// Prompt is supplied via stdin.
func (p *CursorProvider) ConvertCommand(workDir, prompt string) (*exec.Cmd, loop.OutputMode, string, error) {
	cmd := exec.Command(p.cliPath, "-p", "--output-format", "text", "--workspace", workDir)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	return cmd, loop.OutputStdout, "", nil
}

// FixJSONCommand implements loop.Provider.
// Prompt is supplied via stdin.
func (p *CursorProvider) FixJSONCommand(prompt string) (*exec.Cmd, loop.OutputMode, string, error) {
	cmd := exec.Command(p.cliPath, "-p", "--output-format", "text")
	cmd.Stdin = strings.NewReader(prompt)
	return cmd, loop.OutputStdout, "", nil
}

// ParseLine implements loop.Provider.
func (p *CursorProvider) ParseLine(line string) *loop.Event {
	return loop.ParseLineCursor(line)
}

// LogFileName implements loop.Provider.
func (p *CursorProvider) LogFileName() string { return "cursor.log" }

// CleanOutput extracts the result from Cursor's json or stream-json output.
// For stream-json, finds the last type "result", subtype "success" and returns its result field.
// For single-line json, parses and returns result.
func (p *CursorProvider) CleanOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return output
	}
	// Try single JSON object (json output format)
	var single struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype,omitempty"`
		Result  string `json:"result,omitempty"`
	}
	if json.Unmarshal([]byte(output), &single) == nil && single.Type == "result" && single.Subtype == "success" && single.Result != "" {
		return single.Result
	}
	// NDJSON: find last result/success line
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype,omitempty"`
			Result  string `json:"result,omitempty"`
		}
		if json.Unmarshal([]byte(line), &ev) == nil && ev.Type == "result" && ev.Subtype == "success" && ev.Result != "" {
			return ev.Result
		}
	}
	return output
}
