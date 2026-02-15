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

// GetPrompt returns the agent prompt with the PRD path substituted.
func GetPrompt(prdPath string) string {
	return strings.ReplaceAll(promptTemplate, "{{PRD_PATH}}", prdPath)
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

// GetConvertPrompt returns the PRD converter prompt for prd.md to prd.json conversion.
func GetConvertPrompt(prdDir string) string {
	return strings.ReplaceAll(convertPromptTemplate, "{{PRD_DIR}}", prdDir)
}

// GetDetectSetupPrompt returns the prompt for detecting project setup commands.
func GetDetectSetupPrompt() string {
	return detectSetupPromptTemplate
}
