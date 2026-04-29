package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddChiefToGitignore(t *testing.T) {
	t.Run("creates new gitignore", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		err := AddChiefToGitignore(dir)
		if err != nil {
			t.Fatalf("AddChiefToGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		if string(content) != ".chief/\n" {
			t.Errorf("got %q, want %q", string(content), ".chief/\n")
		}
	})

	t.Run("appends to existing gitignore", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		// Create existing .gitignore
		if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		err := AddChiefToGitignore(dir)
		if err != nil {
			t.Fatalf("AddChiefToGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		expected := "node_modules/\n.chief/\n"
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

		err := AddChiefToGitignore(dir)
		if err != nil {
			t.Fatalf("AddChiefToGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		expected := "node_modules/\n.chief/\n"
		if string(content) != expected {
			t.Errorf("got %q, want %q", string(content), expected)
		}
	})

	t.Run("skips if already present", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		// Create existing .gitignore with .chief already present
		original := "node_modules/\n.chief/\n"
		if err := os.WriteFile(gitignorePath, []byte(original), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		err := AddChiefToGitignore(dir)
		if err != nil {
			t.Fatalf("AddChiefToGitignore() error = %v", err)
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

	t.Run("skips if .chief without slash present", func(t *testing.T) {
		dir := t.TempDir()
		gitignorePath := filepath.Join(dir, ".gitignore")

		// Create existing .gitignore with .chief (no slash)
		original := "node_modules/\n.chief\n"
		if err := os.WriteFile(gitignorePath, []byte(original), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		err := AddChiefToGitignore(dir)
		if err != nil {
			t.Fatalf("AddChiefToGitignore() error = %v", err)
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

func TestIsDefaultBranchName(t *testing.T) {
	tests := []struct {
		branch   string
		expected bool
	}{
		{"main", true},
		{"master", true},
		{"develop", false},
		{"feature/foo", false},
		{"chief/my-prd", false},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			result := isDefaultBranchName(tt.branch)
			if result != tt.expected {
				t.Errorf("isDefaultBranchName(%q) = %v, want %v", tt.branch, result, tt.expected)
			}
		})
	}
}
