# Melliza - Project Overview & Context

Melliza is an autonomous agent loop that orchestrates the **Gemini Code CLI** to work through user stories in a **Product Requirements Document (PRD)**. It follows the "Ralph Wiggum loop" pattern: each iteration starts with a fresh context window for Gemini, while progress is persisted in local files (`prd.json` and `progress.md`).

## Core Architecture

- **Language:** Go (1.24+)
- **TUI Framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Engine:** [Gemini Code CLI](https://docs.anthropic.com/en/docs/gemini-code) (invoked as a subprocess)
- **Persistence:** Local JSON and Markdown files in `.melliza/prds/`

## Key Workflows

1.  **PRD Creation (`melliza new`):** Launches an interactive Gemini session to collaborate with the user on a `prd.md` file.
2.  **Auto-Conversion:** Melliza automatically converts human-readable `prd.md` into machine-readable `prd.json`.
3.  **Agent Loop:**
    - Melliza reads `prd.json` to identify the next incomplete story.
    - It invokes Gemini with a specialized system prompt and the current story context.
    - Gemini implements the story, runs tests, commits changes, and updates `prd.json`.
    - Melliza monitors `prd.json` and `stream-json` output to update the TUI in real-time.
4.  **Branch/Worktree Management:** Melliza can optionally create git branches or worktrees for each PRD to keep the main workspace clean.

## Directory Structure

- `cmd/melliza/`: CLI entry point and subcommand handlers.
- `internal/loop/`: The core loop logic and `stream-json` parser for Gemini's output.
- `internal/prd/`: PRD domain models, conversion logic, and file watchers.
- `internal/tui/`: Implementation of the multi-view TUI (Dashboard, Log, Diff, Picker, etc.).
- `internal/git/`: Git utility functions for branches, commits, and worktrees.
- `embed/`: Embedded assets including the agent system prompt (`prompt.txt`) and skills.

## Building and Running

| Command | Description |
| :--- | :--- |
| `make build` | Builds the `melliza` binary to `./bin/melliza` |
| `make install` | Installs the binary to `$GOPATH/bin` |
| `make test` | Runs all tests |
| `make lint` | Runs `golangci-lint` |
| `make run` | Builds and launches the TUI |

## Development Conventions

- **State Management:** All persistent state is stored in `prd.json`. The TUI model is ephemeral and re-reads state on startup or file change.
- **Git:** Gemini is instructed to use conventional commits. Melliza manages high-level git operations like worktree creation.
- **Testing:** New features should include unit tests in the corresponding `internal/` package. TUI components can be tested using `teatest`.
- **Prompts:** Core agent behavior is defined in `embed/prompt.txt`. Avoid hardcoding agent instructions outside of the embedded prompt files.

## Technical Notes

- **Gemini Invocation:** Melliza uses `gemini --dangerously-skip-permissions --output-format stream-json --verbose`.
- **Concurrency:** Melliza supports multiple PRDs running in parallel through the `loop.Manager`.
- **Dependencies:** Uses `fsnotify` for file watching and `yaml.v3` for configuration.
