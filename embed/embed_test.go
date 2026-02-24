package embed

import (
	"strings"
	"testing"
)

func TestGetPrompt(t *testing.T) {
	prdPath := "/path/to/prd.json"
	progressPath := "/path/to/progress.md"
	prompt := GetPrompt(prdPath, progressPath)

	// Verify the PRD path placeholder was substituted
	if strings.Contains(prompt, "{{PRD_PATH}}") {
		t.Error("Expected {{PRD_PATH}} to be substituted")
	}

	// Verify the progress path placeholder was substituted
	if strings.Contains(prompt, "{{PROGRESS_PATH}}") {
		t.Error("Expected {{PROGRESS_PATH}} to be substituted")
	}

	// Verify the PRD path appears in the prompt
	if !strings.Contains(prompt, prdPath) {
		t.Errorf("Expected prompt to contain PRD path %q", prdPath)
	}

	// Verify the progress path appears in the prompt
	if !strings.Contains(prompt, progressPath) {
		t.Errorf("Expected prompt to contain progress path %q", progressPath)
	}

	// Verify the prompt contains key instructions
	if !strings.Contains(prompt, "chief-complete") {
		t.Error("Expected prompt to contain chief-complete instruction")
	}

	if !strings.Contains(prompt, "ralph-status") {
		t.Error("Expected prompt to contain ralph-status instruction")
	}

	if !strings.Contains(prompt, "passes: true") {
		t.Error("Expected prompt to contain passes: true instruction")
	}
}

func TestPromptTemplateNotEmpty(t *testing.T) {
	if promptTemplate == "" {
		t.Error("Expected promptTemplate to be embedded and non-empty")
	}
}

func TestGetConvertPrompt(t *testing.T) {
	prdContent := "# My Feature\n\nA cool feature PRD."
	prompt := GetConvertPrompt(prdContent)

	// Verify the prompt is not empty
	if prompt == "" {
		t.Error("Expected GetConvertPrompt() to return non-empty prompt")
	}

	// Verify PRD content is inlined
	if !strings.Contains(prompt, prdContent) {
		t.Error("Expected prompt to contain the inlined PRD content")
	}
	if strings.Contains(prompt, "{{PRD_CONTENT}}") {
		t.Error("Expected {{PRD_CONTENT}} to be substituted")
	}

	// Verify key instructions are present
	if !strings.Contains(prompt, "JSON") {
		t.Error("Expected prompt to mention JSON")
	}

	if !strings.Contains(prompt, "userStories") {
		t.Error("Expected prompt to describe userStories structure")
	}

	if !strings.Contains(prompt, `"passes": false`) {
		t.Error("Expected prompt to specify passes: false default")
	}
}

func TestGetInitPrompt(t *testing.T) {
	prdDir := "/path/to/.chief/prds/main"

	// Test with no context
	prompt := GetInitPrompt(prdDir, "")
	if !strings.Contains(prompt, "No additional context provided") {
		t.Error("Expected default context message")
	}

	// Verify PRD directory is substituted
	if !strings.Contains(prompt, prdDir) {
		t.Errorf("Expected prompt to contain PRD directory %q", prdDir)
	}
	if strings.Contains(prompt, "{{PRD_DIR}}") {
		t.Error("Expected {{PRD_DIR}} to be substituted")
	}

	// Test with context
	context := "Build a todo app"
	promptWithContext := GetInitPrompt(prdDir, context)
	if !strings.Contains(promptWithContext, context) {
		t.Error("Expected context to be substituted in prompt")
	}
}

func TestGetEditPrompt(t *testing.T) {
	prompt := GetEditPrompt("/test/path/prds/main")
	if prompt == "" {
		t.Error("Expected GetEditPrompt() to return non-empty prompt")
	}
	if !strings.Contains(prompt, "/test/path/prds/main") {
		t.Error("Expected prompt to contain the PRD directory path")
	}
}
