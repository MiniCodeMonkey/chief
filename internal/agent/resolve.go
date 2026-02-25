package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/loop"
)

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

	cliPath := ""
	if flagPath != "" {
		cliPath = flagPath
	} else if v := os.Getenv("CHIEF_AGENT_PATH"); v != "" {
		cliPath = strings.TrimSpace(v)
	} else if cfg != nil && cfg.Agent.CLIPath != "" {
		cliPath = strings.TrimSpace(cfg.Agent.CLIPath)
	}

	switch providerName {
	case "claude":
		return NewClaudeProvider(cliPath), nil
	case "codex":
		return NewCodexProvider(cliPath), nil
	default:
		return nil, fmt.Errorf("unknown agent provider %q: expected \"claude\" or \"codex\"", providerName)
	}
}

// CheckInstalled verifies that the provider's CLI binary is found in PATH (or at cliPath).
func CheckInstalled(p loop.Provider) error {
	_, err := exec.LookPath(p.CLIPath())
	if err != nil {
		return fmt.Errorf("%s CLI not found in PATH. Install it or set agent.cliPath in .chief/config.yaml", p.Name())
	}
	return nil
}
