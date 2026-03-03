---
description: Melliza configuration reference. Project config file, CLI flags, Settings TUI, and first-time setup flow.
---

# Configuration

Melliza uses a project-level configuration file at `.melliza/config.yaml` for persistent settings, plus CLI flags for per-run options.

## Config File (`.melliza/config.yaml`)

Melliza stores project-level settings in `.melliza/config.yaml`. This file is created automatically during first-time setup or when you change settings via the Settings TUI.

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

Changes are saved immediately to `.melliza/config.yaml` on every edit.

When toggling "Create pull request" to Yes, Melliza validates that the `gh` CLI is installed and authenticated. If validation fails, the toggle reverts and an error message is shown with installation instructions.

Navigate with `j`/`k` or arrow keys. Press `Enter` to toggle booleans or edit strings. Press `Esc` to close.

## First-Time Setup

When you launch Melliza for the first time in a project, you'll be prompted to configure:

1. **Post-completion settings** — Whether to automatically push branches and create PRs when a PRD completes
2. **Worktree setup command** — A shell command to run in new worktrees (e.g., installing dependencies)

For the setup command, you can:
- **Let Gemini figure it out** (Recommended) — Gemini analyzes your project and suggests appropriate setup commands
- **Enter manually** — Type a custom command
- **Skip** — Leave it empty

These settings are saved to `.melliza/config.yaml` and can be changed at any time via the Settings TUI (`,`).

## CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--max-iterations <n>`, `-n` | Loop iteration limit | Dynamic |
| `--no-retry` | Disable auto-retry on Gemini crashes | `false` |
| `--verbose` | Show raw Gemini output in log | `false` |
| `--merge` | Auto-merge progress on conversion conflicts | `false` |
| `--force` | Auto-overwrite on conversion conflicts | `false` |

When `--max-iterations` is not specified, Melliza calculates a dynamic limit based on the number of remaining stories plus a buffer. You can also adjust the limit at runtime with `+`/`-` in the TUI.

## Gemini CLI Configuration

Melliza invokes Gemini CLI under the hood. Gemini CLI requires an API key for authentication:

```bash
# Set your API key
export GEMINI_API_KEY="your-api-key-here"
```

You can also specify a specific model using the `--model` flag in your environment or via configuration.

See [Gemini CLI documentation](https://github.com/google/gemini-cli) for details.

## Permission Handling

By default, Gemini CLI asks for permission before executing bash commands, writing files, and making network requests. Melliza automatically disables these prompts when invoking Gemini to enable autonomous operation.

::: warning
Melliza runs Gemini with full permissions to modify your codebase. Only run Melliza on PRDs you trust.

For additional isolation, consider using [Gemini CLI's sandbox mode](https://docs.anthropic.com/en/docs/gemini-code/sandboxing) or running Melliza in a Docker container.
:::
