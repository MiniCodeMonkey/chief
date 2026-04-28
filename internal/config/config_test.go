package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserConfigPath_WithXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	got := UserConfigPath()
	want := "/custom/xdg/chief/config.yaml"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestUserConfigPath_WithoutXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	got := UserConfigPath()
	if !strings.HasSuffix(got, filepath.Join(".chief", "config.yaml")) {
		t.Errorf("expected path ending in .chief/config.yaml, got %q", got)
	}
	if strings.HasPrefix(got, "~") {
		t.Errorf("expected expanded home dir, got %q", got)
	}
}

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

func TestLoadUser_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// No file written — directory exists but config.yaml does not.
	cfg, err := LoadUser()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoadUser_FilePresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgDir := filepath.Join(dir, "chief")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("theme: gruvbox-dark\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUser()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Theme != "gruvbox-dark" {
		t.Errorf("expected theme %q, got %q", "gruvbox-dark", cfg.Theme)
	}
}

func TestLoadUser_FileMalformed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgDir := filepath.Join(dir, "chief")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(":\tinvalid: yaml: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUser()
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestMerge_ProjectOverridesUser(t *testing.T) {
	user := &Config{
		Theme:    "user-theme",
		Worktree: WorktreeConfig{Setup: "user-setup"},
		OnComplete: OnCompleteConfig{Push: false, CreatePR: false},
		Agent:    AgentConfig{Provider: "claude", CLIPath: "/usr/bin/claude"},
	}
	project := &Config{
		Theme:    "project-theme",
		Worktree: WorktreeConfig{Setup: "project-setup"},
		OnComplete: OnCompleteConfig{Push: true, CreatePR: true},
		Agent:    AgentConfig{Provider: "codex", CLIPath: "/usr/bin/codex"},
	}
	got := Merge(user, project)
	if got.Theme != "project-theme" {
		t.Errorf("Theme: want %q got %q", "project-theme", got.Theme)
	}
	if got.Worktree.Setup != "project-setup" {
		t.Errorf("Worktree.Setup: want %q got %q", "project-setup", got.Worktree.Setup)
	}
	if !got.OnComplete.Push {
		t.Error("OnComplete.Push: want true")
	}
	if !got.OnComplete.CreatePR {
		t.Error("OnComplete.CreatePR: want true")
	}
	if got.Agent.Provider != "codex" {
		t.Errorf("Agent.Provider: want %q got %q", "codex", got.Agent.Provider)
	}
	if got.Agent.CLIPath != "/usr/bin/codex" {
		t.Errorf("Agent.CLIPath: want %q got %q", "/usr/bin/codex", got.Agent.CLIPath)
	}
}

func TestMerge_UserFillsGapWhenProjectEmpty(t *testing.T) {
	user := &Config{
		Theme:    "user-theme",
		Worktree: WorktreeConfig{Setup: "user-setup"},
		OnComplete: OnCompleteConfig{Push: true, CreatePR: true},
		Agent:    AgentConfig{Provider: "claude", CLIPath: "/usr/bin/claude"},
	}
	project := Default()
	got := Merge(user, project)
	if got.Theme != "user-theme" {
		t.Errorf("Theme: want %q got %q", "user-theme", got.Theme)
	}
	if got.Worktree.Setup != "user-setup" {
		t.Errorf("Worktree.Setup: want %q got %q", "user-setup", got.Worktree.Setup)
	}
	if !got.OnComplete.Push {
		t.Error("OnComplete.Push: want true")
	}
	if !got.OnComplete.CreatePR {
		t.Error("OnComplete.CreatePR: want true")
	}
	if got.Agent.Provider != "claude" {
		t.Errorf("Agent.Provider: want %q got %q", "claude", got.Agent.Provider)
	}
}

func TestMerge_BothEmpty(t *testing.T) {
	got := Merge(Default(), Default())
	if got.Theme != "" {
		t.Errorf("Theme: want empty got %q", got.Theme)
	}
	if got.Worktree.Setup != "" {
		t.Errorf("Worktree.Setup: want empty got %q", got.Worktree.Setup)
	}
	if got.OnComplete.Push {
		t.Error("OnComplete.Push: want false")
	}
	if got.OnComplete.CreatePR {
		t.Error("OnComplete.CreatePR: want false")
	}
}

func TestEndToEnd_UserConfigOnly(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	cfgDir := filepath.Join(xdgDir, "chief")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("theme: gruvbox-dark\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	cfg, err := Load(projectDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Theme != "gruvbox-dark" {
		t.Errorf("expected theme %q, got %q", "gruvbox-dark", cfg.Theme)
	}
}

func TestEndToEnd_ProjectOverridesUser(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	cfgDir := filepath.Join(xdgDir, "chief")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("theme: gruvbox-dark\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	chiefDir := filepath.Join(projectDir, ".chief")
	if err := os.MkdirAll(chiefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chiefDir, "config.yaml"), []byte("theme: dracula\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(projectDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Theme != "dracula" {
		t.Errorf("expected theme %q, got %q", "dracula", cfg.Theme)
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
