# Changelog

All notable changes to Chief are documented in this file.

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

[0.5.0]: https://github.com/MiniCodeMonkey/chief/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/MiniCodeMonkey/chief/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/MiniCodeMonkey/chief/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/MiniCodeMonkey/chief/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/MiniCodeMonkey/chief/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/MiniCodeMonkey/chief/releases/tag/v0.1.0
