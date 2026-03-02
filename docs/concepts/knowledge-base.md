---
description: How Chief uses knowledge.json for cross-iteration learning, pattern accumulation, failure tracking, and smarter decision-making.
---

# Knowledge Base

Chief's knowledge base (`knowledge.json`) is a structured JSON file that enables cross-iteration learning. Each Claude session starts with no memory of previous sessions — `knowledge.json` bridges this gap by persisting patterns, implementation records, and failure history across iterations.

## Why a Knowledge Base?

The Ralph Loop runs Claude in a loop where each iteration is a fresh session. Without persistent knowledge, each iteration would:

- Repeat mistakes from previous attempts
- Miss conventions established in earlier stories
- Not know what files were changed or why
- Have no context about failed approaches

`knowledge.json` solves this by giving each iteration a structured summary of everything that happened before.

## Schema

```json
{
  "patterns": [
    "Use Zod for input validation",
    "Middleware follows req.user convention",
    "Tests use Vitest with .test.ts extension"
  ],
  "completedStories": {
    "US-001": {
      "filesChanged": [
        "src/routes/register.ts",
        "src/middleware/validate.ts",
        "tests/register.test.ts"
      ],
      "approach": "Added registration route following existing health route pattern",
      "learnings": [
        "JWT secret is in JWT_SECRET env var",
        "Prisma client is imported from src/lib/prisma.ts"
      ],
      "criteriaResults": [
        {
          "criterion": "Registration form with email and password fields",
          "passed": true,
          "evidence": "Form component renders at /register with both fields"
        },
        {
          "criterion": "Email format validation",
          "passed": true,
          "evidence": "Zod schema rejects invalid emails, tested in register.test.ts"
        }
      ],
      "attempts": []
    }
  }
}
```

### Top-Level Fields

| Field | Type | Description |
|-------|------|-------------|
| `patterns` | `string[]` | Reusable conventions discovered across iterations |
| `completedStories` | `object` | Map of story ID to implementation record |

### CompletedStoryRecord

Each entry in `completedStories` has:

| Field | Type | Description |
|-------|------|-------------|
| `filesChanged` | `string[]` | Files created or modified during implementation |
| `approach` | `string` | Brief description of the implementation strategy |
| `learnings` | `string[]` | Insights discovered during implementation |
| `criteriaResults` | `CriteriaResult[]` | Per-criterion verification results (optional) |
| `attempts` | `Attempt[]` | Previous failed attempts (optional) |

### CriteriaResult

Each acceptance criterion is verified individually:

| Field | Type | Description |
|-------|------|-------------|
| `criterion` | `string` | The acceptance criterion text |
| `passed` | `boolean` | Whether this criterion was satisfied |
| `evidence` | `string` | How it was verified or why it failed |

### Attempt

Failed attempts are preserved so the next iteration can learn from them:

| Field | Type | Description |
|-------|------|-------------|
| `approach` | `string` | What was tried |
| `criteriaResults` | `CriteriaResult[]` | Which criteria passed and which failed |
| `failureAnalysis` | `string` | Analysis of why this approach failed |

## How the Agent Uses Knowledge

### At the Start of Each Iteration

1. **Read patterns** — Claude learns the project's conventions (naming, file locations, testing patterns)
2. **Read completed stories** — Claude understands what was already built, which files were changed, and how
3. **Check for failed attempts** — If the current story has previous failures, Claude reads the failure analysis and chooses a different approach

### After Completing a Story

1. **Write approach** — The implementation strategy is recorded before coding begins
2. **Write criteria results** — Each acceptance criterion's pass/fail status and evidence
3. **Write learnings** — Any insights discovered during implementation
4. **Update patterns** — If a reusable convention was discovered, it's added to the top-level patterns array

### On Failure

1. **Record criteria results** — Which criteria passed and which failed
2. **Write failure analysis** — What went wrong and why
3. **Move to attempts** — The current record is moved to the `attempts` array
4. **End iteration** — The story is NOT marked as passed; the next iteration will retry

## Smart Retry

The knowledge base enables intelligent retries. When a story fails:

1. The failed approach, criteria results, and failure analysis are stored in the `attempts` array
2. The next iteration reads these attempts and is explicitly instructed: "Do NOT retry the same approach"
3. After 3 failed attempts (configurable via `MaxAttempts`), the story is marked as exhausted and skipped

This prevents infinite retry loops where the agent keeps trying the same failing approach.

## Pattern Accumulation

Patterns are project-wide conventions that apply across all stories. Examples:

```json
{
  "patterns": [
    "Use sql<number> template for database aggregations",
    "Always use IF NOT EXISTS for migrations",
    "Export types from actions.ts for UI components",
    "Tests follow arrange-act-assert pattern"
  ]
}
```

Patterns are accumulated across iterations — each story may add new patterns but should not remove existing ones. This creates a growing understanding of the codebase that makes later iterations more effective.

## Relationship to progress.md

Both files track similar information but serve different purposes:

| | `knowledge.json` | `progress.md` |
|---|---|---|
| **Format** | Structured JSON | Free-text markdown |
| **Audience** | Claude (machine-readable) | Developers (human-readable) |
| **Purpose** | Cross-iteration learning | Audit trail and review |
| **Updated by** | Claude (read and write) | Claude (append-only) |

`knowledge.json` is the primary knowledge store. `progress.md` is the human-readable log. Both are maintained by Claude during each iteration.

## TUI Integration

The TUI reads `knowledge.json` to display rich information:

- **Per-criterion verification**: Each acceptance criterion shows a status icon (green checkmark for passed, red X for failed, gray dot for not yet verified)
- **Failed criterion evidence**: When a criterion fails, the reason is shown below it
- **Attempt count**: Stories with previous failures show "Attempt 2/3" next to their status
- **Exhausted stories**: Stories that hit the 3-attempt limit show "Exhausted (3/3 attempts)"

## What's Next

- [The Ralph Loop](/concepts/ralph-loop) — How the execution loop uses the knowledge base
- [PRD Format](/concepts/prd-format) — Writing effective user stories and dependencies
- [The .chief Directory](/concepts/chief-directory) — Where knowledge.json fits in the directory structure
