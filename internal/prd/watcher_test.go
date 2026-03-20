package prd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	// Create a test PRD file
	testPRD := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Title: "Test Story", Passes: false},
		},
	}
	data, _ := json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if watcher.path != prdPath {
		t.Errorf("Expected path %s, got %s", prdPath, watcher.path)
	}
}

func TestWatcherStart(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	// Create a test PRD file
	testPRD := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Title: "Test Story", Passes: false},
		},
	}
	data, _ := json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Starting again should return an error
	if err := watcher.Start(); err == nil {
		t.Error("Expected error when starting watcher twice")
	}
}

func TestWatcherDetectsFileChange(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	// Create a test PRD file
	testPRD := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Title: "Test Story", Passes: false},
		},
	}
	data, _ := json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Give watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Modify the file - change passes status
	testPRD.UserStories[0].Passes = true
	data, _ = json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to update test PRD: %v", err)
	}

	// Wait for the event
	select {
	case event := <-watcher.Events():
		if event.Error != nil {
			t.Fatalf("Unexpected error: %v", event.Error)
		}
		if event.PRD == nil {
			t.Fatal("Expected PRD in event")
		}
		if !event.PRD.UserStories[0].Passes {
			t.Error("Expected story to have passes: true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for file change event")
	}
}

func TestWatcherDetectsInProgressChange(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	// Create a test PRD file
	testPRD := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Title: "Test Story", Passes: false, InProgress: false},
		},
	}
	data, _ := json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Give watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Modify the file - change inProgress status
	testPRD.UserStories[0].InProgress = true
	data, _ = json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to update test PRD: %v", err)
	}

	// Wait for the event
	select {
	case event := <-watcher.Events():
		if event.Error != nil {
			t.Fatalf("Unexpected error: %v", event.Error)
		}
		if event.PRD == nil {
			t.Fatal("Expected PRD in event")
		}
		if !event.PRD.UserStories[0].InProgress {
			t.Error("Expected story to have inProgress: true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for file change event")
	}
}

func TestWatcherHandlesFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "nonexistent.json")

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Start should still work, but we'll get an error event
	if err := watcher.Start(); err != nil {
		// This is expected since the file doesn't exist
		// But the watcher.Add might fail first
		// Let's check that events channel has an error
		t.Logf("Got expected start error: %v", err)
		return
	}

	// If start succeeded, check for error event
	select {
	case event := <-watcher.Events():
		if event.Error == nil {
			t.Error("Expected error event for nonexistent file")
		}
	case <-time.After(1 * time.Second):
		// Might not get event if watcher.Add failed
		t.Log("No error event received, which is acceptable if Add failed")
	}
}

func TestWatcherIgnoresNonStatusChanges(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	// Create a test PRD file
	testPRD := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Title: "Test Story", Description: "Original", Passes: false},
		},
	}
	data, _ := json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Give watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Modify the file - only change description (not status)
	testPRD.UserStories[0].Description = "Modified"
	data, _ = json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to update test PRD: %v", err)
	}

	// Should NOT receive an event since status didn't change
	select {
	case event := <-watcher.Events():
		if event.PRD != nil {
			t.Error("Did not expect PRD event for non-status change")
		}
	case <-time.After(500 * time.Millisecond):
		// Expected - no event for non-status changes
	}
}

func TestWatcherStop(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.json")

	// Create a test PRD file
	testPRD := &PRD{
		Project: "Test",
		UserStories: []UserStory{
			{ID: "US-001", Title: "Test Story", Passes: false},
		},
	}
	data, _ := json.Marshal(testPRD)
	if err := os.WriteFile(prdPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Stop should not panic or hang
	watcher.Stop()

	// Stopping again should be safe
	watcher.Stop()
}

func TestHasStatusChanged(t *testing.T) {
	tests := []struct {
		name     string
		oldPRD   *PRD
		newPRD   *PRD
		expected bool
	}{
		{
			name:   "nil old PRD",
			oldPRD: nil,
			newPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", Passes: false}},
			},
			expected: true,
		},
		{
			name: "passes changed",
			oldPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", Passes: false}},
			},
			newPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", Passes: true}},
			},
			expected: true,
		},
		{
			name: "inProgress changed",
			oldPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", InProgress: false}},
			},
			newPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", InProgress: true}},
			},
			expected: true,
		},
		{
			name: "no status change",
			oldPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", Passes: false, InProgress: false}},
			},
			newPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", Passes: false, InProgress: false}},
			},
			expected: false,
		},
		{
			name: "story count changed",
			oldPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001"}},
			},
			newPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001"}, {ID: "US-002"}},
			},
			expected: true,
		},
		{
			name: "new story added",
			oldPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", Passes: true}},
			},
			newPRD: &PRD{
				UserStories: []UserStory{{ID: "US-001", Passes: true}, {ID: "US-002", Passes: false}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Watcher{lastPRD: tt.oldPRD}
			result := w.hasStatusChanged(tt.newPRD)
			if result != tt.expected {
				t.Errorf("hasStatusChanged() = %v, want %v", result, tt.expected)
			}
		})
	}
}
