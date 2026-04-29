package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestBashTimeout(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"empty disables timeout", "", 0},
		{"valid seconds", "30s", 30 * time.Second},
		{"valid minutes", "5m", 5 * time.Minute},
		{"whitespace padded", "  5m  ", 5 * time.Minute},
		{"invalid disables timeout", "not-a-duration", 0},
		{"negative disables timeout", "-10s", 0},
		{"zero disables timeout", "0s", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Bash: BashConfig{Timeout: tc.in}}
			got := cfg.BashTimeout()
			if got != tc.want {
				t.Errorf("BashTimeout(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestBashTimeout_NilSafe(t *testing.T) {
	var cfg *Config
	if got := cfg.BashTimeout(); got != 0 {
		t.Errorf("nil cfg BashTimeout() = %v, want 0", got)
	}
	if got := cfg.BashTimeoutWarning(); got != "" {
		t.Errorf("nil cfg BashTimeoutWarning() = %q, want empty", got)
	}
}

func TestBashTimeoutWarning_TrimsDisplayedValue(t *testing.T) {
	cfg := &Config{Bash: BashConfig{Timeout: "  garbage  "}}
	got := cfg.BashTimeoutWarning()
	if got == "" {
		t.Fatal("expected warning for unparseable value")
	}
	if !strings.Contains(got, `"garbage"`) {
		t.Errorf("expected warning to quote trimmed value, got %q", got)
	}
	if strings.Contains(got, `"  garbage  "`) {
		t.Errorf("expected leading/trailing whitespace stripped from warning, got %q", got)
	}
}

func TestBashTimeoutWarning(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantEmpty bool
	}{
		{"empty -> no warning", "", true},
		{"valid -> no warning", "30s", true},
		{"invalid -> warning", "not-a-duration", false},
		{"negative -> warning", "-10s", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Bash: BashConfig{Timeout: tc.in}}
			got := cfg.BashTimeoutWarning()
			if (got == "") != tc.wantEmpty {
				t.Errorf("BashTimeoutWarning(%q) = %q, wantEmpty=%v", tc.in, got, tc.wantEmpty)
			}
		})
	}
}

func TestAgentWatchdogTimeout(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"empty uses default", "", DefaultAgentWatchdogTimeout},
		{"valid minutes", "20m", 20 * time.Minute},
		{"valid hours", "1h", time.Hour},
		{"whitespace padded", "  20m  ", 20 * time.Minute},
		{"invalid falls back to default", "ten-minutes", DefaultAgentWatchdogTimeout},
		{"negative falls back to default", "-5m", DefaultAgentWatchdogTimeout},
		{"zero disables watchdog", "0s", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Agent: AgentConfig{WatchdogTimeout: tc.in}}
			got := cfg.AgentWatchdogTimeout()
			if got != tc.want {
				t.Errorf("AgentWatchdogTimeout(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestAgentWatchdogTimeout_NilSafe(t *testing.T) {
	var cfg *Config
	if got := cfg.AgentWatchdogTimeout(); got != DefaultAgentWatchdogTimeout {
		t.Errorf("nil cfg AgentWatchdogTimeout() = %v, want %v", got, DefaultAgentWatchdogTimeout)
	}
}

func TestSaveAndLoadAgentWatchdogTimeout(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Agent: AgentConfig{WatchdogTimeout: "20m"}}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Agent.WatchdogTimeout != "20m" {
		t.Errorf("expected agent.watchdogTimeout='20m', got %q", loaded.Agent.WatchdogTimeout)
	}
	if loaded.AgentWatchdogTimeout() != 20*time.Minute {
		t.Errorf("expected AgentWatchdogTimeout()=20m, got %v", loaded.AgentWatchdogTimeout())
	}
}

func TestSaveAndLoadBashTimeout(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Bash: BashConfig{Timeout: "2m"}}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Bash.Timeout != "2m" {
		t.Errorf("expected bash.timeout='2m', got %q", loaded.Bash.Timeout)
	}
	if loaded.BashTimeout() != 2*time.Minute {
		t.Errorf("expected BashTimeout()=2m, got %v", loaded.BashTimeout())
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
