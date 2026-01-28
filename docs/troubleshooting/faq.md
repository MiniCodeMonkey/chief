---
description: Frequently asked questions about Chief, the autonomous PRD agent. Answers about usage, requirements, and how it works.
---

# FAQ

Frequently asked questions about Chief.

## General

### What is Chief?

Chief is an autonomous PRD agent. You write a Product Requirements Document with user stories, run Chief, and watch as Claude builds your code—story by story.

### Why "Chief"?

Named after Chief Wiggum from The Simpsons (Ralph Wiggum's dad). Chief orchestrates the [Ralph loop](https://ghuntley.com/ralph/).

### Is Chief free?

Chief itself is open source and free. However, it uses Claude Code, which requires a Claude Pro subscription or Anthropic API access.

### What models does Chief use?

Chief uses whatever model is configured in Claude Code.

## Usage

### Can I run Chief on a remote server?

Yes! Chief works great on remote servers. SSH in, run `chief`, press `s` to start the loop, and let it work. Use `screen` or `tmux` if you want to disconnect.

```bash
ssh my-server
tmux new -s chief
chief
# Press 's' to start the loop
# Ctrl+B D to detach
```

### How do I resume after stopping?

Run `chief` again and press `s` to start. It reads state from `prd.json` and continues where it left off.

### Can I edit the PRD while Chief is running?

Yes, but be careful. Chief re-reads `prd.json` between iterations. Edits to the current story might cause confusion.

Best practice: pause Chief with `p` (or stop with `x`), edit, then press `s` to resume.

### Can I have multiple PRDs?

Yes. Create separate directories under `.chief/prds/`:

```
.chief/prds/
├── feature-a/
└── feature-b/
```

Run with `chief feature-a` or use the TUI: press `n` to open the PRD picker, or `1-9` to quickly switch between tabs. Multiple PRDs can run in parallel.

### How do I skip a story?

Mark it as passed manually:

```json
{
  "id": "US-003",
  "passes": true,
  "inProgress": false
}
```

Or remove it from the PRD entirely.

## Technical

### Why stream-json?

Claude Code outputs JSON in a streaming format. Chief uses stream-json to parse this in real-time, allowing it to:
- Display progress as it happens
- React to completion signals immediately
- Handle large outputs efficiently

### Why conventional commits?

Conventional commits (`feat:`, `fix:`, etc.) provide:
- Clear history of what each story added
- Easy to review changes per-story
- Works with changelog generators

### What if Claude makes a mistake?

Git is your safety net. Each story is committed separately, so you can:

```bash
# See what changed
git log --oneline

# Revert a story
git revert HEAD

# Or reset and re-run
git reset --hard HEAD~1
chief  # then press 's' to start
```

### Does Chief work with any language?

Yes. Chief doesn't know or care what language you're using. It passes your PRD to Claude, which handles the implementation.

### How does Chief handle tests?

Chief instructs Claude to run quality checks (tests, lint, typecheck) before committing. Claude infers the appropriate commands from your codebase (e.g., `npm test`, `pytest`).

## Troubleshooting

### See [Common Issues](/troubleshooting/common-issues)

For specific problems and solutions.

## Getting Help

### Where can I report bugs?

[GitHub Issues](https://github.com/minicodemonkey/chief/issues)

### Is there a community chat?

Coming soon. For now, use GitHub Discussions.

### Can I contribute?

Yes! See [CONTRIBUTING.md](https://github.com/minicodemonkey/chief/blob/main/CONTRIBUTING.md) in the repository.
