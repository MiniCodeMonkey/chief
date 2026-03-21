---
description: Complete prd.md format reference for Chief. Heading structure, field types, status values, and validation rules.
---

# PRD Format Reference

Complete format documentation for `prd.md`.

## Story Heading Format

Each user story is defined by a level-3 markdown heading with an ID and title:

```markdown
### ID: Title
```

**Examples:**
```markdown
### US-001: User Registration
### AUTH-003: Password Reset Flow
### BUG-012: Fix Login Redirect
```

## Story Fields

Below each story heading, Chief recognizes these bold-label fields:

| Field | Format | Required | Default | Description |
|-------|--------|----------|---------|-------------|
| Status | `**Status:** value` | No | `todo` | Current state: `done`, `in-progress`, or `todo` |
| Priority | `**Priority:** N` | No | Document order | Execution order (lower = higher priority) |
| Description | `**Description:** text` | No | — | Story description (or use freeform prose) |

## Acceptance Criteria

Acceptance criteria use markdown checkboxes:

```markdown
- [ ] Criterion not yet met
- [x] Criterion completed
```

Chief reads checkbox state to track progress. The agent checks boxes as it completes each criterion.

## Status Values

| Value | Meaning |
|-------|---------|
| `done` | Story is complete — Chief skips it |
| `in-progress` | Agent is actively working on this story |
| `todo` | Story is pending (also the default if Status is absent) |

## Full Example

```markdown
# User Authentication

## Overview
Complete auth system with login, registration, and password reset.

## Technical Context
- Backend: Express.js with TypeScript
- Database: PostgreSQL with Prisma ORM
- Auth: JWT tokens in httpOnly cookies

## User Stories

### US-001: User Registration

**Status:** done
**Priority:** 1
**Description:** As a new user, I want to register an account so that I can access the application.

- [x] Registration form with email and password fields
- [x] Email format validation
- [x] Password minimum 8 characters
- [x] Confirmation email sent on registration
- [x] User redirected to login after registration

### US-002: User Login

**Status:** todo
**Priority:** 2
**Description:** As a registered user, I want to log in so that I can access my account.

- [ ] Login form with email and password fields
- [ ] Error message for invalid credentials
- [ ] Remember me checkbox
- [ ] Redirect to dashboard on success
```

## Field Details

### id (from heading)

Parsed from the story heading: `### US-001: Title` → id is `US-001`.

**Format:** Any string before the colon, but `US-XXX` pattern recommended.

**Example:** `US-001`, `US-042`, `AUTH-001`

### title (from heading)

Parsed from the story heading: `### US-001: User Registration` → title is `User Registration`.

**Length:** Keep under 50 characters for clean commit messages.

### description

The text after `**Description:**`, or freeform prose between the heading and the first checkbox list.

**Format:** `"As a [user], I want [feature] so that [benefit]."` recommended but not required.

### acceptanceCriteria (checkboxes)

The `- [ ]` / `- [x]` items under each story heading. The agent uses these to know when the story is complete.

**Guidelines:**
- Specific and testable
- One requirement per item
- 3-7 items per story

### priority

Lower numbers = higher priority. Chief always picks the incomplete story with the lowest priority number first. If omitted, stories are selected in document order.

**Range:** Positive integers, typically 1-100

### status

Tracked by Chief. Set to `in-progress` when work begins, `done` when the agent outputs `<chief-done/>`.

**Values:** `done`, `in-progress`, `todo` (default if absent)

## Validation

Chief validates `prd.md` on startup by parsing the markdown structure:

- At least one story heading (`### ID: Title`) must be present
- Each story must have a unique ID
- Priority values (if present) must be positive numbers
- Status values (if present) must be `done`, `in-progress`, or `todo`

Invalid PRDs cause Chief to exit with an error message describing the parsing issue.
