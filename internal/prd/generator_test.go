package prd

import (
	"os"
	"path/filepath"
	"strings"
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
		{
			name:     "with conversational preamble",
			input:    "Since the file write is being denied, here's the JSON output directly:\n\n{\"project\": \"test\"}",
			expected: `{"project": "test"}`,
		},
		{
			name:     "with preamble and nested objects",
			input:    "Here is the JSON:\n{\"project\": \"test\", \"userStories\": [{\"id\": \"US-001\"}]}",
			expected: `{"project": "test", "userStories": [{"id": "US-001"}]}`,
		},
		{
			name:     "with preamble and trailing text",
			input:    "Here you go:\n{\"project\": \"test\"}\nLet me know if you need changes.",
			expected: `{"project": "test"}`,
		},
		{
			name:     "with code fence and preamble",
			input:    "Here is the output:\n```json\n{\"project\": \"test\"}\n```",
			expected: `{"project": "test"}`,
		},
		{
			name:     "JSON with escaped quotes in preamble scenario",
			input:    "Output:\n{\"project\": \"test \\\"quoted\\\"\"}",
			expected: `{"project": "test \"quoted\""}`,
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

func TestHasProgress(t *testing.T) {
	tests := []struct {
		name     string
		prd      *PRD
		expected bool
	}{
		{
			name:     "nil PRD",
			prd:      nil,
			expected: false,
		},
		{
			name:     "empty PRD",
			prd:      &PRD{},
			expected: false,
		},
		{
			name: "no progress",
			prd: &PRD{
				UserStories: []UserStory{
					{ID: "US-001", Passes: false, InProgress: false},
					{ID: "US-002", Passes: false, InProgress: false},
				},
			},
			expected: false,
		},
		{
			name: "one story passes",
			prd: &PRD{
				UserStories: []UserStory{
					{ID: "US-001", Passes: true, InProgress: false},
					{ID: "US-002", Passes: false, InProgress: false},
				},
			},
			expected: true,
		},
		{
			name: "one story in progress",
			prd: &PRD{
				UserStories: []UserStory{
					{ID: "US-001", Passes: false, InProgress: true},
					{ID: "US-002", Passes: false, InProgress: false},
				},
			},
			expected: true,
		},
		{
			name: "all stories pass",
			prd: &PRD{
				UserStories: []UserStory{
					{ID: "US-001", Passes: true},
					{ID: "US-002", Passes: true},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasProgress(tt.prd)
			if result != tt.expected {
				t.Errorf("HasProgress() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMergeProgress(t *testing.T) {
	t.Run("nil PRDs", func(t *testing.T) {
		// Should not panic
		MergeProgress(nil, nil)
		MergeProgress(&PRD{}, nil)
		MergeProgress(nil, &PRD{})
	})

	t.Run("matching story IDs - preserve status", func(t *testing.T) {
		oldPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Title: "Old Title 1", Passes: true, InProgress: false},
				{ID: "US-002", Title: "Old Title 2", Passes: false, InProgress: true},
				{ID: "US-003", Title: "Old Title 3", Passes: false, InProgress: false},
			},
		}
		newPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Title: "New Title 1", Passes: false, InProgress: false},
				{ID: "US-002", Title: "New Title 2", Passes: false, InProgress: false},
				{ID: "US-003", Title: "New Title 3", Passes: false, InProgress: false},
			},
		}

		MergeProgress(oldPRD, newPRD)

		// US-001 should have passes: true preserved
		if !newPRD.UserStories[0].Passes {
			t.Error("US-001 should have Passes: true after merge")
		}
		// US-002 should have inProgress: true preserved
		if !newPRD.UserStories[1].InProgress {
			t.Error("US-002 should have InProgress: true after merge")
		}
		// US-003 should remain unchanged (no progress)
		if newPRD.UserStories[2].Passes || newPRD.UserStories[2].InProgress {
			t.Error("US-003 should not have any progress after merge")
		}
	})

	t.Run("new stories added - no progress", func(t *testing.T) {
		oldPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Passes: true},
			},
		}
		newPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Passes: false},
				{ID: "US-002", Passes: false}, // New story
			},
		}

		MergeProgress(oldPRD, newPRD)

		// US-001 should have progress preserved
		if !newPRD.UserStories[0].Passes {
			t.Error("US-001 should have Passes: true after merge")
		}
		// US-002 is new, should have no progress
		if newPRD.UserStories[1].Passes || newPRD.UserStories[1].InProgress {
			t.Error("New story US-002 should not have any progress")
		}
	})

	t.Run("removed stories are dropped", func(t *testing.T) {
		oldPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Passes: true},
				{ID: "US-002", Passes: true}, // Will be removed
			},
		}
		newPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Passes: false},
				// US-002 removed from new PRD
			},
		}

		MergeProgress(oldPRD, newPRD)

		// Only US-001 should exist
		if len(newPRD.UserStories) != 1 {
			t.Errorf("Expected 1 story, got %d", len(newPRD.UserStories))
		}
		if newPRD.UserStories[0].ID != "US-001" {
			t.Errorf("Expected US-001, got %s", newPRD.UserStories[0].ID)
		}
		if !newPRD.UserStories[0].Passes {
			t.Error("US-001 should have Passes: true after merge")
		}
	})

	t.Run("mixed scenario - add, remove, keep", func(t *testing.T) {
		oldPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Passes: true},         // Keep with progress
				{ID: "US-002", Passes: true},         // Removed
				{ID: "US-003", InProgress: true},     // Keep with progress
				{ID: "US-004", Passes: false},        // Keep without progress
			},
		}
		newPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Passes: false}, // Existing
				{ID: "US-003", Passes: false}, // Existing
				{ID: "US-004", Passes: false}, // Existing
				{ID: "US-005", Passes: false}, // New
			},
		}

		MergeProgress(oldPRD, newPRD)

		// Verify each story
		storyMap := make(map[string]*UserStory)
		for i := range newPRD.UserStories {
			storyMap[newPRD.UserStories[i].ID] = &newPRD.UserStories[i]
		}

		if s, ok := storyMap["US-001"]; !ok || !s.Passes {
			t.Error("US-001 should exist with Passes: true")
		}
		if _, ok := storyMap["US-002"]; ok {
			t.Error("US-002 should be removed")
		}
		if s, ok := storyMap["US-003"]; !ok || !s.InProgress {
			t.Error("US-003 should exist with InProgress: true")
		}
		if s, ok := storyMap["US-004"]; !ok || s.Passes || s.InProgress {
			t.Error("US-004 should exist without progress")
		}
		if s, ok := storyMap["US-005"]; !ok || s.Passes || s.InProgress {
			t.Error("US-005 should exist without progress (new story)")
		}
	})

	t.Run("reordered stories - preserves progress by ID", func(t *testing.T) {
		oldPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-001", Priority: 1, Passes: true},
				{ID: "US-002", Priority: 2, Passes: false},
				{ID: "US-003", Priority: 3, InProgress: true},
			},
		}
		newPRD := &PRD{
			UserStories: []UserStory{
				{ID: "US-003", Priority: 1, Passes: false}, // Moved to top
				{ID: "US-001", Priority: 2, Passes: false}, // Moved down
				{ID: "US-002", Priority: 3, Passes: false}, // Moved down
			},
		}

		MergeProgress(oldPRD, newPRD)

		// Verify progress is preserved regardless of order
		if !newPRD.UserStories[0].InProgress {
			t.Error("US-003 should have InProgress: true after merge")
		}
		if !newPRD.UserStories[1].Passes {
			t.Error("US-001 should have Passes: true after merge")
		}
		if newPRD.UserStories[2].Passes || newPRD.UserStories[2].InProgress {
			t.Error("US-002 should not have progress after merge")
		}
	})
}

func TestLoadAndValidateConvertedPRD(t *testing.T) {
	t.Run("valid prd.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdJsonPath := filepath.Join(tmpDir, "prd.json")
		content := `{
  "project": "Test Project",
  "description": "A test project",
  "userStories": [
    {
      "id": "US-001",
      "title": "First Story",
      "description": "Do something",
      "acceptanceCriteria": ["It works"],
      "priority": 1,
      "passes": false
    }
  ]
}`
		if err := os.WriteFile(prdJsonPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test prd.json: %v", err)
		}

		prd, err := loadAndValidateConvertedPRD(prdJsonPath)
		if err != nil {
			t.Errorf("loadAndValidateConvertedPRD() unexpected error: %v", err)
		}
		if prd == nil {
			t.Fatal("Expected non-nil PRD")
		}
		if prd.Project != "Test Project" {
			t.Errorf("Expected project 'Test Project', got %q", prd.Project)
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdJsonPath := filepath.Join(tmpDir, "prd.json")

		_, err := loadAndValidateConvertedPRD(prdJsonPath)
		if err == nil {
			t.Error("Expected error when prd.json does not exist")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdJsonPath := filepath.Join(tmpDir, "prd.json")
		if err := os.WriteFile(prdJsonPath, []byte(`{invalid json`), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := loadAndValidateConvertedPRD(prdJsonPath)
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("missing project field", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdJsonPath := filepath.Join(tmpDir, "prd.json")
		content := `{
  "project": "",
  "description": "A test",
  "userStories": [{"id": "US-001", "title": "Story", "description": "Desc", "acceptanceCriteria": [], "priority": 1, "passes": false}]
}`
		if err := os.WriteFile(prdJsonPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := loadAndValidateConvertedPRD(prdJsonPath)
		if err == nil {
			t.Error("Expected error for missing project field")
		}
		if err != nil && !strings.Contains(err.Error(), "project") {
			t.Errorf("Expected error about 'project' field, got: %v", err)
		}
	})

	t.Run("no user stories", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdJsonPath := filepath.Join(tmpDir, "prd.json")
		content := `{
  "project": "Test",
  "description": "A test",
  "userStories": []
}`
		if err := os.WriteFile(prdJsonPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err := loadAndValidateConvertedPRD(prdJsonPath)
		if err == nil {
			t.Error("Expected error for empty user stories")
		}
		if err != nil && !strings.Contains(err.Error(), "user stories") {
			t.Errorf("Expected error about 'user stories', got: %v", err)
		}
	})

	t.Run("JSON with escaped quotes parses correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdJsonPath := filepath.Join(tmpDir, "prd.json")
		content := `{
  "project": "Test Project",
  "description": "A project with \"quoted\" text",
  "userStories": [
    {
      "id": "US-001",
      "title": "Story with \"quotes\"",
      "description": "Click the \"Submit\" button",
      "acceptanceCriteria": ["User sees \"Success\" message", "Button says \"OK\""],
      "priority": 1,
      "passes": false
    }
  ]
}`
		if err := os.WriteFile(prdJsonPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		prd, err := loadAndValidateConvertedPRD(prdJsonPath)
		if err != nil {
			t.Errorf("loadAndValidateConvertedPRD() unexpected error: %v", err)
		}
		if prd == nil {
			t.Fatal("Expected non-nil PRD")
		}
		// Verify the escaped quotes are properly parsed
		if prd.UserStories[0].Title != `Story with "quotes"` {
			t.Errorf("Expected title with unescaped quotes, got %q", prd.UserStories[0].Title)
		}
		if prd.UserStories[0].AcceptanceCriteria[0] != `User sees "Success" message` {
			t.Errorf("Expected acceptance criteria with unescaped quotes, got %q", prd.UserStories[0].AcceptanceCriteria[0])
		}
	})
}

// Note: Full integration tests for Convert(), runClaudeConversion(), runClaudeJSONFix(),
// and waitWithSpinner() require Claude to be available and are not included here.

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
