package agent

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/loop"
)

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Resolve returns the agent Provider using priority: flagAgent > CHIEF_AGENT env > config > "claude".
// flagPath overrides the CLI path when non-empty (flag > CHIEF_AGENT_PATH > config agent.cliPath).
// Returns an error if the resolved provider name is not recognised.
func Resolve(flagAgent, flagPath string, cfg *config.Config) (loop.Provider, error) {
	providerName := "claude"
	if flagAgent != "" {
		providerName = strings.ToLower(strings.TrimSpace(flagAgent))
	} else if v := os.Getenv("CHIEF_AGENT"); v != "" {
		providerName = strings.ToLower(strings.TrimSpace(v))
	} else if cfg != nil && cfg.Agent.Provider != "" {
		providerName = strings.ToLower(strings.TrimSpace(cfg.Agent.Provider))
	}

	cliPath := resolveCLIPath(providerName, flagPath, cfg)

	switch providerName {
	case "claude":
		return NewClaudeProvider(cliPath), nil
	case "codex":
		return NewCodexProvider(cliPath), nil
	case "opencode":
		if err := validateOpenCodeConfig(cfg); err != nil {
			return nil, err
		}
		return NewOpenCodeProvider(cliPath, resolveOpenCodeModel(cfg)), nil
	default:
		return nil, fmt.Errorf("unknown agent provider %q: expected \"claude\", \"codex\", or \"opencode\"", providerName)
	}
}

func resolveCLIPath(providerName, flagPath string, cfg *config.Config) string {
	if flagPath != "" {
		return flagPath
	}
	if v := strings.TrimSpace(os.Getenv("CHIEF_AGENT_PATH")); v != "" {
		return v
	}
	if cfg == nil {
		return ""
	}
	if providerName == "opencode" {
		if v := strings.TrimSpace(cfg.Agent.OpenCode.CLIPath); v != "" {
			return v
		}
	}
	return strings.TrimSpace(cfg.Agent.CLIPath)
}

func resolveOpenCodeModel(cfg *config.Config) string {
	if v := strings.TrimSpace(os.Getenv("CHIEF_OPENCODE_MODEL")); v != "" {
		return v
	}
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Agent.OpenCode.Model)
}

func validateOpenCodeConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}

	requiredEnv := cfg.Agent.OpenCode.RequiredEnv
	if len(requiredEnv) == 0 {
		return nil
	}

	var missing []string
	for _, raw := range requiredEnv {
		name := strings.TrimSpace(raw)
		if name == "" {
			return fmt.Errorf("invalid agent.opencode.requiredEnv: entries must be non-empty environment variable names")
		}
		if !envVarNamePattern.MatchString(name) {
			return fmt.Errorf("invalid agent.opencode.requiredEnv entry %q: expected [A-Za-z_][A-Za-z0-9_]*", raw)
		}
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	slices.Sort(missing)
	return fmt.Errorf("missing required opencode environment variables: %s. Set them in your shell or adjust agent.opencode.requiredEnv in .chief/config.yaml", strings.Join(missing, ", "))
}

// CheckInstalled verifies that the provider's CLI binary is found in PATH (or at cliPath).
func CheckInstalled(p loop.Provider) error {
	_, err := exec.LookPath(p.CLIPath())
	if err != nil {
		return fmt.Errorf("%s CLI not found in PATH. Install it or set agent.cliPath in .chief/config.yaml", p.Name())
	}
	return nil
}
