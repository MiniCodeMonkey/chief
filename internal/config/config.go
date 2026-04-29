package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const configFile = ".chief/config.yaml"

// Config holds project-level settings for Chief.
type Config struct {
	Worktree   WorktreeConfig   `yaml:"worktree"`
	OnComplete OnCompleteConfig `yaml:"onComplete"`
	Agent      AgentConfig      `yaml:"agent"`
	Bash       BashConfig       `yaml:"bash"`
}

// BashConfig holds settings for external bash commands invoked by Chief
// (currently the worktree setup command).
type BashConfig struct {
	// Timeout is a Go duration string (e.g. "30s", "5m"). Empty disables
	// the timeout (no upper bound on bash command runtime). Unparseable or
	// negative values are also treated as "no timeout" and surface a
	// warning via Config.BashTimeoutWarning.
	Timeout string `yaml:"timeout"`
}

// BashTimeout returns the configured bash command timeout as a time.Duration.
// A return value of 0 means "no timeout" — callers (e.g. runSetupCommand) skip
// wrapping the command in a deadline context. Empty values, unparseable
// strings, and negative durations all return 0; BashTimeoutWarning describes
// the fallback for unparseable/negative inputs so a typo does not silently
// disable a configured limit.
//
// Nil-safe: returns 0 when c is nil.
func (c *Config) BashTimeout() time.Duration {
	if c == nil {
		return 0
	}
	// Default 0 = "no timeout": setup commands are unbounded unless the
	// user opts in by configuring an explicit duration.
	return parseDurationOrDefault(c.Bash.Timeout, 0)
}

// BashTimeoutWarning returns a human-readable warning when the configured
// bash.timeout value is non-empty but unparseable or negative. Returns "" when
// the value is empty, valid, or when c is nil.
func (c *Config) BashTimeoutWarning() string {
	if c == nil {
		return ""
	}
	v := strings.TrimSpace(c.Bash.Timeout)
	if v == "" {
		return ""
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Sprintf("bash.timeout %q is not a valid duration; ignoring (no timeout)", v)
	}
	if d < 0 {
		return fmt.Sprintf("bash.timeout %q is negative; ignoring (no timeout)", v)
	}
	return ""
}

// parseDurationOrDefault parses value as a Go duration. Empty input,
// unparseable input, and negative durations all return def. Surrounding
// whitespace is ignored. An explicit "0s" returns 0 — callers interpret 0
// according to their own semantics (e.g. "no timeout" / "watchdog disabled").
func parseDurationOrDefault(value string, def time.Duration) time.Duration {
	v := strings.TrimSpace(value)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return def
	}
	return d
}

// AgentConfig holds agent CLI settings (Claude, Codex, OpenCode, or Cursor).
type AgentConfig struct {
	Provider string `yaml:"provider"` // "claude" (default) | "codex" | "opencode" | "cursor"
	CLIPath  string `yaml:"cliPath"`  // optional custom path to CLI binary
	// WatchdogTimeout bounds how long Chief will wait without any output
	// from the agent before killing the process as hung. Go duration string
	// (e.g. "5m", "30m"). Empty / unparseable values use
	// DefaultAgentWatchdogTimeout. "0s" disables the watchdog.
	//
	// This is the right knob to bump when the agent runs long, quiet
	// commands as part of acceptance criteria (e.g. integration test
	// suites that produce no stdout for several minutes).
	WatchdogTimeout string `yaml:"watchdogTimeout"`
}

// DefaultAgentWatchdogTimeout is applied when agent.watchdogTimeout is unset
// or unparseable. Kept in sync with loop.DefaultWatchdogTimeout — that one is
// what NewLoop initialises a fresh Loop with when no config is passed; this
// one is the value AgentWatchdogTimeout returns when the manager *does* have
// a config but the user did not configure the field. If you change one,
// change the other.
const DefaultAgentWatchdogTimeout = 5 * time.Minute

// AgentWatchdogTimeout returns the configured agent watchdog timeout.
// Empty, unparseable, and negative values all return DefaultAgentWatchdogTimeout
// so behaviour matches a fresh Loop initialised without config. An explicit
// "0s" returns 0, which loop.SetWatchdogTimeout interprets as "watchdog
// disabled".
//
// Nil-safe: returns DefaultAgentWatchdogTimeout when c is nil.
func (c *Config) AgentWatchdogTimeout() time.Duration {
	if c == nil {
		return DefaultAgentWatchdogTimeout
	}
	// Default DefaultAgentWatchdogTimeout (5m) preserves the historical
	// hardcoded watchdog behaviour for users who don't configure the field.
	return parseDurationOrDefault(c.Agent.WatchdogTimeout, DefaultAgentWatchdogTimeout)
}

// WorktreeConfig holds worktree-related settings.
type WorktreeConfig struct {
	Setup string `yaml:"setup"`
}

// OnCompleteConfig holds post-completion automation settings.
type OnCompleteConfig struct {
	Push     bool `yaml:"push"`
	CreatePR bool `yaml:"createPR"`
}

// Default returns a Config with zero-value defaults.
func Default() *Config {
	return &Config{}
}

// configPath returns the full path to the config file.
func configPath(baseDir string) string {
	return filepath.Join(baseDir, configFile)
}

// Exists checks if the config file exists.
func Exists(baseDir string) bool {
	_, err := os.Stat(configPath(baseDir))
	return err == nil
}

// Load reads the config from .chief/config.yaml.
// Returns Default() when the file doesn't exist (no error).
func Load(baseDir string) (*Config, error) {
	path := configPath(baseDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the config to .chief/config.yaml.
func Save(baseDir string, cfg *Config) error {
	path := configPath(baseDir)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
