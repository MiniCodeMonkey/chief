package prd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetStoryStatusInString_ExistingStatusLine(t *testing.T) {
	md := `# P

### US-001: First
**Status:** todo
- [ ] A
- [ ] B

### US-002: Second
- [ ] C
`
	result, err := setStoryStatusInString(md, "US-001", "done")
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	if !strings.Contains(result, "**Status:** done") {
		t.Error("expected **Status:** done in result")
	}
	// Should not contain the old status
	if strings.Contains(result, "**Status:** todo") {
		t.Error("old status should be replaced")
	}
	// Checkboxes should be flipped to checked
	if strings.Contains(result, "- [ ] A") {
		t.Error("expected checkbox A to be checked")
	}
	if !strings.Contains(result, "- [x] A") {
		t.Error("expected checkbox A to be [x]")
	}
	// US-002 should be untouched
	if !strings.Contains(result, "- [ ] C") {
		t.Error("US-002 checkboxes should be untouched")
	}
}

func TestSetStoryStatusInString_MissingStatusLine(t *testing.T) {
	md := `# P

### US-001: First
- [ ] A
`
	result, err := setStoryStatusInString(md, "US-001", "in-progress")
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	if !strings.Contains(result, "**Status:** in-progress") {
		t.Error("expected **Status:** in-progress to be inserted")
	}

	// Status line should appear after the heading
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if strings.Contains(line, "### US-001") {
			if i+1 >= len(lines) || !strings.Contains(lines[i+1], "**Status:** in-progress") {
				t.Error("status line should be directly after heading")
			}
			break
		}
	}
}

func TestSetStoryStatusInString_CheckboxFlipping(t *testing.T) {
	md := `# P

### US-001: First
**Status:** in-progress
- [ ] Unchecked A
- [x] Already checked B
- [ ] Unchecked C
`
	result, err := setStoryStatusInString(md, "US-001", "done")
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	if strings.Contains(result, "- [ ] Unchecked A") {
		t.Error("checkbox A should be checked")
	}
	if !strings.Contains(result, "- [x] Unchecked A") {
		t.Error("expected [x] Unchecked A")
	}
	if !strings.Contains(result, "- [x] Already checked B") {
		t.Error("already checked B should remain checked")
	}
	if !strings.Contains(result, "- [x] Unchecked C") {
		t.Error("checkbox C should be checked")
	}
}

func TestSetStoryStatusInString_MultiStory(t *testing.T) {
	md := `# P

### US-001: First
**Status:** todo
- [ ] A

### US-002: Second
**Status:** todo
- [ ] B

### US-003: Third
- [ ] C
`
	// Mark US-002 as done
	result, err := setStoryStatusInString(md, "US-002", "done")
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	// US-001 should be unchanged
	if !strings.Contains(result, "- [ ] A") {
		t.Error("US-001 checkboxes should be untouched")
	}
	// US-003 should be unchanged
	if !strings.Contains(result, "- [ ] C") {
		t.Error("US-003 checkboxes should be untouched")
	}
	// US-002 should be done with checked boxes
	if !strings.Contains(result, "- [x] B") {
		t.Error("US-002 checkbox should be checked")
	}
}

func TestSetStoryStatusInString_StoryNotFound(t *testing.T) {
	md := `# P

### US-001: First
- [ ] A
`
	_, err := setStoryStatusInString(md, "US-999", "done")
	if err == nil {
		t.Error("expected error for missing story")
	}
}

func TestSetStoryStatus_File(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.md")

	md := `# P

### US-001: First
- [ ] A
`
	if err := os.WriteFile(prdPath, []byte(md), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	if err := SetStoryStatus(prdPath, "US-001", "done"); err != nil {
		t.Fatalf("SetStoryStatus() error = %v", err)
	}

	data, err := os.ReadFile(prdPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	result := string(data)
	if !strings.Contains(result, "**Status:** done") {
		t.Error("expected **Status:** done in file")
	}
	if !strings.Contains(result, "- [x] A") {
		t.Error("expected checkbox to be checked")
	}
}

func TestSetStoryStatusInString_NoCheckboxFlipForNonDone(t *testing.T) {
	md := `# P

### US-001: First
- [ ] A
`
	result, err := setStoryStatusInString(md, "US-001", "in-progress")
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	// Checkboxes should NOT be flipped for in-progress
	if !strings.Contains(result, "- [ ] A") {
		t.Error("checkboxes should not be flipped for non-done status")
	}
}

func TestSetStoryStatusInString_RoundTrip(t *testing.T) {
	md := `# My Project

A description.

### US-001: First
**Status:** todo
- [ ] A
- [ ] B

### US-002: Second
- [ ] C
`
	// Set US-001 to in-progress
	result, err := setStoryStatusInString(md, "US-001", "in-progress")
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	// Parse and verify
	p, err := ParseMarkdownPRDFromString(result)
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if !p.UserStories[0].InProgress {
		t.Error("US-001 should be in-progress")
	}
	if p.UserStories[0].Passes {
		t.Error("US-001 should not be passes")
	}

	// Now set US-001 to done
	result, err = setStoryStatusInString(result, "US-001", "done")
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	// Parse and verify
	p, err = ParseMarkdownPRDFromString(result)
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if !p.UserStories[0].Passes {
		t.Error("US-001 should be passes")
	}
	if p.UserStories[0].InProgress {
		t.Error("US-001 should not be in-progress")
	}
}
