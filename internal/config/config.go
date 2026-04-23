package config

import (
	"os"
	"path/filepath"

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
	Worktree   WorktreeConfig   `yaml:"worktree"`
	OnComplete OnCompleteConfig `yaml:"onComplete"`
	Agent      AgentConfig      `yaml:"agent"`
}

// AgentConfig holds agent CLI settings (Claude, Codex, OpenCode, or Cursor).
type AgentConfig struct {
	Provider string `yaml:"provider"` // "claude" (default) | "codex" | "opencode" | "cursor"
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

// Merge combines user and project configs, with project values taking precedence.
// For every field, a non-zero value in project overwrites the value from user.
// A zero/empty value in project falls through to the user value.
func Merge(user, project *Config) *Config {
	out := *user
	if project.Theme != "" {
		out.Theme = project.Theme
	}
	if project.Worktree.Setup != "" {
		out.Worktree.Setup = project.Worktree.Setup
	}
	if project.OnComplete.Push {
		out.OnComplete.Push = project.OnComplete.Push
	}
	if project.OnComplete.CreatePR {
		out.OnComplete.CreatePR = project.OnComplete.CreatePR
	}
	if project.Agent.Provider != "" {
		out.Agent.Provider = project.Agent.Provider
	}
	if project.Agent.CLIPath != "" {
		out.Agent.CLIPath = project.Agent.CLIPath
	}
	return &out
}

// Load reads the project config from .chief/config.yaml, merges it with the
// global user config (user values are overridden by project values), and returns
// the merged result.
func Load(baseDir string) (*Config, error) {
	userCfg, err := LoadUser()
	if err != nil {
		return nil, err
	}

	path := configPath(baseDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return userCfg, nil
		}
		return nil, err
	}

	projectCfg := Default()
	if err := yaml.Unmarshal(data, projectCfg); err != nil {
		return nil, err
	}

	return Merge(userCfg, projectCfg), nil
}

// LoadUser reads the global user config from UserConfigPath().
// Returns Default() (no error) when the file does not exist.
func LoadUser() (*Config, error) {
	path := UserConfigPath()

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
