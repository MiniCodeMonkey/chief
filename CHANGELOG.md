# Changelog

All notable changes to Chief are documented in this file.

## [0.6.1] - 2026-02-24

### Bug Fixes
- Agent prompt now uses explicit `{{PROGRESS_PATH}}` template variable so progress.md is written next to prd.json instead of the working directory

## [0.6.0] - 2026-02-21

### Features
- Continuous, responsive confetti animation on the completion screen
- Quit confirmation modal when Ralph loop is running
- Live progress from progress.md in dashboard details panel
- Per-story timing and total duration on the completion screen

### Bug Fixes
- Match story commits by ID + title to prevent false positives
- Show uncommitted WIP changes when story has no commit yet
- Load claude.md on each iteration instead of only at startup
- Update elapsed time display every second while running
- Dynamically recalculate max iterations when switching PRDs

## [0.5.4] - 2026-02-20

### Bug Fixes
- TUI now manages story in-progress state directly on `EventStoryStarted`, fixing a race where the status was never shown
- TUI auto-selects the active story so its details are visible immediately
- Clear in-progress flags on completion, error, or max iterations
- Prevent non-JSON output from PRD conversion by disabling tools

## [0.5.3] - 2026-02-20

### Performance
- Cache pre-rendered log lines to eliminate per-frame TUI rebuilds

### Documentation
- Update documentation for v0.3.1–v0.5.2 release changes

## [0.5.2] - 2026-02-20

### Bug Fixes
- Log raw output when PRD JSON conversion fails, making it easier to diagnose parsing errors

## [0.5.1] - 2026-02-19

### Features
- Diff view now shows the commit for the selected user story instead of the entire branch diff
- Add `PgUp`/`PgDn` key bindings for page scrolling in log and diff views
- Diff header shows which story's commit is being viewed

### Bug Fixes
- Fix stale `GetConvertPrompt` test after inline content refactor
- Diff view now uses the correct worktree directory for PRDs with worktrees

## [0.5.0] - 2026-02-19

### Features
- Add version check and self-update command (`chief update`)
- Add diff view for viewing task changes
- Add `e` keybinding to edit current PRD directly
- Add live progress display during PRD-to-JSON conversion
- Add first-time setup post-completion config (auto-push, create PR)
- Add git worktree support for isolated PRD branches
- Add config system for per-project settings
- Improve PRD conversion UX with styled progress panel

### Bug Fixes
- Fix Rosetta 2 deadlock on Apple Silicon caused by oto/v2 audio library (#13)
- Fix missing `--verbose` flag for stream-json output

### Breaking Changes
- Remove `--no-sound` flag (sound feature removed entirely)

### Performance
- Inline prompt for PRD conversion instead of agentic tool use

## [0.4.0] - 2026-02-06

### Features
- Add `l` keybinding to open PRD picker in selection mode

### Bug Fixes
- Prevent Claude from implementing PRD after creation
- Let Claude write prd.json directly with better error handling

## [0.3.1] - 2026-02-04

### Bug Fixes
- Fix TUI becoming unresponsive after ralph loop completes

## [0.3.0] - 2026-01-31

### Features
- Add syntax highlighting for code snippets in log view
- Add editable branch name in branch warning dialog
- Add first-time setup flow with gitignore prompt

### Bug Fixes
- Launch Claude from project root for full codebase context

## [0.2.0] - 2026-01-29

### Features
- Add max iterations control with `+`/`-` keys
- Enhanced log viewer with tool call icons and full-width results
- Add branch protection warning when starting on main/master
- Add crash recovery with automatic retry

### Bug Fixes
- Remove duplicate "Converting prd.md to prd.json..." message

## [0.1.0] - 2026-01-28

Initial release.

### Features
- Core agent loop with Claude Code integration
- TUI dashboard with Bubble Tea
- PRD file watching and auto-conversion
- Parallel PRD execution
- Log viewer with tool cards
- PRD picker with tab bar
- Help overlay
- Narrow terminal support
- CLI commands: `chief new`, `chief edit`, `chief status`, `chief list`
- Homebrew formula and install script

[0.5.3]: https://github.com/MiniCodeMonkey/chief/compare/v0.5.2...v0.5.3
[0.5.2]: https://github.com/MiniCodeMonkey/chief/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/MiniCodeMonkey/chief/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/MiniCodeMonkey/chief/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/MiniCodeMonkey/chief/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/MiniCodeMonkey/chief/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/MiniCodeMonkey/chief/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/MiniCodeMonkey/chief/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/MiniCodeMonkey/chief/releases/tag/v0.1.0
