# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # Build binary to ./bin/melliza
make test           # Run all tests (verbose)
make test-short     # Run tests without verbose output
make lint           # Run golangci-lint
make fmt            # Format code
make vet            # Run go vet
make tidy           # go mod tidy + verify
make run            # Build and launch the TUI
```

Run a single test package:
```bash
go test -v ./internal/prd/...
go test -v -run TestLoop_WatchdogKillsHungProcess ./internal/loop/...
```

Build with version info:
```bash
go build -ldflags "-X main.Version=v1.0.0" -o ./bin/melliza ./cmd/melliza
```

## Architecture

Melliza is an autonomous agent loop that orchestrates the **Gemini CLI** to implement user stories from a PRD. The language is Go 1.24+.

### Core Data Flow

1. User creates a `prd.md` via `melliza new`
2. Gemini converts `prd.md` → `prd.json` (machine-readable, via `embed/convert_prompt.txt`)
3. The **Loop** reads `prd.json`, picks the next incomplete story, invokes Gemini with `stream-json` output, and parses events in real time
4. Gemini implements the story, commits via conventional commits, and updates `passes: true` in `prd.json`
5. Loop repeats until all stories pass or max iterations reached

### Package Map

| Package | Role |
|---|---|
| `cmd/melliza/` | Entry point. Parses CLI args, bootstraps TUI. All subcommands (`new`, `edit`, `status`, `list`, `update`) dispatch to `internal/cmd/`. |
| `internal/cmd/` | Subcommand implementations (`RunNew`, `RunEdit`, `RunStatus`, `RunList`, `RunUpdate`). |
| `internal/loop/` | Core agent logic. `Loop` runs Gemini subprocess, streams events. `Manager` runs multiple PRDs in parallel. `Parser` decodes Gemini's `stream-json` output into typed `Event`s. |
| `internal/prd/` | Domain types (`PRD`, `UserStory`), load/save/watch `prd.json`, parse `progress.md`, convert `prd.md` → `prd.json`. |
| `internal/tui/` | Bubble Tea TUI. `App` is the root model. Views: Dashboard, Log, Diff, Picker, Settings, Worktree spinner, etc. |
| `internal/git/` | Git utilities: branches, worktrees, PR creation, merge, push, `gh` CLI integration. |
| `internal/gemini/` | Builds args for headless Gemini invocations (used for conversion/init flows). |
| `internal/config/` | Loads/saves `.melliza/config.yaml` (worktree setup, auto-push, auto-PR). |
| `embed/` | Embedded prompt templates (`prompt.txt`, `init_prompt.txt`, `edit_prompt.txt`, `convert_prompt.txt`). Compiled into the binary. |

### Persistent State

All state lives in `.melliza/` relative to the project root:
- `.melliza/prds/<name>/prd.json` — machine-readable PRD with story progress (`passes`, `inProgress`)
- `.melliza/prds/<name>/prd.md` — human-readable PRD source
- `.melliza/prds/<name>/progress.md` — free-form progress log written by Gemini
- `.melliza/config.yaml` — project-level config (worktree mode, on-complete hooks)
- `.melliza/worktrees/<name>/` — git worktrees for each PRD (when enabled)

### Gemini Invocation

The main agent loop invokes:
```
gemini --dangerously-skip-permissions --output-format stream-json --verbose
```

Headless (non-interactive) calls for conversion/init use `--output-format json`.

Authentication: `GEMINI_API_KEY` or Vertex AI env vars (`GOOGLE_GENAI_USE_VERTEXAI`, `GOOGLE_CLOUD_PROJECT`, `GOOGLE_VERTEX_PROJECT`).

### TUI Architecture

The TUI is a single Bubble Tea `App` model (`internal/tui/app.go`) with tab-based views. The `loop.Manager` manages concurrent `Loop` instances; events are forwarded as Bubble Tea messages (`LoopEventMsg`, `LoopFinishedMsg`, `PRDCompletedMsg`). A `prd.Watcher` (fsnotify) fires `PRDUpdateMsg` when `prd.json` changes on disk.

### Key Conventions

- **Agent behavior** is defined exclusively in `embed/prompt.txt`. Do not hardcode instructions outside embedded prompt files.
- **Priority ordering**: lower `Priority` value = higher priority (worked on first). In-progress stories always take precedence.
- **Story completion**: Gemini sets `"passes": true` in `prd.json`. The loop detects this via file watching or parsing output.
- **TUI golden tests**: `internal/tui/testdata/` holds golden output files for E2E tests. Update them with `-update` flag when intentionally changing TUI output.
- **Worktrees**: When enabled, each PRD gets its own git worktree under `.melliza/worktrees/`. The loop runs Gemini in that directory.
