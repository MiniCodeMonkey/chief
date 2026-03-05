package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lvcoi/melliza/internal/git"
)

// DeleteOptions contains configuration for the delete command.
type DeleteOptions struct {
	Name    string // PRD name to delete
	BaseDir string // Base directory for .melliza/ (default: current directory)
}

// RunDelete moves a PRD to the trash directory (.melliza/.trash/).
func RunDelete(opts DeleteOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("PRD name is required\nUsage: melliza delete <name>")
	}
	if opts.BaseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		opts.BaseDir = cwd
	}

	srcDir := filepath.Join(opts.BaseDir, ".melliza", "prds", opts.Name)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("PRD %q not found in .melliza/prds/", opts.Name)
	}

	// Best-effort worktree cleanup
	worktreePath := git.WorktreePathForPRD(opts.BaseDir, opts.Name)
	_ = git.RemoveWorktree(opts.BaseDir, worktreePath)

	trashDir := filepath.Join(opts.BaseDir, ".melliza", ".trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Determine destination, handling name collisions
	destDir := filepath.Join(trashDir, opts.Name)
	if _, err := os.Stat(destDir); err == nil {
		suffix := time.Now().Format("20060102-150405")
		destDir = filepath.Join(trashDir, opts.Name+"-"+suffix)
	}

	if err := os.Rename(srcDir, destDir); err != nil {
		return fmt.Errorf("failed to move PRD to trash: %w", err)
	}

	fmt.Printf("Deleted PRD %q (moved to .melliza/.trash/%s)\n", opts.Name, filepath.Base(destDir))
	return nil
}
