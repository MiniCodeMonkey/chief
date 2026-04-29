package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Worktree.Setup != "" {
		t.Errorf("expected empty setup, got %q", cfg.Worktree.Setup)
	}
	if cfg.Worktree.AlwaysPrompt {
		t.Error("expected AlwaysPrompt to be false")
	}
	if cfg.Worktree.PromptBranchPattern != "^(main|master)$" {
		t.Errorf("expected default PromptBranchPattern, got %q", cfg.Worktree.PromptBranchPattern)
	}
	if cfg.OnComplete.Push {
		t.Error("expected Push to be false")
	}
	if cfg.OnComplete.CreatePR {
		t.Error("expected CreatePR to be false")
	}
	if !cfg.ShouldPromptForWorktree("main") {
		t.Error("expected default to prompt on main")
	}
	if cfg.ShouldPromptForWorktree("feature/x") {
		t.Error("expected default to not prompt on feature/x")
	}
}

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Worktree.Setup != "" {
		t.Errorf("expected empty setup, got %q", cfg.Worktree.Setup)
	}
	if !cfg.ShouldPromptForWorktree("main") {
		t.Error("expected default load to prompt on main")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Worktree: WorktreeConfig{
			Setup: "npm install",
		},
		OnComplete: OnCompleteConfig{
			Push:     true,
			CreatePR: true,
		},
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Worktree.Setup != "npm install" {
		t.Errorf("expected setup %q, got %q", "npm install", loaded.Worktree.Setup)
	}
	if !loaded.OnComplete.Push {
		t.Error("expected Push to be true")
	}
	if !loaded.OnComplete.CreatePR {
		t.Error("expected CreatePR to be true")
	}
}

func TestSaveAndLoadPromptFields(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Worktree: WorktreeConfig{
			AlwaysPrompt:        true,
			PromptBranchPattern: "^release/.*$",
		},
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !loaded.Worktree.AlwaysPrompt {
		t.Error("expected AlwaysPrompt to be true after round-trip")
	}
	if loaded.Worktree.PromptBranchPattern != "^release/.*$" {
		t.Errorf("expected PromptBranchPattern %q, got %q", "^release/.*$", loaded.Worktree.PromptBranchPattern)
	}
	if !loaded.ShouldPromptForWorktree("release/v1") {
		t.Error("expected ShouldPromptForWorktree to return true for release/v1")
	}
}

func TestLoadInvalidPromptRegex(t *testing.T) {
	dir := t.TempDir()
	chiefDir := filepath.Join(dir, ".chief")
	if err := os.MkdirAll(chiefDir, 0755); err != nil {
		t.Fatal(err)
	}
	yaml := "worktree:\n  promptBranchPattern: \"[unclosed\"\n"
	if err := os.WriteFile(filepath.Join(chiefDir, "config.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
	if !strings.Contains(err.Error(), "worktree.promptBranchPattern") {
		t.Errorf("expected error to mention worktree.promptBranchPattern, got %q", err.Error())
	}
}

func TestLoadLegacyConfigInheritsDefaults(t *testing.T) {
	dir := t.TempDir()
	chiefDir := filepath.Join(dir, ".chief")
	if err := os.MkdirAll(chiefDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Legacy configs without the new keys inherit Default() values; protected-branch
	// safety is preserved on upgrade.
	yaml := "worktree:\n  setup: \"npm install\"\n"
	if err := os.WriteFile(filepath.Join(chiefDir, "config.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Worktree.Setup != "npm install" {
		t.Errorf("expected setup %q, got %q", "npm install", cfg.Worktree.Setup)
	}
	if cfg.Worktree.AlwaysPrompt {
		t.Error("expected AlwaysPrompt to be false (default)")
	}
	if cfg.Worktree.PromptBranchPattern != "^(main|master)$" {
		t.Errorf("expected PromptBranchPattern to inherit default %q, got %q", "^(main|master)$", cfg.Worktree.PromptBranchPattern)
	}
	if !cfg.ShouldPromptForWorktree("main") {
		t.Error("expected ShouldPromptForWorktree(main) to be true after legacy load")
	}
	if cfg.ShouldPromptForWorktree("feature/x") {
		t.Error("expected ShouldPromptForWorktree(feature/x) to be false after legacy load")
	}
}

func TestLoadExplicitEmptyPromptPattern(t *testing.T) {
	dir := t.TempDir()
	chiefDir := filepath.Join(dir, ".chief")
	if err := os.MkdirAll(chiefDir, 0755); err != nil {
		t.Fatal(err)
	}
	// An explicit empty value opts out of branch-name matching.
	yaml := "worktree:\n  promptBranchPattern: \"\"\n"
	if err := os.WriteFile(filepath.Join(chiefDir, "config.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Worktree.PromptBranchPattern != "" {
		t.Errorf("expected PromptBranchPattern to be empty, got %q", cfg.Worktree.PromptBranchPattern)
	}
	if cfg.ShouldPromptForWorktree("main") {
		t.Error("expected ShouldPromptForWorktree(main) to be false with explicit empty pattern")
	}
}

func TestSaveValidatesBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Worktree: WorktreeConfig{
			PromptBranchPattern: "[bad",
		},
	}

	err := Save(dir, cfg)
	if err == nil {
		t.Fatal("expected error from Save with invalid regex, got nil")
	}
	if !strings.Contains(err.Error(), "worktree.promptBranchPattern") {
		t.Errorf("expected error to mention worktree.promptBranchPattern, got %q", err.Error())
	}

	if _, statErr := os.Stat(filepath.Join(dir, ".chief", "config.yaml")); !os.IsNotExist(statErr) {
		t.Errorf("expected config file not to exist after failed Save, got stat err %v", statErr)
	}
}

func TestShouldPromptForWorktree(t *testing.T) {
	tests := []struct {
		name         string
		alwaysPrompt bool
		pattern      string
		branch       string
		want         bool
	}{
		{"alwaysPrompt wins with empty pattern", true, "", "anything", true},
		{"alwaysPrompt wins over non-matching pattern", true, "^main$", "feature/x", true},
		{"default pattern matches main", false, "^(main|master)$", "main", true},
		{"default pattern matches master", false, "^(main|master)$", "master", true},
		{"default pattern rejects feature branch", false, "^(main|master)$", "feature/x", false},
		{"empty pattern with no alwaysPrompt rejects main", false, "", "main", false},
		{"release pattern matches release branch", false, "^release/.*$", "release/v1", true},
		{"release pattern rejects main", false, "^release/.*$", "main", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Worktree: WorktreeConfig{
					AlwaysPrompt:        tt.alwaysPrompt,
					PromptBranchPattern: tt.pattern,
				},
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate failed: %v", err)
			}
			got := cfg.ShouldPromptForWorktree(tt.branch)
			if got != tt.want {
				t.Errorf("ShouldPromptForWorktree(%q) = %v; want %v", tt.branch, got, tt.want)
			}
		})
	}
}

func TestValidateInvalidRegex(t *testing.T) {
	cfg := &Config{
		Worktree: WorktreeConfig{
			PromptBranchPattern: "[unclosed",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
	if !strings.Contains(err.Error(), "worktree.promptBranchPattern") {
		t.Errorf("expected error to mention worktree.promptBranchPattern, got %q", err.Error())
	}
}

func TestValidateEmptyPatternIsValid(t *testing.T) {
	cfg := &Config{
		Worktree: WorktreeConfig{
			PromptBranchPattern: "",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.ShouldPromptForWorktree("main") {
		t.Error("expected ShouldPromptForWorktree to be false for empty pattern")
	}
	if cfg.ShouldPromptForWorktree("anything") {
		t.Error("expected ShouldPromptForWorktree to be false for empty pattern")
	}
}

func TestValidateBranchPattern(t *testing.T) {
	t.Run("empty returns nil regex and nil error", func(t *testing.T) {
		re, err := ValidateBranchPattern("")
		if err != nil {
			t.Fatalf("unexpected error for empty pattern: %v", err)
		}
		if re != nil {
			t.Error("expected nil *Regexp for empty pattern")
		}
	})

	t.Run("valid pattern returns compiled regex", func(t *testing.T) {
		re, err := ValidateBranchPattern("^release/.*$")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if re == nil {
			t.Fatal("expected non-nil *Regexp for valid pattern")
		}
		if !re.MatchString("release/v1") {
			t.Error("expected compiled regex to match release/v1")
		}
		if re.MatchString("main") {
			t.Error("expected compiled regex not to match main")
		}
	})

	t.Run("invalid pattern returns error and nil regex", func(t *testing.T) {
		re, err := ValidateBranchPattern("[unclosed")
		if err == nil {
			t.Fatal("expected error for invalid pattern, got nil")
		}
		if re != nil {
			t.Error("expected nil *Regexp on validation failure")
		}
	})
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	chiefDir := filepath.Join(dir, ".chief")
	if err := os.MkdirAll(chiefDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chiefDir, "config.yaml"), []byte("worktree: [unterminated\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestLoadReadError(t *testing.T) {
	dir := t.TempDir()
	chiefDir := filepath.Join(dir, ".chief")
	if err := os.MkdirAll(chiefDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Make config.yaml a directory so ReadFile returns a non-IsNotExist error.
	if err := os.MkdirAll(filepath.Join(chiefDir, "config.yaml"), 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("expected error when config.yaml is a directory, got nil")
	}
}

func TestSaveMkdirError(t *testing.T) {
	dir := t.TempDir()
	// Create a file at the path where the .chief directory needs to live.
	blocking := filepath.Join(dir, ".chief")
	if err := os.WriteFile(blocking, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Save(dir, &Config{}); err == nil {
		t.Fatal("expected error when .chief path is a file, got nil")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()

	if Exists(dir) {
		t.Error("expected Exists to return false for missing config")
	}

	// Create the config
	chiefDir := filepath.Join(dir, ".chief")
	if err := os.MkdirAll(chiefDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chiefDir, "config.yaml"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if !Exists(dir) {
		t.Error("expected Exists to return true for existing config")
	}
}
