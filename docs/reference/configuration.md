---
description: Chief configuration reference. Project config file, CLI flags, Settings TUI, and first-time setup flow.
---

# Configuration

Chief uses a project-level configuration file at `.chief/config.yaml` for persistent settings, plus CLI flags for per-run options.

## Config File (`.chief/config.yaml`)

Chief stores project-level settings in `.chief/config.yaml`. This file is created automatically during first-time setup or when you change settings via the Settings TUI.

### Format

```yaml
worktree:
  setup: "npm install"
onComplete:
  push: true
  createPR: true
```

### Config Keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
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
- **Let Claude figure it out** (Recommended) — Claude analyzes your project and suggests appropriate setup commands
- **Enter manually** — Type a custom command
- **Skip** — Leave it empty

These settings are saved to `.chief/config.yaml` and can be changed at any time via the Settings TUI (`,`).

## CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--max-iterations <n>`, `-n` | Loop iteration limit | `10` |
| `--no-sound` | Disable completion sound | `false` |
| `--verbose` | Show raw Claude output in log | `false` |
| `--merge` | Auto-merge progress on conversion conflicts | `false` |
| `--force` | Auto-overwrite on conversion conflicts | `false` |

## Claude Code Configuration

Chief invokes Claude Code under the hood. Claude Code has its own configuration:

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
