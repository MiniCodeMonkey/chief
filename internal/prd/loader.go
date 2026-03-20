package prd

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadPRD reads and parses a PRD JSON file from the given path.
func LoadPRD(path string) (*PRD, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PRD file: %w", err)
	}

	var p PRD
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse PRD JSON: %w", err)
	}

	return &p, nil
}

// Save writes the PRD back to a JSON file at the given path.
func (p *PRD) Save(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PRD: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write PRD file: %w", err)
	}

	return nil
}
