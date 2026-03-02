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
	DependsOn          []string `json:"dependsOn,omitempty"`
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

// depsResolved returns true if all of a story's DependsOn IDs have passes: true.
func (p *PRD) depsResolved(story *UserStory) bool {
	if len(story.DependsOn) == 0 {
		return true
	}
	passed := make(map[string]bool, len(p.UserStories))
	for _, s := range p.UserStories {
		if s.Passes {
			passed[s.ID] = true
		}
	}
	for _, dep := range story.DependsOn {
		if !passed[dep] {
			return false
		}
	}
	return true
}

// NextStory returns the next story to work on.
// It returns:
//   - First story with inProgress: true (interrupted story), or
//   - Lowest priority story with passes: false whose dependencies are all satisfied, or
//   - nil if all stories are complete
//
// Returns an error if incomplete stories remain but none are eligible (circular or
// unresolvable dependencies).
func (p *PRD) NextStory() (*UserStory, error) {
	// First, check for any in-progress story (interrupted)
	for i := range p.UserStories {
		if p.UserStories[i].InProgress {
			return &p.UserStories[i], nil
		}
	}

	// Find the lowest priority story that hasn't passed and has dependencies met
	var next *UserStory
	hasIncomplete := false
	for i := range p.UserStories {
		story := &p.UserStories[i]
		if !story.Passes {
			hasIncomplete = true
			if p.depsResolved(story) {
				if next == nil || story.Priority < next.Priority {
					next = story
				}
			}
		}
	}

	if next != nil {
		return next, nil
	}
	if hasIncomplete {
		return nil, fmt.Errorf("no eligible story found: incomplete stories remain but all have unresolved dependencies (possible circular dependency)")
	}
	return nil, nil
}

// NextStoryContext returns the next story to work on as a formatted string
// suitable for inlining into the agent prompt. Returns nil when all stories
// are complete. Returns an error if dependencies are unresolvable.
func (p *PRD) NextStoryContext() (*string, error) {
	story, err := p.NextStory()
	if err != nil {
		return nil, err
	}
	if story == nil {
		return nil, nil
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
		return &result, nil
	}

	result := string(data)
	return &result, nil
}
