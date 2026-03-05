package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppE2E_DashboardInit(t *testing.T) {
	// Setup temporary workspace
	tmpDir := t.TempDir()

	// Set up the .melliza structure
	prdDir := filepath.Join(tmpDir, ".melliza", "prds", "e2e-test")
	err := os.MkdirAll(prdDir, 0755)
	if err != nil {
		t.Fatalf("failed to create prd dir: %v", err)
	}

	// Create a mock prd.json
	mockPRD := `{
        "project": "E2E Test Project",
        "description": "Testing the TUI UX",
        "userStories": [
            {
                "id": "US-001",
                "title": "First E2E Story",
                "description": "As a user, I want the TUI to work.",
                "acceptanceCriteria": ["Criteria 1"],
                "priority": 1,
                "passes": false
            },
			{
                "id": "US-002",
                "title": "Second E2E Story",
                "description": "Another story.",
                "acceptanceCriteria": ["Criteria 2"],
                "priority": 2,
                "passes": false
            }
        ]
    }`
	prdPath := filepath.Join(prdDir, "prd.json")
	err = os.WriteFile(prdPath, []byte(mockPRD), 0644)
	if err != nil {
		t.Fatalf("failed to write mock PRD: %v", err)
	}

	// Initialize the app
	app, err := NewAppWithOptions(prdPath, 1)
	if err != nil {
		t.Fatalf("failed to initialize app: %v", err)
	}

	// Verify the app loaded correctly
	if app.prdName != "e2e-test" {
		t.Errorf("expected prdName 'e2e-test', got '%s'", app.prdName)
	}
	if len(app.prd.UserStories) != 2 {
		t.Errorf("expected 2 user stories, got %d", len(app.prd.UserStories))
	}
	if app.prd.UserStories[0].Title != "First E2E Story" {
		t.Errorf("expected first story title 'First E2E Story', got '%s'", app.prd.UserStories[0].Title)
	}
	if app.viewMode != ViewDashboard {
		t.Errorf("expected initial viewMode ViewDashboard, got %d", app.viewMode)
	}
}

func TestAppE2E_SimulatePRDCompletion(t *testing.T) {
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".melliza", "prds", "e2e-completion")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("failed to create prd dir: %v", err)
	}

	mockPRD := `{
        "project": "E2E Completion",
        "userStories": [
            {
                "id": "US-001",
                "title": "A completed story",
                "passes": false
            }
        ]
    }`
	prdPath := filepath.Join(prdDir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(mockPRD), 0644); err != nil {
		t.Fatalf("failed to write mock PRD: %v", err)
	}

	app, err := NewAppWithOptions(prdPath, 5)
	if err != nil {
		t.Fatalf("failed to init app: %v", err)
	}

	// Simulate PRD completion by sending the message through Update
	completionMsg := PRDCompletedMsg{PRDName: "e2e-completion"}
	model, _ := app.Update(completionMsg)

	updatedApp, ok := model.(App)
	if !ok {
		t.Fatal("expected model to be App after update")
	}

	// PRDCompletedMsg triggers completion callback and refreshes, stays on dashboard
	if updatedApp.viewMode != ViewDashboard {
		t.Errorf("expected ViewDashboard after PRDCompletedMsg, got %d", updatedApp.viewMode)
	}
}
