package prd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMarkdownPRDFromString_Normal(t *testing.T) {
	md := `# PRD: My Test Project

A sample project for testing.

## User Stories

### US-001: Setup Project
As a developer, I need a properly structured project.

**Priority:** 1
**Status:** done

- [x] Create project structure
- [x] Add dependencies

### US-002: Add Feature
**Description:** As a user, I want a new feature.

**Status:** in-progress

- [ ] Feature works correctly
- [ ] Tests pass

### US-003: Final Polish
Some prose description here.

- [ ] Polish the UI
- [ ] Write docs
`
	p, err := ParseMarkdownPRDFromString(md)
	if err != nil {
		t.Fatalf("ParseMarkdownPRDFromString() error = %v", err)
	}

	if p.Project != "My Test Project" {
		t.Errorf("Project = %q, want %q", p.Project, "My Test Project")
	}
	if p.Description != "A sample project for testing." {
		t.Errorf("Description = %q, want %q", p.Description, "A sample project for testing.")
	}
	if len(p.UserStories) != 3 {
		t.Fatalf("len(UserStories) = %d, want 3", len(p.UserStories))
	}

	// Story 1: done
	s1 := p.UserStories[0]
	if s1.ID != "US-001" {
		t.Errorf("s1.ID = %q, want %q", s1.ID, "US-001")
	}
	if s1.Title != "Setup Project" {
		t.Errorf("s1.Title = %q, want %q", s1.Title, "Setup Project")
	}
	if !s1.Passes {
		t.Error("s1.Passes = false, want true")
	}
	if s1.InProgress {
		t.Error("s1.InProgress = true, want false")
	}
	if s1.Priority != 1 {
		t.Errorf("s1.Priority = %d, want 1", s1.Priority)
	}
	if len(s1.AcceptanceCriteria) != 2 {
		t.Errorf("len(s1.AcceptanceCriteria) = %d, want 2", len(s1.AcceptanceCriteria))
	}

	// Story 2: in-progress
	s2 := p.UserStories[1]
	if s2.ID != "US-002" {
		t.Errorf("s2.ID = %q, want %q", s2.ID, "US-002")
	}
	if !s2.InProgress {
		t.Error("s2.InProgress = false, want true")
	}
	if s2.Passes {
		t.Error("s2.Passes = true, want false")
	}
	if s2.Description != "As a user, I want a new feature." {
		t.Errorf("s2.Description = %q, want %q", s2.Description, "As a user, I want a new feature.")
	}

	// Story 3: pending (no status)
	s3 := p.UserStories[2]
	if s3.ID != "US-003" {
		t.Errorf("s3.ID = %q, want %q", s3.ID, "US-003")
	}
	if s3.Passes || s3.InProgress {
		t.Error("s3 should be pending (both false)")
	}
	if s3.Description != "Some prose description here." {
		t.Errorf("s3.Description = %q, want %q", s3.Description, "Some prose description here.")
	}
	if len(s3.AcceptanceCriteria) != 2 {
		t.Errorf("len(s3.AcceptanceCriteria) = %d, want 2", len(s3.AcceptanceCriteria))
	}
}

func TestParseMarkdownPRDFromString_ProjectWithoutPRDPrefix(t *testing.T) {
	md := `# My Project

Overview text.

### FEAT-001: First Feature
- [ ] It works
`
	p, err := ParseMarkdownPRDFromString(md)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.Project != "My Project" {
		t.Errorf("Project = %q, want %q", p.Project, "My Project")
	}
	if len(p.UserStories) != 1 {
		t.Fatalf("len(UserStories) = %d, want 1", len(p.UserStories))
	}
	if p.UserStories[0].ID != "FEAT-001" {
		t.Errorf("ID = %q, want %q", p.UserStories[0].ID, "FEAT-001")
	}
}

func TestParseMarkdownPRDFromString_MissingFields(t *testing.T) {
	md := `# Minimal

### US-001: Only Title
`
	p, err := ParseMarkdownPRDFromString(md)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(p.UserStories) != 1 {
		t.Fatalf("len(UserStories) = %d, want 1", len(p.UserStories))
	}
	s := p.UserStories[0]
	if s.ID != "US-001" {
		t.Errorf("ID = %q", s.ID)
	}
	if s.Priority != 1 {
		t.Errorf("Priority = %d, want 1 (auto-assigned)", s.Priority)
	}
	if s.Passes || s.InProgress {
		t.Error("should be pending")
	}
}

func TestParseMarkdownPRDFromString_PhaseHeadingsIgnored(t *testing.T) {
	md := `# My Project

## Phase 1: Setup

### US-001: Do Setup
- [ ] Setup done

## Phase 2: Build

### US-002: Do Build
- [ ] Build done
`
	p, err := ParseMarkdownPRDFromString(md)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(p.UserStories) != 2 {
		t.Fatalf("len(UserStories) = %d, want 2", len(p.UserStories))
	}
	if p.UserStories[0].ID != "US-001" {
		t.Errorf("first story ID = %q", p.UserStories[0].ID)
	}
	if p.UserStories[1].ID != "US-002" {
		t.Errorf("second story ID = %q", p.UserStories[1].ID)
	}
}

func TestParseMarkdownPRDFromString_IntroductionSection(t *testing.T) {
	md := `# PRD: Test

## Introduction

This is the introduction paragraph.

## Stories

### US-001: First
- [ ] Done
`
	p, err := ParseMarkdownPRDFromString(md)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.Description != "This is the introduction paragraph." {
		t.Errorf("Description = %q, want %q", p.Description, "This is the introduction paragraph.")
	}
}

func TestParseMarkdownPRDFromString_FreeSections(t *testing.T) {
	md := `# Test Project

Overview.

## Background

Some background text.

## User Stories

### US-001: First
- [ ] Criterion A

## Appendix

Extra info here.
`
	p, err := ParseMarkdownPRDFromString(md)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(p.UserStories) != 1 {
		t.Fatalf("len(UserStories) = %d, want 1", len(p.UserStories))
	}
	if p.UserStories[0].ID != "US-001" {
		t.Errorf("ID = %q", p.UserStories[0].ID)
	}
}

func TestParseMarkdownPRDFromString_StatusMapping(t *testing.T) {
	tests := []struct {
		status     string
		wantPasses bool
		wantIP     bool
	}{
		{"done", true, false},
		{"complete", true, false},
		{"completed", true, false},
		{"passed", true, false},
		{"in-progress", false, true},
		{"in progress", false, true},
		{"started", false, true},
		{"todo", false, false},
		{"pending", false, false},
		{"", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			md := "# P\n\n### US-001: S\n**Status:** " + tt.status + "\n"
			p, err := ParseMarkdownPRDFromString(md)
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if len(p.UserStories) != 1 {
				t.Fatalf("len(UserStories) = %d", len(p.UserStories))
			}
			if p.UserStories[0].Passes != tt.wantPasses {
				t.Errorf("Passes = %v, want %v", p.UserStories[0].Passes, tt.wantPasses)
			}
			if p.UserStories[0].InProgress != tt.wantIP {
				t.Errorf("InProgress = %v, want %v", p.UserStories[0].InProgress, tt.wantIP)
			}
		})
	}
}

func TestParseMarkdownPRD_File(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "prd.md")

	md := `# Test

### US-001: First Story
- [ ] Works
`
	if err := os.WriteFile(prdPath, []byte(md), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	p, err := ParseMarkdownPRD(prdPath)
	if err != nil {
		t.Fatalf("ParseMarkdownPRD() error = %v", err)
	}
	if p.Project != "Test" {
		t.Errorf("Project = %q", p.Project)
	}
	if len(p.UserStories) != 1 {
		t.Fatalf("len(UserStories) = %d", len(p.UserStories))
	}
}

func TestParseMarkdownPRD_FileNotFound(t *testing.T) {
	_, err := ParseMarkdownPRD("/nonexistent/prd.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseMarkdownPRDFromString_AutoPriority(t *testing.T) {
	md := `# P

### US-001: First
- [ ] A

### US-002: Second
**Priority:** 5
- [ ] B

### US-003: Third
- [ ] C
`
	p, err := ParseMarkdownPRDFromString(md)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(p.UserStories) != 3 {
		t.Fatalf("len = %d", len(p.UserStories))
	}
	// First: auto-priority 1
	if p.UserStories[0].Priority != 1 {
		t.Errorf("s1.Priority = %d, want 1", p.UserStories[0].Priority)
	}
	// Second: explicit priority 5
	if p.UserStories[1].Priority != 5 {
		t.Errorf("s2.Priority = %d, want 5", p.UserStories[1].Priority)
	}
	// Third: auto-priority 6 (after 5)
	if p.UserStories[2].Priority != 6 {
		t.Errorf("s3.Priority = %d, want 6", p.UserStories[2].Priority)
	}
}
