package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lvcoi/melliza/internal/git"
)

// trashPRD moves a PRD directory from .melliza/prds/<name>/ to .melliza/.trash/<name>/.
// If a name collision exists in trash, appends a timestamp suffix.
// Best-effort worktree cleanup is attempted before the move.
func trashPRD(baseDir, prdName string) error {
	srcDir := filepath.Join(baseDir, ".melliza", "prds", prdName)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("PRD %q not found", prdName)
	}

	trashDir := filepath.Join(baseDir, ".melliza", ".trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Best-effort worktree cleanup
	worktreePath := git.WorktreePathForPRD(baseDir, prdName)
	_ = git.RemoveWorktree(baseDir, worktreePath)

	// Determine destination, handling name collisions
	destDir := filepath.Join(trashDir, prdName)
	if _, err := os.Stat(destDir); err == nil {
		// Collision - append timestamp
		suffix := time.Now().Format("20060102-150405")
		destDir = filepath.Join(trashDir, prdName+"-"+suffix)
	}

	if err := os.Rename(srcDir, destDir); err != nil {
		return fmt.Errorf("failed to move PRD to trash: %w", err)
	}

	return nil
}
