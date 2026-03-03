package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddMellizaToGitignore(t *testing.T) {
	t.Run("creates new gitignore", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		err := AddMellizaToGitignore(dir)
		if err != nil {
			t.Fatalf("AddMellizaToGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		if string(content) != ".melliza/\n" {
			t.Errorf("got %q, want %q", string(content), ".melliza/\n")
		}
	})

	t.Run("appends to existing gitignore", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		// Create existing .gitignore
		if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		err := AddMellizaToGitignore(dir)
		if err != nil {
			t.Fatalf("AddMellizaToGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		expected := "node_modules/\n.melliza/\n"
		if string(content) != expected {
			t.Errorf("got %q, want %q", string(content), expected)
		}
	})

	t.Run("appends newline if missing", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		// Create existing .gitignore without trailing newline
		if err := os.WriteFile(gitignorePath, []byte("node_modules/"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		err := AddMellizaToGitignore(dir)
		if err != nil {
			t.Fatalf("AddMellizaToGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		expected := "node_modules/\n.melliza/\n"
		if string(content) != expected {
			t.Errorf("got %q, want %q", string(content), expected)
		}
	})

	t.Run("skips if already present", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		// Create existing .gitignore with .melliza already present
		original := "node_modules/\n.melliza/\n"
		if err := os.WriteFile(gitignorePath, []byte(original), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		err := AddMellizaToGitignore(dir)
		if err != nil {
			t.Fatalf("AddMellizaToGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		// Should remain unchanged
		if string(content) != original {
			t.Errorf("got %q, want %q", string(content), original)
		}
	})

	t.Run("skips if .melliza without slash present", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		// Create existing .gitignore with .melliza (no slash)
		original := "node_modules/\n.melliza\n"
		if err := os.WriteFile(gitignorePath, []byte(original), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		err := AddMellizaToGitignore(dir)
		if err != nil {
			t.Fatalf("AddMellizaToGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		// Should remain unchanged
		if string(content) != original {
			t.Errorf("got %q, want %q", string(content), original)
		}
	})
}

func TestIsProtectedBranch(t *testing.T) {
	tests := []struct {
		branch   string
		expected bool
	}{
		{"main", true},
		{"master", true},
		{"develop", false},
		{"feature/foo", false},
		{"melliza/my-prd", false},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			result := IsProtectedBranch(tt.branch)
			if result != tt.expected {
				t.Errorf("IsProtectedBranch(%q) = %v, want %v", tt.branch, result, tt.expected)
			}
		})
	}
}
