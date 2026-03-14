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
  provider: claude   # or "codex", "opencode", or "cursor"
  cliPath: ""        # optional path to CLI binary
worktree:
  setup: "npm install"
onComplete:
  push: true
  createPR: true
```

### Config Keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `agent.provider` | string | `"claude"` | Agent CLI to use: `claude`, `codex`, `opencode`, or `cursor` |
| `agent.cliPath` | string | `""` | Optional path to the agent binary (e.g. `/usr/local/bin/opencode`). If empty, Chief uses the provider name from PATH. |
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
- **Auto-detect** (Recommended) — The agent analyzes your project and suggests appropriate setup commands
- **Enter manually** — Type a custom command
- **Skip** — Leave it empty

These settings are saved to `.chief/config.yaml` and can be changed at any time via the Settings TUI (`,`).

## CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--agent <provider>` | Agent CLI to use: `claude`, `codex`, `opencode`, or `cursor` | From config / env / `claude` |
| `--agent-path <path>` | Custom path to the agent CLI binary | From config / env |
| `--max-iterations <n>`, `-n` | Loop iteration limit | Dynamic |
| `--no-retry` | Disable auto-retry on agent crashes | `false` |
| `--verbose` | Show raw agent output in log | `false` |
| `--merge` | Auto-merge progress on conversion conflicts | `false` |
| `--force` | Auto-overwrite on conversion conflicts | `false` |

Agent resolution order: `--agent` / `--agent-path` → `CHIEF_AGENT` / `CHIEF_AGENT_PATH` env vars → `agent.provider` / `agent.cliPath` in `.chief/config.yaml` → default `claude`.

When `--max-iterations` is not specified, Chief calculates a dynamic limit based on the number of remaining stories plus a buffer. You can also adjust the limit at runtime with `+`/`-` in the TUI.

## Agent

Chief can use **Claude Code** (default), **Codex CLI**, **OpenCode CLI**, or **Cursor CLI** as the agent. Choose via:

- **Config:** `agent.provider: opencode` and optionally `agent.cliPath: /path/to/opencode` in `.chief/config.yaml`
- **Environment:** `CHIEF_AGENT=opencode`, `CHIEF_AGENT_PATH=/path/to/opencode`
- **CLI:** `chief --agent opencode --agent-path /path/to/opencode`

## Agent-Specific Configuration

Each agent has its own configuration. For example, when using Claude Code:

```bash
# Authentication
claude login

# Model selection (if you have access)
claude config set model claude-3-opus-20240229
```

See [Claude Code documentation](https://github.com/anthropics/claude-code) for details.

When using Cursor CLI:

```bash
# Authentication (or set CURSOR_API_KEY for headless)
agent login
```

Chief runs Cursor in headless mode with `--trust` and `--force` so it can modify files without prompts. See [Cursor CLI documentation](https://cursor.com/docs/cli/overview) for details.

## Permission Handling

Some agents (like Claude Code) ask for permission before executing bash commands, writing files, and making network requests. Chief automatically configures the agent for autonomous operation by disabling these prompts.

::: warning
Chief runs the agent with full permissions to modify your codebase. Only run Chief on PRDs you trust.

For additional isolation, consider using [Claude Code's sandbox mode](https://docs.anthropic.com/en/docs/claude-code/sandboxing) or running Chief in a Docker container.
:::
