package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const configFile = ".chief/config.yaml"

// Config holds project-level settings for Chief.
type Config struct {
	Worktree      WorktreeConfig   `yaml:"worktree"`
	OnComplete    OnCompleteConfig `yaml:"onComplete"`
	MaxIterations int              `yaml:"maxIterations,omitempty"`
	AutoCommit    *bool            `yaml:"autoCommit,omitempty"`
	CommitPrefix  string           `yaml:"commitPrefix,omitempty"`
	ClaudeModel   string           `yaml:"claudeModel,omitempty"`
	TestCommand   string           `yaml:"testCommand,omitempty"`
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

// DefaultMaxIterations is the default value for MaxIterations when not set.
const DefaultMaxIterations = 5

// Default returns a Config with zero-value defaults.
func Default() *Config {
	return &Config{}
}

// EffectiveMaxIterations returns MaxIterations or the default if not set.
func (c *Config) EffectiveMaxIterations() int {
	if c.MaxIterations > 0 {
		return c.MaxIterations
	}
	return DefaultMaxIterations
}

// EffectiveAutoCommit returns AutoCommit or true if not set.
func (c *Config) EffectiveAutoCommit() bool {
	if c.AutoCommit != nil {
		return *c.AutoCommit
	}
	return true
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
