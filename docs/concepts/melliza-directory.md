
# The .melliza Directory

Melliza stores all of its state in a single `.melliza/` directory at the root of your project. This is a deliberate design choice — there are no global config files, no hidden state in your home directory, no external databases. Everything Melliza needs lives right alongside your code.

## Directory Structure

A typical `.melliza/` directory looks like this:

```
your-project/
├── src/
├── package.json
└── .melliza/
    ├── config.yaml             # Project settings (worktree, auto-push, PR)
    ├── prds/
    │   └── my-feature/
    │       ├── prd.md          # Human-readable PRD (you write this)
    │       ├── prd.json        # Machine-readable PRD (Melliza reads/writes)
    │       ├── progress.md     # Progress log (Melliza appends after each story)
    │       └── gemini.log      # Raw Gemini output (for debugging)
    └── worktrees/              # Isolated checkouts for parallel PRDs
        └── my-feature/         # Git worktree (full project checkout)
```

The root `.melliza/` directory contains:
- `config.yaml` — Project-level settings (see [Configuration](../reference/configuration.md))
- `prds/` — One subdirectory per PRD with requirements, state, and logs
- `worktrees/` — Git worktrees for parallel PRD isolation (created on demand)

## The `prds/` Subdirectory

Every PRD lives in its own named folder under `.melliza/prds/`. The folder name is what you pass to Melliza when running a specific PRD:

```bash
melliza my-feature
```

Melliza uses this folder as the working context for the entire run. All reads and writes happen within this folder — the PRD state, progress log, and Gemini output are all scoped to the specific PRD being executed.

## File Explanations

### `prd.md`

The human-readable product requirements document. You write this file (or generate it with `melliza new`). It contains context, background, technical notes, and anything else that helps Gemini understand what to build.

This file is included in the prompt sent to Gemini at the start of each iteration. Write it as if you're briefing a senior developer who's new to the project — the more context you provide, the better the output.

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

The structured, machine-readable PRD. This is where user stories, their priorities, and their completion status live. Melliza reads this file at the start of each iteration to determine which story to work on, and writes to it after completing a story.

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
| `userStories[].inProgress` | boolean | Whether Melliza is currently working on this story |

Melliza selects the next story by finding the highest-priority story (lowest `priority` number) where `passes` is `false`. See the [PRD Format](prd-format.md) reference for full details.

### `progress.md`

An append-only log of completed work. After each story, Melliza adds an entry documenting what was implemented, which files changed, and lessons learned. This file serves two purposes:

1. **Context for future iterations** — Melliza reads this at the start of each run to understand what has already been built and avoid repeating mistakes
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
