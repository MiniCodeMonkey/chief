
# FAQ

Frequently asked questions about Melliza.

## General

### What is Melliza?

Melliza is an autonomous PRD agent. You write a Product Requirements Document with user stories, run Melliza, and watch as Gemini builds your code—story by story.

### Why "Melliza"?

Named after Melliza Wiggum from The Simpsons (Ralph Wiggum's dad). Melliza orchestrates the [Ralph loop](https://ghuntley.com/ralph/).

### Is Melliza free?

Melliza itself is open source and free. However, it uses Gemini CLI, which requires a Gemini Pro subscription or Anthropic API access.

### What models does Melliza use?

Melliza uses whatever model is configured in Gemini CLI.

## Usage

### Can I run Melliza on a remote server?

Yes! Melliza works great on remote servers. SSH in, run `melliza`, press `s` to start the loop, and let it work. Use `screen` or `tmux` if you want to disconnect.

```bash
ssh my-server
tmux new -s melliza
melliza
# Press 's' to start the loop
# Ctrl+B D to detach
```

### How do I resume after stopping?

Run `melliza` again and press `s` to start. It reads state from `prd.json` and continues where it left off.

### Can I edit the PRD while Melliza is running?

Yes, but be careful. Melliza re-reads `prd.json` between iterations. Edits to the current story might cause confusion.

Best practice: pause Melliza with `p` (or stop with `x`), edit, then press `s` to resume.

### Can I have multiple PRDs?

Yes. Create separate directories under `.melliza/prds/`:

```
.melliza/prds/
├── feature-a/
└── feature-b/
```

Run with `melliza feature-a` or use the TUI: press `n` to open the PRD picker, or `1-9` to quickly switch between tabs. Multiple PRDs can run in parallel.

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

### What are worktrees?

Git worktrees let you have multiple checkouts of a repository at the same time, each on a different branch. Melliza uses worktrees to isolate parallel PRDs so they don't interfere with each other's files or commits. Each worktree lives at `.melliza/worktrees/<prd-name>/`.

### Do I have to use worktrees?

No. When you start a PRD, Melliza offers worktree creation as an option. You can choose "Run in current directory" to skip it. Worktrees are most useful when running multiple PRDs simultaneously.

### How do I merge a completed branch?

Press `n` to open the PRD picker, select the completed PRD, and press `m` to merge. If there are conflicts, Melliza shows the conflicting files and instructions for manual resolution.

### How do I clean up a worktree?

Press `n` to open the PRD picker, select the PRD, and press `c`. You can choose to remove just the worktree or remove the worktree and delete the branch.

### What happens if Melliza crashes mid-worktree?

Melliza detects orphaned worktrees on startup and marks them in the picker. You can clean them up with `c`. Your work on the branch is preserved — git worktrees are just directories with a separate checkout.

### Can I automatically push and create PRs?

Yes. During first-time setup, Melliza asks if you want to enable auto-push and auto-PR creation. You can also toggle these in the Settings TUI (`,`). Auto-PR requires the `gh` CLI to be installed and authenticated.

## Technical

### Why stream-json?

Gemini CLI outputs JSON in a streaming format. Melliza uses stream-json to parse this in real-time, allowing it to:
- Display progress as it happens
- React to completion signals immediately
- Handle large outputs efficiently

### Why conventional commits?

Conventional commits (`feat:`, `fix:`, etc.) provide:
- Clear history of what each story added
- Easy to review changes per-story
- Works with changelog generators

### What if Gemini makes a mistake?

Git is your safety net. Each story is committed separately, so you can:

```bash
# See what changed
git log --oneline

# Revert a story
git revert HEAD

# Or reset and re-run
git reset --hard HEAD~1
melliza  # then press 's' to start
```

### Does Melliza work with any language?

Yes. Melliza doesn't know or care what language you're using. It passes your PRD to Gemini, which handles the implementation.

### How does Melliza handle tests?

Melliza instructs Gemini to run quality checks (tests, lint, typecheck) before committing. Gemini infers the appropriate commands from your codebase (e.g., `npm test`, `pytest`).

## Troubleshooting

### See [Common Issues](common-issues.md)

For specific problems and solutions.

## Getting Help

### Where can I report bugs?

[GitHub Issues](https://github.com/lvcoi/melliza/issues)

### Is there a community chat?

Use [GitHub Discussions](https://github.com/lvcoi/melliza/discussions) for questions and community support.

### Can I contribute?

Yes! See [CONTRIBUTING.md](https://github.com/lvcoi/melliza/blob/main/CONTRIBUTING.md) in the repository.
