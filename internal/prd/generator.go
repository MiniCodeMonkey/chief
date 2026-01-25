package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/minicodemonkey/chief/embed"
)

// ConvertOptions contains configuration for PRD conversion.
type ConvertOptions struct {
	PRDDir string // Directory containing prd.md
	Merge  bool   // Auto-merge progress on conversion conflicts (US-019)
	Force  bool   // Auto-overwrite on conversion conflicts (US-019)
}

// Convert converts prd.md to prd.json using Claude one-shot mode.
// This function is called:
// - After chief init (new PRD creation)
// - After chief edit (PRD modification)
// - Before chief run if prd.md is newer than prd.json
func Convert(opts ConvertOptions) error {
	prdMdPath := filepath.Join(opts.PRDDir, "prd.md")
	prdJsonPath := filepath.Join(opts.PRDDir, "prd.json")

	// Check if prd.md exists
	if _, err := os.Stat(prdMdPath); os.IsNotExist(err) {
		return fmt.Errorf("prd.md not found in %s", opts.PRDDir)
	}

	// Get the converter prompt
	prompt := embed.GetConvertPrompt()

	// Run Claude one-shot conversion (non-interactive)
	cmd := exec.Command("claude",
		"--dangerously-skip-permissions",
		"-p", prompt,
		"--output-format", "text",
	)
	cmd.Dir = opts.PRDDir

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("Claude conversion failed: %s", string(exitErr.Stderr))
		}
		return fmt.Errorf("Claude conversion failed: %w", err)
	}

	// Clean up output (remove any markdown code blocks if present)
	jsonContent := cleanJSONOutput(string(output))

	// Validate that it's valid JSON
	if err := validateJSON(jsonContent); err != nil {
		return fmt.Errorf("conversion produced invalid JSON: %w", err)
	}

	// Write prd.json
	if err := os.WriteFile(prdJsonPath, []byte(jsonContent), 0644); err != nil {
		return fmt.Errorf("failed to write prd.json: %w", err)
	}

	// Verify the PRD can be loaded properly
	if _, err := LoadPRD(prdJsonPath); err != nil {
		return fmt.Errorf("conversion produced invalid PRD structure: %w", err)
	}

	return nil
}

// NeedsConversion checks if prd.md is newer than prd.json, indicating conversion is needed.
// Returns true if:
// - prd.md exists and prd.json does not exist
// - prd.md exists and is newer than prd.json
// Returns false if:
// - prd.md does not exist
// - prd.json is newer than or same age as prd.md
func NeedsConversion(prdDir string) (bool, error) {
	prdMdPath := filepath.Join(prdDir, "prd.md")
	prdJsonPath := filepath.Join(prdDir, "prd.json")

	// Check if prd.md exists
	mdInfo, err := os.Stat(prdMdPath)
	if os.IsNotExist(err) {
		// No prd.md, no conversion needed
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat prd.md: %w", err)
	}

	// Check if prd.json exists
	jsonInfo, err := os.Stat(prdJsonPath)
	if os.IsNotExist(err) {
		// prd.md exists but prd.json doesn't - needs conversion
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat prd.json: %w", err)
	}

	// Both exist - compare modification times
	return mdInfo.ModTime().After(jsonInfo.ModTime()), nil
}

// cleanJSONOutput removes markdown code blocks and trims whitespace from Claude's output.
func cleanJSONOutput(output string) string {
	output = strings.TrimSpace(output)

	// Remove markdown code blocks if present
	if strings.HasPrefix(output, "```json") {
		output = strings.TrimPrefix(output, "```json")
	} else if strings.HasPrefix(output, "```") {
		output = strings.TrimPrefix(output, "```")
	}

	if strings.HasSuffix(output, "```") {
		output = strings.TrimSuffix(output, "```")
	}

	return strings.TrimSpace(output)
}

// validateJSON checks if the given string is valid JSON.
func validateJSON(content string) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
