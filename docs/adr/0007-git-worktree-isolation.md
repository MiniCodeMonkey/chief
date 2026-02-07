# ADR-0007: Git Worktree Isolation for Parallel PRDs

## Status

Accepted

## Context

Chief supports running multiple PRDs in parallel via the Loop Manager. However, all PRDs share the same working directory and git state. This causes several problems when parallel Claude instances:

1. **File conflicts**: Two instances editing the same file simultaneously produce corrupted or overwritten content
2. **Interleaved commits**: Commits from different PRDs are mixed in the same branch's history
3. **Branch conflicts**: All instances work on the same branch, making it impossible to review or merge PRD work independently

We considered three approaches:

1. **Locking**: Serialize access to files and git operations. Simple but eliminates parallelism.
2. **Separate clones**: Clone the repo for each PRD. Provides isolation but wastes disk space and has slow setup.
3. **Git worktrees**: Create lightweight worktrees for each PRD. Full isolation with minimal overhead.

## Decision

Use git worktrees to isolate parallel PRD execution. Each PRD gets:
- A dedicated branch (`chief/<prd-name>`)
- A worktree at `.chief/worktrees/<prd-name>/`
- An optional setup command to install dependencies in the worktree

Additionally, add post-completion automation:
- Automatic branch push to remote
- Automatic PR creation via `gh` CLI
- A Settings TUI (`,`) for managing these options

## Implementation

### Worktree Lifecycle

1. **Creation**: When a user starts a PRD, the TUI offers to create a worktree. Chief creates the branch from the default branch and sets up the worktree via `git worktree add`.
2. **Usage**: The Loop runs Claude Code with the worktree as the working directory. All file operations and commits happen in isolation.
3. **Completion**: When the PRD finishes, auto-push and auto-PR actions run if configured.
4. **Cleanup**: Users can merge the branch (`m`) and clean the worktree (`c`) from the picker.

### Reuse and Stale Detection

`CreateWorktree` handles edge cases:
- If a worktree already exists on the expected branch, it is reused
- If a worktree exists but is stale (wrong branch or invalid), it is removed and recreated
- Orphaned worktrees from crashed sessions are detected on startup

### Configuration

Settings are stored in `.chief/config.yaml`:

```yaml
worktree:
  setup: "npm install"   # Command to run in new worktrees
onComplete:
  push: true             # Auto-push branch on completion
  createPR: true         # Auto-create PR on completion
```

### UI Integration

- Tab bar shows branch names: `auth [chief/auth] > 3/8`
- Dashboard header shows working directory and branch
- Picker shows branch and worktree path per PRD
- Completion screen shows auto-action progress and results
- Settings overlay (`,`) for live config editing

## Rationale

1. **Git worktrees are lightweight**: They share the object store with the main repo. Creating a worktree is nearly instant compared to cloning.

2. **Full isolation**: Each worktree has its own working tree, index, and HEAD. Parallel Claude instances cannot interfere with each other.

3. **Clean git history**: Each PRD's commits live on a separate branch, making code review and merging straightforward.

4. **User control**: Worktrees are optional. Users who run one PRD at a time can skip them entirely.

5. **Self-cleaning**: Orphaned worktrees are detected on startup. Users manage cleanup explicitly via the picker to avoid accidental data loss.

## Consequences

### Positive

- Multiple PRDs can run truly in parallel without conflicts
- Each PRD has a clean, reviewable branch
- Post-completion automation enables "start and walk away" workflows
- Backward compatible — single-PRD usage works unchanged

### Negative

- Disk usage increases (each worktree is a full checkout minus the object store)
- Setup commands (e.g., `npm install`) add time to worktree creation
- Users must understand basic git branch concepts (merge, conflicts)
- `gh` CLI is a new optional dependency for PR creation

## References

- [git-worktree documentation](https://git-scm.com/docs/git-worktree)
- `internal/git/worktree.go` — Worktree CRUD operations
- `internal/git/push.go` — Push and PR primitives
- `internal/config/config.go` — Configuration system
- `internal/tui/settings.go` — Settings TUI overlay
