package prd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKnowledgePath(t *testing.T) {
	got := KnowledgePath("/foo/bar/.chief/prds/my-prd/prd.json")
	want := "/foo/bar/.chief/prds/my-prd/knowledge.json"
	if got != want {
		t.Errorf("KnowledgePath() = %q, want %q", got, want)
	}
}

func TestLoadKnowledge_MissingFile(t *testing.T) {
	k, err := LoadKnowledge("/nonexistent/knowledge.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if k == nil {
		t.Fatal("expected non-nil Knowledge for missing file")
	}
	if len(k.Patterns) != 0 {
		t.Errorf("expected empty patterns, got %d", len(k.Patterns))
	}
	if len(k.CompletedStories) != 0 {
		t.Errorf("expected empty completedStories, got %d", len(k.CompletedStories))
	}
}

func TestLoadSaveKnowledge_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "knowledge.json")

	original := &Knowledge{
		Patterns: []string{"Use X for Y", "Always check Z"},
		CompletedStories: map[string]CompletedStoryRecord{
			"US-001": {
				FilesChanged: []string{"internal/prd/types.go", "internal/prd/prd_test.go"},
				Approach:     "Added DependsOn field and updated NextStory()",
				Learnings:    []string{"Priority 1 = highest", "Tests in prd_test.go"},
			},
		},
	}

	// Save
	if err := SaveKnowledge(path, original); err != nil {
		t.Fatalf("SaveKnowledge failed: %v", err)
	}

	// Load
	loaded, err := LoadKnowledge(path)
	if err != nil {
		t.Fatalf("LoadKnowledge failed: %v", err)
	}

	// Verify patterns
	if len(loaded.Patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(loaded.Patterns))
	}
	if loaded.Patterns[0] != "Use X for Y" {
		t.Errorf("expected first pattern 'Use X for Y', got %q", loaded.Patterns[0])
	}
	if loaded.Patterns[1] != "Always check Z" {
		t.Errorf("expected second pattern 'Always check Z', got %q", loaded.Patterns[1])
	}

	// Verify completed stories
	record, ok := loaded.CompletedStories["US-001"]
	if !ok {
		t.Fatal("expected US-001 in completedStories")
	}
	if len(record.FilesChanged) != 2 {
		t.Errorf("expected 2 files changed, got %d", len(record.FilesChanged))
	}
	if record.Approach != "Added DependsOn field and updated NextStory()" {
		t.Errorf("unexpected approach: %q", record.Approach)
	}
	if len(record.Learnings) != 2 {
		t.Errorf("expected 2 learnings, got %d", len(record.Learnings))
	}
}

func TestLoadKnowledge_NullFields(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "knowledge.json")

	// Write JSON with null/missing fields
	content := `{"patterns": null, "completedStories": null}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	k, err := LoadKnowledge(path)
	if err != nil {
		t.Fatalf("LoadKnowledge failed: %v", err)
	}

	// Ensure slices/maps are initialized even with null JSON
	if k.Patterns == nil {
		t.Error("expected Patterns to be initialized, got nil")
	}
	if k.CompletedStories == nil {
		t.Error("expected CompletedStories to be initialized, got nil")
	}
}

func TestLoadKnowledge_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "knowledge.json")

	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadKnowledge(path)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestLoadSaveKnowledge_CriteriaResults(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "knowledge.json")

	original := &Knowledge{
		Patterns: []string{},
		CompletedStories: map[string]CompletedStoryRecord{
			"US-004": {
				FilesChanged: []string{"knowledge.go"},
				Approach:     "Added criteria results",
				Learnings:    []string{"Test criteria round-trip"},
				CriteriaResults: []CriteriaResult{
					{Criterion: "Typecheck passes", Passed: true, Evidence: "go build succeeded"},
					{Criterion: "Tests pass", Passed: false, Evidence: "TestFoo failed: expected 1 got 2"},
				},
			},
		},
	}

	if err := SaveKnowledge(path, original); err != nil {
		t.Fatalf("SaveKnowledge failed: %v", err)
	}

	loaded, err := LoadKnowledge(path)
	if err != nil {
		t.Fatalf("LoadKnowledge failed: %v", err)
	}

	record, ok := loaded.CompletedStories["US-004"]
	if !ok {
		t.Fatal("expected US-004 in completedStories")
	}
	if len(record.CriteriaResults) != 2 {
		t.Fatalf("expected 2 criteria results, got %d", len(record.CriteriaResults))
	}
	if record.CriteriaResults[0].Criterion != "Typecheck passes" {
		t.Errorf("unexpected criterion: %q", record.CriteriaResults[0].Criterion)
	}
	if !record.CriteriaResults[0].Passed {
		t.Error("expected first criterion to pass")
	}
	if record.CriteriaResults[1].Passed {
		t.Error("expected second criterion to fail")
	}
	if record.CriteriaResults[1].Evidence != "TestFoo failed: expected 1 got 2" {
		t.Errorf("unexpected evidence: %q", record.CriteriaResults[1].Evidence)
	}
}

func TestLoadKnowledge_NoCriteriaResults(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "knowledge.json")

	// Write JSON without criteriaResults field (backward compatibility)
	content := `{"patterns": [], "completedStories": {"US-001": {"filesChanged": ["a.go"], "approach": "test", "learnings": []}}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	k, err := LoadKnowledge(path)
	if err != nil {
		t.Fatalf("LoadKnowledge failed: %v", err)
	}

	record := k.CompletedStories["US-001"]
	if record.CriteriaResults != nil {
		t.Errorf("expected nil CriteriaResults for old format, got %v", record.CriteriaResults)
	}
}

func TestLoadKnowledge_EmptyCompletedStories(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "knowledge.json")

	content := `{"patterns": ["pattern1"], "completedStories": {}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	k, err := LoadKnowledge(path)
	if err != nil {
		t.Fatalf("LoadKnowledge failed: %v", err)
	}

	if len(k.Patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(k.Patterns))
	}
	if len(k.CompletedStories) != 0 {
		t.Errorf("expected 0 completed stories, got %d", len(k.CompletedStories))
	}
}
