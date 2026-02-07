---
description: Understand the .chief directory structure where Chief stores all state. Self-contained, portable, and git-friendly.
---

# The .chief Directory

Chief stores all of its state in a single `.chief/` directory at the root of your project. This is a deliberate design choice — there are no global config files, no hidden state in your home directory, no external databases. Everything Chief needs lives right alongside your code.

## Directory Structure

A typical `.chief/` directory looks like this:

```
your-project/
├── src/
├── package.json
└── .chief/
    ├── config.yaml             # Project settings (worktree, auto-push, PR)
    ├── prds/
    │   └── my-feature/
    │       ├── prd.md          # Human-readable PRD (you write this)
    │       ├── prd.json        # Machine-readable PRD (Chief reads/writes)
    │       ├── progress.md     # Progress log (Chief appends after each story)
    │       └── claude.log      # Raw Claude output (for debugging)
    └── worktrees/              # Isolated checkouts for parallel PRDs
        └── my-feature/         # Git worktree (full project checkout)
```

The root `.chief/` directory contains:
- `config.yaml` — Project-level settings (see [Configuration](/reference/configuration))
- `prds/` — One subdirectory per PRD with requirements, state, and logs
- `worktrees/` — Git worktrees for parallel PRD isolation (created on demand)

## The `prds/` Subdirectory

Every PRD lives in its own named folder under `.chief/prds/`. The folder name is what you pass to Chief when running a specific PRD:

```bash
chief my-feature
```

Chief uses this folder as the working context for the entire run. All reads and writes happen within this folder — the PRD state, progress log, and Claude output are all scoped to the specific PRD being executed.

## File Explanations

### `prd.md`

The human-readable product requirements document. You write this file (or generate it with `chief new`). It contains context, background, technical notes, and anything else that helps Claude understand what to build.

This file is included in the prompt sent to Claude at the start of each iteration. Write it as if you're briefing a senior developer who's new to the project — the more context you provide, the better the output.

```markdown
# My Feature

## Background
We need to add user authentication to our API...

## Technical Notes
- We use Express.js with TypeScript
- Database is PostgreSQL with Prisma ORM
- Follow existing middleware patterns in `src/middleware/`
```

### `prd.json`

The structured, machine-readable PRD. This is where user stories, their priorities, and their completion status live. Chief reads this file at the start of each iteration to determine which story to work on, and writes to it after completing a story.

Key fields:

| Field | Type | Description |
|-------|------|-------------|
| `project` | string | Project name |
| `description` | string | Brief project description |
| `userStories` | array | List of user stories |
| `userStories[].id` | string | Story identifier (e.g., `US-001`) |
| `userStories[].title` | string | Short story title |
| `userStories[].description` | string | User story in "As a... I want... so that..." format |
| `userStories[].acceptanceCriteria` | array | List of criteria that must be met |
| `userStories[].priority` | number | Execution order (lower = higher priority) |
| `userStories[].passes` | boolean | Whether the story is complete |
| `userStories[].inProgress` | boolean | Whether Chief is currently working on this story |

Chief selects the next story by finding the highest-priority story (lowest `priority` number) where `passes` is `false`. See the [PRD Format](/concepts/prd-format) reference for full details.

### `progress.md`

An append-only log of completed work. After each story, Chief adds an entry documenting what was implemented, which files changed, and lessons learned. This file serves two purposes:

1. **Context for future iterations** — Chief reads this at the start of each run to understand what has already been built and avoid repeating mistakes
2. **Audit trail** — You can review exactly what happened during each iteration

A typical entry looks like:

```markdown
## 2024-01-15 - US-003
- What was implemented: User authentication middleware
- Files changed:
  - src/middleware/auth.ts - new JWT verification middleware
  - src/routes/login.ts - login endpoint
  - tests/auth.test.ts - authentication tests
- **Learnings for future iterations:**
  - Middleware pattern uses `req.user` for authenticated user data
  - JWT secret is in environment variable `JWT_SECRET`
---
```

The `Codebase Patterns` section at the top of this file consolidates reusable patterns discovered across iterations — things like naming conventions, file locations, and architectural decisions that future iterations should follow.

### `claude.log`

Raw output from Claude Code during execution. This file captures everything Claude outputs, including tool calls, reasoning, and results. It's primarily useful for debugging when something goes wrong.

This file can get large (multiple megabytes per run) and is regenerated on each execution. You typically don't need to read it unless you're investigating an issue.

## The `worktrees/` Subdirectory

When you run multiple PRDs in parallel, each PRD can get its own isolated git worktree under `.chief/worktrees/`. A worktree is a full checkout of your project on a separate branch, so parallel Claude instances never conflict over files or git state.

```
.chief/worktrees/
├── auth-system/         # Full checkout on branch chief/auth-system
└── payment-integration/ # Full checkout on branch chief/payment-integration
```

Worktrees are created when you choose "Create worktree + branch" from the start dialog. Each worktree:
- Has its own branch (named `chief/<prd-name>`)
- Is a complete copy of your project
- Runs the configured setup command (e.g., `npm install`) automatically

You can merge completed branches via `m` in the picker, and clean up worktrees via `c`.

For more details, see [ADR-0007: Git Worktree Isolation](/adr/0007-git-worktree-isolation).

## The `config.yaml` File

Project-level settings are stored in `.chief/config.yaml`. This file is created during first-time setup or when you change settings via the Settings TUI (`,`).

```yaml
worktree:
  setup: "npm install"
onComplete:
  push: true
  createPR: true
```

See [Configuration](/reference/configuration) for all available settings.

## Self-Contained by Design

Chief has no global configuration. There is no `~/.chiefrc`, no `~/.config/chief/`, no environment variables required. Every piece of state Chief needs is inside `.chief/`.

This means:

- **No setup beyond installation** — Install the binary, run `chief new`, and you're ready
- **No conflicts between projects** — Each project has its own isolated state
- **No "works on my machine" issues** — The state is the same for everyone who clones the repo
- **No cleanup needed** — Delete `.chief/` and it's as if Chief was never there

## Portability

Because everything is self-contained, your project is fully portable:

```bash
# Move your project anywhere — Chief picks up right where it left off
mv my-project /new/location/
cd /new/location/my-project
chief  # Continues from the last completed story
```

```bash
# Clone on a different machine — same state, same progress
git clone git@github.com:you/my-project.git
cd my-project
chief  # Sees the same PRD state as the original machine
```

This also works for remote servers. SSH into a machine, clone your repo, and run Chief — no additional setup required.

## Multiple PRDs in One Project

A single project can have multiple PRDs, each tracking a separate feature or initiative:

```
.chief/
├── config.yaml
├── prds/
│   ├── auth-system/
│   │   ├── prd.md
│   │   ├── prd.json
│   │   └── progress.md
│   ├── payment-integration/
│   │   ├── prd.md
│   │   ├── prd.json
│   │   └── progress.md
│   └── admin-dashboard/
│       ├── prd.md
│       ├── prd.json
│       └── progress.md
└── worktrees/
    ├── auth-system/
    └── payment-integration/
```

Run a specific PRD by name:

```bash
chief auth-system
chief payment-integration
```

Each PRD tracks its own stories, progress, and logs independently. When running multiple PRDs in parallel, each gets its own git worktree and branch for full isolation. You can run them simultaneously without worrying about file conflicts or interleaved commits.

## Git Considerations

You have two options depending on whether you want to share Chief state with your team.

### Option 1: Keep It Private

If Chief is just for your personal workflow, ignore the entire directory:

```gitignore
# In your repo's .gitignore
.chief/
```

Or add it to your global gitignore to keep it private across all projects without modifying each repo:

```bash
# Check if you have a global gitignore configured
git config --global core.excludesFile

# If not set, create one
git config --global core.excludesFile ~/.gitignore

# Then add .chief/ to that file
echo ".chief/" >> "$(git config --global core.excludesFile)"
```

### Option 2: Share With Your Team

If you want collaborators to see progress and continue where you left off, commit everything except the log files:

```gitignore
# In your repo's .gitignore
.chief/prds/*/claude.log
```

This shares:
- `prd.md`: Your requirements, the source of truth for what to build
- `prd.json`: Story state and progress, so collaborators see what's done
- `progress.md`: Implementation history and learnings, valuable project context

The `claude.log` files are large, regenerated each run, and only useful for debugging.

## What's Next

- [PRD Format](/concepts/prd-format) — Learn how to write effective PRDs
- [The Ralph Loop](/concepts/ralph-loop) — Understand what happens during execution
- [CLI Reference](/reference/cli) — See all available commands
