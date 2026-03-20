package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunStatusWithValidPRD(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PRD directory with prd.json
	prdDir := filepath.Join(tmpDir, ".chief", "prds", "test")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create a test prd.json
	prdJSON := `{
  "project": "Test Project",
  "description": "Test description",
  "userStories": [
    {"id": "US-001", "title": "Story 1", "passes": true, "priority": 1},
    {"id": "US-002", "title": "Story 2", "passes": false, "priority": 2},
    {"id": "US-003", "title": "Story 3", "passes": false, "inProgress": true, "priority": 3}
  ]
}`
	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(prdJSON), 0644); err != nil {
		t.Fatalf("Failed to create prd.json: %v", err)
	}

	opts := StatusOptions{
		Name:    "test",
		BaseDir: tmpDir,
	}

	// Should not return error
	err := RunStatus(opts)
	if err != nil {
		t.Errorf("RunStatus() returned error: %v", err)
	}
}

func TestRunStatusWithDefaultName(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PRD directory with prd.json using default name "main"
	prdDir := filepath.Join(tmpDir, ".chief", "prds", "main")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	prdJSON := `{
  "project": "Main Project",
  "userStories": []
}`
	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(prdJSON), 0644); err != nil {
		t.Fatalf("Failed to create prd.json: %v", err)
	}

	opts := StatusOptions{
		Name:    "", // Empty should default to "main"
		BaseDir: tmpDir,
	}

	err := RunStatus(opts)
	if err != nil {
		t.Errorf("RunStatus() with default name returned error: %v", err)
	}
}

func TestRunStatusWithMissingPRD(t *testing.T) {
	tmpDir := t.TempDir()

	opts := StatusOptions{
		Name:    "nonexistent",
		BaseDir: tmpDir,
	}

	err := RunStatus(opts)
	if err == nil {
		t.Error("Expected error for missing PRD")
	}
}

func TestRunListWithNoPRDs(t *testing.T) {
	tmpDir := t.TempDir()

	opts := ListOptions{
		BaseDir: tmpDir,
	}

	// Should not return error, just print "No PRDs found"
	err := RunList(opts)
	if err != nil {
		t.Errorf("RunList() returned error: %v", err)
	}
}

func TestRunListWithPRDs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple PRD directories
	prds := []struct {
		name    string
		project string
		stories string
	}{
		{
			"auth",
			"Authentication",
			`[{"id": "US-001", "title": "Login", "passes": true, "priority": 1},
			 {"id": "US-002", "title": "Logout", "passes": false, "priority": 2}]`,
		},
		{
			"api",
			"API Service",
			`[{"id": "US-001", "title": "Endpoints", "passes": true, "priority": 1},
			 {"id": "US-002", "title": "Auth", "passes": true, "priority": 2},
			 {"id": "US-003", "title": "Rate limiting", "passes": true, "priority": 3}]`,
		},
	}

	for _, p := range prds {
		prdDir := filepath.Join(tmpDir, ".chief", "prds", p.name)
		if err := os.MkdirAll(prdDir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		prdJSON := `{"project": "` + p.project + `", "userStories": ` + p.stories + `}`
		prdPath := filepath.Join(prdDir, "prd.json")
		if err := os.WriteFile(prdPath, []byte(prdJSON), 0644); err != nil {
			t.Fatalf("Failed to create prd.json: %v", err)
		}
	}

	opts := ListOptions{
		BaseDir: tmpDir,
	}

	err := RunList(opts)
	if err != nil {
		t.Errorf("RunList() returned error: %v", err)
	}
}

func TestRunListSkipsInvalidPRDs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid PRD
	validDir := filepath.Join(tmpDir, ".chief", "prds", "valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	validJSON := `{"project": "Valid", "userStories": []}`
	if err := os.WriteFile(filepath.Join(validDir, "prd.json"), []byte(validJSON), 0644); err != nil {
		t.Fatalf("Failed to create prd.json: %v", err)
	}

	// Create an invalid PRD directory (no prd.json)
	invalidDir := filepath.Join(tmpDir, ".chief", "prds", "invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create another invalid PRD (invalid JSON)
	badJsonDir := filepath.Join(tmpDir, ".chief", "prds", "badjson")
	if err := os.MkdirAll(badJsonDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badJsonDir, "prd.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to create prd.json: %v", err)
	}

	opts := ListOptions{
		BaseDir: tmpDir,
	}

	// Should not return error, just skip invalid PRDs
	err := RunList(opts)
	if err != nil {
		t.Errorf("RunList() returned error: %v", err)
	}
}

func TestRunStatusAllComplete(t *testing.T) {
	tmpDir := t.TempDir()

	prdDir := filepath.Join(tmpDir, ".chief", "prds", "done")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	prdJSON := `{
  "project": "Complete Project",
  "userStories": [
    {"id": "US-001", "title": "Story 1", "passes": true, "priority": 1},
    {"id": "US-002", "title": "Story 2", "passes": true, "priority": 2}
  ]
}`
	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(prdJSON), 0644); err != nil {
		t.Fatalf("Failed to create prd.json: %v", err)
	}

	opts := StatusOptions{
		Name:    "done",
		BaseDir: tmpDir,
	}

	err := RunStatus(opts)
	if err != nil {
		t.Errorf("RunStatus() returned error: %v", err)
	}
}

func TestRunStatusEmptyPRD(t *testing.T) {
	tmpDir := t.TempDir()

	prdDir := filepath.Join(tmpDir, ".chief", "prds", "empty")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	prdJSON := `{"project": "Empty Project", "userStories": []}`
	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(prdJSON), 0644); err != nil {
		t.Fatalf("Failed to create prd.json: %v", err)
	}

	opts := StatusOptions{
		Name:    "empty",
		BaseDir: tmpDir,
	}

	err := RunStatus(opts)
	if err != nil {
		t.Errorf("RunStatus() returned error: %v", err)
	}
}
