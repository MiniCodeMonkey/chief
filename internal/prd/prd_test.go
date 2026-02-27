package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPRD(t *testing.T) {
	// Create a temp file with valid PRD JSON
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	validJSON := `{
		"project": "Test Project",
		"description": "A test PRD",
		"userStories": [
			{
				"id": "US-001",
				"title": "First Story",
				"description": "Test description",
				"acceptanceCriteria": ["AC1", "AC2"],
				"priority": 1,
				"passes": false
			}
		]
	}`

	if err := os.WriteFile(prdPath, []byte(validJSON), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p, err := LoadPRD(prdPath)
	if err != nil {
		t.Fatalf("LoadPRD failed: %v", err)
	}

	if p.Project != "Test Project" {
		t.Errorf("expected project 'Test Project', got '%s'", p.Project)
	}
	if p.Description != "A test PRD" {
		t.Errorf("expected description 'A test PRD', got '%s'", p.Description)
	}
	if len(p.UserStories) != 1 {
		t.Errorf("expected 1 user story, got %d", len(p.UserStories))
	}
	if p.UserStories[0].ID != "US-001" {
		t.Errorf("expected story ID 'US-001', got '%s'", p.UserStories[0].ID)
	}
}

func TestLoadPRD_FileNotFound(t *testing.T) {
	_, err := LoadPRD("/nonexistent/path/prd.json")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestLoadPRD_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	if err := os.WriteFile(prdPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadPRD(prdPath)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestPRD_Save(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	p := &PRD{
		Project:     "Saved Project",
		Description: "A saved PRD",
		UserStories: []UserStory{
			{
				ID:                 "US-001",
				Title:              "Test Story",
				Description:        "Test",
				AcceptanceCriteria: []string{"AC1"},
				Priority:           1,
				Passes:             true,
			},
		},
	}

	if err := p.Save(prdPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify by loading it back
	loaded, err := LoadPRD(prdPath)
	if err != nil {
		t.Fatalf("LoadPRD after Save failed: %v", err)
	}

	if loaded.Project != p.Project {
		t.Errorf("expected project '%s', got '%s'", p.Project, loaded.Project)
	}
	if len(loaded.UserStories) != 1 {
		t.Errorf("expected 1 user story, got %d", len(loaded.UserStories))
	}
	if !loaded.UserStories[0].Passes {
		t.Error("expected story to have passes: true")
	}
}

func TestPRD_AllComplete_EmptyPRD(t *testing.T) {
	p := &PRD{
		Project:     "Empty",
		UserStories: []UserStory{},
	}

	if !p.AllComplete() {
		t.Error("expected AllComplete() to return true for empty PRD")
	}
}

func TestPRD_AllComplete_AllPassing(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Passes: true},
			{ID: "US-002", Passes: true},
			{ID: "US-003", Passes: true},
		},
	}

	if !p.AllComplete() {
		t.Error("expected AllComplete() to return true when all stories pass")
	}
}

func TestPRD_AllComplete_SomePending(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Passes: true},
			{ID: "US-002", Passes: false},
			{ID: "US-003", Passes: true},
		},
	}

	if p.AllComplete() {
		t.Error("expected AllComplete() to return false when some stories are pending")
	}
}

func TestPRD_NextStory_EmptyPRD(t *testing.T) {
	p := &PRD{
		Project:     "Empty",
		UserStories: []UserStory{},
	}

	next := p.NextStory()
	if next != nil {
		t.Errorf("expected nil for empty PRD, got %v", next)
	}
}

func TestPRD_NextStory_AllComplete(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Passes: true},
			{ID: "US-002", Passes: true},
		},
	}

	next := p.NextStory()
	if next != nil {
		t.Errorf("expected nil when all complete, got %v", next)
	}
}

func TestPRD_NextStory_InterruptedStory(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Priority: 1, Passes: false},
			{ID: "US-002", Priority: 2, Passes: false, InProgress: true},
			{ID: "US-003", Priority: 3, Passes: false},
		},
	}

	next := p.NextStory()
	if next == nil {
		t.Fatal("expected non-nil story")
	}
	if next.ID != "US-002" {
		t.Errorf("expected interrupted story US-002, got %s", next.ID)
	}
}

func TestPRD_NextStory_LowestPriority(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Priority: 3, Passes: false},
			{ID: "US-002", Priority: 1, Passes: false},
			{ID: "US-003", Priority: 2, Passes: true},
		},
	}

	next := p.NextStory()
	if next == nil {
		t.Fatal("expected non-nil story")
	}
	if next.ID != "US-002" {
		t.Errorf("expected lowest priority story US-002, got %s", next.ID)
	}
}

func TestPRD_NextStory_SkipsCompleted(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Priority: 1, Passes: true},
			{ID: "US-002", Priority: 2, Passes: false},
			{ID: "US-003", Priority: 3, Passes: false},
		},
	}

	next := p.NextStory()
	if next == nil {
		t.Fatal("expected non-nil story")
	}
	if next.ID != "US-002" {
		t.Errorf("expected US-002 (lowest priority not passing), got %s", next.ID)
	}
}

func TestPRD_NextStory_InterruptedTakesPrecedence(t *testing.T) {
	// Even if there's a lower priority story, in-progress takes precedence
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Priority: 1, Passes: false},
			{ID: "US-002", Priority: 5, Passes: false, InProgress: true},
		},
	}

	next := p.NextStory()
	if next == nil {
		t.Fatal("expected non-nil story")
	}
	if next.ID != "US-002" {
		t.Errorf("expected in-progress story US-002 to take precedence, got %s", next.ID)
	}
}

func TestUserStory_Fields(t *testing.T) {
	story := UserStory{
		ID:                 "US-TEST",
		Title:              "Test Title",
		Description:        "Test Description",
		AcceptanceCriteria: []string{"AC1", "AC2", "AC3"},
		Priority:           5,
		Passes:             true,
		InProgress:         false,
	}

	if story.ID != "US-TEST" {
		t.Errorf("expected ID 'US-TEST', got '%s'", story.ID)
	}
	if len(story.AcceptanceCriteria) != 3 {
		t.Errorf("expected 3 acceptance criteria, got %d", len(story.AcceptanceCriteria))
	}
}

func TestPRD_Save_PreservesInProgress(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{
				ID:         "US-001",
				Title:      "Story",
				Priority:   1,
				Passes:     false,
				InProgress: true,
			},
		},
	}

	if err := p.Save(prdPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadPRD(prdPath)
	if err != nil {
		t.Fatalf("LoadPRD failed: %v", err)
	}

	if !loaded.UserStories[0].InProgress {
		t.Error("expected InProgress to be preserved as true")
	}
}

func TestPRD_NextStoryContext_ReturnsHighestPriority(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Title: "Low priority", Priority: 3, Passes: false},
			{ID: "US-002", Title: "High priority", Priority: 1, Passes: false},
			{ID: "US-003", Title: "Mid priority", Priority: 2, Passes: false},
		},
	}

	ctx := p.NextStoryContext()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// Parse the JSON to verify it's the highest-priority story
	var story UserStory
	if err := json.Unmarshal([]byte(*ctx), &story); err != nil {
		t.Fatalf("failed to parse story context JSON: %v", err)
	}
	if story.ID != "US-002" {
		t.Errorf("expected highest-priority story US-002, got %s", story.ID)
	}
}

func TestPRD_NextStoryContext_ReturnsNilWhenAllComplete(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Passes: true},
			{ID: "US-002", Passes: true},
		},
	}

	ctx := p.NextStoryContext()
	if ctx != nil {
		t.Errorf("expected nil when all stories complete, got %q", *ctx)
	}
}

func TestPRD_NextStoryContext_SkipsPassingStories(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Title: "Done", Priority: 1, Passes: true},
			{ID: "US-002", Title: "Pending", Priority: 2, Passes: false},
		},
	}

	ctx := p.NextStoryContext()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	var story UserStory
	if err := json.Unmarshal([]byte(*ctx), &story); err != nil {
		t.Fatalf("failed to parse story context JSON: %v", err)
	}
	if story.ID != "US-002" {
		t.Errorf("expected US-002 (only pending story), got %s", story.ID)
	}
}

func TestPRD_NextStoryContext_EmptyPRD(t *testing.T) {
	p := &PRD{
		Project:     "Empty",
		UserStories: []UserStory{},
	}

	ctx := p.NextStoryContext()
	if ctx != nil {
		t.Errorf("expected nil for empty PRD, got %q", *ctx)
	}
}

func TestPRD_NextStoryContext_ValidJSON(t *testing.T) {
	p := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{
				ID:                 "US-001",
				Title:              "Test Story",
				Description:        "A test description",
				AcceptanceCriteria: []string{"AC1", "AC2"},
				Priority:           1,
				Passes:             false,
			},
		},
	}

	ctx := p.NextStoryContext()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	var story UserStory
	if err := json.Unmarshal([]byte(*ctx), &story); err != nil {
		t.Fatalf("NextStoryContext did not return valid JSON: %v", err)
	}
	if story.ID != "US-001" {
		t.Errorf("expected ID US-001, got %s", story.ID)
	}
	if story.Title != "Test Story" {
		t.Errorf("expected title 'Test Story', got '%s'", story.Title)
	}
	if len(story.AcceptanceCriteria) != 2 {
		t.Errorf("expected 2 acceptance criteria, got %d", len(story.AcceptanceCriteria))
	}
}

func TestPRD_NextStoryContext_PromptSizeUnder10KB(t *testing.T) {
	// Create a 300-story PRD to verify the context stays small
	stories := make([]UserStory, 300)
	for i := range stories {
		stories[i] = UserStory{
			ID:                 fmt.Sprintf("US-%03d", i+1),
			Title:              fmt.Sprintf("Story %d with a reasonably long title for realism", i+1),
			Description:        "This is a description that is moderately long to simulate realistic PRD content for testing purposes.",
			AcceptanceCriteria: []string{"Criterion A", "Criterion B", "Criterion C"},
			Priority:           i + 1,
			Passes:             i > 0, // Only first story is pending
		}
	}
	p := &PRD{
		Project:     "Large Project",
		Description: "A large PRD with 300 stories",
		UserStories: stories,
	}

	ctx := p.NextStoryContext()
	if ctx == nil {
		t.Fatal("expected non-nil context for 300-story PRD")
	}
	if len(*ctx) > 10*1024 {
		t.Errorf("story context is %d bytes, expected under 10KB", len(*ctx))
	}
}
