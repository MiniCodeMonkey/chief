package prd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateFromJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create prd.json with some statuses
	jsonContent := `{
  "project": "Test",
  "description": "A test",
  "userStories": [
    {"id": "US-001", "title": "Done Story", "passes": true, "priority": 1},
    {"id": "US-002", "title": "In Progress Story", "passes": false, "inProgress": true, "priority": 2},
    {"id": "US-003", "title": "Pending Story", "passes": false, "priority": 3}
  ]
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "prd.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write prd.json: %v", err)
	}

	// Create prd.md with the same stories
	mdContent := `# Test

### US-001: Done Story
- [ ] A

### US-002: In Progress Story
- [ ] B

### US-003: Pending Story
- [ ] C
`
	if err := os.WriteFile(filepath.Join(tmpDir, "prd.md"), []byte(mdContent), 0644); err != nil {
		t.Fatalf("failed to write prd.md: %v", err)
	}

	// Run migration
	if err := MigrateFromJSON(tmpDir); err != nil {
		t.Fatalf("MigrateFromJSON() error = %v", err)
	}

	// Verify prd.json is renamed to prd.json.bak
	if _, err := os.Stat(filepath.Join(tmpDir, "prd.json")); !os.IsNotExist(err) {
		t.Error("prd.json should be renamed")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "prd.json.bak")); err != nil {
		t.Error("prd.json.bak should exist")
	}

	// Verify prd.md has the correct statuses
	data, err := os.ReadFile(filepath.Join(tmpDir, "prd.md"))
	if err != nil {
		t.Fatalf("failed to read prd.md: %v", err)
	}
	result := string(data)

	// US-001 should be done with checked boxes
	if !strings.Contains(result, "- [x] A") {
		t.Error("US-001 should have checked checkbox")
	}

	// Parse and verify
	p, err := ParseMarkdownPRD(filepath.Join(tmpDir, "prd.md"))
	if err != nil {
		t.Fatalf("ParseMarkdownPRD() error = %v", err)
	}

	if !p.UserStories[0].Passes {
		t.Error("US-001 should be passes")
	}
	if !p.UserStories[1].InProgress {
		t.Error("US-002 should be in-progress")
	}
	if p.UserStories[2].Passes || p.UserStories[2].InProgress {
		t.Error("US-003 should be pending")
	}
}

func TestMigrateFromJSON_MissingStoryInMd(t *testing.T) {
	tmpDir := t.TempDir()

	// prd.json has a story that doesn't exist in prd.md
	jsonContent := `{
  "project": "Test",
  "userStories": [
    {"id": "US-001", "title": "Exists", "passes": true, "priority": 1},
    {"id": "US-999", "title": "Missing", "passes": true, "priority": 2}
  ]
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "prd.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	mdContent := `# Test

### US-001: Exists
- [ ] A
`
	if err := os.WriteFile(filepath.Join(tmpDir, "prd.md"), []byte(mdContent), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Should not error — missing stories are skipped
	if err := MigrateFromJSON(tmpDir); err != nil {
		t.Fatalf("MigrateFromJSON() error = %v", err)
	}

	// Verify the existing story was migrated
	p, err := ParseMarkdownPRD(filepath.Join(tmpDir, "prd.md"))
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if !p.UserStories[0].Passes {
		t.Error("US-001 should be passes")
	}
}

func TestMigrateFromJSON_NoJsonFile(t *testing.T) {
	tmpDir := t.TempDir()

	mdContent := `# Test
### US-001: First
- [ ] A
`
	if err := os.WriteFile(filepath.Join(tmpDir, "prd.md"), []byte(mdContent), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	err := MigrateFromJSON(tmpDir)
	if err == nil {
		t.Error("expected error when prd.json doesn't exist")
	}
}

func TestMigrateFromJSON_NoMdFile(t *testing.T) {
	tmpDir := t.TempDir()

	jsonContent := `{"project": "Test", "userStories": [{"id": "US-001", "passes": true, "priority": 1}]}`
	if err := os.WriteFile(filepath.Join(tmpDir, "prd.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	err := MigrateFromJSON(tmpDir)
	if err == nil {
		t.Error("expected error when prd.md doesn't exist")
	}
}
