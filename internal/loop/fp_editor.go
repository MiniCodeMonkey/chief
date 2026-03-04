package loop

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/embed"
)

// FPDecision represents the editor's decision after reviewing a front pressure concern.
type FPDecision int

const (
	// FPDecisionEdit means the editor found a genuine plan-level issue and updated the PRD.
	FPDecisionEdit FPDecision = iota
	// FPDecisionDismiss means the concern was not a plan-level issue and was dismissed.
	FPDecisionDismiss
	// FPDecisionScrap means the foundational assumptions are wrong and the plan should be scrapped.
	FPDecisionScrap
)

// FrontPressureEditor runs a Claude session to review a front pressure concern
// and returns a structured decision.
type FrontPressureEditor struct {
	// ClaudeRunner runs claude with the given prompt and workDir, returning all assistant text.
	// Defaults to running claude --dangerously-skip-permissions -p <prompt> --output-format stream-json.
	ClaudeRunner func(ctx context.Context, prompt, workDir string) (string, error)
}

// NewFrontPressureEditor creates a FrontPressureEditor with the default ClaudeRunner.
func NewFrontPressureEditor() *FrontPressureEditor {
	e := &FrontPressureEditor{}
	e.ClaudeRunner = e.defaultClaudeRunner
	return e
}

// defaultClaudeRunner runs claude and collects all assistant text from stream-json output.
func (e *FrontPressureEditor) defaultClaudeRunner(ctx context.Context, prompt, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude",
		"--dangerously-skip-permissions",
		"-p", prompt,
		"--output-format", "stream-json",
	)
	if workDir != "" {
		cmd.Dir = workDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if event := ParseLine(line); event != nil {
			if event.Type == EventAssistantText || event.Type == EventFrontPressure {
				sb.WriteString(event.Text)
				sb.WriteString("\n")
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("claude exited with error: %w", err)
	}

	return sb.String(), nil
}

// Review runs the front pressure editor and returns a decision.
// If no <fp-decision> tag is found in the output, it returns FPDecisionDismiss as a safe default.
func (e *FrontPressureEditor) Review(ctx context.Context, prdPath, storyID, concern, dismissedConcerns, workDir string) (FPDecision, error) {
	prompt := embed.GetFPEditorPrompt(prdPath, storyID, concern, dismissedConcerns)

	output, err := e.ClaudeRunner(ctx, prompt, workDir)
	if err != nil {
		return FPDecisionDismiss, fmt.Errorf("editor run failed: %w", err)
	}

	decision := extractStoryID(output, "<fp-decision>", "</fp-decision>")
	switch strings.TrimSpace(decision) {
	case "edit":
		return FPDecisionEdit, nil
	case "scrap":
		return FPDecisionScrap, nil
	default:
		// "dismiss" or any unrecognized/missing decision defaults to dismiss
		return FPDecisionDismiss, nil
	}
}
