package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MigrateFromJSON reads prd.json, transfers story statuses into prd.md
// using SetStoryStatus, and renames prd.json to prd.json.bak.
func MigrateFromJSON(prdDir string) error {
	jsonPath := filepath.Join(prdDir, "prd.json")
	mdPath := filepath.Join(prdDir, "prd.md")

	// Read and parse prd.json
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read prd.json: %w", err)
	}

	var p PRD
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("failed to parse prd.json: %w", err)
	}

	// Check that prd.md exists
	if _, err := os.Stat(mdPath); err != nil {
		return fmt.Errorf("prd.md not found: %w", err)
	}

	// Transfer statuses
	for _, story := range p.UserStories {
		if story.Passes {
			if err := SetStoryStatus(mdPath, story.ID, "done"); err != nil {
				// Non-fatal: story might not exist in prd.md (was removed)
				continue
			}
		} else if story.InProgress {
			if err := SetStoryStatus(mdPath, story.ID, "in-progress"); err != nil {
				continue
			}
		}
	}

	// Rename prd.json → prd.json.bak
	bakPath := filepath.Join(prdDir, "prd.json.bak")
	if err := os.Rename(jsonPath, bakPath); err != nil {
		return fmt.Errorf("failed to rename prd.json to prd.json.bak: %w", err)
	}

	return nil
}
