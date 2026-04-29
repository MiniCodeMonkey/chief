package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

const configFile = ".chief/config.yaml"

// Config holds project-level settings for Chief.
type Config struct {
	Worktree   WorktreeConfig   `yaml:"worktree"`
	OnComplete OnCompleteConfig `yaml:"onComplete"`
	Agent      AgentConfig      `yaml:"agent"`

	promptBranchRegex *regexp.Regexp
}

// AgentConfig holds agent CLI settings (Claude, Codex, OpenCode, or Cursor).
type AgentConfig struct {
	Provider string `yaml:"provider"` // "claude" (default) | "codex" | "opencode" | "cursor"
	CLIPath  string `yaml:"cliPath"`  // optional custom path to CLI binary
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
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
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
