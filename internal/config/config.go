package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const configFile = ".chief/config.yaml"

// Config holds project-level settings for Chief.
type Config struct {
	Worktree   WorktreeConfig   `yaml:"worktree"`
	OnComplete OnCompleteConfig `yaml:"onComplete"`
	Agent      AgentConfig      `yaml:"agent"`
	Uplink     UplinkConfig     `yaml:"uplink"`

	// Remote-configurable fields (used by serve settings handler).
	MaxIterations int    `yaml:"maxIterations,omitempty"`
	AutoCommit    *bool  `yaml:"autoCommit,omitempty"`
	CommitPrefix  string `yaml:"commitPrefix,omitempty"`
	ClaudeModel   string `yaml:"claudeModel,omitempty"`
	TestCommand   string `yaml:"testCommand,omitempty"`
}

// EffectiveMaxIterations returns the configured max iterations or a default of 5.
func (c *Config) EffectiveMaxIterations() int {
	if c.MaxIterations > 0 {
		return c.MaxIterations
	}
	return 5
}

// EffectiveAutoCommit returns the auto-commit setting, defaulting to true.
func (c *Config) EffectiveAutoCommit() bool {
	if c.AutoCommit != nil {
		return *c.AutoCommit
	}
	return true
}

// DefaultServerURL is the default uplink server URL.
const DefaultServerURL = "https://uplink.chiefloop.com"

// UplinkConfig holds uplink connection settings.
type UplinkConfig struct {
	Enabled   bool   `yaml:"enabled"`
	ServerURL string `yaml:"serverURL,omitempty"`
}

// EffectiveServerURL returns the server URL, falling back to DefaultServerURL.
func (u UplinkConfig) EffectiveServerURL() string {
	if u.ServerURL != "" {
		return u.ServerURL
	}
	return DefaultServerURL
}

// AgentConfig holds agent CLI settings (Claude, Codex, or OpenCode).
type AgentConfig struct {
	Provider string `yaml:"provider"` // "claude" (default) | "codex" | "opencode"
	CLIPath  string `yaml:"cliPath"`  // optional custom path to CLI binary
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
