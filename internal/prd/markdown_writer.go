package prd

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// SetStoryStatus performs a surgical update of a story's status in a prd.md file.
// It finds the story block by its heading, updates or inserts the **Status:** line,
// and when status is "done", flips all unchecked checkboxes to checked.
func SetStoryStatus(path, storyID, status string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read PRD file: %w", err)
	}

	result, err := setStoryStatusInString(string(data), storyID, status)
	if err != nil {
		return err
	}

	return os.WriteFile(path, []byte(result), 0644)
}

// setStoryStatusInString performs the status update on a string and returns the modified string.
func setStoryStatusInString(content, storyID, status string) (string, error) {
	lines := strings.Split(content, "\n")

	// Find the story block
	storyStart := -1
	storyEnd := len(lines) // default to end of file

	headingPattern := regexp.MustCompile(`^###\s+` + regexp.QuoteMeta(storyID) + `:\s+`)

	for i, line := range lines {
		if storyStart == -1 {
			// Looking for the story heading
			if headingPattern.MatchString(strings.TrimSpace(line)) {
				storyStart = i
			}
		} else {
			// Looking for the end of the story block (next ## or ### heading)
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
				storyEnd = i
				break
			}
		}
	}

	if storyStart == -1 {
		return "", fmt.Errorf("story %s not found in PRD", storyID)
	}

	// Process the story block
	statusLineIdx := -1
	statusLine := fmt.Sprintf("**Status:** %s", status)

	for i := storyStart + 1; i < storyEnd; i++ {
		if statusLineRegex.MatchString(strings.TrimSpace(lines[i])) {
			statusLineIdx = i
			break
		}
	}

	if statusLineIdx >= 0 {
		// Replace existing status line
		lines[statusLineIdx] = statusLine
	} else {
		// Insert status line as first line after heading
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:storyStart+1]...)
		newLines = append(newLines, statusLine)
		newLines = append(newLines, lines[storyStart+1:]...)
		lines = newLines
		storyEnd++ // adjust for the inserted line
	}

	// When status is "done", flip all unchecked checkboxes to checked
	if status == "done" {
		for i := storyStart + 1; i < storyEnd; i++ {
			lines[i] = strings.Replace(lines[i], "- [ ]", "- [x]", 1)
		}
	}

	return strings.Join(lines, "\n"), nil
}
