package tui

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestAppE2E_DashboardAndLog(t *testing.T) {
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
	app, err := NewAppWithOptions(prdPath, 1) // maxIter=1
	if err != nil {
		t.Fatalf("failed to initialize app: %v", err)
	}

	// Start teatest Program with standard size
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))

	// 1. Verify initial Dashboard view (Wait for rendering to finish)
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("E2E Test Project")) || bytes.Contains(b, []byte("First E2E Story"))
	}, teatest.WithDuration(3*time.Second))

	// Take snapshot of initial dashboard
	tm.Send(tea.WindowSizeMsg{Width: 100, Height: 30}) // Trigger a resize just in case
	time.Sleep(100 * time.Millisecond) // Give it a moment to process internally

	// 2. Switch to Log View ('t')
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Log")) // The log panel title should appear
	}, teatest.WithDuration(2*time.Second))

	// 3. Switch back to Dashboard View ('t')
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Stories"))
	}, teatest.WithDuration(2*time.Second))

	// 4. Navigate down and up
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	time.Sleep(50 * time.Millisecond)

	// 5. Open Picker ('l')
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	time.Sleep(500 * time.Millisecond)

	// Close Picker ('esc')
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	time.Sleep(500 * time.Millisecond)

	// 6. Quit gracefully ('q')
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	
	// App might present Quit Confirm if running, but our state is Ready, so it should quit immediately.
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	// Capture the final output snapshot using teatest's built-in snapshotting
	// This generates the .golden file. To update it, run 'go test -update'
	out := tm.FinalOutput(t)
	outBytes, err := io.ReadAll(out)
	if err != nil {
		t.Fatalf("failed to read final output: %v", err)
	}
	teatest.RequireEqualOutput(t, outBytes)
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

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))

	// Initial render
	time.Sleep(500 * time.Millisecond)

	// Simulate starting loop
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	time.Sleep(500 * time.Millisecond)

	// Simulate story in progress (by updating the PRD and sending PRDUpdateMsg or triggering file watcher)
	// We can cheat and just simulate the completion via the underlying Loop messages
	// Actually, Melliza loop checks prd.json on iteration start/end.
	// But we can trigger completion UI by sending PRDCompletedMsg
	tm.Send(PRDCompletedMsg{PRDName: "e2e-completion"})
	
	// Wait for the completion view to render confetti
	time.Sleep(1 * time.Second)

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	time.Sleep(500 * time.Millisecond)
	
	// Confirm quit in case the manager still marks the loop as running
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	outBytes, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatalf("failed to read final output: %v", err)
	}
	teatest.RequireEqualOutput(t, outBytes)
}
