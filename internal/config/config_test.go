package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Worktree.Setup != "" {
		t.Errorf("expected empty setup, got %q", cfg.Worktree.Setup)
	}
	if cfg.OnComplete.Push {
		t.Error("expected Push to be false")
	}
	if cfg.OnComplete.CreatePR {
		t.Error("expected CreatePR to be false")
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

func TestSaveAndLoadSettingsFields(t *testing.T) {
	dir := t.TempDir()
	autoCommit := false
	cfg := &Config{
		MaxIterations: 10,
		AutoCommit:    &autoCommit,
		CommitPrefix:  "feat:",
		ClaudeModel:   "claude-sonnet-4-5-20250929",
		TestCommand:   "go test ./...",
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.MaxIterations != 10 {
		t.Errorf("expected MaxIterations 10, got %d", loaded.MaxIterations)
	}
	if loaded.AutoCommit == nil || *loaded.AutoCommit != false {
		t.Errorf("expected AutoCommit false, got %v", loaded.AutoCommit)
	}
	if loaded.CommitPrefix != "feat:" {
		t.Errorf("expected CommitPrefix %q, got %q", "feat:", loaded.CommitPrefix)
	}
	if loaded.ClaudeModel != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected ClaudeModel %q, got %q", "claude-sonnet-4-5-20250929", loaded.ClaudeModel)
	}
	if loaded.TestCommand != "go test ./..." {
		t.Errorf("expected TestCommand %q, got %q", "go test ./...", loaded.TestCommand)
	}
}

func TestEffectiveDefaults(t *testing.T) {
	cfg := Default()

	if cfg.EffectiveMaxIterations() != 5 {
		t.Errorf("expected EffectiveMaxIterations 5, got %d", cfg.EffectiveMaxIterations())
	}
	if !cfg.EffectiveAutoCommit() {
		t.Error("expected EffectiveAutoCommit true")
	}

	// With explicit values
	cfg.MaxIterations = 3
	autoCommit := false
	cfg.AutoCommit = &autoCommit

	if cfg.EffectiveMaxIterations() != 3 {
		t.Errorf("expected EffectiveMaxIterations 3, got %d", cfg.EffectiveMaxIterations())
	}
	if cfg.EffectiveAutoCommit() {
		t.Error("expected EffectiveAutoCommit false")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()

	if Exists(dir) {
		t.Error("expected Exists to return false for missing config")
	}

	// Create the config
	chiefDir := filepath.Join(dir, ".chief")
	if err := os.MkdirAll(chiefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chiefDir, "config.yaml"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !Exists(dir) {
		t.Error("expected Exists to return true for existing config")
	}
}
