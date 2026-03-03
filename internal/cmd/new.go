// Package cmd provides CLI command implementations for Melliza.
// This includes new, edit, status, and list commands that can be
// run from the command line without launching the full TUI.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lvcoi/melliza/embed"
	"github.com/lvcoi/melliza/internal/prd"
)

// NewOptions contains configuration for the new command.
type NewOptions struct {
	Name    string // PRD name (default: "main")
	Context string // Optional context to pass to Gemini
	BaseDir string // Base directory for .melliza/prds/ (default: current directory)
}

// RunNew creates a new PRD by launching an interactive Gemini session.
func RunNew(opts NewOptions) error {
	// Set defaults
	if opts.Name == "" {
		opts.Name = "main"
	}
	if opts.BaseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		opts.BaseDir = cwd
	}

	// Validate name (alphanumeric, -, _)
	if !isValidPRDName(opts.Name) {
		return fmt.Errorf("invalid PRD name %q: must contain only letters, numbers, hyphens, and underscores", opts.Name)
	}

	// Create directory structure: .melliza/prds/<name>/
	prdDir := filepath.Join(opts.BaseDir, ".melliza", "prds", opts.Name)
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		return fmt.Errorf("failed to create PRD directory: %w", err)
	}

	// Check if prd.md already exists
	prdMdPath := filepath.Join(prdDir, "prd.md")
	if _, err := os.Stat(prdMdPath); err == nil {
		return fmt.Errorf("PRD already exists at %s. Use 'melliza edit %s' to modify it", prdMdPath, opts.Name)
	}

	// Get the init prompt with the PRD directory path
	prompt := embed.GetInitPrompt(prdDir, opts.Context)

	// Launch interactive Gemini session
	fmt.Printf("Creating PRD in %s...\n", prdDir)
	fmt.Println("Launching Gemini to help you create your PRD...")
	fmt.Println()

	if err := runInteractiveGemini(opts.BaseDir, prompt); err != nil {
		return fmt.Errorf("Gemini session failed: %w", err)
	}

	// Check if prd.md was created
	if _, err := os.Stat(prdMdPath); os.IsNotExist(err) {
		// Clean up empty directory to prevent broken picker entries
		os.Remove(prdDir)
		fmt.Println("\nNo prd.md was created. Run 'melliza new' again to try again.")
		return nil
	}

	fmt.Println("\nPRD created successfully!")

	// Run conversion from prd.md to prd.json
	if err := RunConvert(prdDir); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	fmt.Printf("\nYour PRD is ready! Run 'melliza' or 'melliza %s' to start working on it.\n", opts.Name)
	return nil
}

// runInteractiveGemini launches an interactive Gemini session in the specified directory.
func runInteractiveGemini(workDir, prompt string) error {
	// Pass prompt as argument (not -p which is print mode / non-interactive)
	cmd := exec.Command("gemini", prompt)
	cmd.Dir = workDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// ConvertOptions contains configuration for the conversion command.
type ConvertOptions struct {
	PRDDir string // PRD directory containing prd.md
	Merge  bool   // Auto-merge without prompting on conversion conflicts
	Force  bool   // Auto-overwrite without prompting on conversion conflicts
}

// RunConvert converts prd.md to prd.json using Gemini.
func RunConvert(prdDir string) error {
	return RunConvertWithOptions(ConvertOptions{PRDDir: prdDir})
}

// RunConvertWithOptions converts prd.md to prd.json using Gemini with options.
// The Merge and Force flags will be fully implemented in US-019.
func RunConvertWithOptions(opts ConvertOptions) error {
	return prd.Convert(prd.ConvertOptions{
		PRDDir: opts.PRDDir,
		Merge:  opts.Merge,
		Force:  opts.Force,
	})
}

// isValidPRDName checks if the name contains only valid characters.
func isValidPRDName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
