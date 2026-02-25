package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/loop"
)

func mustResolve(t *testing.T, flagAgent, flagPath string, cfg *config.Config) loop.Provider {
	t.Helper()
	p, err := Resolve(flagAgent, flagPath, cfg)
	if err != nil {
		t.Fatalf("Resolve(%q, %q, cfg) unexpected error: %v", flagAgent, flagPath, err)
	}
	return p
}

func TestResolve_priority(t *testing.T) {
	// Default: no flag, no env, nil config -> Claude
	got := mustResolve(t, "", "", nil)
	if got.Name() != "Claude" {
		t.Errorf("Resolve(_, _, nil) name = %q, want Claude", got.Name())
	}
	if got.CLIPath() != "claude" {
		t.Errorf("Resolve(_, _, nil) CLIPath = %q, want claude", got.CLIPath())
	}

	// Flag overrides everything
	got = mustResolve(t, "codex", "", nil)
	if got.Name() != "Codex" {
		t.Errorf("Resolve(codex, _, nil) name = %q, want Codex", got.Name())
	}
	got = mustResolve(t, "opencode", "", nil)
	if got.Name() != "OpenCode" {
		t.Errorf("Resolve(opencode, _, nil) name = %q, want OpenCode", got.Name())
	}
	if got.CLIPath() != "opencode" {
		t.Errorf("Resolve(opencode, _, nil) CLIPath = %q, want opencode", got.CLIPath())
	}

	// Config only (no flag, no env)
	cfg := &config.Config{}
	cfg.Agent.Provider = "codex"
	cfg.Agent.CLIPath = "/usr/local/bin/codex"
	got = mustResolve(t, "", "", cfg)
	if got.Name() != "Codex" {
		t.Errorf("Resolve(_, _, config codex) name = %q, want Codex", got.Name())
	}
	if got.CLIPath() != "/usr/local/bin/codex" {
		t.Errorf("Resolve(_, _, config) CLIPath = %q, want /usr/local/bin/codex", got.CLIPath())
	}

	// Flag overrides config
	got = mustResolve(t, "claude", "", cfg)
	if got.Name() != "Claude" {
		t.Errorf("Resolve(claude, _, config codex) name = %q, want Claude", got.Name())
	}
	// flag path overrides config path
	got = mustResolve(t, "codex", "/opt/codex", cfg)
	if got.CLIPath() != "/opt/codex" {
		t.Errorf("Resolve(codex, /opt/codex, cfg) CLIPath = %q, want /opt/codex", got.CLIPath())
	}
}

func TestResolve_env(t *testing.T) {
	const keyAgent = "CHIEF_AGENT"
	const keyPath = "CHIEF_AGENT_PATH"
	saveAgent := os.Getenv(keyAgent)
	savePath := os.Getenv(keyPath)
	defer func() {
		if saveAgent != "" {
			os.Setenv(keyAgent, saveAgent)
		} else {
			os.Unsetenv(keyAgent)
		}
		if savePath != "" {
			os.Setenv(keyPath, savePath)
		} else {
			os.Unsetenv(keyPath)
		}
	}()

	os.Unsetenv(keyAgent)
	os.Unsetenv(keyPath)

	// Env provider when no flag
	os.Setenv(keyAgent, "codex")
	got := mustResolve(t, "", "", nil)
	if got.Name() != "Codex" {
		t.Errorf("with CHIEF_AGENT=codex, name = %q, want Codex", got.Name())
	}
	os.Unsetenv(keyAgent)

	os.Setenv(keyAgent, "opencode")
	got = mustResolve(t, "", "", nil)
	if got.Name() != "OpenCode" {
		t.Errorf("with CHIEF_AGENT=opencode, name = %q, want OpenCode", got.Name())
	}
	os.Unsetenv(keyAgent)

	// Env path when no flag path
	os.Setenv(keyAgent, "codex")
	os.Setenv(keyPath, "/env/codex")
	got = mustResolve(t, "", "", nil)
	if got.CLIPath() != "/env/codex" {
		t.Errorf("with CHIEF_AGENT_PATH, CLIPath = %q, want /env/codex", got.CLIPath())
	}
	os.Unsetenv(keyPath)
	os.Unsetenv(keyAgent)
}

func TestResolve_normalize(t *testing.T) {
	got := mustResolve(t, "  CODEX  ", "", nil)
	if got.Name() != "Codex" {
		t.Errorf("Resolve('  CODEX  ') name = %q, want Codex", got.Name())
	}
	got = mustResolve(t, "  OPENCODE  ", "", nil)
	if got.Name() != "OpenCode" {
		t.Errorf("Resolve('  OPENCODE  ') name = %q, want OpenCode", got.Name())
	}
}

func TestResolve_unknownProvider(t *testing.T) {
	_, err := Resolve("typo", "", nil)
	if err == nil {
		t.Fatal("Resolve(typo) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "typo") {
		t.Errorf("error should mention the bad provider name: %v", err)
	}
	if !strings.Contains(err.Error(), "opencode") {
		t.Errorf("error should list opencode as a supported provider: %v", err)
	}
}

func TestCheckInstalled_notFound(t *testing.T) {
	// Use a path that does not exist
	p := NewCodexProvider("/nonexistent/codex-binary-that-does-not-exist")
	err := CheckInstalled(p)
	if err == nil {
		t.Error("CheckInstalled(nonexistent) expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "Codex") {
		t.Errorf("CheckInstalled error should mention Codex: %v", err)
	}
}

func TestCheckInstalled_found(t *testing.T) {
	// Go test binary is in PATH
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not in PATH, skipping CheckInstalled found test")
	}
	p := NewClaudeProvider(goPath) // abuse: use "go" as cli path to get a binary that exists
	err = CheckInstalled(p)
	if err != nil {
		t.Errorf("CheckInstalled(existing binary) err = %v", err)
	}
}

func TestResolve_configFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".chief", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const yamlContent = `
agent:
  provider: codex
  cliPath: /usr/local/bin/codex
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := mustResolve(t, "", "", cfg)
	if got.Name() != "Codex" || got.CLIPath() != "/usr/local/bin/codex" {
		t.Errorf("Resolve from config: name=%q path=%q", got.Name(), got.CLIPath())
	}
}

func TestResolve_configFileOpenCode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".chief", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const yamlContent = `
agent:
  provider: opencode
  cliPath: /usr/local/bin/opencode
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := mustResolve(t, "", "", cfg)
	if got.Name() != "OpenCode" || got.CLIPath() != "/usr/local/bin/opencode" {
		t.Errorf("Resolve from config: name=%q path=%q", got.Name(), got.CLIPath())
	}
}
