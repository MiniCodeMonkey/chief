package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// UserConfigPath returns the path to the global user config file.
// It respects XDG_CONFIG_HOME: returns $XDG_CONFIG_HOME/chief/config.yaml when set,
// otherwise falls back to ~/.chief/config.yaml.
func UserConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "chief", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return filepath.Join(home, ".chief", "config.yaml")
}

const configFile = ".chief/config.yaml"

// Config holds project-level settings for Chief.
type Config struct {
	Theme      string           `yaml:"theme"`
	Worktree   WorktreeConfig   `yaml:"worktree"`
	OnComplete OnCompleteConfig `yaml:"onComplete"`
	Agent      AgentConfig      `yaml:"agent"`
	Bash       BashConfig       `yaml:"bash"`

	promptBranchRegex *regexp.Regexp
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
	Provider string `yaml:"provider"` // "claude" (default) | "codex" | "opencode" | "cursor" | "gemini"
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
	Setup               string `yaml:"setup"`
	AlwaysPrompt        bool   `yaml:"alwaysPrompt"`
	PromptBranchPattern string `yaml:"promptBranchPattern"`
}

// OnCompleteConfig holds post-completion automation settings.
type OnCompleteConfig struct {
	Push     bool `yaml:"push"`
	CreatePR bool `yaml:"createPR"`
}

// Default returns a Config with zero-value defaults.
func Default() *Config {
	cfg := &Config{
		Worktree: WorktreeConfig{
			PromptBranchPattern: "^(main|master)$",
		},
	}
	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("config: default config failed to validate: %v", err))
	}
	return cfg
}

// Validate compiles derived config state (e.g., the prompt-branch regex
// cache) and reports configuration errors. Idempotent — safe to call
// multiple times. Callers must call Validate after mutating Config fields
// that affect derived state.
func (c *Config) Validate() error {
	return c.compilePromptRegex()
}

// ValidateBranchPattern compiles pattern as a worktree prompt-branch regex.
// An empty pattern is valid and returns (nil, nil). The returned compile
// error is bare; callers add field-name context when surfacing it.
func ValidateBranchPattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	return regexp.Compile(pattern)
}

// compilePromptRegex compiles and caches the worktree prompt-branch regex.
func (c *Config) compilePromptRegex() error {
	re, err := ValidateBranchPattern(c.Worktree.PromptBranchPattern)
	if err != nil {
		return fmt.Errorf("invalid worktree.promptBranchPattern %q: %w", c.Worktree.PromptBranchPattern, err)
	}
	c.promptBranchRegex = re
	return nil
}

// ShouldPromptForWorktree reports whether Chief should prompt the user about using a git worktree for the given branch.
func (c *Config) ShouldPromptForWorktree(branch string) bool {
	if c.Worktree.AlwaysPrompt {
		return true
	}
	if c.promptBranchRegex == nil {
		return false
	}
	return c.promptBranchRegex.MatchString(branch)
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

// LoadUser reads the global user config from UserConfigPath().
// Returns Default() (no error) when the file does not exist.
func LoadUser() (*Config, error) {
	cfg := Default()
	if err := decodeInto(UserConfigPath(), cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Load returns the effective config: the global user config with the project
// config from .chief/config.yaml layered on top. Both files decode into the
// same struct, and yaml.v3 only overwrites fields whose keys are present, so
// any key the project file sets wins (including false or ""), and every key it
// omits keeps the user value.
func Load(baseDir string) (*Config, error) {
	cfg := Default()
	if err := decodeInto(UserConfigPath(), cfg); err != nil {
		return nil, err
	}
	if err := decodeInto(configPath(baseDir), cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// decodeInto unmarshals the YAML file at path into cfg. A missing file leaves
// cfg unchanged and is not an error.
func decodeInto(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// SaveValue sets a single dotted key (for example "onComplete.push") in the
// project's .chief/config.yaml and leaves every other key in the file as it
// is. Keys the project never set stay absent, so they keep inheriting from
// the user config.
func SaveValue(baseDir, key string, value any) error {
	path := configPath(baseDir)

	document := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &document); err != nil {
			return err
		}
		if document == nil {
			document = map[string]any{}
		}
	}

	parts := strings.Split(key, ".")
	node := document
	for _, part := range parts[:len(parts)-1] {
		child, ok := node[part].(map[string]any)
		if !ok {
			child = map[string]any{}
			node[part] = child
		}
		node = child
	}
	node[parts[len(parts)-1]] = value

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// Save writes the config to .chief/config.yaml.
func Save(baseDir string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	path := configPath(baseDir)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
