# Chief Uplink — Network Protocol & Infrastructure Design

## Overview

Chief Uplink is a remote management and control system for Chief. A central web app (uplink.chiefloop.com) manages Chief instances running on remote machines, supporting PRD creation via chat, Ralph loop control, live Claude output streaming, code review, file browsing, and project management — all from a browser.

The web app does zero AI work. All Claude Code execution happens on the user's machine or VPS using their own Claude subscription. Chief connects outbound to the web app (like Plex or Tailscale), so no port forwarding or firewall configuration is needed.

## System Architecture

Three components, one deployment:

```
┌─────────────────┐         ┌──────────────────────────────┐         ┌─────────────┐
│   chief serve   │         │     Laravel App (Octane)      │         │   Browser    │
│   (Go process)  │◄──WS──►│                              │◄─Reverb─►│ (Vue/Inertia)│
│                 │         │  /ws/device    → DeviceSocket │         │             │
│  Owns all state │         │  /api/*        → REST API     │         │  Reads from  │
│  Runs Claude    │         │  Reverb        → Browser push │         │  server cache│
│                 │         │  DB            → State cache   │         │             │
└─────────────────┘         └──────────────────────────────┘         └─────────────┘
```

### Key Principles

- **Chief is the sole owner of truth.** The server's database is a read-only cache — a projection of what chief has pushed. The server never modifies cached state on its own.
- **Two separate WebSocket layers.** The device protocol (`/ws/device`) is a dedicated WebSocket route, completely separate from Reverb. Different protocol, different handler, different purpose.
- **Browser never talks to chief directly.** All interaction goes through the Laravel app.
- **One-way state flow.** State: chief → server cache → browser. Commands: browser → server → chief.

### Data Flow Patterns

1. **State sync (chief → server):** Chief pushes small JSON messages whenever state changes (PRD updated, run started, project added). Server writes to DB and broadcasts via Reverb to connected browsers.
2. **Commands (server → chief):** Browser sends a command via REST API. Laravel validates, records it in `pending_commands`, and forwards to chief over the device WebSocket. Chief acknowledges with `ack` or `error`.
3. **Streaming (chief → server → browser):** Live Claude output streams through the device WebSocket. If a browser is subscribed to that device's Reverb channel, the server relays the frames directly without storing them. If no browser is watching, the frames are dropped.

### Why This Topology

The previous implementation (`feat/uplink2`) used a single Reverb/Pusher WebSocket for both device and browser communication. This led to protocol boundary confusion and inconsistency bugs. Separating the two layers means:

- Chief only knows the device protocol
- The browser only knows the Laravel app
- Laravel is the translator/orchestrator between them

## Device Protocol

### Message Envelope

Every message between chief and the server uses the same JSON envelope:

```json
{
  "type": "state.prd.updated",
  "id": "msg_01abc123",
  "device_id": "dev_xyz",
  "timestamp": "2026-03-21T10:30:00Z",
  "payload": { }
}
```

### Message Catalog

#### State Messages (chief → server) — 18 types

| Type | Description | Storage |
|------|-------------|---------|
| `state.sync` | Full state snapshot (sent on connect). Includes: device info, all projects (with git status), all PRDs (with content, progress, chat_history, session_id), all active/recent runs. | Replaces entire device cache |
| `state.projects.updated` | Project list changed; includes per-project git status (branch, clean/dirty, last commit) | Cached |
| `state.prd.created` | New PRD created (includes full content, chat history, and session_id) | Cached (content encrypted) |
| `state.prd.updated` | PRD content, progress, chat history, or session_id changed | Cached (content encrypted) |
| `state.prd.deleted` | PRD removed | Deletes from cache |
| `state.prd.chat.output` | Streaming PRD chat response (agent thinking/writing) | Ephemeral (relay only) |
| `state.run.started` | Ralph loop began | Cached |
| `state.run.progress` | Story/pass progress update | Cached |
| `state.run.output` | Streaming Claude output during run | Ephemeral (relay only) |
| `state.run.stopped` | Run stopped by command | Cached |
| `state.run.completed` | Run finished; includes `result` (success/failure/error) and optional `error_message` | Cached |
| `state.diffs.response` | Git diff result (response to `cmd.diffs.get`) | Cached (encrypted) |
| `state.log.output` | Streaming log tail | Ephemeral (relay only) |
| `state.log.response` | Recent log lines (response to `cmd.log.get`) | Cached briefly |
| `state.settings.updated` | Device settings changed | Cached |
| `state.device.heartbeat` | Keepalive | Updates `last_seen_at` |
| `state.files.list` | Directory listing (response to `cmd.files.list`) | Not cached |
| `state.file.response` | File content (response to `cmd.file.get`); includes syntax hint from extension | Not cached |
| `state.project.clone.progress` | Git clone progress updates | Ephemeral (relay only) |

#### Command Messages (server → chief) — 13 types

| Type | Description |
|------|-------------|
| `cmd.prd.create` | Create new PRD with initial chat message |
| `cmd.prd.message` | Send chat message to refine an existing PRD |
| `cmd.prd.update` | Direct PRD content update (from markdown editor) |
| `cmd.prd.delete` | Delete a PRD |
| `cmd.run.start` | Start Ralph loop on a PRD |
| `cmd.run.stop` | Stop a running Ralph loop |
| `cmd.project.clone` | Clone a git repo into the workspace |
| `cmd.diffs.get` | Request git diffs (optionally filtered by story) |
| `cmd.log.get` | Request recent log lines (optional `lines` param, default 100) |
| `cmd.files.list` | List directory contents (with `path` relative to project root) |
| `cmd.file.get` | Read file content (with `path` relative to project root) |
| `cmd.settings.get` | Request current device settings |
| `cmd.settings.update` | Update device settings |

#### Control Messages (bidirectional) — 3 types

| Type | Direction | Description |
|------|-----------|-------------|
| `welcome` | server → chief | Sent on connect; includes session ID and server capabilities |
| `ack` | chief → server | Acknowledges receipt of a command; references original message `id` |
| `error` | chief → server | Command failed; references original message `id`, includes error details |

**Total: 34 message types**

### Protocol Rules

1. Every command gets an `ack` or `error` response referencing the original `id`.
2. State messages are fire-and-forget — no ack needed, the server caches the latest.
3. Ephemeral messages (`state.run.output`, `state.log.output`, `state.prd.chat.output`, `state.project.clone.progress`) are relayed to Reverb if a browser is listening, otherwise dropped. Never stored in DB.
4. The `state.sync` snapshot on connect replaces the server's entire cache for that device — no delta tracking or "catch up" logic needed.
5. No message batching — send each state change as it happens over the persistent connection.
6. Commands that produce async data responses (e.g., `cmd.diffs.get` → `state.diffs.response`) follow a two-phase pattern: immediate `ack` (command received), then a data response message with `ref_id` set to the original command's `id` for correlation.
7. Chief must send `state.run.stopped` or `state.run.completed` whenever a run ends, regardless of whether it was triggered via uplink or locally. This ensures the web app always reflects the current run state.
8. File paths in `cmd.files.list` and `cmd.file.get` are relative to the project root. Chief must validate paths to prevent directory traversal beyond the project boundary.

## Connection Lifecycle & Authentication

### OAuth Device Flow

1. User runs `chief login` (optionally with `--url http://localhost:8000` for local dev)
2. Chief POSTs to `/api/auth/device/request` — receives `device_code` and `user_code`
3. Terminal displays: "Visit uplink.chiefloop.com/activate and enter code: ABCD-1234"
4. User approves in browser
5. Chief polls `/api/auth/device/verify` until approved — receives `access_token` and `refresh_token`
6. Credentials stored in `~/.chief/credentials.yaml` along with the uplink URL

### Uplink URL Configuration

The uplink URL defaults to `https://uplink.chiefloop.com` and can be overridden:

```yaml
# ~/.chief/config.yaml
uplink:
  enabled: true
  url: https://uplink.chiefloop.com  # default
```

- `chief login --url http://localhost:8000` overrides for that auth flow and saves the URL with credentials
- Supports local development and self-hosted installations

### WebSocket Connection

1. Chief opens WebSocket to `wss://<uplink-url>/ws/device` with `Authorization: Bearer <access_token>` header
2. Server validates token, looks up device and user
3. Server sends `welcome` message with session ID and server capabilities
4. Chief responds with `state.sync` — full snapshot of all projects, PRDs, run statuses
5. Connection is live — state pushes and commands flow freely

### Reconnection

- Exponential backoff with jitter: 1s, 2s, 4s, 8s... capped at 60s
- On reconnect, chief sends a fresh `state.sync` — server replaces its entire cache for that device
- After processing `state.sync`, server drains any `pending_commands` (oldest first) to the device
- No need to track "what changed since disconnect"

### Token Refresh

- Chief refreshes the access token before expiry using the refresh token
- If refresh fails (token revoked or expired), chief logs the error and stops serving — user needs to `chief login` again

### Multiple Devices

- Each `chief serve` process gets its own device ID during OAuth
- Each process is independent — user runs `chief serve` in each project directory
- Server tracks connections per device, grouped by team
- Browser shows all connected devices and their projects

### Team Scoping

The protocol has no team concept — team assignment is purely server-side. When a device authenticates, the server resolves team membership from the device record in the database. The flow:

1. User registers → a default team is created for them (Owner role)
2. User approves a device code at `/activate` → device joins their default team (or user selects a team if they belong to multiple)
3. All protocol messages use `device_id` only — the server maps device → team internally
4. Team members see all devices belonging to their team in the browser

### Provisioned Server Authentication

When the web app provisions a VPS, it bypasses the interactive device code flow:

1. Web app calls internal `POST /api/auth/device/provision` with `team_id` and `server_name`
2. Server creates a device record and generates an `access_token` + `refresh_token` directly
3. Provisioning script receives the tokens and writes `~/.chief/credentials.yaml` on the VPS
4. `chief serve` starts and connects using these credentials — no user interaction needed
5. The device appears in the team's device list immediately

This endpoint is internal-only (not exposed to external clients).

## Server-Side Architecture (Laravel)

### Device WebSocket Handling

- Dedicated WebSocket route `/ws/device` handled under Octane
- Separate from Reverb — this is a raw WebSocket endpoint for the device protocol
- Handler authenticates the connection, then dispatches incoming messages to a `DeviceMessageHandler` service
- All messages validated against JSON schemas from `contract/schemas/`

### Database Schema

```
devices          → id, team_id, name, os, arch, chief_version, access_token (hashed), refresh_token (encrypted), last_seen_at, connected
projects         → id, device_id, path, name, git_remote, git_branch, git_status, last_commit_hash, last_commit_message, last_commit_at
prds             → id, project_id, device_id, title, status, content (encrypted), progress (encrypted), chat_history (encrypted), session_id
runs             → id, prd_id, device_id, status, result, error_message, started_at, completed_at, story_index
pending_commands → id, device_id, type, payload (encrypted), status (pending/delivered/failed), created_at, delivered_at
```

- Content fields (PRD body, progress, chat history, diffs) encrypted at rest using Laravel's built-in encryption (AES-256-CBC)
- Metadata fields (status, timestamps, names) stored plaintext for querying

### Command Flow

1. Browser calls REST endpoint (e.g., `POST /api/devices/{id}/commands`)
2. Controller validates the request, creates a `pending_commands` record with status `pending`
3. If device is connected, forwards the command over the device WebSocket
4. When chief sends `ack`, the record is marked `delivered`
5. If device is offline, the command stays `pending` and is delivered on reconnect

### Broadcasting to Browser

- Standard Laravel events + Reverb private channels
- `private-team.{teamId}` — cross-device updates (device online/offline, new projects)
- `private-device.{deviceId}` — device-specific state (PRD updates, run progress, streaming)
- Browser subscribes on page load and receives real-time updates

### Browser Push Notifications

- When `state.run.completed` arrives and the user has no active browser tab, trigger a web push notification
- Notification preferences configurable in the web app settings

## Streaming & Live Relay

### How Streaming Works

1. Chief runs Claude Code, parsing stdout (NDJSON)
2. For each parsed event, chief sends a `state.run.output` message over the device WebSocket
3. Server checks if any browser is subscribed to that device's Reverb channel
4. If yes — relays directly to Reverb, no database write
5. If no — drops the frame

### Persisted vs. Ephemeral

| Persisted (DB) | Ephemeral (relay only) |
|---|---|
| Run status (started/stopped/completed) | Raw Claude token output |
| Run result (success/failure/error) | Tool use streaming |
| Story progress (which story, pass count) | Live log lines |
| PRD state and chat history | PRD chat streaming response |
| Final error messages | Clone progress |

Opening the browser mid-run shows cached status/progress instantly and begins receiving live output from that point forward. No replay of missed output.

### Backpressure

- If the browser can't keep up (slow mobile connection), Reverb handles buffering at the channel level
- If the device WebSocket backs up, chief skips output frames rather than blocking Claude — Claude's execution is never gated on the relay

## PRD Chat Sessions (Claude Code Integration)

### How PRD Chat Works Over Uplink

PRD creation and editing use Claude Code's `--resume` flag to maintain multi-turn conversations with structured NDJSON output. No PTY or terminal parsing needed.

**First message (create PRD):**
```
claude --print --output-format stream-json --verbose \
  --dangerously-skip-permissions \
  -p "<init_prompt + user message>" \
  --dir <project_dir>
```

**Subsequent messages (continue conversation):**
```
claude --print --output-format stream-json --verbose \
  --dangerously-skip-permissions \
  --resume <session_id> \
  -p "<user message>"
```

### Session Flow

1. Browser sends `cmd.prd.create` with the user's first message
2. Chief spawns Claude Code with the init prompt + user message
3. Chief parses NDJSON stdout using the same parser as Ralph loops
4. As Claude responds, chief streams `state.prd.chat.output` (ephemeral) to the server for live display
5. When Claude finishes its turn, chief captures the `session_id` from the `result` event
6. Chief sends `state.prd.updated` with the updated chat history and the `session_id`
7. For each subsequent `cmd.prd.message`, chief runs `claude --resume <session_id>` with the new message
8. Claude maintains full conversation context through its own session persistence

### State Storage

- **Chief machine:** Claude Code stores session data internally (used by `--resume`)
- **Chief tracking:** Session ID stored alongside PRD metadata
- **Server DB:** Full chat history stored (encrypted) in the `prds.chat_history` column
- **Browser:** Reads chat history from server cache on page load; receives live streaming via Reverb

### Why Not PTY

The previous implementation (`feat/uplink2`) used `creack/pty` to run Claude in a persistent terminal session. This caused bugs from ANSI escape code parsing and required complex session lifecycle management. The `--resume` approach:

- Uses the same NDJSON parser as Ralph loops (proven, tested)
- Each turn is a clean process invocation (no zombie processes, no session timeouts)
- ~500ms-1s startup overhead per turn, imperceptible against API response time
- No terminal output parsing, no ANSI codes, fully structured events

## Contract Testing

The protocol is defined once and both sides validate against it.

### Directory Structure

```
contract/
  schemas/
    envelope.json              ← outer message format
    state/
      sync.json
      prd-created.json
      prd-updated.json
      run-started.json
      run-progress.json
      run-output.json
      run-completed.json
      ...
    cmd/
      prd-create.json
      prd-message.json
      run-start.json
      ...
    control/
      welcome.json
      ack.json
      error.json
  fixtures/
    state/
      sync.valid.json
      sync.invalid-missing-projects.json
      ...
    cmd/
      run-start.valid.json
      ...
```

### How It Works

- JSON Schema files are the single source of truth for the protocol
- Both Go (chief) and PHP (Laravel) load and validate against the same schema files
- Go tests: marshal a struct, validate against schema, compare with fixture
- Laravel tests: validate fixture against schema, deserialize, assert structure
- CI runs both test suites against the same `contract/` directory

### Shared Schema Distribution

The `contract/` directory lives in the chief repo and is vendored or submoduled into the Laravel project. A schema change forces both sides to update. Both sides fail CI if fixtures don't match schemas.

### Adding a New Message Type

1. Write the JSON Schema in `contract/schemas/`
2. Write valid and invalid fixture files in `contract/fixtures/`
3. Go side: add struct, write serialize/deserialize test against fixtures
4. Laravel side: add DTO, write validate/deserialize test against fixtures

Both sides can be developed independently — as long as contract tests pass, they're compatible.

## Security Model

### In Transit

- All connections over TLS — `wss://` for device WebSocket, `https://` for REST and Reverb
- No plaintext connections accepted in production

### At Rest

- Sensitive content fields (PRD body, progress, chat history, diffs) encrypted using Laravel's built-in encryption (AES-256-CBC with app key)
- Metadata (device names, project paths, run status, timestamps) stored plaintext for querying
- A database dump cannot expose user content; the Laravel app can decrypt for serving to authenticated users

### Authentication & Authorization

- OAuth device flow issues scoped tokens per device
- Every WebSocket message is on an authenticated connection — no per-message auth needed
- REST API uses Laravel Sanctum tokens (browser session)
- Reverb channels are private — Laravel's channel authorization ensures users only see their own devices

### Device Revocation

- `chief logout` calls the server to revoke the device token
- Server can also revoke from the web dashboard (e.g., lost laptop)
- Revocation immediately closes the device WebSocket and deletes cached state for that device

### Rate Limiting

- Device WebSocket: message rate limit per connection (prevents a buggy chief from flooding the server)
- REST API: standard Laravel rate limiting per user

## Feature Summary

### PRD Management
- Create PRD via conversational chat interface with live preview
- Refine PRD by chatting back and forth with the agent
- Edit PRD directly with a markdown editor
- View PRD list and details (from server cache, instant load)
- Delete PRDs

### Ralph Loop Control
- Start/stop Ralph loops on any PRD
- Multiple concurrent runs across different PRDs/projects
- Real-time story progress and pass count updates
- Run completion with success/failure/error status
- Browser push notifications on run completion

### Live Streaming
- Stream Claude output in real-time during runs
- Tail agent log (`claude.log`) with live updates
- Fetch recent log history on page load
- Stream PRD chat responses as the agent thinks/writes

### Project Management
- List all projects across connected devices
- Clone repos into workspace
- View per-project git status (branch, clean/dirty, last commit)

### Code Review
- View syntax-highlighted git diffs
- Filter diffs per story

### File System Browsing
- Browse project directory trees
- View file contents with syntax highlighting

### Device Management
- View all connected devices with online/offline status
- Revoke device access from web dashboard
- View/edit device settings remotely

## Infrastructure

- **Web app stack:** Laravel 13 / Vue 3 / Inertia / Tailwind 4
- **Hosting:** Single Hetzner Cloud server running Laravel (Octane), Reverb, and database
- **One-click VPS deployment:** Hetzner or DigitalOcean for persistent `chief serve` instances
- **Scale target:** Dozens to low hundreds of users, each with a few devices and projects
