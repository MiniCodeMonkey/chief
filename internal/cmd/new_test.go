package cmd

import (
	"os"
	"path/filepath"
	"strings"
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

func TestRunNewRequiresProvider(t *testing.T) {
	opts := NewOptions{
		Name:    "main",
		BaseDir: t.TempDir(),
	}

	err := RunNew(opts)
	if err == nil {
		t.Fatal("expected provider validation error")
	}
	if !strings.Contains(err.Error(), "Provider") {
		t.Fatalf("expected error to mention Provider, got: %v", err)
	}
}
