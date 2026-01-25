package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minicodemonkey/chief/internal/prd"
)

// StatusOptions contains configuration for the status command.
type StatusOptions struct {
	Name    string // PRD name (default: "main")
	BaseDir string // Base directory for .chief/prds/ (default: current directory)
}

// RunStatus prints progress for a PRD.
// Returns nil on success, error otherwise. Exit code should be 0 on success.
func RunStatus(opts StatusOptions) error {
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

	// Build PRD path
	prdPath := filepath.Join(opts.BaseDir, ".chief", "prds", opts.Name, "prd.json")

	// Load PRD
	p, err := prd.LoadPRD(prdPath)
	if err != nil {
		return fmt.Errorf("failed to load PRD %q: %w", opts.Name, err)
	}

	// Count completed stories
	total := len(p.UserStories)
	completed := 0
	var incomplete []prd.UserStory
	for _, story := range p.UserStories {
		if story.Passes {
			completed++
		} else {
			incomplete = append(incomplete, story)
		}
	}

	// Print project name
	fmt.Println(p.Project)

	// Print progress summary
	if total == 0 {
		fmt.Println("No stories defined")
		return nil
	}

	fmt.Printf("%d/%d stories complete\n", completed, total)

	// Print incomplete stories
	if len(incomplete) > 0 {
		fmt.Println("\nIncomplete stories:")
		for _, story := range incomplete {
			status := ""
			if story.InProgress {
				status = " (in progress)"
			}
			fmt.Printf("  %s: %s%s\n", story.ID, story.Title, status)
		}
	} else {
		fmt.Println("\nAll stories complete!")
	}

	return nil
}

// ListOptions contains configuration for the list command.
type ListOptions struct {
	BaseDir string // Base directory for .chief/prds/ (default: current directory)
}

// PRDInfo holds summary info about a PRD for the list command.
type PRDInfo struct {
	Name       string
	Title      string
	Completed  int
	Total      int
	Percentage int
}

// RunList prints all PRDs with their progress.
// Returns nil on success, error otherwise. Exit code should be 0 on success.
func RunList(opts ListOptions) error {
	// Set defaults
	if opts.BaseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		opts.BaseDir = cwd
	}

	// Find all PRDs in .chief/prds/
	prdsDir := filepath.Join(opts.BaseDir, ".chief", "prds")
	entries, err := os.ReadDir(prdsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No PRDs found. Run 'chief init' to create one.")
			return nil
		}
		return fmt.Errorf("failed to read PRDs directory: %w", err)
	}

	// Collect PRD info
	var prds []PRDInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		prdPath := filepath.Join(prdsDir, name, "prd.json")

		// Try to load the PRD
		p, err := prd.LoadPRD(prdPath)
		if err != nil {
			// Skip PRDs that can't be loaded (might be partially created)
			continue
		}

		// Count completed stories
		total := len(p.UserStories)
		completed := 0
		for _, story := range p.UserStories {
			if story.Passes {
				completed++
			}
		}

		percentage := 0
		if total > 0 {
			percentage = (completed * 100) / total
		}

		prds = append(prds, PRDInfo{
			Name:       name,
			Title:      p.Project,
			Completed:  completed,
			Total:      total,
			Percentage: percentage,
		})
	}

	if len(prds) == 0 {
		fmt.Println("No PRDs found. Run 'chief init' to create one.")
		return nil
	}

	// Print PRDs
	for _, info := range prds {
		fmt.Printf("%s: %s (%d/%d, %d%%)\n", info.Name, info.Title, info.Completed, info.Total, info.Percentage)
	}

	return nil
}
