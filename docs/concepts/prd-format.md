---
description: Complete guide to Chief's PRD format including prd.md and prd.json structure, user story fields, selection logic, and best practices.
---

# PRD Format

Chief uses a structured PRD format with two files: a human-readable markdown file (`prd.md`) and a machine-readable JSON file (`prd.json`). Together, they give Chief everything it needs to autonomously build your feature.

## File Structure

Each PRD lives in its own subdirectory inside `.chief/prds/`:

```
.chief/prds/my-feature/
├── prd.md        # Human-readable context for Claude
├── prd.json      # Structured data Chief reads and updates
├── progress.md   # Auto-generated progress log
└── claude.log    # Raw Claude output from each iteration
```

- **`prd.md`** — Written by you. Provides context, background, and guidance.
- **`prd.json`** — The source of truth. Chief reads, updates, and drives execution from this file.
- **`progress.md`** — Written by Claude. Tracks what was done, what changed, and what was learned.
- **`claude.log`** — Written by Chief. Raw output from Claude for debugging.

## prd.md — The Human-Readable File

The markdown file is your chance to give Claude context that doesn't fit into structured fields. Write whatever helps Claude understand the project — there's no required format.

### What to Include

- **Overview** — What are you building and why?
- **Technical context** — What stack, frameworks, and patterns does the project use?
- **Design notes** — Any constraints, preferences, or conventions to follow.
- **Examples** — Reference implementations, API shapes, or UI mockups.
- **Links** — Related docs, design files, or prior art.

### Example prd.md

```markdown
# User Authentication System

## Overview
We're building a complete authentication system for our SaaS application.
Users need to register, log in, reset passwords, and manage sessions.

## Technical Context
- Backend: Express.js with TypeScript
- Database: PostgreSQL with Prisma ORM
- Frontend: React with Next.js
- Auth: JWT tokens stored in httpOnly cookies

## Design Notes
- Follow existing middleware patterns in `src/middleware/`
- Use Zod for input validation (already a dependency)
- All API routes should return consistent error shapes
- Tests use Vitest — run with `npm test`

## Reference
- Existing user model: `prisma/schema.prisma`
- API route pattern: `src/routes/health.ts`
```

This file is included in Claude's context but never parsed programmatically. Claude reads it to understand what you're building and how.

::: tip
The better your `prd.md`, the better Claude's output. Spend time here — it pays off across every story.
:::

## prd.json — The Machine-Readable File

The JSON file is what Chief actually uses to drive execution. It defines the project metadata, optional settings, and an ordered list of user stories.

### Top-Level Schema

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `project` | `string` | Yes | Project name, used in logs and TUI |
| `description` | `string` | Yes | Brief description of what you're building |
| `userStories` | `array` | Yes | Ordered list of user stories |

### UserStory Object

Each story in the `userStories` array has the following fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `id` | `string` | Yes | — | Unique identifier (e.g., `US-001`). Appears in commit messages. |
| `title` | `string` | Yes | — | Short, descriptive title. Keep under 50 characters. |
| `description` | `string` | Yes | — | Full description. User story format recommended. |
| `acceptanceCriteria` | `string[]` | Yes | — | List of requirements. Claude uses these to know when the story is done. |
| `priority` | `number` | Yes | — | Execution order. Lower number = higher priority. |
| `passes` | `boolean` | Yes | `false` | Whether the story has been completed and verified. |
| `inProgress` | `boolean` | Yes | `false` | Whether Claude is currently working on this story. |

### Minimal Example

```json
{
  "project": "My Feature",
  "description": "A new feature for my application",
  "userStories": [
    {
      "id": "US-001",
      "title": "Basic Setup",
      "description": "As a developer, I want the project scaffolded so I can start building.",
      "acceptanceCriteria": [
        "Project directory created",
        "Dependencies installed",
        "Dev server starts successfully"
      ],
      "priority": 1,
      "passes": false,
      "inProgress": false
    }
  ]
}
```

## Story Selection Logic

Chief picks the next story to work on using a simple, deterministic algorithm:

```
1. Filter stories where passes = false
2. Sort remaining stories by priority (ascending)
3. Pick the first one
4. Set inProgress = true on that story
5. Start the iteration
```

### How Priority Works

Priority is a number where **lower = higher priority**. Chief always picks the lowest-numbered incomplete story:

| Story | Priority | Passes | Selected? |
|-------|----------|--------|-----------|
| US-001 | 1 | `true` | No — already complete |
| US-002 | 2 | `false` | **Yes — lowest priority number with passes: false** |
| US-003 | 3 | `false` | No — US-002 goes first |

### What `inProgress` Does

When Chief starts working on a story, it sets `inProgress: true`. This serves as a signal that the story is being actively worked on. When the story completes:

- `passes` is set to `true`
- `inProgress` is set back to `false`

If Chief is interrupted mid-iteration (e.g., you stop it), `inProgress` may remain `true`. On the next run, Chief will pick up the same story and continue.

### Completion Signal

When all stories have `passes: true`, the iteration ends and Chief reports completion. No more iterations are started.

## Annotated Example PRD

Here's a complete `prd.json` with annotations explaining each part:

```json
{
  // The project name — shown in the TUI header and logs
  "project": "User Authentication",

  // A brief description — helps Claude understand scope
  "description": "Complete auth system with login, registration, and password reset",

  "userStories": [
    {
      // Unique ID — appears in commit messages as: feat: [US-001] - User Registration
      "id": "US-001",

      // Short title — keep it under 50 chars for clean commits
      "title": "User Registration",

      // Description — user story format gives Claude clear context
      "description": "As a new user, I want to register an account so that I can access the application.",

      // Acceptance criteria — Claude checks these off as it works
      // Each item should be specific and verifiable
      "acceptanceCriteria": [
        "Registration form with email and password fields",
        "Email format validation",
        "Password minimum 8 characters",
        "Confirmation email sent on registration",
        "User redirected to login after registration"
      ],

      // Priority 1 = done first
      "priority": 1,

      // Chief sets this to true when the story passes all checks
      "passes": false,

      // Chief sets this to true while Claude is working on it
      "inProgress": false
    },
    {
      "id": "US-002",
      "title": "User Login",
      "description": "As a registered user, I want to log in so that I can access my account.",
      "acceptanceCriteria": [
        "Login form with email and password fields",
        "Error message for invalid credentials",
        "JWT token issued on success",
        "Redirect to dashboard on success"
      ],
      // Priority 2 = done after US-001
      "priority": 2,
      "passes": false,
      "inProgress": false
    },
    {
      "id": "US-003",
      "title": "Password Reset",
      "description": "As a user, I want to reset my password so that I can recover my account.",
      "acceptanceCriteria": [
        "\"Forgot password\" link on login page",
        "Email with reset link sent to user",
        "Reset token expires after 1 hour",
        "New password form with confirmation field"
      ],
      "priority": 3,
      "passes": false,
      "inProgress": false
    }
  ]
}
```

::: info
JSON doesn't support comments. The annotations above are for illustration only — your actual `prd.json` should be valid JSON without comments.
:::

## Best Practices

### Write Specific Acceptance Criteria

Each criterion should be concrete and verifiable. Claude uses these to determine what to build and when the story is done.

```json
// ✓ Good — specific and testable
"acceptanceCriteria": [
  "Login form with email and password fields",
  "Error message shown for invalid credentials",
  "JWT token stored in httpOnly cookie on success",
  "Redirect to /dashboard after login"
]

// ✗ Bad — vague and subjective
"acceptanceCriteria": [
  "Nice login page",
  "Good error handling",
  "Secure authentication"
]
```

### Keep Stories Small

A story should represent one logical piece of work. If a story has more than 5–7 acceptance criteria, consider splitting it into multiple stories.

**Too large:**
```json
{
  "title": "Complete Authentication System",
  "acceptanceCriteria": [
    "Registration form", "Login form", "Password reset",
    "Email verification", "OAuth integration", "Session management",
    "Rate limiting", "Account lockout", "Audit logging"
  ]
}
```

**Better — split into focused stories:**
```json
[
  { "id": "US-001", "title": "User Registration", "priority": 1, ... },
  { "id": "US-002", "title": "User Login", "priority": 2, ... },
  { "id": "US-003", "title": "Password Reset", "priority": 3, ... },
  { "id": "US-004", "title": "OAuth Integration", "priority": 4, ... }
]
```

### Order Stories by Dependency

Use priority to ensure foundational stories are completed before dependent ones. Claude works through stories sequentially, so earlier stories can set up what later stories need.

```json
[
  { "id": "US-001", "title": "Database Schema", "priority": 1 },
  { "id": "US-002", "title": "API Endpoints", "priority": 2 },
  { "id": "US-003", "title": "Frontend Forms", "priority": 3 },
  { "id": "US-004", "title": "Integration Tests", "priority": 4 }
]
```

### Use Consistent ID Patterns

Story IDs appear in commit messages (`feat: [US-001] - User Registration`). Pick a pattern and stick with it:

- `US-001`, `US-002` — generic user stories
- `AUTH-001`, `AUTH-002` — feature-scoped prefixes
- `BUG-001`, `FIX-001` — for bug fix PRDs

### Give Claude Context in prd.md

The more context you provide in `prd.md`, the better the output. Include:

- What frameworks and tools the project uses
- Where to find existing patterns to follow
- Any constraints or conventions
- What "done" looks like beyond acceptance criteria

### Use `chief new` to Get Started

Running `chief new` scaffolds both files with a template. You can also run `chief edit` to open an existing PRD for editing. This is the easiest way to create a well-structured PRD.

## What's Next

- [PRD Schema Reference](/reference/prd-schema) — Complete TypeScript type definitions and field details
- [The .chief Directory](/concepts/chief-directory) — Understanding the full directory structure
- [How Chief Works](/concepts/how-it-works) — How Chief uses these files during execution
