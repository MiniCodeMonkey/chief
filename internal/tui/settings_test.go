package tui

import (
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/config"
)

func TestSettingsOverlay_LoadFromConfig(t *testing.T) {
	s := NewSettingsOverlay()
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{
			Setup:               "npm install",
			AlwaysPrompt:        false,
			PromptBranchPattern: "^(main|master)$",
		},
		OnComplete: config.OnCompleteConfig{
			Push:     true,
			CreatePR: false,
		},
	}
	s.LoadFromConfig(cfg)

	if len(s.items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(s.items))
	}
	if s.items[0].Key != "worktree.setup" || s.items[0].StringVal != "npm install" {
		t.Errorf("worktree.setup item: got key=%s val=%s", s.items[0].Key, s.items[0].StringVal)
	}
	if s.items[1].Key != "worktree.alwaysPrompt" || s.items[1].BoolVal {
		t.Errorf("worktree.alwaysPrompt item: got key=%s val=%v", s.items[1].Key, s.items[1].BoolVal)
	}
	if s.items[2].Key != "worktree.promptBranchPattern" || s.items[2].StringVal != "^(main|master)$" {
		t.Errorf("worktree.promptBranchPattern item: got key=%s val=%s", s.items[2].Key, s.items[2].StringVal)
	}
	if s.items[3].Key != "onComplete.push" || !s.items[3].BoolVal {
		t.Errorf("onComplete.push item: got key=%s val=%v", s.items[3].Key, s.items[3].BoolVal)
	}
	if s.items[4].Key != "onComplete.createPR" || s.items[4].BoolVal {
		t.Errorf("onComplete.createPR item: got key=%s val=%v", s.items[4].Key, s.items[4].BoolVal)
	}
	if s.selectedIndex != 0 {
		t.Errorf("expected selectedIndex=0, got %d", s.selectedIndex)
	}
}

func TestSettingsOverlay_ApplyToConfig(t *testing.T) {
	s := NewSettingsOverlay()
	cfg := config.Default()
	s.LoadFromConfig(cfg)

	// Modify items
	s.items[0].StringVal = "go mod download" // worktree.setup
	s.items[1].BoolVal = true                // worktree.alwaysPrompt
	s.items[2].StringVal = "^release/.*$"    // worktree.promptBranchPattern
	s.items[3].BoolVal = true                // onComplete.push
	s.items[4].BoolVal = true                // onComplete.createPR

	resultCfg := config.Default()
	s.ApplyToConfig(resultCfg)

	if resultCfg.Worktree.Setup != "go mod download" {
		t.Errorf("expected setup='go mod download', got '%s'", resultCfg.Worktree.Setup)
	}
	if !resultCfg.Worktree.AlwaysPrompt {
		t.Error("expected alwaysPrompt=true")
	}
	if resultCfg.Worktree.PromptBranchPattern != "^release/.*$" {
		t.Errorf("expected promptBranchPattern='^release/.*$', got '%s'", resultCfg.Worktree.PromptBranchPattern)
	}
	if !resultCfg.OnComplete.Push {
		t.Error("expected push=true")
	}
	if !resultCfg.OnComplete.CreatePR {
		t.Error("expected createPR=true")
	}
}

func TestSettingsOverlay_Navigation(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())

	if s.selectedIndex != 0 {
		t.Fatalf("expected initial index=0, got %d", s.selectedIndex)
	}

	// Move down through every item
	for i := 1; i <= 4; i++ {
		s.MoveDown()
		if s.selectedIndex != i {
			t.Errorf("expected index=%d after MoveDown, got %d", i, s.selectedIndex)
		}
	}

	// Can't go beyond last item
	s.MoveDown()
	if s.selectedIndex != 4 {
		t.Errorf("expected index=4 (clamped), got %d", s.selectedIndex)
	}

	s.MoveUp()
	if s.selectedIndex != 3 {
		t.Errorf("expected index=3 after MoveUp, got %d", s.selectedIndex)
	}

	// Can't go before first item
	for i := 0; i < 10; i++ {
		s.MoveUp()
	}
	if s.selectedIndex != 0 {
		t.Errorf("expected index=0 (clamped), got %d", s.selectedIndex)
	}
}

func TestSettingsOverlay_ToggleBool(t *testing.T) {
	s := NewSettingsOverlay()
	cfg := &config.Config{
		OnComplete: config.OnCompleteConfig{Push: false},
	}
	s.LoadFromConfig(cfg)

	// Navigate to "Push to remote" (index 3)
	for i := 0; i < 3; i++ {
		s.MoveDown()
	}

	key, val := s.ToggleBool()
	if key != "onComplete.push" {
		t.Errorf("expected key='onComplete.push', got '%s'", key)
	}
	if !val {
		t.Error("expected val=true after toggle")
	}

	// Toggle back
	_, val = s.ToggleBool()
	if val {
		t.Error("expected val=false after second toggle")
	}
}

func TestSettingsOverlay_ToggleBool_OnStringItem(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())

	// Selected item is "Setup command" (string type)
	key, _ := s.ToggleBool()
	if key != "" {
		t.Errorf("expected empty key for string item toggle, got '%s'", key)
	}
}

func TestSettingsOverlay_RevertToggle(t *testing.T) {
	s := NewSettingsOverlay()
	cfg := &config.Config{
		OnComplete: config.OnCompleteConfig{Push: false},
	}
	s.LoadFromConfig(cfg)

	// Navigate to "Push to remote" (index 3)
	for i := 0; i < 3; i++ {
		s.MoveDown()
	}
	s.ToggleBool()
	if !s.items[3].BoolVal {
		t.Fatal("expected true after toggle")
	}

	s.RevertToggle()
	if s.items[3].BoolVal {
		t.Error("expected false after revert")
	}
}

func TestSettingsOverlay_StringEditing(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())

	// Selected item is "Setup command" (index 0)
	if s.IsEditing() {
		t.Fatal("should not be editing initially")
	}

	s.StartEditing()
	if !s.IsEditing() {
		t.Fatal("should be editing after StartEditing")
	}
	if s.editBuffer != "" {
		t.Errorf("expected empty edit buffer, got '%s'", s.editBuffer)
	}

	s.AddEditChar('n')
	s.AddEditChar('p')
	s.AddEditChar('m')
	if s.editBuffer != "npm" {
		t.Errorf("expected 'npm', got '%s'", s.editBuffer)
	}

	s.DeleteEditChar()
	if s.editBuffer != "np" {
		t.Errorf("expected 'np' after delete, got '%s'", s.editBuffer)
	}

	if err := s.ConfirmEdit(); err != nil {
		t.Fatalf("ConfirmEdit unexpected error: %v", err)
	}
	if s.IsEditing() {
		t.Fatal("should not be editing after ConfirmEdit")
	}
	if s.items[0].StringVal != "np" {
		t.Errorf("expected StringVal='np', got '%s'", s.items[0].StringVal)
	}
}

func TestSettingsOverlay_CancelEdit(t *testing.T) {
	s := NewSettingsOverlay()
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{Setup: "original"},
	}
	s.LoadFromConfig(cfg)

	s.StartEditing()
	s.AddEditChar('x')
	s.CancelEdit()

	if s.IsEditing() {
		t.Fatal("should not be editing after CancelEdit")
	}
	if s.items[0].StringVal != "original" {
		t.Errorf("expected 'original' preserved, got '%s'", s.items[0].StringVal)
	}
}

func TestSettingsOverlay_StartEditingOnBoolItem(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())
	s.MoveDown() // Select "Always prompt for worktree" (bool, index 1)

	s.StartEditing()
	if s.IsEditing() {
		t.Error("should not start editing on a bool item")
	}
}

func TestSettingsOverlay_ConfirmEdit_InvalidRegex(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())

	// Navigate to "Prompt branch pattern" (index 2)
	s.MoveDown()
	s.MoveDown()

	original := s.items[2].StringVal
	s.StartEditing()
	for s.editBuffer != "" {
		s.DeleteEditChar()
	}
	for _, ch := range "[bad" {
		s.AddEditChar(ch)
	}

	err := s.ConfirmEdit()
	if err == nil {
		t.Fatal("expected error from ConfirmEdit on invalid regex, got nil")
	}
	if !s.IsEditing() {
		t.Error("expected to remain in edit mode after rejection")
	}
	if s.items[2].StringVal != original {
		t.Errorf("expected item value unchanged on rejection, got %q (was %q)", s.items[2].StringVal, original)
	}
	if !strings.Contains(s.editError, "invalid regex") {
		t.Errorf("expected editError to mention invalid regex, got %q", s.editError)
	}
}

func TestSettingsOverlay_ConfirmEdit_ValidRegex(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())

	// Navigate to "Prompt branch pattern" (index 2)
	s.MoveDown()
	s.MoveDown()
	s.StartEditing()
	for s.editBuffer != "" {
		s.DeleteEditChar()
	}
	for _, ch := range "^release/.*$" {
		s.AddEditChar(ch)
	}

	if err := s.ConfirmEdit(); err != nil {
		t.Fatalf("ConfirmEdit unexpected error: %v", err)
	}
	if s.IsEditing() {
		t.Error("expected editor to close after valid input")
	}
	if s.items[2].StringVal != "^release/.*$" {
		t.Errorf("expected pattern saved, got %q", s.items[2].StringVal)
	}
}

func TestSettingsOverlay_ConfirmEdit_EmptyPatternIsValid(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())

	// Navigate to "Prompt branch pattern" (index 2) and clear it.
	s.MoveDown()
	s.MoveDown()
	s.StartEditing()
	for s.editBuffer != "" {
		s.DeleteEditChar()
	}

	if err := s.ConfirmEdit(); err != nil {
		t.Fatalf("ConfirmEdit on empty pattern returned error: %v", err)
	}
	if s.items[2].StringVal != "" {
		t.Errorf("expected empty pattern saved, got %q", s.items[2].StringVal)
	}
}

func TestSettingsOverlay_GHError(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())

	if s.HasGHError() {
		t.Fatal("should not have GH error initially")
	}

	s.SetGHError("gh not found")
	if !s.HasGHError() {
		t.Fatal("should have GH error after SetGHError")
	}

	s.DismissGHError()
	if s.HasGHError() {
		t.Fatal("should not have GH error after dismiss")
	}
}

func TestSettingsOverlay_Render(t *testing.T) {
	s := NewSettingsOverlay()
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{Setup: "npm install"},
		OnComplete: config.OnCompleteConfig{
			Push:     true,
			CreatePR: false,
		},
	}
	s.LoadFromConfig(cfg)
	s.SetSize(80, 24)

	rendered := s.Render()

	// Check header
	if !strings.Contains(rendered, "Settings") {
		t.Error("expected 'Settings' in header")
	}
	if !strings.Contains(rendered, ".chief/config.yaml") {
		t.Error("expected config path in header")
	}

	// Check section headers
	if !strings.Contains(rendered, "Worktree") {
		t.Error("expected 'Worktree' section")
	}
	if !strings.Contains(rendered, "On Complete") {
		t.Error("expected 'On Complete' section")
	}

	// Check values
	if !strings.Contains(rendered, "npm install") {
		t.Error("expected 'npm install' value")
	}
	if !strings.Contains(rendered, "Yes") {
		t.Error("expected 'Yes' for push")
	}
	if !strings.Contains(rendered, "No") {
		t.Error("expected 'No' for createPR")
	}

	// Check footer
	if !strings.Contains(rendered, "Esc: close") {
		t.Error("expected 'Esc: close' in footer")
	}
}

func TestSettingsOverlay_RenderEditError(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())
	s.SetSize(80, 24)

	// Navigate to promptBranchPattern, type a bad regex, attempt confirm.
	s.MoveDown()
	s.MoveDown()
	s.StartEditing()
	for s.editBuffer != "" {
		s.DeleteEditChar()
	}
	for _, ch := range "[bad" {
		s.AddEditChar(ch)
	}
	_ = s.ConfirmEdit()

	rendered := s.Render()
	if !strings.Contains(rendered, "invalid regex") {
		t.Error("expected rendered output to contain 'invalid regex' message")
	}
}

func TestSettingsOverlay_RenderGHError(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())
	s.SetSize(80, 24)

	s.SetGHError("gh not found")
	rendered := s.Render()

	if !strings.Contains(rendered, "GitHub CLI Error") {
		t.Error("expected 'GitHub CLI Error' in rendered output")
	}
	if !strings.Contains(rendered, "gh not found") {
		t.Error("expected error message in rendered output")
	}
	if !strings.Contains(rendered, "Press any key to dismiss") {
		t.Error("expected dismiss hint in footer")
	}
}

func TestSettingsOverlay_RenderEditing(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())
	s.SetSize(80, 24)

	s.StartEditing()
	rendered := s.Render()

	if !strings.Contains(rendered, "Enter: save") {
		t.Error("expected 'Enter: save' in footer during editing")
	}
}

func TestSettingsOverlay_RenderSelectedIndicator(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())
	s.SetSize(80, 24)

	rendered := s.Render()

	// The selected item should have a ">" indicator
	if !strings.Contains(rendered, ">") {
		t.Error("expected '>' cursor indicator for selected item")
	}
}

func TestSettingsOverlay_RenderEmptyStringValue(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())
	s.SetSize(80, 24)

	rendered := s.Render()

	if !strings.Contains(rendered, "(not set)") {
		t.Error("expected '(not set)' for empty setup command")
	}
}

func TestSettingsOverlay_GetSelectedItem(t *testing.T) {
	s := NewSettingsOverlay()
	s.LoadFromConfig(config.Default())

	item := s.GetSelectedItem()
	if item == nil {
		t.Fatal("expected non-nil selected item")
	}
	if item.Key != "worktree.setup" {
		t.Errorf("expected first item key='worktree.setup', got '%s'", item.Key)
	}

	s.MoveDown()
	item = s.GetSelectedItem()
	if item.Key != "worktree.alwaysPrompt" {
		t.Errorf("expected second item key='worktree.alwaysPrompt', got '%s'", item.Key)
	}
}
