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
		Agent: AgentConfig{
			Provider: "opencode",
			CLIPath:  "/usr/local/bin/opencode-global",
			OpenCode: OpenCodeAgentConfig{
				CLIPath:     "/usr/local/bin/opencode",
				RequiredEnv: []string{"OPENAI_API_KEY", "OPENCODE_PROFILE"},
			},
		},
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
	if loaded.Agent.Provider != "opencode" {
		t.Errorf("expected provider %q, got %q", "opencode", loaded.Agent.Provider)
	}
	if loaded.Agent.CLIPath != "/usr/local/bin/opencode-global" {
		t.Errorf("expected global cliPath %q, got %q", "/usr/local/bin/opencode-global", loaded.Agent.CLIPath)
	}
	if loaded.Agent.OpenCode.CLIPath != "/usr/local/bin/opencode" {
		t.Errorf("expected opencode cliPath %q, got %q", "/usr/local/bin/opencode", loaded.Agent.OpenCode.CLIPath)
	}
	if len(loaded.Agent.OpenCode.RequiredEnv) != 2 {
		t.Fatalf("expected 2 requiredEnv entries, got %d", len(loaded.Agent.OpenCode.RequiredEnv))
	}
	if loaded.Agent.OpenCode.RequiredEnv[0] != "OPENAI_API_KEY" || loaded.Agent.OpenCode.RequiredEnv[1] != "OPENCODE_PROFILE" {
		t.Errorf("unexpected requiredEnv values: %v", loaded.Agent.OpenCode.RequiredEnv)
	}
	if !loaded.OnComplete.Push {
		t.Error("expected Push to be true")
	}
	if !loaded.OnComplete.CreatePR {
		t.Error("expected CreatePR to be true")
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
