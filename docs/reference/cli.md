# CLI Reference

Chief provides a minimal but powerful CLI. All commands operate on the current working directory's `.chief/` folder.

## Usage

```
chief [command] [flags]
```

**Available Commands:**

| Command | Description |
|---------|-------------|
| *(default)* | Run the Ralph Loop on the active PRD |
| `init` | Create a new PRD in the current project |
| `edit` | Open the PRD for editing |
| `status` | Show current PRD progress |
| `list` | List all PRDs in the project |

## Commands

### chief (default)

Run the Ralph Loop on the active PRD. This is the main command — it reads your PRD, selects the next story, invokes Claude Code, and iterates until all stories pass.

```bash
chief
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--prd <name>` | Select which PRD to run | Auto-detect |
| `--max-iterations <n>` | Maximum loop iterations | `100` |
| `--dangerously-skip-permissions` | Skip Claude Code permission prompts | `false` |
| `--no-sound` | Disable completion sound | `false` |

**Examples:**

```bash
# Run with auto-detected PRD
chief

# Run a specific PRD by name
chief --prd auth-system

# Increase iteration limit for large PRDs
chief --max-iterations 200

# Run without permission prompts (for CI/automation)
chief --dangerously-skip-permissions

# Combine flags
chief --prd auth-system --max-iterations 50 --no-sound
```

::: tip
If your project has only one PRD, Chief auto-detects it. Use `--prd` when you have multiple PRDs.
:::

---

### chief init

Create a new PRD in the current project. This command walks you through an interactive setup, then scaffolds the `.chief/prds/<name>/` directory with template files.

```bash
chief init
```

**Interactive prompts:**

1. **PRD name** — a short identifier (e.g., `auth-system`, `landing-page`)
2. **Project description** — what the PRD is about
3. **First user story** — your initial story to get started

**What it creates:**

```
.chief/
└── prds/
    └── <name>/
        ├── prd.md       # Markdown PRD for context
        └── prd.json     # Structured stories for Chief
```

**Examples:**

```bash
# Initialize a new PRD (interactive)
chief init

# Then follow the prompts:
#   PRD name: auth-system
#   Description: User authentication with JWT tokens
#   First story: As a user, I want to log in with email and password
```

::: info
Run `chief init` from the root of your project. Chief creates the `.chief/` directory if it doesn't exist.
:::

---

### chief edit

Open the PRD files for editing in your default editor.

```bash
chief edit
```

Opens both `prd.md` and `prd.json` in the editor specified by your `$EDITOR` environment variable.

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--prd <name>` | Edit a specific PRD | Auto-detect |

**Examples:**

```bash
# Edit the auto-detected PRD
chief edit

# Edit a specific PRD
chief edit --prd auth-system
```

::: tip
Set your preferred editor with `export EDITOR=vim` (or `code`, `nano`, etc.) in your shell profile.
:::

---

### chief status

Show progress for the current PRD. Displays a summary of story completion at a glance.

```bash
chief status
```

**Output includes:**

- Current PRD name
- Total number of stories
- Completed / In Progress / Pending counts
- Next story to be worked on

**Examples:**

```bash
# Check progress on the auto-detected PRD
chief status

# Example output:
#   PRD: auth-system
#   Stories: 8 total
#     ✓ 5 completed
#     → 1 in progress
#     ○ 2 pending
#   Next: US-006 - Password Reset Flow
```

---

### chief list

List all PRDs in the current project.

```bash
chief list
```

Scans `.chief/prds/` and shows each PRD with its completion status.

**Examples:**

```bash
# List all PRDs
chief list

# Example output:
#   auth-system    5/8 stories complete
#   landing-page   12/12 stories complete ✓
#   api-v2         0/6 stories complete
```

---

## Keyboard Shortcuts (TUI)

When Chief is running, the TUI (Terminal UI) provides real-time feedback and interactive controls:

| Key | Action |
|-----|--------|
| `Tab` | Switch between panels |
| `↑` / `↓` | Scroll log output |
| `q` | Quit (gracefully stops Claude) |
| `Ctrl+C` | Force quit |
| `?` | Show help overlay |

::: tip
The TUI shows two panels: a **status panel** with story progress and a **log panel** streaming Claude's output in real time.
:::

## Exit Codes

Chief uses exit codes to indicate how the process ended:

| Code | Meaning |
|------|---------|
| `0` | Success — all stories complete |
| `1` | Error — something went wrong |
| `2` | Interrupted — user quit via `q` or `Ctrl+C` |
| `3` | Max iterations reached without completing all stories |

These are useful for scripting and CI integration:

```bash
chief --dangerously-skip-permissions
if [ $? -eq 3 ]; then
  echo "Hit iteration limit — consider increasing --max-iterations"
fi
```

## Environment Variables

Chief respects the following environment variables as defaults:

| Variable | Description | Equivalent Flag |
|----------|-------------|-----------------|
| `CHIEF_PRD` | Default PRD to use | `--prd` |
| `CHIEF_MAX_ITERATIONS` | Default iteration limit | `--max-iterations` |
| `EDITOR` | Editor for `chief edit` | — |

Environment variables are overridden by command-line flags when both are provided.

```bash
# Set defaults in your shell profile
export CHIEF_PRD=auth-system
export CHIEF_MAX_ITERATIONS=200

# This now uses auth-system with 200 max iterations
chief

# Flags override environment variables
chief --prd landing-page  # uses landing-page, not auth-system
```
