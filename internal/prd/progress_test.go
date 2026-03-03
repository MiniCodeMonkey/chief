package prd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProgress_BasicStory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "progress.md")

	content := `## 2026-02-20 - US-001
- Created parser at lib/Parser.php
- Added 10 unit tests
---
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ParseProgress(path)
	if err != nil {
		t.Fatalf("ParseProgress failed: %v", err)
	}

	if len(entries["US-001"]) != 1 {
		t.Fatalf("expected 1 entry for US-001, got %d", len(entries["US-001"]))
	}

	entry := entries["US-001"][0]
	if entry.Date != "2026-02-20" {
		t.Errorf("expected date '2026-02-20', got '%s'", entry.Date)
	}
	if entry.StoryID != "US-001" {
		t.Errorf("expected story ID 'US-001', got '%s'", entry.StoryID)
	}
	if !strings.Contains(entry.Content, "Created parser at lib/Parser.php") {
		t.Errorf("expected content to contain first bullet, got: %s", entry.Content)
	}
	if !strings.Contains(entry.Content, "Added 10 unit tests") {
		t.Errorf("expected content to contain second bullet, got: %s", entry.Content)
	}
}

func TestParseProgress_MultipleStories(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "progress.md")

	content := `## 2026-02-20 - US-001
- First story work
---

## 2026-02-20 - US-002
- Second story work
- More work
---
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ParseProgress(path)
	if err != nil {
		t.Fatalf("ParseProgress failed: %v", err)
	}

	if len(entries["US-001"]) != 1 {
		t.Errorf("expected 1 entry for US-001, got %d", len(entries["US-001"]))
	}
	if len(entries["US-002"]) != 1 {
		t.Errorf("expected 1 entry for US-002, got %d", len(entries["US-002"]))
	}
	if !strings.Contains(entries["US-002"][0].Content, "More work") {
		t.Errorf("expected US-002 content to contain 'More work'")
	}
}

func TestParseProgress_IncludesLearnings(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "progress.md")

	content := `## 2026-02-20 - US-001
- Created the thing
- Files changed: a.go, b.go
- **Learnings for future iterations:**
  - Always run tests first
  - Use strict mode
---
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ParseProgress(path)
	if err != nil {
		t.Fatalf("ParseProgress failed: %v", err)
	}

	entry := entries["US-001"][0]
	if !strings.Contains(entry.Content, "**Learnings for future iterations:**") {
		t.Errorf("expected content to include learnings header, got: %s", entry.Content)
	}
	if !strings.Contains(entry.Content, "Always run tests first") {
		t.Errorf("expected content to include learnings sub-bullet, got: %s", entry.Content)
	}
}

func TestParseProgress_SkipsCodebasePatternsSection(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "progress.md")

	content := `## Codebase Patterns
- Key format: something
- Existing formatter at lib/Formatter.php

---

## 2026-02-20 - US-001
- Did the work
---
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ParseProgress(path)
	if err != nil {
		t.Fatalf("ParseProgress failed: %v", err)
	}

	// "Codebase Patterns" should not match the story header regex
	if _, ok := entries["Codebase Patterns"]; ok {
		t.Error("expected 'Codebase Patterns' to be ignored")
	}

	if len(entries["US-001"]) != 1 {
		t.Errorf("expected 1 entry for US-001, got %d", len(entries["US-001"]))
	}
}

func TestParseProgress_MultipleEntriesSameStory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "progress.md")

	content := `## 2026-02-19 - US-001
- Initial work
---

## 2026-02-20 - US-001
- Continued work
---
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ParseProgress(path)
	if err != nil {
		t.Fatalf("ParseProgress failed: %v", err)
	}

	if len(entries["US-001"]) != 2 {
		t.Fatalf("expected 2 entries for US-001, got %d", len(entries["US-001"]))
	}
	if entries["US-001"][0].Date != "2026-02-19" {
		t.Errorf("expected first entry date '2026-02-19', got '%s'", entries["US-001"][0].Date)
	}
	if entries["US-001"][1].Date != "2026-02-20" {
		t.Errorf("expected second entry date '2026-02-20', got '%s'", entries["US-001"][1].Date)
	}
}

func TestParseProgress_FileNotFound(t *testing.T) {
	entries, err := ParseProgress("/nonexistent/progress.md")
	if err != nil {
		t.Errorf("expected nil error for missing file, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for missing file, got %v", entries)
	}
}

func TestParseProgress_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "progress.md")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ParseProgress(path)
	if err != nil {
		t.Fatalf("ParseProgress failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty file, got %d", len(entries))
	}
}

func TestParseProgress_NoTrailingSeparator(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "progress.md")

	content := `## 2026-02-20 - US-001
- Work done here
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ParseProgress(path)
	if err != nil {
		t.Fatalf("ParseProgress failed: %v", err)
	}

	if len(entries["US-001"]) != 1 {
		t.Fatalf("expected 1 entry for US-001, got %d", len(entries["US-001"]))
	}
	if !strings.Contains(entries["US-001"][0].Content, "Work done here") {
		t.Errorf("expected content to contain bullet text")
	}
}

func TestParseProgress_PreservesRawMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "progress.md")

	content := `## 2026-02-20 - US-001
- Created ` + "`AddressKeyParser`" + ` at ` + "`lib/Parser.php`" + `
- **Learnings for future iterations:**
  - Use ` + "`declare(strict_types=1)`" + ` in test files
---
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ParseProgress(path)
	if err != nil {
		t.Fatalf("ParseProgress failed: %v", err)
	}

	entry := entries["US-001"][0]
	// Verify raw markdown is preserved (backticks, bold markers, indentation)
	if !strings.Contains(entry.Content, "`AddressKeyParser`") {
		t.Errorf("expected backtick code to be preserved")
	}
	if !strings.Contains(entry.Content, "**Learnings") {
		t.Errorf("expected bold markers to be preserved")
	}
	if !strings.Contains(entry.Content, "  - Use") {
		t.Errorf("expected indented sub-bullet to be preserved")
	}
}

func TestProgressPath(t *testing.T) {
	got := ProgressPath("/foo/bar/.melliza/prds/my-prd/prd.json")
	want := "/foo/bar/.melliza/prds/my-prd/progress.md"
	if got != want {
		t.Errorf("ProgressPath() = %q, want %q", got, want)
	}
}
