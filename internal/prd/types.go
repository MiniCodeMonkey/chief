// Package prd provides types and utilities for working with Product
// Requirements Documents (PRDs). It includes loading, saving, watching
// for changes, and converting between prd.md and prd.json formats.
package prd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UserStory represents a single user story in a PRD.
type UserStory struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
	Priority           int      `json:"priority"`
	Passes             bool     `json:"passes"`
	InProgress         bool     `json:"inProgress,omitempty"`
}

// PRD represents a Product Requirements Document.
type PRD struct {
	Project     string      `json:"project"`
	Description string      `json:"description"`
	UserStories []UserStory `json:"userStories"`
}

// ExtractIDPrefix returns the ID prefix used by the stories in this PRD.
// For example, "US" from "US-001", "MFR" from "MFR-001", "T" from "T-001".
// Returns "US" as the default when the PRD has no stories or IDs lack a hyphen.
func (p *PRD) ExtractIDPrefix() string {
	for _, story := range p.UserStories {
		if idx := strings.LastIndex(story.ID, "-"); idx > 0 {
			return story.ID[:idx]
		}
	}
	return "US"
}

// AllComplete returns true when all stories have passes: true.
func (p *PRD) AllComplete() bool {
	if len(p.UserStories) == 0 {
		return true
	}
	for _, story := range p.UserStories {
		if !story.Passes {
			return false
		}
	}
	return true
}

// NextStory returns the next story to work on.
// It returns:
//   - First story with inProgress: true (interrupted story), or
//   - Lowest priority story with passes: false, or
//   - nil if all stories are complete
func (p *PRD) NextStory() *UserStory {
	// First, check for any in-progress story (interrupted)
	for i := range p.UserStories {
		if p.UserStories[i].InProgress {
			return &p.UserStories[i]
		}
	}

	// Find the lowest priority story that hasn't passed
	var next *UserStory
	for i := range p.UserStories {
		story := &p.UserStories[i]
		if !story.Passes {
			if next == nil || story.Priority < next.Priority {
				next = story
			}
		}
	}
	return next
}

// NextStoryContext returns the next story to work on as a formatted string
// suitable for inlining into the agent prompt. Returns nil when all stories
// are complete.
func (p *PRD) NextStoryContext() *string {
	story := p.NextStory()
	if story == nil {
		return nil
	}

	data, err := json.MarshalIndent(story, "", "  ")
	if err != nil {
		// Fallback to a simple text format
		var b strings.Builder
		fmt.Fprintf(&b, "ID: %s\nTitle: %s\nDescription: %s\n", story.ID, story.Title, story.Description)
		fmt.Fprintf(&b, "Acceptance Criteria:\n")
		for _, ac := range story.AcceptanceCriteria {
			fmt.Fprintf(&b, "- %s\n", ac)
		}
		result := b.String()
		return &result
	}

	result := string(data)
	return &result
}
