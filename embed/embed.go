// Package embed provides embedded prompt templates used by Chief.
// All prompts are embedded at compile time using Go's embed directive.
package embed

import (
	_ "embed"
	"strings"
)

//go:embed prompt.txt
var promptTemplate string

//go:embed init_prompt.txt
var initPromptTemplate string

//go:embed edit_prompt.txt
var editPromptTemplate string

//go:embed convert_prompt.txt
var convertPromptTemplate string

//go:embed detect_setup_prompt.txt
var detectSetupPromptTemplate string

// GetPrompt returns the agent prompt with the PRD path, progress path, and
// current story context substituted. The storyContext is the JSON of the
// current story to work on, inlined directly into the prompt so that the
// agent does not need to read the entire prd.json file.
// When frontPressureEnabled is true, a "## Front Pressure" section is appended
// explaining how to raise a concern. If dismissedConcerns is non-empty, a
// "## Previously Dismissed Concerns" section is also appended.
func GetPrompt(prdPath, progressPath, storyContext, storyID, storyTitle string, frontPressureEnabled bool, dismissedConcerns []string) string {
	result := strings.ReplaceAll(promptTemplate, "{{PRD_PATH}}", prdPath)
	result = strings.ReplaceAll(result, "{{PROGRESS_PATH}}", progressPath)
	result = strings.ReplaceAll(result, "{{STORY_CONTEXT}}", storyContext)
	result = strings.ReplaceAll(result, "{{STORY_ID}}", storyID)
	result = strings.ReplaceAll(result, "{{STORY_TITLE}}", storyTitle)

	if frontPressureEnabled {
		result += "\n\n## Front Pressure\n\nIf you discover a **plan-level problem** — not a failing test or a code error, but a fundamental assumption in the PRD that turns out to be wrong — you may raise a concern using the following tag:\n\n```\n<front-pressure>One clear sentence describing what is wrong and why it affects the plan.</front-pressure>\n```\n\nRules:\n- Only use this for plan-level problems (e.g., a data model that makes downstream stories impossible, a missing integration that invalidates the current approach).\n- Do NOT use this for code errors, test failures, or implementation details you can solve yourself.\n- Include exactly one clear sentence describing what is wrong and why it affects the plan."

		if len(dismissedConcerns) > 0 {
			result += "\n\n## Previously Dismissed Concerns\n\nThe following concerns were previously reviewed by an editor and dismissed. Do NOT re-raise them:\n"
			for _, c := range dismissedConcerns {
				result += "\n- " + c
			}
		}
	}

	return result
}

// GetInitPrompt returns the PRD generator prompt with the PRD directory and optional context substituted.
func GetInitPrompt(prdDir, context string) string {
	if context == "" {
		context = "No additional context provided. Ask the user what they want to build."
	}
	result := strings.ReplaceAll(initPromptTemplate, "{{PRD_DIR}}", prdDir)
	return strings.ReplaceAll(result, "{{CONTEXT}}", context)
}

// GetEditPrompt returns the PRD editor prompt with the PRD directory substituted.
func GetEditPrompt(prdDir string) string {
	return strings.ReplaceAll(editPromptTemplate, "{{PRD_DIR}}", prdDir)
}

// GetConvertPrompt returns the PRD converter prompt with the file path and ID prefix substituted.
// Claude reads the file itself using file-reading tools instead of receiving inlined content.
// The idPrefix determines the story ID convention (e.g., "US" → US-001, "MFR" → MFR-001).
func GetConvertPrompt(prdFilePath, idPrefix string) string {
	result := strings.ReplaceAll(convertPromptTemplate, "{{PRD_FILE_PATH}}", prdFilePath)
	return strings.ReplaceAll(result, "{{ID_PREFIX}}", idPrefix)
}

// GetDetectSetupPrompt returns the prompt for detecting project setup commands.
func GetDetectSetupPrompt() string {
	return detectSetupPromptTemplate
}
