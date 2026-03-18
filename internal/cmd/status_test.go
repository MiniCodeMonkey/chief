package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunStatusWithValidPRD(t *testing.T) {
	tmpDir := t.TempDir()

	prdDir := filepath.Join(tmpDir, ".chief", "prds", "test")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	prdMd := `# Test Project

Test description

### US-001: Story 1
**Status:** done
- [x] Done

### US-002: Story 2
- [ ] Pending

### US-003: Story 3
**Status:** in-progress
- [ ] Working
`
	prdPath := filepath.Join(prdDir, "prd.md")
	if err := os.WriteFile(prdPath, []byte(prdMd), 0644); err != nil {
		t.Fatalf("Failed to create prd.md: %v", err)
	}

	opts := StatusOptions{
		Name:    "test",
		BaseDir: tmpDir,
	}

	err := RunStatus(opts)
	if err != nil {
		t.Errorf("RunStatus() returned error: %v", err)
	}
}

func TestRunStatusWithDefaultName(t *testing.T) {
	tmpDir := t.TempDir()

	prdDir := filepath.Join(tmpDir, ".chief", "prds", "main")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	prdMd := "# Main Project\n"
	prdPath := filepath.Join(prdDir, "prd.md")
	if err := os.WriteFile(prdPath, []byte(prdMd), 0644); err != nil {
		t.Fatalf("Failed to create prd.md: %v", err)
	}

	opts := StatusOptions{
		Name:    "",
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

	err := RunList(opts)
	if err != nil {
		t.Errorf("RunList() returned error: %v", err)
	}
}

func TestRunListWithPRDs(t *testing.T) {
	tmpDir := t.TempDir()

	prds := []struct {
		name string
		md   string
	}{
		{
			"auth",
			"# Authentication\n\n### US-001: Login\n**Status:** done\n- [x] Works\n\n### US-002: Logout\n- [ ] Works\n",
		},
		{
			"api",
			"# API Service\n\n### US-001: Endpoints\n**Status:** done\n- [x] Done\n\n### US-002: Auth\n**Status:** done\n- [x] Done\n\n### US-003: Rate limiting\n**Status:** done\n- [x] Done\n",
		},
	}

	for _, p := range prds {
		prdDir := filepath.Join(tmpDir, ".chief", "prds", p.name)
		if err := os.MkdirAll(prdDir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		prdPath := filepath.Join(prdDir, "prd.md")
		if err := os.WriteFile(prdPath, []byte(p.md), 0644); err != nil {
			t.Fatalf("Failed to create prd.md: %v", err)
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
	if err := os.WriteFile(filepath.Join(validDir, "prd.md"), []byte("# Valid\n"), 0644); err != nil {
		t.Fatalf("Failed to create prd.md: %v", err)
	}

	// Create an invalid PRD directory (no prd.md)
	invalidDir := filepath.Join(tmpDir, ".chief", "prds", "invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	opts := ListOptions{
		BaseDir: tmpDir,
	}

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

	prdMd := "# Complete Project\n\n### US-001: Story 1\n**Status:** done\n- [x] Done\n\n### US-002: Story 2\n**Status:** done\n- [x] Done\n"
	prdPath := filepath.Join(prdDir, "prd.md")
	if err := os.WriteFile(prdPath, []byte(prdMd), 0644); err != nil {
		t.Fatalf("Failed to create prd.md: %v", err)
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

	prdMd := "# Empty Project\n"
	prdPath := filepath.Join(prdDir, "prd.md")
	if err := os.WriteFile(prdPath, []byte(prdMd), 0644); err != nil {
		t.Fatalf("Failed to create prd.md: %v", err)
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
