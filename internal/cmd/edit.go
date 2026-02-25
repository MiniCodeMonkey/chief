package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minicodemonkey/chief/embed"
	"github.com/minicodemonkey/chief/internal/loop"
)

// EditOptions contains configuration for the edit command.
type EditOptions struct {
	Name     string        // PRD name (default: "main")
	BaseDir  string        // Base directory for .chief/prds/ (default: current directory)
	Merge    bool          // Auto-merge without prompting on conversion conflicts
	Force    bool          // Auto-overwrite without prompting on conversion conflicts
	Provider loop.Provider // Agent CLI provider (Claude or Codex)
}

// RunEdit edits an existing PRD by launching an interactive Claude session.
func RunEdit(opts EditOptions) error {
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

	// Validate name
	if !isValidPRDName(opts.Name) {
		return fmt.Errorf("invalid PRD name %q: must contain only letters, numbers, hyphens, and underscores", opts.Name)
	}

	// Build the PRD directory path
	prdDir := filepath.Join(opts.BaseDir, ".chief", "prds", opts.Name)
	prdMdPath := filepath.Join(prdDir, "prd.md")

	// Check if prd.md exists
	if _, err := os.Stat(prdMdPath); os.IsNotExist(err) {
		return fmt.Errorf("PRD not found at %s. Use 'chief new %s' to create it first", prdMdPath, opts.Name)
	}

	// Get the edit prompt with the PRD directory path
	prompt := embed.GetEditPrompt(prdDir)
	if opts.Provider == nil {
		return fmt.Errorf("edit command requires Provider to be set")
	}

	// Launch interactive agent session
	fmt.Printf("Editing PRD at %s...\n", prdDir)
	fmt.Printf("Launching %s to help you edit your PRD...\n", opts.Provider.Name())
	fmt.Println()

	if err := runInteractiveAgent(opts.Provider, opts.BaseDir, prompt); err != nil {
		return fmt.Errorf("%s session failed: %w", opts.Provider.Name(), err)
	}

	fmt.Println("\nPRD editing complete!")

	// Run conversion from prd.md to prd.json with progress protection
	convertOpts := ConvertOptions{
		PRDDir:   prdDir,
		Merge:    opts.Merge,
		Force:    opts.Force,
		Provider: opts.Provider,
	}
	if err := RunConvertWithOptions(convertOpts); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	fmt.Printf("\nYour PRD is updated! Run 'chief' or 'chief %s' to continue working on it.\n", opts.Name)
	return nil
}
