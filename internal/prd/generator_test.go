package prd

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanJSONOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON",
			input:    `{"project": "test"}`,
			expected: `{"project": "test"}`,
		},
		{
			name:     "with json code block",
			input:    "```json\n{\"project\": \"test\"}\n```",
			expected: `{"project": "test"}`,
		},
		{
			name:     "with plain code block",
			input:    "```\n{\"project\": \"test\"}\n```",
			expected: `{"project": "test"}`,
		},
		{
			name:     "with extra whitespace",
			input:    "  \n{\"project\": \"test\"}\n  ",
			expected: `{"project": "test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanJSONOutput(tt.input)
			if result != tt.expected {
				t.Errorf("cleanJSONOutput() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid JSON object",
			input:   `{"project": "test", "stories": []}`,
			wantErr: false,
		},
		{
			name:    "valid JSON array",
			input:   `[1, 2, 3]`,
			wantErr: false,
		},
		{
			name:    "valid nested JSON",
			input:   `{"project": "test", "userStories": [{"id": "US-001", "title": "Test"}]}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON - missing closing brace",
			input:   `{"project": "test"`,
			wantErr: true,
		},
		{
			name:    "invalid JSON - trailing comma",
			input:   `{"project": "test",}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON - plain text",
			input:   `This is not JSON`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNeedsConversion(t *testing.T) {
	t.Run("no prd.md exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		needs, err := NeedsConversion(tmpDir)
		if err != nil {
			t.Errorf("NeedsConversion() unexpected error: %v", err)
		}
		if needs {
			t.Error("NeedsConversion() = true, want false when no prd.md exists")
		}
	})

	t.Run("prd.md exists but prd.json does not", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdMdPath := filepath.Join(tmpDir, "prd.md")
		if err := os.WriteFile(prdMdPath, []byte("# Test PRD"), 0644); err != nil {
			t.Fatalf("Failed to create prd.md: %v", err)
		}

		needs, err := NeedsConversion(tmpDir)
		if err != nil {
			t.Errorf("NeedsConversion() unexpected error: %v", err)
		}
		if !needs {
			t.Error("NeedsConversion() = false, want true when prd.json doesn't exist")
		}
	})

	t.Run("prd.md is newer than prd.json", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create prd.json first
		prdJsonPath := filepath.Join(tmpDir, "prd.json")
		if err := os.WriteFile(prdJsonPath, []byte(`{"project":"test"}`), 0644); err != nil {
			t.Fatalf("Failed to create prd.json: %v", err)
		}

		// Wait a moment to ensure different timestamps
		time.Sleep(100 * time.Millisecond)

		// Create prd.md after (so it's newer)
		prdMdPath := filepath.Join(tmpDir, "prd.md")
		if err := os.WriteFile(prdMdPath, []byte("# Test PRD"), 0644); err != nil {
			t.Fatalf("Failed to create prd.md: %v", err)
		}

		needs, err := NeedsConversion(tmpDir)
		if err != nil {
			t.Errorf("NeedsConversion() unexpected error: %v", err)
		}
		if !needs {
			t.Error("NeedsConversion() = false, want true when prd.md is newer")
		}
	})

	t.Run("prd.json is newer than prd.md", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create prd.md first
		prdMdPath := filepath.Join(tmpDir, "prd.md")
		if err := os.WriteFile(prdMdPath, []byte("# Test PRD"), 0644); err != nil {
			t.Fatalf("Failed to create prd.md: %v", err)
		}

		// Wait a moment to ensure different timestamps
		time.Sleep(100 * time.Millisecond)

		// Create prd.json after (so it's newer)
		prdJsonPath := filepath.Join(tmpDir, "prd.json")
		if err := os.WriteFile(prdJsonPath, []byte(`{"project":"test"}`), 0644); err != nil {
			t.Fatalf("Failed to create prd.json: %v", err)
		}

		needs, err := NeedsConversion(tmpDir)
		if err != nil {
			t.Errorf("NeedsConversion() unexpected error: %v", err)
		}
		if needs {
			t.Error("NeedsConversion() = true, want false when prd.json is newer")
		}
	})
}

func TestConvertMissingPrdMd(t *testing.T) {
	tmpDir := t.TempDir()

	err := Convert(ConvertOptions{PRDDir: tmpDir})
	if err == nil {
		t.Error("Convert() expected error when prd.md is missing")
	}
}

// Note: Full integration tests for Convert() would require Claude to be available.
// These tests focus on the pre-conversion validation logic.

func TestSamplePRDMarkdown(t *testing.T) {
	// Test that a sample prd.md structure is recognized
	// This verifies the file detection logic, not the actual conversion
	tmpDir := t.TempDir()

	sampleMd := `# My Test Project

A sample project for testing.

## User Stories

### US-001: Setup Project
As a developer, I need a properly structured project.

**Acceptance Criteria:**
- Create project structure
- Add dependencies
- Verify build works

### US-002: Add Feature
As a user, I want a new feature.

**Acceptance Criteria:**
- Feature works correctly
- Tests pass
`
	prdMdPath := filepath.Join(tmpDir, "prd.md")
	if err := os.WriteFile(prdMdPath, []byte(sampleMd), 0644); err != nil {
		t.Fatalf("Failed to create sample prd.md: %v", err)
	}

	// Verify the file can be detected for conversion
	needs, err := NeedsConversion(tmpDir)
	if err != nil {
		t.Errorf("NeedsConversion() unexpected error: %v", err)
	}
	if !needs {
		t.Error("Sample prd.md should trigger conversion need")
	}
}
