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

func TestLoadSaveKnowledge_Attempts(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "knowledge.json")

	original := &Knowledge{
		Patterns: []string{},
		CompletedStories: map[string]CompletedStoryRecord{
			"US-005": {
				FilesChanged: []string{"knowledge.go"},
				Approach:     "Current approach",
				Learnings:    []string{},
				Attempts: []Attempt{
					{
						Approach: "First failed approach",
						CriteriaResults: []CriteriaResult{
							{Criterion: "Tests pass", Passed: false, Evidence: "2 tests failed"},
						},
						FailureAnalysis: "Did not handle edge case X",
					},
					{
						Approach:        "Second failed approach",
						CriteriaResults: []CriteriaResult{},
						FailureAnalysis: "Wrong abstraction, should use Y instead",
					},
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

	record := loaded.CompletedStories["US-005"]
	if len(record.Attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(record.Attempts))
	}
	if record.Attempts[0].Approach != "First failed approach" {
		t.Errorf("unexpected first attempt approach: %q", record.Attempts[0].Approach)
	}
	if record.Attempts[0].FailureAnalysis != "Did not handle edge case X" {
		t.Errorf("unexpected first attempt failureAnalysis: %q", record.Attempts[0].FailureAnalysis)
	}
	if len(record.Attempts[0].CriteriaResults) != 1 {
		t.Fatalf("expected 1 criteria result in first attempt, got %d", len(record.Attempts[0].CriteriaResults))
	}
	if record.Attempts[0].CriteriaResults[0].Passed {
		t.Error("expected first attempt criteria to have failed")
	}
	if record.Attempts[1].FailureAnalysis != "Wrong abstraction, should use Y instead" {
		t.Errorf("unexpected second attempt failureAnalysis: %q", record.Attempts[1].FailureAnalysis)
	}
}

func TestLoadKnowledge_NoAttempts(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "knowledge.json")

	// Write JSON without attempts field (backward compatibility)
	content := `{"patterns": [], "completedStories": {"US-001": {"filesChanged": ["a.go"], "approach": "test", "learnings": []}}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	k, err := LoadKnowledge(path)
	if err != nil {
		t.Fatalf("LoadKnowledge failed: %v", err)
	}

	record := k.CompletedStories["US-001"]
	if record.Attempts != nil {
		t.Errorf("expected nil Attempts for old format, got %v", record.Attempts)
	}
}

func TestKnowledge_ExhaustedStoryIDs(t *testing.T) {
	k := &Knowledge{
		Patterns: []string{},
		CompletedStories: map[string]CompletedStoryRecord{
			"US-001": {Attempts: []Attempt{{}, {}, {}}},       // 3 attempts = exhausted
			"US-002": {Attempts: []Attempt{{}, {}}},           // 2 attempts = not exhausted
			"US-003": {Attempts: nil},                         // no attempts = not exhausted
			"US-004": {Attempts: []Attempt{{}, {}, {}, {}}},   // 4 attempts = exhausted
		},
	}

	exhausted := k.ExhaustedStoryIDs()
	if !exhausted["US-001"] {
		t.Error("expected US-001 to be exhausted (3 attempts)")
	}
	if exhausted["US-002"] {
		t.Error("expected US-002 to NOT be exhausted (2 attempts)")
	}
	if exhausted["US-003"] {
		t.Error("expected US-003 to NOT be exhausted (no attempts)")
	}
	if !exhausted["US-004"] {
		t.Error("expected US-004 to be exhausted (4 attempts)")
	}
}

func TestKnowledge_ExhaustedStoryIDs_Empty(t *testing.T) {
	k := &Knowledge{
		Patterns:         []string{},
		CompletedStories: map[string]CompletedStoryRecord{},
	}

	exhausted := k.ExhaustedStoryIDs()
	if len(exhausted) != 0 {
		t.Errorf("expected empty exhausted set, got %d", len(exhausted))
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
