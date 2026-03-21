# Uplink Contract & Protocol Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Define the Uplink device protocol as JSON Schemas with fixture files and a Go protocol package that serializes, deserializes, and validates all 34 message types.

**Architecture:** A `contract/` directory holds JSON Schema definitions and fixture files. An `internal/protocol/` Go package provides typed structs for every message, marshal/unmarshal functions, and a schema validator. Both the Go CLI and the Laravel web app (separate repo) validate against the same `contract/` schemas.

**Tech Stack:** Go 1.24, JSON Schema Draft 2020-12, `santhosh-tekuri/jsonschema` (Go schema validator)

**Spec:** `docs/superpowers/specs/2026-03-21-uplink-network-protocol-design.md`

---

## File Structure

```
contract/
  schemas/
    envelope.json                    ← outer message format (type, id, device_id, timestamp, payload)
    types/
      project.json                   ← shared project type definition
      prd.json                       ← shared PRD type definition
      run.json                       ← shared run type definition
    state/
      sync.json                      ← full state snapshot payload
      projects-updated.json
      prd-created.json
      prd-updated.json
      prd-deleted.json
      prd-chat-output.json
      run-started.json
      run-progress.json
      run-output.json
      run-stopped.json
      run-completed.json
      diffs-response.json
      log-output.json
      log-response.json
      settings-updated.json
      device-heartbeat.json
      files-list.json
      file-response.json
      project-clone-progress.json
    cmd/
      prd-create.json
      prd-message.json
      prd-update.json
      prd-delete.json
      run-start.json
      run-stop.json
      project-clone.json
      diffs-get.json
      log-get.json
      files-list.json
      file-get.json
      settings-get.json
      settings-update.json
    control/
      welcome.json
      ack.json
      error.json
  fixtures/
    state/
      sync.valid.json
      sync.invalid-missing-projects.json
      prd-updated.valid.json
      run-completed.valid.json
      run-completed.invalid-missing-result.json
    cmd/
      prd-create.valid.json
      run-start.valid.json
      run-start.invalid-missing-prd-id.json
    control/
      welcome.valid.json
      ack.valid.json
      error.valid.json

internal/protocol/
  envelope.go                        ← Envelope struct, Marshal/Unmarshal, message routing
  envelope_test.go
  state.go                           ← all state message payload structs
  state_test.go
  cmd.go                             ← all command message payload structs
  cmd_test.go
  control.go                         ← welcome, ack, error payload structs
  control_test.go
  validate.go                        ← JSON Schema validation using contract/schemas/
  validate_test.go
  types.go                           ← shared types (Project, PRD, Run, etc.)
```

---

### Task 1: Set Up Contract Directory with Envelope Schema

**Files:**
- Create: `contract/schemas/envelope.json`
- Create: `contract/fixtures/envelope.valid.json`

- [ ] **Step 1: Create envelope JSON Schema**

```json
// contract/schemas/envelope.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "envelope.json",
  "title": "Uplink Protocol Envelope",
  "description": "Outer message format for all chief ↔ server communication",
  "type": "object",
  "required": ["type", "id", "device_id", "timestamp"],
  "properties": {
    "type": {
      "type": "string",
      "pattern": "^(state|cmd)\\.|^(welcome|ack|error)$"
    },
    "id": {
      "type": "string",
      "description": "Unique message ID (msg_xxxx format)"
    },
    "device_id": {
      "type": "string",
      "description": "Device identifier (dev_xxxx format)"
    },
    "timestamp": {
      "type": "string",
      "format": "date-time"
    },
    "payload": {
      "type": "object"
    }
  },
  "additionalProperties": false
}
```

- [ ] **Step 2: Create valid envelope fixture**

```json
// contract/fixtures/envelope.valid.json
{
  "type": "state.prd.updated",
  "id": "msg_01abc123",
  "device_id": "dev_xyz789",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": {}
}
```

- [ ] **Step 3: Commit**

```bash
git add contract/
git commit -m "feat(protocol): add envelope JSON Schema and fixture"
```

---

### Task 2: Define Control Message Schemas

**Files:**
- Create: `contract/schemas/control/welcome.json`
- Create: `contract/schemas/control/ack.json`
- Create: `contract/schemas/control/error.json`
- Create: `contract/fixtures/control/welcome.valid.json`
- Create: `contract/fixtures/control/ack.valid.json`
- Create: `contract/fixtures/control/error.valid.json`

- [ ] **Step 1: Create welcome schema**

```json
// contract/schemas/control/welcome.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "control/welcome.json",
  "title": "Welcome",
  "description": "Sent by server on WebSocket connect",
  "type": "object",
  "required": ["session_id", "server_version", "capabilities"],
  "properties": {
    "session_id": { "type": "string" },
    "server_version": { "type": "string" },
    "capabilities": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "additionalProperties": false
}
```

- [ ] **Step 2: Create ack schema**

```json
// contract/schemas/control/ack.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "control/ack.json",
  "title": "Ack",
  "description": "Acknowledges receipt of a command",
  "type": "object",
  "required": ["ref_id"],
  "properties": {
    "ref_id": {
      "type": "string",
      "description": "ID of the command being acknowledged"
    }
  },
  "additionalProperties": false
}
```

- [ ] **Step 3: Create error schema**

```json
// contract/schemas/control/error.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "control/error.json",
  "title": "Error",
  "description": "Command failed",
  "type": "object",
  "required": ["ref_id", "code", "message"],
  "properties": {
    "ref_id": {
      "type": "string",
      "description": "ID of the command that failed"
    },
    "code": {
      "type": "string",
      "description": "Machine-readable error code"
    },
    "message": {
      "type": "string",
      "description": "Human-readable error description"
    }
  },
  "additionalProperties": false
}
```

- [ ] **Step 4: Create fixtures for each**

```json
// contract/fixtures/control/welcome.valid.json
{
  "type": "welcome",
  "id": "msg_welcome_001",
  "device_id": "dev_server",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": {
    "session_id": "sess_abc123",
    "server_version": "1.0.0",
    "capabilities": ["state_sync", "commands", "streaming"]
  }
}

// contract/fixtures/control/ack.valid.json
{
  "type": "ack",
  "id": "msg_ack_001",
  "device_id": "dev_abc123",
  "timestamp": "2026-03-21T10:30:01Z",
  "payload": {
    "ref_id": "msg_cmd_001"
  }
}

// contract/fixtures/control/error.valid.json
{
  "type": "error",
  "id": "msg_err_001",
  "device_id": "dev_abc123",
  "timestamp": "2026-03-21T10:30:01Z",
  "payload": {
    "ref_id": "msg_cmd_001",
    "code": "prd_not_found",
    "message": "PRD with ID prd_123 not found"
  }
}
```

- [ ] **Step 5: Commit**

```bash
git add contract/schemas/control/ contract/fixtures/control/
git commit -m "feat(protocol): add control message schemas (welcome, ack, error)"
```

---

### Task 3: Define State Message Schemas (Core)

**Files:**
- Create: `contract/schemas/state/sync.json`
- Create: `contract/schemas/state/projects-updated.json`
- Create: `contract/schemas/state/prd-created.json`
- Create: `contract/schemas/state/prd-updated.json`
- Create: `contract/schemas/state/prd-deleted.json`
- Create: `contract/schemas/state/run-started.json`
- Create: `contract/schemas/state/run-progress.json`
- Create: `contract/schemas/state/run-stopped.json`
- Create: `contract/schemas/state/run-completed.json`
- Create: `contract/schemas/state/device-heartbeat.json`
- Create: `contract/schemas/state/settings-updated.json`
- Create: Corresponding fixtures

These schemas define the cached state messages. Each payload schema references shared type definitions.

- [ ] **Step 1: Create shared type definitions**

Create `contract/schemas/types/` with reusable definitions:

```json
// contract/schemas/types/project.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "types/project.json",
  "type": "object",
  "required": ["id", "path", "name"],
  "properties": {
    "id": { "type": "string" },
    "path": { "type": "string" },
    "name": { "type": "string" },
    "git_remote": { "type": "string" },
    "git_branch": { "type": "string" },
    "git_status": { "type": "string", "enum": ["clean", "dirty"] },
    "last_commit": {
      "type": "object",
      "properties": {
        "hash": { "type": "string" },
        "message": { "type": "string" },
        "timestamp": { "type": "string", "format": "date-time" }
      }
    }
  }
}

// contract/schemas/types/prd.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "types/prd.json",
  "type": "object",
  "required": ["id", "project_id", "title", "status"],
  "properties": {
    "id": { "type": "string" },
    "project_id": { "type": "string" },
    "title": { "type": "string" },
    "status": { "type": "string", "enum": ["draft", "ready", "running", "completed"] },
    "content": { "type": "string" },
    "progress": { "type": "string" },
    "chat_history": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["role", "content", "timestamp"],
        "properties": {
          "role": { "type": "string", "enum": ["user", "assistant"] },
          "content": { "type": "string" },
          "timestamp": { "type": "string", "format": "date-time" }
        }
      }
    },
    "session_id": { "type": "string", "description": "Claude Code session ID for --resume" }
  }
}

// contract/schemas/types/run.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "types/run.json",
  "type": "object",
  "required": ["id", "prd_id", "status"],
  "properties": {
    "id": { "type": "string" },
    "prd_id": { "type": "string" },
    "status": { "type": "string", "enum": ["running", "stopped", "completed"] },
    "result": { "type": "string", "enum": ["success", "failure", "error"] },
    "error_message": { "type": "string" },
    "story_index": { "type": "integer" },
    "story_id": { "type": "string" },
    "started_at": { "type": "string", "format": "date-time" },
    "completed_at": { "type": "string", "format": "date-time" }
  }
}
```

- [ ] **Step 2: Create sync schema** (the most important — full snapshot)

```json
// contract/schemas/state/sync.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/sync.json",
  "title": "State Sync",
  "description": "Full state snapshot sent on connect",
  "type": "object",
  "required": ["device", "projects", "prds", "runs"],
  "properties": {
    "device": {
      "type": "object",
      "required": ["name", "os", "arch", "chief_version"],
      "properties": {
        "name": { "type": "string" },
        "os": { "type": "string" },
        "arch": { "type": "string" },
        "chief_version": { "type": "string" }
      }
    },
    "projects": {
      "type": "array",
      "items": { "$ref": "../types/project.json" }
    },
    "prds": {
      "type": "array",
      "items": { "$ref": "../types/prd.json" }
    },
    "runs": {
      "type": "array",
      "items": { "$ref": "../types/run.json" }
    },
    "settings": { "type": "object" }
  }
}
```

- [ ] **Step 3: Create remaining core state schemas**

Each follows the same pattern — required fields matching the type definitions. Create schemas for: `projects-updated`, `prd-created`, `prd-updated`, `prd-deleted`, `run-started`, `run-progress`, `run-stopped`, `run-completed`, `device-heartbeat`, `settings-updated`.

Pattern for simple state schemas:
```json
// contract/schemas/state/prd-updated.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/prd-updated.json",
  "title": "PRD Updated",
  "type": "object",
  "required": ["prd"],
  "properties": {
    "prd": { "$ref": "../types/prd.json" }
  }
}

// contract/schemas/state/run-completed.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/run-completed.json",
  "title": "Run Completed",
  "type": "object",
  "required": ["run"],
  "properties": {
    "run": { "$ref": "../types/run.json" }
  }
}
```

- [ ] **Step 4: Create valid fixtures for sync and key state messages**

```json
// contract/fixtures/state/sync.valid.json
{
  "type": "state.sync",
  "id": "msg_sync_001",
  "device_id": "dev_abc123",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": {
    "device": {
      "name": "my-vps",
      "os": "linux",
      "arch": "amd64",
      "chief_version": "0.5.0"
    },
    "projects": [
      {
        "id": "proj_001",
        "path": "/home/user/projects/myapp",
        "name": "myapp",
        "git_remote": "git@github.com:user/myapp.git",
        "git_branch": "main",
        "git_status": "clean",
        "last_commit": {
          "hash": "abc123",
          "message": "feat: add user auth",
          "timestamp": "2026-03-21T09:00:00Z"
        }
      }
    ],
    "prds": [
      {
        "id": "prd_001",
        "project_id": "proj_001",
        "title": "User Authentication",
        "status": "ready",
        "content": "# User Auth PRD\n...",
        "progress": "## Progress\n...",
        "chat_history": [
          { "role": "user", "content": "I want to add OAuth login", "timestamp": "2026-03-21T08:00:00Z" },
          { "role": "assistant", "content": "I'll help you design that. What providers?", "timestamp": "2026-03-21T08:00:05Z" }
        ],
        "session_id": "de263703-4d83-40ba-8574-d1c30ef1acc6"
      }
    ],
    "runs": []
  }
}

// contract/fixtures/state/sync.invalid-missing-projects.json
{
  "type": "state.sync",
  "id": "msg_sync_002",
  "device_id": "dev_abc123",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": {
    "device": { "name": "test", "os": "linux", "arch": "amd64", "chief_version": "0.5.0" },
    "prds": [],
    "runs": []
  }
}
```

- [ ] **Step 5: Create invalid fixture for run-completed**

```json
// contract/fixtures/state/run-completed.invalid-missing-result.json
{
  "type": "state.run.completed",
  "id": "msg_run_001",
  "device_id": "dev_abc123",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": {
    "run": {
      "id": "run_001",
      "prd_id": "prd_001",
      "status": "completed"
    }
  }
}
```

- [ ] **Step 6: Commit**

```bash
git add contract/
git commit -m "feat(protocol): add state message schemas, shared types, and fixtures"
```

---

### Task 4: Define Ephemeral State and Response Schemas

**Files:**
- Create: `contract/schemas/state/prd-chat-output.json`
- Create: `contract/schemas/state/run-output.json`
- Create: `contract/schemas/state/log-output.json`
- Create: `contract/schemas/state/log-response.json`
- Create: `contract/schemas/state/diffs-response.json`
- Create: `contract/schemas/state/files-list.json`
- Create: `contract/schemas/state/file-response.json`
- Create: `contract/schemas/state/project-clone-progress.json`

- [ ] **Step 1: Create ephemeral streaming schemas**

```json
// contract/schemas/state/run-output.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/run-output.json",
  "title": "Run Output (Ephemeral)",
  "type": "object",
  "required": ["run_id", "event_type"],
  "properties": {
    "run_id": { "type": "string" },
    "event_type": {
      "type": "string",
      "enum": ["assistant_text", "tool_start", "tool_result", "iteration_start", "story_done"]
    },
    "text": { "type": "string" },
    "tool": { "type": "string" },
    "tool_input": { "type": "object" },
    "story_id": { "type": "string" },
    "iteration": { "type": "integer" }
  }
}

// contract/schemas/state/prd-chat-output.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/prd-chat-output.json",
  "title": "PRD Chat Output (Ephemeral)",
  "type": "object",
  "required": ["prd_id", "event_type"],
  "properties": {
    "prd_id": { "type": "string" },
    "event_type": {
      "type": "string",
      "enum": ["assistant_text", "tool_start", "tool_result"]
    },
    "text": { "type": "string" },
    "tool": { "type": "string" },
    "tool_input": { "type": "object" }
  }
}

// contract/schemas/state/log-output.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/log-output.json",
  "title": "Log Output (Ephemeral)",
  "type": "object",
  "required": ["lines"],
  "properties": {
    "lines": {
      "type": "array",
      "items": { "type": "string" }
    }
  }
}
```

- [ ] **Step 2: Create response schemas (on-demand data)**

```json
// contract/schemas/state/diffs-response.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/diffs-response.json",
  "title": "Diffs Response",
  "type": "object",
  "required": ["ref_id", "project_id", "diffs"],
  "properties": {
    "ref_id": { "type": "string", "description": "ID of cmd.diffs.get that requested this" },
    "project_id": { "type": "string" },
    "diffs": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["file_path", "diff"],
        "properties": {
          "file_path": { "type": "string" },
          "diff": { "type": "string" },
          "status": { "type": "string", "enum": ["added", "modified", "deleted", "renamed"] }
        }
      }
    },
    "story_id": { "type": "string" }
  }
}

// contract/schemas/state/files-list.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/files-list.json",
  "title": "Files List",
  "type": "object",
  "required": ["ref_id", "project_id", "path", "entries"],
  "properties": {
    "ref_id": { "type": "string" },
    "project_id": { "type": "string" },
    "path": { "type": "string" },
    "entries": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "type"],
        "properties": {
          "name": { "type": "string" },
          "type": { "type": "string", "enum": ["file", "directory"] },
          "size": { "type": "integer" },
          "modified": { "type": "string", "format": "date-time" }
        }
      }
    }
  }
}

// contract/schemas/state/file-response.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/file-response.json",
  "title": "File Response",
  "type": "object",
  "required": ["ref_id", "project_id", "path", "content"],
  "properties": {
    "ref_id": { "type": "string" },
    "project_id": { "type": "string" },
    "path": { "type": "string" },
    "content": { "type": "string" },
    "language": { "type": "string", "description": "Syntax hint from file extension" }
  }
}

// contract/schemas/state/log-response.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/log-response.json",
  "title": "Log Response",
  "type": "object",
  "required": ["ref_id", "lines"],
  "properties": {
    "ref_id": { "type": "string" },
    "lines": {
      "type": "array",
      "items": { "type": "string" }
    }
  }
}

// contract/schemas/state/project-clone-progress.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "state/project-clone-progress.json",
  "title": "Project Clone Progress (Ephemeral)",
  "type": "object",
  "required": ["ref_id", "status"],
  "properties": {
    "ref_id": { "type": "string" },
    "status": { "type": "string", "enum": ["cloning", "completed", "failed"] },
    "progress": { "type": "string" },
    "error": { "type": "string" }
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add contract/schemas/state/
git commit -m "feat(protocol): add ephemeral and response state schemas"
```

---

### Task 5: Define Command Message Schemas

**Files:**
- Create: `contract/schemas/cmd/*.json` (13 schemas)
- Create: `contract/fixtures/cmd/*.json`

- [ ] **Step 1: Create PRD command schemas**

```json
// contract/schemas/cmd/prd-create.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/prd-create.json",
  "title": "Create PRD",
  "type": "object",
  "required": ["project_id", "message"],
  "properties": {
    "project_id": { "type": "string" },
    "message": { "type": "string", "description": "User's initial message for PRD creation" },
    "context": { "type": "string", "description": "Optional additional context" }
  }
}

// contract/schemas/cmd/prd-message.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/prd-message.json",
  "title": "PRD Message",
  "type": "object",
  "required": ["prd_id", "message"],
  "properties": {
    "prd_id": { "type": "string" },
    "message": { "type": "string" }
  }
}

// contract/schemas/cmd/prd-update.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/prd-update.json",
  "title": "PRD Update (Direct Edit)",
  "type": "object",
  "required": ["prd_id", "content"],
  "properties": {
    "prd_id": { "type": "string" },
    "content": { "type": "string", "description": "Full PRD markdown content" }
  }
}

// contract/schemas/cmd/prd-delete.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/prd-delete.json",
  "title": "Delete PRD",
  "type": "object",
  "required": ["prd_id"],
  "properties": {
    "prd_id": { "type": "string" }
  }
}
```

- [ ] **Step 2: Create run command schemas**

```json
// contract/schemas/cmd/run-start.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/run-start.json",
  "title": "Start Run",
  "type": "object",
  "required": ["prd_id"],
  "properties": {
    "prd_id": { "type": "string" },
    "max_iterations": { "type": "integer", "minimum": 1 }
  }
}

// contract/schemas/cmd/run-stop.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/run-stop.json",
  "title": "Stop Run",
  "type": "object",
  "required": ["run_id"],
  "properties": {
    "run_id": { "type": "string" }
  }
}
```

- [ ] **Step 3: Create remaining command schemas**

Follow the same pattern for: `project-clone`, `diffs-get`, `log-get`, `files-list`, `file-get`, `settings-get`, `settings-update`.

```json
// contract/schemas/cmd/project-clone.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/project-clone.json",
  "type": "object",
  "required": ["repo_url"],
  "properties": {
    "repo_url": { "type": "string" },
    "branch": { "type": "string" },
    "path": { "type": "string", "description": "Target directory name" }
  }
}

// contract/schemas/cmd/diffs-get.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/diffs-get.json",
  "type": "object",
  "required": ["project_id"],
  "properties": {
    "project_id": { "type": "string" },
    "story_id": { "type": "string" }
  }
}

// contract/schemas/cmd/log-get.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/log-get.json",
  "type": "object",
  "required": ["project_id"],
  "properties": {
    "project_id": { "type": "string" },
    "lines": { "type": "integer", "minimum": 1, "default": 100 }
  }
}

// contract/schemas/cmd/files-list.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/files-list.json",
  "type": "object",
  "required": ["project_id", "path"],
  "properties": {
    "project_id": { "type": "string" },
    "path": { "type": "string", "description": "Relative to project root" }
  }
}

// contract/schemas/cmd/file-get.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/file-get.json",
  "type": "object",
  "required": ["project_id", "path"],
  "properties": {
    "project_id": { "type": "string" },
    "path": { "type": "string" }
  }
}

// contract/schemas/cmd/settings-get.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/settings-get.json",
  "type": "object",
  "properties": {}
}

// contract/schemas/cmd/settings-update.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "cmd/settings-update.json",
  "type": "object",
  "required": ["settings"],
  "properties": {
    "settings": { "type": "object" }
  }
}
```

- [ ] **Step 4: Create command fixtures**

```json
// contract/fixtures/cmd/prd-create.valid.json
{
  "type": "cmd.prd.create",
  "id": "msg_cmd_001",
  "device_id": "dev_abc123",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": {
    "project_id": "proj_001",
    "message": "I want to build a REST API for user management"
  }
}

// contract/fixtures/cmd/run-start.valid.json
{
  "type": "cmd.run.start",
  "id": "msg_cmd_002",
  "device_id": "dev_abc123",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": {
    "prd_id": "prd_001",
    "max_iterations": 50
  }
}

// contract/fixtures/cmd/run-start.invalid-missing-prd-id.json
{
  "type": "cmd.run.start",
  "id": "msg_cmd_003",
  "device_id": "dev_abc123",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": {
    "max_iterations": 50
  }
}
```

- [ ] **Step 5: Commit**

```bash
git add contract/schemas/cmd/ contract/fixtures/cmd/
git commit -m "feat(protocol): add command message schemas and fixtures"
```

---

### Task 6: Go Protocol Package — Types and Envelope

**Files:**
- Create: `internal/protocol/types.go`
- Create: `internal/protocol/envelope.go`
- Create: `internal/protocol/envelope_test.go`

- [ ] **Step 1: Add dependency**

```bash
go get github.com/google/uuid
```

- [ ] **Step 2: Write failing test for envelope marshal/unmarshal**

```go
// internal/protocol/envelope_test.go
package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEnvelopeMarshalRoundTrip(t *testing.T) {
	env := NewEnvelope("state.prd.updated", "dev_test", StatePRDUpdated{
		PRD: PRD{
			ID:        "prd_001",
			ProjectID: "proj_001",
			Title:     "Test PRD",
			Status:    PRDStatusDraft,
		},
	})

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Envelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "state.prd.updated" {
		t.Errorf("Type = %q, want %q", decoded.Type, "state.prd.updated")
	}
	if decoded.DeviceID != "dev_test" {
		t.Errorf("DeviceID = %q, want %q", decoded.DeviceID, "dev_test")
	}
	if decoded.ID == "" {
		t.Error("ID should be auto-generated")
	}
}

func TestEnvelopeUnmarshalPayload(t *testing.T) {
	raw := `{"type":"cmd.run.start","id":"msg_001","device_id":"dev_test","timestamp":"2026-03-21T10:30:00Z","payload":{"prd_id":"prd_001","max_iterations":50}}`

	env, err := ParseEnvelope([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEnvelope failed: %v", err)
	}

	payload, err := DecodePayload[CmdRunStart](env)
	if err != nil {
		t.Fatalf("DecodePayload failed: %v", err)
	}

	if payload.PRDID != "prd_001" {
		t.Errorf("PRDID = %q, want %q", payload.PRDID, "prd_001")
	}
	if payload.MaxIterations != 50 {
		t.Errorf("MaxIterations = %d, want %d", payload.MaxIterations, 50)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run TestEnvelope
```
Expected: compilation error (types don't exist yet)

- [ ] **Step 4: Implement types.go**

```go
// internal/protocol/types.go
package protocol

// PRD status constants
type PRDStatus string

const (
	PRDStatusDraft     PRDStatus = "draft"
	PRDStatusReady     PRDStatus = "ready"
	PRDStatusRunning   PRDStatus = "running"
	PRDStatusCompleted PRDStatus = "completed"
)

// Run result constants
type RunResult string

const (
	RunResultSuccess RunResult = "success"
	RunResultFailure RunResult = "failure"
	RunResultError   RunResult = "error"
)

// Run status constants
type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusStopped   RunStatus = "stopped"
	RunStatusCompleted RunStatus = "completed"
)

// Git status constants
type GitStatus string

const (
	GitStatusClean GitStatus = "clean"
	GitStatusDirty GitStatus = "dirty"
)

// Shared types matching contract/schemas/types/

type Project struct {
	ID         string     `json:"id"`
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	GitRemote  string     `json:"git_remote,omitempty"`
	GitBranch  string     `json:"git_branch,omitempty"`
	GitStatus  GitStatus  `json:"git_status,omitempty"`
	LastCommit *GitCommit `json:"last_commit,omitempty"`
}

type GitCommit struct {
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type PRD struct {
	ID          string        `json:"id"`
	ProjectID   string        `json:"project_id"`
	Title       string        `json:"title"`
	Status      PRDStatus     `json:"status"`
	Content     string        `json:"content,omitempty"`
	Progress    string        `json:"progress,omitempty"`
	ChatHistory []ChatMessage `json:"chat_history,omitempty"`
	SessionID   string        `json:"session_id,omitempty"`
}

type ChatMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type Run struct {
	ID           string    `json:"id"`
	PRDID        string    `json:"prd_id"`
	Status       RunStatus `json:"status"`
	Result       RunResult `json:"result,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	StoryIndex   int       `json:"story_index,omitempty"`
	StoryID      string    `json:"story_id,omitempty"`
	StartedAt    string    `json:"started_at,omitempty"`
	CompletedAt  string    `json:"completed_at,omitempty"`
}

type FileEntry struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "file" or "directory"
	Size     int64  `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"`
}

type DiffEntry struct {
	FilePath string `json:"file_path"`
	Diff     string `json:"diff"`
	Status   string `json:"status"` // added, modified, deleted, renamed
}
```

- [ ] **Step 5: Implement envelope.go**

```go
// internal/protocol/envelope.go
package protocol

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Envelope is the outer message format for all protocol messages.
type Envelope struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	DeviceID  string          `json:"device_id"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// NewEnvelope creates an envelope with auto-generated ID and timestamp.
func NewEnvelope(msgType, deviceID string, payload interface{}) Envelope {
	var raw json.RawMessage
	if payload != nil {
		data, _ := json.Marshal(payload)
		raw = data
	}

	return Envelope{
		Type:      msgType,
		ID:        "msg_" + uuid.New().String()[:12],
		DeviceID:  deviceID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   raw,
	}
}

// ParseEnvelope unmarshals a raw JSON message into an Envelope.
func ParseEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse envelope: %w", err)
	}
	return &env, nil
}

// DecodePayload extracts a typed payload from an envelope.
func DecodePayload[T any](env *Envelope) (*T, error) {
	var payload T
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decode payload for %s: %w", env.Type, err)
	}
	return &payload, nil
}

// Marshal serializes an envelope to JSON bytes.
func (e Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
```

- [ ] **Step 6: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run TestEnvelope
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/protocol/types.go internal/protocol/envelope.go internal/protocol/envelope_test.go go.mod go.sum
git commit -m "feat(protocol): add Go envelope and shared types"
```

---

### Task 7: Go Protocol Package — State Message Structs

**Files:**
- Create: `internal/protocol/state.go`
- Create: `internal/protocol/state_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/protocol/state_test.go
package protocol

import (
	"encoding/json"
	"os"
	"testing"
)

func TestStateSyncMatchesFixture(t *testing.T) {
	fixture, err := os.ReadFile("../../contract/fixtures/state/sync.valid.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	env, err := ParseEnvelope(fixture)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}

	payload, err := DecodePayload[StateSync](env)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}

	if payload.Device.Name == "" {
		t.Error("Device.Name should not be empty")
	}
	if len(payload.Projects) == 0 {
		t.Error("Projects should not be empty in this fixture")
	}
	if len(payload.PRDs) == 0 {
		t.Error("PRDs should not be empty in this fixture")
	}

	// Round-trip: marshal back and verify valid JSON
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal StateSync: %v", err)
	}
	if len(data) == 0 {
		t.Error("Marshaled data should not be empty")
	}
}

func TestRunCompletedPayload(t *testing.T) {
	env := NewEnvelope("state.run.completed", "dev_test", StateRunCompleted{
		Run: Run{
			ID:     "run_001",
			PRDID:  "prd_001",
			Status: RunStatusCompleted,
			Result: RunResultSuccess,
		},
	})

	payload, err := DecodePayload[StateRunCompleted](&env)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}

	if payload.Run.Result != RunResultSuccess {
		t.Errorf("Result = %q, want %q", payload.Run.Result, RunResultSuccess)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run "TestStateSync|TestRunCompleted"
```
Expected: compilation error

- [ ] **Step 3: Implement state.go**

```go
// internal/protocol/state.go
package protocol

// Message type constants for state messages
const (
	TypeStateSync               = "state.sync"
	TypeStateProjectsUpdated    = "state.projects.updated"
	TypeStatePRDCreated         = "state.prd.created"
	TypeStatePRDUpdated         = "state.prd.updated"
	TypeStatePRDDeleted         = "state.prd.deleted"
	TypeStatePRDChatOutput      = "state.prd.chat.output"
	TypeStateRunStarted         = "state.run.started"
	TypeStateRunProgress        = "state.run.progress"
	TypeStateRunOutput          = "state.run.output"
	TypeStateRunStopped         = "state.run.stopped"
	TypeStateRunCompleted       = "state.run.completed"
	TypeStateDiffsResponse      = "state.diffs.response"
	TypeStateLogOutput          = "state.log.output"
	TypeStateLogResponse        = "state.log.response"
	TypeStateSettingsUpdated    = "state.settings.updated"
	TypeStateDeviceHeartbeat    = "state.device.heartbeat"
	TypeStateFilesList          = "state.files.list"
	TypeStateFileResponse       = "state.file.response"
	TypeStateProjectCloneProgress = "state.project.clone.progress"
)

// Cached state payloads

type DeviceInfo struct {
	Name         string `json:"name"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	ChiefVersion string `json:"chief_version"`
}

type StateSync struct {
	Device   DeviceInfo        `json:"device"`
	Projects []Project         `json:"projects"`
	PRDs     []PRD             `json:"prds"`
	Runs     []Run             `json:"runs"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

type StateProjectsUpdated struct {
	Projects []Project `json:"projects"`
}

type StatePRDCreated struct {
	PRD PRD `json:"prd"`
}

type StatePRDUpdated struct {
	PRD PRD `json:"prd"`
}

type StatePRDDeleted struct {
	PRDID string `json:"prd_id"`
}

type StateRunStarted struct {
	Run Run `json:"run"`
}

type StateRunProgress struct {
	Run Run `json:"run"`
}

type StateRunStopped struct {
	Run Run `json:"run"`
}

type StateRunCompleted struct {
	Run Run `json:"run"`
}

type StateSettingsUpdated struct {
	Settings map[string]interface{} `json:"settings"`
}

type StateDeviceHeartbeat struct{}

// Ephemeral state payloads

type StatePRDChatOutput struct {
	PRDID     string                 `json:"prd_id"`
	EventType string                 `json:"event_type"` // assistant_text, tool_start, tool_result
	Text      string                 `json:"text,omitempty"`
	Tool      string                 `json:"tool,omitempty"`
	ToolInput map[string]interface{} `json:"tool_input,omitempty"`
}

type StateRunOutput struct {
	RunID     string                 `json:"run_id"`
	EventType string                 `json:"event_type"` // assistant_text, tool_start, tool_result, iteration_start, story_done
	Text      string                 `json:"text,omitempty"`
	Tool      string                 `json:"tool,omitempty"`
	ToolInput map[string]interface{} `json:"tool_input,omitempty"`
	StoryID   string                 `json:"story_id,omitempty"`
	Iteration int                    `json:"iteration,omitempty"`
}

type StateLogOutput struct {
	Lines []string `json:"lines"`
}

// Response payloads (on-demand)

type StateDiffsResponse struct {
	RefID     string      `json:"ref_id"`
	ProjectID string      `json:"project_id"`
	Diffs     []DiffEntry `json:"diffs"`
	StoryID   string      `json:"story_id,omitempty"`
}

type StateLogResponse struct {
	RefID string   `json:"ref_id"`
	Lines []string `json:"lines"`
}

type StateFilesList struct {
	RefID     string      `json:"ref_id"`
	ProjectID string      `json:"project_id"`
	Path      string      `json:"path"`
	Entries   []FileEntry `json:"entries"`
}

type StateFileResponse struct {
	RefID     string `json:"ref_id"`
	ProjectID string `json:"project_id"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Language  string `json:"language,omitempty"`
}

type StateProjectCloneProgress struct {
	RefID    string `json:"ref_id"`
	Status   string `json:"status"` // cloning, completed, failed
	Progress string `json:"progress,omitempty"`
	Error    string `json:"error,omitempty"`
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run "TestStateSync|TestRunCompleted"
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/protocol/state.go internal/protocol/state_test.go
git commit -m "feat(protocol): add state message structs with fixture tests"
```

---

### Task 8: Go Protocol Package — Command and Control Structs

**Files:**
- Create: `internal/protocol/cmd.go`
- Create: `internal/protocol/cmd_test.go`
- Create: `internal/protocol/control.go`
- Create: `internal/protocol/control_test.go`

- [ ] **Step 1: Write failing test for command structs**

```go
// internal/protocol/cmd_test.go
package protocol

import (
	"os"
	"testing"
)

func TestCmdRunStartMatchesFixture(t *testing.T) {
	fixture, err := os.ReadFile("../../contract/fixtures/cmd/run-start.valid.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	env, err := ParseEnvelope(fixture)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}

	payload, err := DecodePayload[CmdRunStart](env)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}

	if payload.PRDID != "prd_001" {
		t.Errorf("PRDID = %q, want %q", payload.PRDID, "prd_001")
	}
}

func TestCmdPRDCreateMatchesFixture(t *testing.T) {
	fixture, err := os.ReadFile("../../contract/fixtures/cmd/prd-create.valid.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	env, err := ParseEnvelope(fixture)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}

	payload, err := DecodePayload[CmdPRDCreate](env)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}

	if payload.ProjectID == "" {
		t.Error("ProjectID should not be empty")
	}
	if payload.Message == "" {
		t.Error("Message should not be empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run "TestCmd"
```

- [ ] **Step 3: Implement cmd.go**

```go
// internal/protocol/cmd.go
package protocol

// Message type constants for command messages
const (
	TypeCmdPRDCreate      = "cmd.prd.create"
	TypeCmdPRDMessage     = "cmd.prd.message"
	TypeCmdPRDUpdate      = "cmd.prd.update"
	TypeCmdPRDDelete      = "cmd.prd.delete"
	TypeCmdRunStart       = "cmd.run.start"
	TypeCmdRunStop        = "cmd.run.stop"
	TypeCmdProjectClone   = "cmd.project.clone"
	TypeCmdDiffsGet       = "cmd.diffs.get"
	TypeCmdLogGet         = "cmd.log.get"
	TypeCmdFilesList      = "cmd.files.list"
	TypeCmdFileGet        = "cmd.file.get"
	TypeCmdSettingsGet    = "cmd.settings.get"
	TypeCmdSettingsUpdate = "cmd.settings.update"
)

type CmdPRDCreate struct {
	ProjectID string `json:"project_id"`
	Message   string `json:"message"`
	Context   string `json:"context,omitempty"`
}

type CmdPRDMessage struct {
	PRDID   string `json:"prd_id"`
	Message string `json:"message"`
}

type CmdPRDUpdate struct {
	PRDID   string `json:"prd_id"`
	Content string `json:"content"`
}

type CmdPRDDelete struct {
	PRDID string `json:"prd_id"`
}

type CmdRunStart struct {
	PRDID         string `json:"prd_id"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

type CmdRunStop struct {
	RunID string `json:"run_id"`
}

type CmdProjectClone struct {
	RepoURL string `json:"repo_url"`
	Branch  string `json:"branch,omitempty"`
	Path    string `json:"path,omitempty"`
}

type CmdDiffsGet struct {
	ProjectID string `json:"project_id"`
	StoryID   string `json:"story_id,omitempty"`
}

type CmdLogGet struct {
	ProjectID string `json:"project_id"`
	Lines     int    `json:"lines,omitempty"`
}

type CmdFilesList struct {
	ProjectID string `json:"project_id"`
	Path      string `json:"path"`
}

type CmdFileGet struct {
	ProjectID string `json:"project_id"`
	Path      string `json:"path"`
}

type CmdSettingsGet struct{}

type CmdSettingsUpdate struct {
	Settings map[string]interface{} `json:"settings"`
}
```

- [ ] **Step 4: Implement control.go**

```go
// internal/protocol/control.go
package protocol

// Message type constants for control messages
const (
	TypeWelcome = "welcome"
	TypeAck     = "ack"
	TypeError   = "error"
)

type Welcome struct {
	SessionID     string   `json:"session_id"`
	ServerVersion string   `json:"server_version"`
	Capabilities  []string `json:"capabilities"`
}

type Ack struct {
	RefID string `json:"ref_id"`
}

type Error struct {
	RefID   string `json:"ref_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
```

- [ ] **Step 5: Write control test**

```go
// internal/protocol/control_test.go
package protocol

import (
	"os"
	"testing"
)

func TestWelcomeMatchesFixture(t *testing.T) {
	fixture, err := os.ReadFile("../../contract/fixtures/control/welcome.valid.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	env, err := ParseEnvelope(fixture)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}

	payload, err := DecodePayload[Welcome](env)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}

	if payload.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
}

func TestAckMatchesFixture(t *testing.T) {
	fixture, err := os.ReadFile("../../contract/fixtures/control/ack.valid.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	env, err := ParseEnvelope(fixture)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}

	payload, err := DecodePayload[Ack](env)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}

	if payload.RefID == "" {
		t.Error("RefID should not be empty")
	}
}
```

- [ ] **Step 6: Run all tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v
```
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/protocol/cmd.go internal/protocol/cmd_test.go internal/protocol/control.go internal/protocol/control_test.go
git commit -m "feat(protocol): add command and control message structs"
```

---

### Task 9: Go Protocol Package — Schema Validation

**Files:**
- Create: `internal/protocol/validate.go`
- Create: `internal/protocol/validate_test.go`

- [ ] **Step 1: Add schema validation dependency**

```bash
cd /Users/codemonkey/projects/chief && go get github.com/santhosh-tekuri/jsonschema/v6
```

- [ ] **Step 2: Write failing test**

```go
// internal/protocol/validate_test.go
package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateValidFixture(t *testing.T) {
	v, err := NewValidator("../../contract/schemas")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	fixture, err := os.ReadFile("../../contract/fixtures/state/sync.valid.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if err := v.ValidateEnvelope(fixture); err != nil {
		t.Errorf("ValidateEnvelope should pass for valid fixture: %v", err)
	}
}

func TestValidateInvalidFixture(t *testing.T) {
	v, err := NewValidator("../../contract/schemas")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	fixture, err := os.ReadFile("../../contract/fixtures/state/sync.invalid-missing-projects.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if err := v.ValidateEnvelope(fixture); err == nil {
		t.Error("ValidateEnvelope should fail for invalid fixture")
	}
}

func TestValidateAllValidFixtures(t *testing.T) {
	v, err := NewValidator("../../contract/schemas")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	fixtureRoot := "../../contract/fixtures"
	err = filepath.Walk(fixtureRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".valid.json") {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if err := v.ValidateEnvelope(data); err != nil {
				t.Errorf("Valid fixture should pass validation: %v", err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

func TestValidateAllInvalidFixtures(t *testing.T) {
	v, err := NewValidator("../../contract/schemas")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	fixtureRoot := "../../contract/fixtures"
	err = filepath.Walk(fixtureRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.Contains(filepath.Base(path), ".invalid-") {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if err := v.ValidateEnvelope(data); err == nil {
				t.Error("Invalid fixture should fail validation")
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run TestValidate
```

- [ ] **Step 4: Implement validate.go**

```go
// internal/protocol/validate.go
package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator validates protocol messages against JSON Schemas.
type Validator struct {
	compiler   *jsonschema.Compiler
	schemas    map[string]*jsonschema.Schema
	schemasDir string
}

// NewValidator creates a validator from the contract schemas directory.
func NewValidator(schemasDir string) (*Validator, error) {
	c := jsonschema.NewCompiler()

	v := &Validator{
		compiler:   c,
		schemas:    make(map[string]*jsonschema.Schema),
		schemasDir: schemasDir,
	}

	// Load envelope schema
	envelopeSchema, err := c.Compile(schemasDir + "/envelope.json")
	if err != nil {
		return nil, fmt.Errorf("compile envelope schema: %w", err)
	}
	v.schemas["envelope"] = envelopeSchema

	return v, nil
}

// ValidateEnvelope validates a raw JSON message against the envelope schema
// and then validates the payload against the type-specific schema.
func (v *Validator) ValidateEnvelope(data []byte) error {
	// Parse to get the type
	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	// Validate envelope structure
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal for validation: %w", err)
	}

	if schema, ok := v.schemas["envelope"]; ok {
		if err := schema.Validate(raw); err != nil {
			return fmt.Errorf("envelope validation: %w", err)
		}
	}

	// Validate payload against type-specific schema
	schemaPath := typeToSchemaPath(env.Type)
	if schemaPath != "" {
		payloadSchema, err := v.compiler.Compile(v.schemasDir + "/" + schemaPath)
		if err != nil {
			return fmt.Errorf("compile payload schema %s: %w", schemaPath, err)
		}

		var payloadRaw interface{}
		if err := json.Unmarshal(env.Payload, &payloadRaw); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}

		if err := payloadSchema.Validate(payloadRaw); err != nil {
			return fmt.Errorf("payload validation for %s: %w", env.Type, err)
		}
	}

	return nil
}

// typeToSchemaPath maps message types to their schema file paths.
func typeToSchemaPath(msgType string) string {
	// "state.prd.updated" -> "state/prd-updated.json"
	// "cmd.run.start" -> "cmd/run-start.json"
	// "welcome" / "ack" / "error" -> "control/welcome.json"

	parts := strings.SplitN(msgType, ".", 2)
	if len(parts) < 2 {
		// Control messages: welcome, ack, error
		return "control/" + msgType + ".json"
	}

	category := parts[0] // "state" or "cmd"
	rest := strings.ReplaceAll(parts[1], ".", "-")
	return category + "/" + rest + ".json"
}
```

**Important implementation notes:**
- The `santhosh-tekuri/jsonschema` library needs proper filesystem-based `$ref` resolution. Use `c.AddResource()` to register schema files by their `$id`, or use `file://` URIs so that relative `$ref` paths like `../types/project.json` resolve correctly from the schemas directory root.
- Consult the library docs for `jsonschema.Compiler` file loading — you may need to walk the schemas directory and register all schemas upfront in `NewValidator`.
- Add `"strings"` to the import block for `strings.SplitN` and `strings.ReplaceAll`.

- [ ] **Step 5: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run TestValidate
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/protocol/validate.go internal/protocol/validate_test.go go.mod go.sum
git commit -m "feat(protocol): add JSON Schema validation against contract"
```

---

### Task 10: Message Router (type → handler dispatch)

**Files:**
- Create: `internal/protocol/router.go`
- Create: `internal/protocol/router_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/protocol/router_test.go
package protocol

import (
	"testing"
)

func TestRouterDispatch(t *testing.T) {
	r := NewRouter()

	var called bool
	r.Handle(TypeCmdRunStart, func(env *Envelope) error {
		called = true
		payload, err := DecodePayload[CmdRunStart](env)
		if err != nil {
			t.Fatalf("DecodePayload: %v", err)
		}
		if payload.PRDID != "prd_test" {
			t.Errorf("PRDID = %q, want %q", payload.PRDID, "prd_test")
		}
		return nil
	})

	env := NewEnvelope(TypeCmdRunStart, "dev_test", CmdRunStart{PRDID: "prd_test"})
	if err := r.Dispatch(&env); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !called {
		t.Error("Handler should have been called")
	}
}

func TestRouterUnknownType(t *testing.T) {
	r := NewRouter()
	env := NewEnvelope("unknown.type", "dev_test", nil)
	err := r.Dispatch(&env)
	if err == nil {
		t.Error("Dispatch should return error for unknown type")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run TestRouter
```

- [ ] **Step 3: Implement router.go**

```go
// internal/protocol/router.go
package protocol

import "fmt"

// HandlerFunc processes a protocol message envelope.
type HandlerFunc func(env *Envelope) error

// Router dispatches envelopes to registered handlers by message type.
type Router struct {
	handlers map[string]HandlerFunc
}

// NewRouter creates an empty message router.
func NewRouter() *Router {
	return &Router{
		handlers: make(map[string]HandlerFunc),
	}
}

// Handle registers a handler for a message type.
func (r *Router) Handle(msgType string, handler HandlerFunc) {
	r.handlers[msgType] = handler
}

// Dispatch routes an envelope to its registered handler.
func (r *Router) Dispatch(env *Envelope) error {
	handler, ok := r.handlers[env.Type]
	if !ok {
		return fmt.Errorf("no handler for message type: %s", env.Type)
	}
	return handler(env)
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/codemonkey/projects/chief && go test ./internal/protocol/ -v -run TestRouter
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/protocol/router.go internal/protocol/router_test.go
git commit -m "feat(protocol): add message router for type-based dispatch"
```
