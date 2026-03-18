package prd

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestPRDMd creates a markdown PRD file for testing.
func createTestPRDMd(t *testing.T, dir string, stories []UserStory) string {
	t.Helper()
	prdPath := filepath.Join(dir, "prd.md")

	md := "# Test\n\n"
	for _, s := range stories {
		md += "### " + s.ID + ": " + s.Title + "\n"
		if s.Passes {
			md += "**Status:** done\n"
		} else if s.InProgress {
			md += "**Status:** in-progress\n"
		}
		if s.Description != "" {
			md += "**Description:** " + s.Description + "\n"
		}
		md += "- [ ] criterion\n\n"
	}

	if err := os.WriteFile(prdPath, []byte(md), 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}
	return prdPath
}

func TestNewWatcher(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRDMd(t, tmpDir, []UserStory{
		{ID: "US-001", Title: "Test Story", Passes: false},
	})

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
	prdPath := createTestPRDMd(t, tmpDir, []UserStory{
		{ID: "US-001", Title: "Test Story", Passes: false},
	})

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
	prdPath := createTestPRDMd(t, tmpDir, []UserStory{
		{ID: "US-001", Title: "Test Story", Passes: false},
	})

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Modify the file - change passes status
	if err := SetStoryStatus(prdPath, "US-001", "done"); err != nil {
		t.Fatalf("Failed to update test PRD: %v", err)
	}

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
	prdPath := createTestPRDMd(t, tmpDir, []UserStory{
		{ID: "US-001", Title: "Test Story", Passes: false, InProgress: false},
	})

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Modify the file - change inProgress status
	if err := SetStoryStatus(prdPath, "US-001", "in-progress"); err != nil {
		t.Fatalf("Failed to update test PRD: %v", err)
	}

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
	prdPath := filepath.Join(tmpDir, "nonexistent.md")

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Logf("Got expected start error: %v", err)
		return
	}

	select {
	case event := <-watcher.Events():
		if event.Error == nil {
			t.Error("Expected error event for nonexistent file")
		}
	case <-time.After(1 * time.Second):
		t.Log("No error event received, which is acceptable if Add failed")
	}
}

func TestWatcherStop(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := createTestPRDMd(t, tmpDir, []UserStory{
		{ID: "US-001", Title: "Test Story", Passes: false},
	})

	watcher, err := NewWatcher(prdPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	watcher.Stop()
	watcher.Stop() // Should be safe
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
