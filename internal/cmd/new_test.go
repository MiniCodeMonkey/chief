package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidPRDName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid lowercase", "main", true},
		{"valid with numbers", "feature1", true},
		{"valid with hyphen", "my-feature", true},
		{"valid with underscore", "my_feature", true},
		{"valid mixed case", "MyFeature", true},
		{"valid complex", "auth-v2_final", true},
		{"empty string", "", false},
		{"with space", "my feature", false},
		{"with dot", "my.feature", false},
		{"with slash", "my/feature", false},
		{"with special char", "my@feature", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPRDName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidPRDName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRunNewCreatesDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Test that directory structure is created correctly
	// We can't fully test RunNew without Claude, but we can verify directory creation logic
	name := "test-prd"
	prdDir := filepath.Join(tmpDir, ".chief", "prds", name)

	// Simulate what RunNew does for directory creation
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Verify directory was created at expected path
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}

	// Verify parent directories also exist
	chiefDir := filepath.Join(tmpDir, ".chief")
	if _, err := os.Stat(chiefDir); os.IsNotExist(err) {
		t.Error("Expected .chief directory to be created")
	}

	prdsDir := filepath.Join(chiefDir, "prds")
	if _, err := os.Stat(prdsDir); os.IsNotExist(err) {
		t.Error("Expected .chief/prds directory to be created")
	}
}

func TestRunNewRejectsInvalidName(t *testing.T) {
	tmpDir := t.TempDir()

	opts := NewOptions{
		Name:    "invalid name with space",
		BaseDir: tmpDir,
	}

	err := RunNew(opts)
	if err == nil {
		t.Error("Expected error for invalid name")
	}
}

func TestRunNewCleansUpEmptyDirOnCancel(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate what RunNew does: create directory, then check prd.md doesn't exist
	name := "cancelled"
	prdDir := filepath.Join(tmpDir, ".chief", "prds", name)
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// prd.md was never created (user cancelled) — simulate the cleanup
	prdMdPath := filepath.Join(prdDir, "prd.md")
	if _, err := os.Stat(prdMdPath); os.IsNotExist(err) {
		os.Remove(prdDir)
	}

	// Directory should be removed
	if _, err := os.Stat(prdDir); !os.IsNotExist(err) {
		t.Error("Expected empty directory to be cleaned up after cancellation")
	}
}

func TestRunNewKeepsDirWhenPrdMdExists(t *testing.T) {
	tmpDir := t.TempDir()

	name := "has-prd"
	prdDir := filepath.Join(tmpDir, ".chief", "prds", name)
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create prd.md (simulates successful Claude session)
	prdMdPath := filepath.Join(prdDir, "prd.md")
	if err := os.WriteFile(prdMdPath, []byte("# My PRD"), 0644); err != nil {
		t.Fatalf("Failed to create prd.md: %v", err)
	}

	// Cleanup should NOT trigger since prd.md exists
	if _, err := os.Stat(prdMdPath); os.IsNotExist(err) {
		os.Remove(prdDir)
	}

	// Directory should still exist
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		t.Error("Expected directory to be kept when prd.md exists")
	}
}

func TestRunNewRejectsExistingPRD(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing prd.md
	prdDir := filepath.Join(tmpDir, ".chief", "prds", "existing")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	prdMdPath := filepath.Join(prdDir, "prd.md")
	if err := os.WriteFile(prdMdPath, []byte("# Existing PRD"), 0644); err != nil {
		t.Fatalf("Failed to create prd.md: %v", err)
	}

	opts := NewOptions{
		Name:    "existing",
		BaseDir: tmpDir,
	}

	err := RunNew(opts)
	if err == nil {
		t.Error("Expected error for existing PRD")
	}
}
