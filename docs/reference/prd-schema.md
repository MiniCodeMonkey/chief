---
description: Complete prd.json schema reference for Chief. TypeScript interfaces, field types, and validation rules for PRD files.
---

# PRD Schema Reference

Complete schema documentation for `prd.json`.

## Top-Level Schema

```typescript
interface PRD {
  project: string;          // Project name
  description: string;      // Brief description
  userStories: UserStory[]; // Array of user stories
}
```

## UserStory Object

```typescript
interface UserStory {
  id: string;                    // Unique identifier
  title: string;                 // Short title
  description: string;           // Full description
  acceptanceCriteria: string[];  // What must be true
  priority: number;              // Lower = higher priority
  passes: boolean;               // Is this complete?
  inProgress: boolean;           // Being worked on?
}
```

## Full Example

```json
{
  "project": "User Authentication",
  "description": "Complete auth system with login, registration, and password reset",
  "userStories": [
    {
      "id": "US-001",
      "title": "User Registration",
      "description": "As a new user, I want to register an account so that I can access the application.",
      "acceptanceCriteria": [
        "Registration form with email and password fields",
        "Email format validation",
        "Password minimum 8 characters",
        "Confirmation email sent on registration",
        "User redirected to login after registration"
      ],
      "priority": 1,
      "passes": false,
      "inProgress": false
    },
    {
      "id": "US-002",
      "title": "User Login",
      "description": "As a registered user, I want to log in so that I can access my account.",
      "acceptanceCriteria": [
        "Login form with email and password fields",
        "Error message for invalid credentials",
        "Remember me checkbox",
        "Redirect to dashboard on success"
      ],
      "priority": 2,
      "passes": false,
      "inProgress": false
    }
  ]
}
```

## Field Details

### id

A unique identifier for the story. Appears in commit messages.

**Format:** Any string, but `US-XXX` pattern recommended.

**Example:** `"US-001"`, `"US-042"`, `"AUTH-001"`

### title

Short, descriptive title. Should fit in a commit message.

**Length:** Keep under 50 characters

**Example:** `"User Registration"`, `"Password Reset Flow"`

### description

Full description of the story. User story format recommended but not required.

**Format:** `"As a [user], I want [feature] so that [benefit]."`

### acceptanceCriteria

Array of strings, each describing a requirement. Claude uses these to know when the story is complete.

**Guidelines:**
- Specific and testable
- One requirement per item
- 3-7 items per story

### priority

Lower numbers = higher priority. Chief always picks the incomplete story with the lowest priority number first.

**Range:** Positive integers, typically 1-100

### passes

Boolean indicating if the story is complete. Chief updates this automatically.

**Default:** `false`

### inProgress

Boolean indicating if Claude is currently working on this story.

**Default:** `false`

## Validation

Chief validates `prd.json` on startup:

- All required fields must be present
- `userStories` must be non-empty
- Each story must have unique `id`
- `priority` must be a positive number

Invalid PRDs cause Chief to exit with an error message.
