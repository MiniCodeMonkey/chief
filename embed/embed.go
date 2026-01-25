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

// GetPrompt returns the agent prompt with the PRD path substituted.
func GetPrompt(prdPath string) string {
	return strings.ReplaceAll(promptTemplate, "{{PRD_PATH}}", prdPath)
}

// GetInitPrompt returns the PRD generator prompt with optional context substituted.
func GetInitPrompt(context string) string {
	if context == "" {
		context = "No additional context provided. Ask the user what they want to build."
	}
	return strings.ReplaceAll(initPromptTemplate, "{{CONTEXT}}", context)
}

// GetEditPrompt returns the PRD editor prompt.
func GetEditPrompt() string {
	return editPromptTemplate
}

// GetConvertPrompt returns the PRD converter prompt for prd.md to prd.json conversion.
func GetConvertPrompt() string {
	return convertPromptTemplate
}
