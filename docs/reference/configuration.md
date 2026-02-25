---
description: Chief configuration reference. Project config file, CLI flags, Settings TUI, and first-time setup flow.
---

# Configuration

Chief uses a project-level configuration file at `.chief/config.yaml` for persistent settings, plus CLI flags for per-run options.

## Config File (`.chief/config.yaml`)

Chief stores project-level settings in `.chief/config.yaml`. This file is created automatically during first-time setup or when you change settings via the Settings TUI.

### Format

```yaml
agent:
  provider: claude   # or "codex" / "opencode"
  cliPath: ""        # optional path to CLI binary
  opencode:
    cliPath: ""      # optional OpenCode-specific binary path
    model: ""        # optional OpenCode model (provider/model)
    requiredEnv: []  # optional env vars that must be set when provider=opencode
worktree:
  setup: "npm install"
onComplete:
  push: true
  createPR: true
```

### Config Keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `agent.provider` | string | `"claude"` | Agent CLI to use: `claude`, `codex`, or `opencode` |
| `agent.cliPath` | string | `""` | Optional path to the agent binary for all providers. If empty, Chief uses the provider name from PATH. |
| `agent.opencode.cliPath` | string | `""` | Optional OpenCode-specific binary path. Used only when `agent.provider` resolves to `opencode`. |
| `agent.opencode.model` | string | `""` | Optional OpenCode model override in `provider/model` format (for example `openai/gpt-5`). |
| `agent.opencode.requiredEnv` | string[] | `[]` | Optional list of environment variable names that must be set before running OpenCode (for example API/auth variables). Invalid names are rejected at startup. |
| `worktree.setup` | string | `""` | Shell command to run in new worktrees (e.g., `npm install`, `go mod download`) |
| `onComplete.push` | bool | `false` | Automatically push the branch to remote when a PRD completes |
| `onComplete.createPR` | bool | `false` | Automatically create a pull request when a PRD completes (requires `gh` CLI) |

### Example Configurations

**Minimal (defaults):**

```yaml
worktree:
  setup: ""
onComplete:
  push: false
  createPR: false
```

**Full automation:**

```yaml
worktree:
  setup: "npm install && npm run build"
onComplete:
  push: true
  createPR: true
```

**OpenCode with required env checks:**

```yaml
agent:
  provider: opencode
  opencode:
    cliPath: /usr/local/bin/opencode
    model: openai/gpt-5
    requiredEnv:
      - OPENAI_API_KEY
      - OPENCODE_PROFILE
```

## Settings TUI

Press `,` from any view in the TUI to open the Settings overlay. This provides an interactive way to view and edit all config values.

Settings are organized by section:

- **Worktree** — Setup command (string, editable inline)
- **On Complete** — Push to remote (toggle), Create pull request (toggle)

Changes are saved immediately to `.chief/config.yaml` on every edit.

When toggling "Create pull request" to Yes, Chief validates that the `gh` CLI is installed and authenticated. If validation fails, the toggle reverts and an error message is shown with installation instructions.

Navigate with `j`/`k` or arrow keys. Press `Enter` to toggle booleans or edit strings. Press `Esc` to close.

## First-Time Setup

When you launch Chief for the first time in a project, you'll be prompted to configure:

1. **Post-completion settings** — Whether to automatically push branches and create PRs when a PRD completes
2. **Worktree setup command** — A shell command to run in new worktrees (e.g., installing dependencies)

For the setup command, you can:
- **Let Claude figure it out** (Recommended) — Claude analyzes your project and suggests appropriate setup commands
- **Enter manually** — Type a custom command
- **Skip** — Leave it empty

These settings are saved to `.chief/config.yaml` and can be changed at any time via the Settings TUI (`,`).

## CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--agent <provider>` | Agent CLI to use: `claude`, `codex`, or `opencode` | From config / env / `claude` |
| `--agent-path <path>` | Custom path to the agent CLI binary | From config / env |
| `--max-iterations <n>`, `-n` | Loop iteration limit | Dynamic |
| `--no-retry` | Disable auto-retry on agent crashes | `false` |
| `--verbose` | Show raw agent output in log | `false` |
| `--merge` | Auto-merge progress on conversion conflicts | `false` |
| `--force` | Auto-overwrite on conversion conflicts | `false` |

Agent resolution order: `--agent` / `--agent-path` → `CHIEF_AGENT` / `CHIEF_AGENT_PATH` env vars → `agent.provider` / provider-specific config (`agent.opencode.cliPath`) / `agent.cliPath` in `.chief/config.yaml` → default `claude`.
OpenCode model resolution order: `CHIEF_OPENCODE_MODEL` env var → `agent.opencode.model` in `.chief/config.yaml` → OpenCode CLI default model.

When `--max-iterations` is not specified, Chief calculates a dynamic limit based on the number of remaining stories plus a buffer. You can also adjust the limit at runtime with `+`/`-` in the TUI.

## Agent

Chief can use **Claude Code** (default), **Codex CLI**, or **OpenCode CLI** as the agent. Choose via:

- **Config:** `agent.provider: codex|opencode` and optionally `agent.cliPath: /path/to/binary` (or `agent.opencode.cliPath` for OpenCode) in `.chief/config.yaml`
- **Environment:** `CHIEF_AGENT=codex|opencode`, `CHIEF_AGENT_PATH=/path/to/binary`, optional `CHIEF_OPENCODE_MODEL=provider/model`
- **CLI:** `chief --agent codex|opencode --agent-path /path/to/binary`

### OpenCode Model Selection

Chief supports OpenCode model selection through config or environment variable.

Priority order:

1. `CHIEF_OPENCODE_MODEL`
2. `agent.opencode.model` in `.chief/config.yaml`
3. OpenCode CLI default model

Example using environment variable:

```bash
export CHIEF_OPENCODE_MODEL=openai/gpt-5
chief --agent opencode
```

Example using config:

```yaml
agent:
  provider: opencode
  opencode:
    model: openai/gpt-5
```

When set, Chief passes `--model <value>` to OpenCode for interactive PRD generation/editing, loop execution, and PRD conversion/fix steps.

When `agent.opencode.requiredEnv` is configured, Chief validates those env vars before execution starts. Missing vars produce an actionable startup error with the missing names.

For release validation of OpenCode behavior, follow the [OpenCode Reliability Checklist](/reference/opencode-reliability-checklist).

## Claude Code Configuration

When using Claude, Chief invokes Claude Code under the hood. Claude Code has its own configuration:

```bash
# Authentication
claude login

# Model selection (if you have access)
claude config set model claude-3-opus-20240229
```

See [Claude Code documentation](https://github.com/anthropics/claude-code) for details.

## Permission Handling

By default, Claude Code asks for permission before executing bash commands, writing files, and making network requests. Chief automatically disables these prompts when invoking Claude to enable autonomous operation.

::: warning
Chief runs Claude with full permissions to modify your codebase. Only run Chief on PRDs you trust.

For additional isolation, consider using [Claude Code's sandbox mode](https://docs.anthropic.com/en/docs/claude-code/sandboxing) or running Chief in a Docker container.
:::
