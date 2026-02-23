// Package agent provides an abstraction layer for different coding agents.
// Supported agents: Claude Code (claude), Pi (pi)
package agent

import (
	"context"
	"os/exec"
)

// AgentType represents the type of coding agent.
type AgentType string

const (
	AgentClaude AgentType = "claude"
	AgentPi     AgentType = "pi"
)

// Agent defines the interface for coding agents.
type Agent interface {
	// Type returns the agent type.
	Type() AgentType

	// Command returns the CLI command name.
	Command() string

	// BuildPromptArgs returns the arguments for passing a prompt.
	BuildPromptArgs(prompt string) []string

	// BuildOutputFormatArgs returns the arguments for output format.
	BuildOutputFormatArgs(format string) []string

	// RequiredFlags returns additional required flags.
	RequiredFlags() []string
}

// Claude represents Claude Code agent.
type Claude struct{}

// Type returns the agent type.
func (c *Claude) Type() AgentType { return AgentClaude }

// Command returns the CLI command name.
func (c *Claude) Command() string { return "claude" }

// BuildPromptArgs returns the arguments for passing a prompt.
func (c *Claude) BuildPromptArgs(prompt string) []string {
	return []string{"-p", prompt}
}

// BuildOutputFormatArgs returns the arguments for output format.
func (c *Claude) BuildOutputFormatArgs(format string) []string {
	return []string{"--output-format", format}
}

// RequiredFlags returns additional required flags.
func (c *Claude) RequiredFlags() []string {
	return []string{"--dangerously-skip-permissions", "--verbose"}
}

// Pi represents the Pi coding agent.
type Pi struct{}

// Type returns the agent type.
func (p *Pi) Type() AgentType { return AgentPi }

// Command returns the CLI command name.
func (p *Pi) Command() string { return "pi" }

// BuildPromptArgs returns the arguments for passing a prompt.
func (p *Pi) BuildPromptArgs(prompt string) []string {
	return []string{"-p", prompt}
}

// BuildOutputFormatArgs returns the arguments for output format.
// Pi uses JSON output mode differently - it supports --json for JSON output.
func (p *Pi) BuildOutputFormatArgs(format string) []string {
	// Pi uses --json for JSON output, but for stream-json compatibility
	// we'll use the default output and parse accordingly
	return []string{}
}

// RequiredFlags returns additional required flags.
func (p *Pi) RequiredFlags() []string {
	return []string{}
}

// NewAgent creates a new agent based on the type.
func NewAgent(agentType AgentType) Agent {
	switch agentType {
	case AgentPi:
		return &Pi{}
	case AgentClaude:
		fallthrough
	default:
		return &Claude{}
	}
}

// BuildCommand builds the command for the given agent.
func BuildCommand(ctx context.Context, agentType AgentType, prompt, workDir string) *exec.Cmd {
	agent := NewAgent(agentType)

	args := []string{}
	args = append(args, agent.RequiredFlags()...)
	args = append(args, agent.BuildPromptArgs(prompt)...)
	args = append(args, agent.BuildOutputFormatArgs("stream-json")...)

	cmd := exec.CommandContext(ctx, agent.Command(), args...)
	cmd.Dir = workDir

	return cmd
}
