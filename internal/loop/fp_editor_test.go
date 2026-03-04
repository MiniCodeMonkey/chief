package loop

import (
	"context"
	"strings"
	"testing"
)

func TestFrontPressureEditor_EditDecision(t *testing.T) {
	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "After reviewing the PRD, the concern is valid.\n<fp-decision>edit</fp-decision>\n", nil
		},
	}

	decision, err := editor.Review(context.Background(), "/path/to/prd.json", "US-001", "The data model is wrong", "(none)", "/work")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decision != FPDecisionEdit {
		t.Errorf("Expected FPDecisionEdit, got %v", decision)
	}
}

func TestFrontPressureEditor_DismissDecision(t *testing.T) {
	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "This is an implementation detail the agent can handle.\n<fp-decision>dismiss</fp-decision>\n", nil
		},
	}

	decision, err := editor.Review(context.Background(), "/path/to/prd.json", "US-001", "Not sure about the library to use", "(none)", "/work")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decision != FPDecisionDismiss {
		t.Errorf("Expected FPDecisionDismiss, got %v", decision)
	}
}

func TestFrontPressureEditor_ScrapDecision(t *testing.T) {
	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "The foundational assumptions are completely wrong.\n<fp-decision>scrap</fp-decision>\n", nil
		},
	}

	decision, err := editor.Review(context.Background(), "/path/to/prd.json", "US-001", "The entire approach is wrong", "(none)", "/work")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decision != FPDecisionScrap {
		t.Errorf("Expected FPDecisionScrap, got %v", decision)
	}
}

func TestFrontPressureEditor_NoDecisionTag_DefaultsToDismiss(t *testing.T) {
	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			return "I reviewed the PRD but forgot to output a decision tag.", nil
		},
	}

	decision, err := editor.Review(context.Background(), "/path/to/prd.json", "US-001", "Some concern", "(none)", "/work")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decision != FPDecisionDismiss {
		t.Errorf("Expected FPDecisionDismiss (safe default), got %v", decision)
	}
}

func TestFrontPressureEditor_PromptContainsConcern(t *testing.T) {
	var capturedPrompt string
	editor := &FrontPressureEditor{
		ClaudeRunner: func(ctx context.Context, prompt, workDir string) (string, error) {
			capturedPrompt = prompt
			return "<fp-decision>dismiss</fp-decision>", nil
		},
	}

	concern := "The API contract assumed in stories 8-12 does not exist"
	_, err := editor.Review(context.Background(), "/path/to/prd.json", "US-007", concern, "(none)", "/work")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if capturedPrompt == "" {
		t.Fatal("Expected prompt to be captured")
	}
	if !strings.Contains(capturedPrompt, concern) {
		t.Errorf("Expected prompt to contain concern text %q", concern)
	}
	if !strings.Contains(capturedPrompt, "US-007") {
		t.Errorf("Expected prompt to contain story ID %q", "US-007")
	}
}
