# Chief Web App

A web UI that remote-controls a running `chief serve` instance. Built with Laravel + Tailwind CSS, hosted on Laravel Cloud.

---

## Core Concept

The web app does **zero AI**. It's a thin frontend that talks to a `chief serve` process running on the user's machine or VPS. All Claude Code calls happen on the server side through the CLI, using the user's own Claude Max plan.

```
Browser (Laravel + Tailwind on Laravel Cloud)
    ↕ WebSocket (outbound from chief server — no port forwarding)
chief serve (user's machine or VPS — workspace mode)
    ↕ shells out to
Claude Code CLI (user's Max plan)
    ↕ operates on
~/projects/
    ├── repo-a/
    ├── repo-b/
    └── repo-c/
```

The chief server initiates the connection outbound to the web app (like Plex, Tailscale). No port forwarding, no firewall config. It just works.

---

## Accounts & Connection

A single random URL is too ephemeral — users need a persistent place to come back to, especially when running chief on a VM that stays online 24/7.

### Account system (lightweight)

- **GitHub OAuth login**: One click, no forms. Developers already have GitHub accounts. Laravel Socialite makes this trivial.
- **No email/password**: Keep it simple. GitHub is the only auth provider (at least initially).
- **What an account stores**: User ID, connected servers, project list, preferences. No PRDs, no code, no AI state — that all lives on the chief server.

### Server connection flow

1. User logs into chief.dev
2. Dashboard shows "No servers connected. Run `chief serve --token <your-token>` to connect."
3. The token is tied to their account (generated in the web UI, stored hashed in the DB)
4. User runs `chief serve --token abc123` on their machine or VPS
5. Chief connects outbound via WebSocket to chief.dev, authenticates with the token
6. Server appears in the dashboard as online — persists across browser sessions, reconnects automatically

### Server tokens

- Users can generate multiple tokens (e.g., "home laptop", "hetzner vps", "work machine")
- Tokens can be revoked from the web UI
- Each token maps to one chief server instance
- `chief serve` reconnects automatically on network interruptions with exponential backoff

---

## Multi-Project Workspace

Today chief runs inside a single project directory. For the web app, `chief serve` needs to manage multiple repos.

### `chief serve --workspace ~/projects`

Chief runs from a workspace directory containing multiple repos:

```
~/projects/
    ├── my-saas-app/          ← git repo with .chief/
    ├── marketing-site/       ← git repo with .chief/
    ├── api-service/          ← git repo, no .chief/ yet
    └── mobile-app/           ← git repo with .chief/
```

- **Auto-discovery**: On startup, chief scans the workspace for directories containing `.chief/` or `.git/`. Lists them as available projects.
- **Create new project**: From the web UI, user can `git clone <url>` a repo into the workspace. Chief runs the clone on the server.
- **Init new project**: Create a new directory, `git init`, then `chief new` to generate a PRD — all from the web UI.
- **Per-project state**: Each project has its own `.chief/` directory with PRDs, progress, logs. Nothing shared between projects.
- **Concurrent execution**: Run PRDs on multiple projects simultaneously. Each gets its own Claude Code process.

### Project dashboard

The main view after login. Shows all projects in the workspace:

| Project | Status | Current PRD | Progress | Last Activity |
|---------|--------|-------------|----------|---------------|
| my-saas-app | Running | auth-system | 7/12 stories | 2 min ago |
| marketing-site | Idle | — | — | 3 days ago |
| api-service | No PRD | — | — | — |

- Click a project to enter its detail view
- Status: Running / Idle / Error / No PRD
- Quick actions: New PRD, Run, Stop, View diffs

---

## Core Features

### Conversational PRD creation

The primary interaction. Same experience as `chief new` but in the browser.

1. User clicks "New PRD" on a project
2. Chat interface opens — user describes what they want
3. Chief server spawns `claude -p "..." --output-format stream-json` with `init_prompt.txt`
4. Claude's responses stream back through the relay to the browser in real-time
5. As Claude generates the PRD, a live preview panel shows the stories forming
6. User can respond to Claude's clarifying questions in the chat
7. When done, the PRD is saved to `.chief/prds/<name>/` on the server

### PRD refinement

After initial generation, users refine conversationally:

- "Split US-003 into two stories"
- "Add error handling acceptance criteria to US-005"
- "Reorder so the database setup comes first"
- "Make this work with PostgreSQL instead of SQLite"

Each message triggers a new Claude CLI call with the current PRD as context. The preview panel updates live.

### Run control & monitoring

- **Start/stop/pause** a PRD run from the browser
- **Live story progress**: Which story is being worked on, which iteration, pass/fail status
- **Live Claude output**: Stream the raw Claude output as it works (collapsible, for power users)
- **Diff viewer**: Per-story git diffs. What files changed, lines added/removed. Syntax highlighted.
- **Progress timeline**: Visual timeline showing iteration history, retries, time per story
- **Token usage**: Per-iteration and cumulative token counts (parsed from Claude's stream-json output)

### Manual PRD editing

Secondary to the conversational flow, but available:

- Markdown editor with syntax highlighting for the PRD format
- Side-by-side edit + preview
- For quick tweaks: fix a typo, reword acceptance criteria, adjust a story title
- Should feel like "view source" — not the primary way to work

---

## Technical Architecture

### Laravel backend (hosted on Laravel Cloud)

- **Authentication**: Laravel Socialite with GitHub OAuth
- **Database**: Users, server tokens, project metadata (names, last seen status). Minimal schema — no PRDs or code stored.
- **WebSocket relay**: Laravel Reverb (native Laravel WebSocket server). Matches browser clients to chief servers by token.
- **API**: Simple REST endpoints for account management, token CRUD. All project operations are relayed to the chief server.
- **Queue**: None needed initially. The app is essentially stateless — just routing WebSocket messages.

### Chief server (`chief serve`)

Changes needed in the Go codebase:

- **New `serve` command**: `chief serve --workspace ~/projects --token <token>`
- **WebSocket client**: Connects outbound to the Laravel app's Reverb endpoint. Authenticates with token. Reconnects with backoff.
- **Workspace manager**: Scans workspace directory, tracks projects, manages concurrent operations across repos.
- **JSON protocol**: Structured messages between web app and chief server:
  - **Server → Web**: `project_list`, `prd_status`, `story_update`, `claude_output`, `diff`, `error`
  - **Web → Server**: `new_prd`, `refine_prd`, `run`, `pause`, `stop`, `clone_repo`, `list_projects`
- **Process management**: Multiple concurrent `claude` CLI processes (one per active project). Track PIDs, handle cleanup.

### Frontend (Tailwind CSS)

- **Livewire or Inertia.js** for reactive UI without a separate SPA build step
- **Project dashboard**: Grid/list of projects with status indicators
- **Chat interface**: For PRD creation/refinement. Message bubbles, streaming text, typing indicators.
- **PRD preview panel**: Rendered markdown with story cards. Updates live during generation.
- **Diff viewer**: Syntax-highlighted diffs with file tree navigation. Could use a library like diff2html.
- **Terminal output panel**: Collapsible raw Claude output stream. Monospace, auto-scroll.

---

## One-Click Cloud Deploy

For users who want a persistent chief server without running it locally.

### "Deploy to Hetzner" flow

1. User clicks "Deploy Server" in the web app dashboard
2. Picks a provider (Hetzner, DigitalOcean) and region
3. Web app uses the provider API to create a small VPS (~$5/mo CX22)
4. Cloud-init script:
   - Installs Go, git, chief, Claude Code CLI
   - Configures `chief serve --workspace /home/chief/projects --token <auto-generated>`
   - Sets up systemd service for auto-start on boot
   - Connects to the web app automatically
5. Server appears in the user's dashboard within ~60 seconds
6. **Auth step**: User needs to authenticate Claude Code on the VPS. Options:
   - SSH in once and run `claude` to do the interactive auth (friction but works with Max plan)
   - Provide an Anthropic API key during setup (simpler but pay-as-you-go, no Max)
   - Web-based terminal in the dashboard (xterm.js over WebSocket to the VPS) so user never leaves the browser

### VPS management from the web UI

- See server status (online/offline, CPU, memory, disk)
- Restart chief process
- SSH terminal in browser (xterm.js)
- Destroy server (with confirmation)
- Auto-suspend after N hours of inactivity to save costs (Hetzner supports this)

---

## Data Model

What the Laravel app stores (intentionally minimal):

```
users
    id, github_id, github_username, avatar_url, created_at

server_tokens
    id, user_id, name, token_hash, last_connected_at, created_at

servers (ephemeral — populated from live WebSocket connections)
    token_id, status (online/offline), workspace_path, connected_at

cloud_deployments (optional — for managed VPS)
    id, user_id, provider, region, server_ip, provider_server_id, status, created_at
```

No PRDs, no code, no Claude output, no project files. All of that lives on the chief server. The web app is a relay + account system, nothing more.

---

## What's NOT in Scope (Initially)

- **Team/org accounts**: V1 is single-user. One account, one or more servers.
- **Billing/payments**: The web app is free. Users pay for their own VPS + Claude subscription.
- **PRD storage on the server**: PRDs live on the chief server's filesystem, not in the web app's database.
- **Mobile-specific features**: The web app should be responsive from day one, but no native app or PWA-specific features yet.
- **Self-hosted web app option**: V1 is hosted on Laravel Cloud only. Self-hosting instructions can come later if there's demand.

---

## Open Questions

1. **Livewire vs Inertia.js?** Livewire keeps everything in PHP/Blade, simpler stack. Inertia gives a more app-like feel with Vue/React components. For a real-time streaming UI (chat, live diffs, terminal output), Inertia + Vue might be more natural.

2. **How to handle Claude Code auth on cloud VPS?** The SSH-in-once approach works but is friction. A web-based terminal (xterm.js) in the dashboard would eliminate the need to open a separate SSH client. Is that worth building for V1?

3. **Should `chief serve` work without the web app?** A local-only mode (`chief serve --local`) that exposes a localhost HTTP UI could be useful for users who don't want to connect to chief.dev. The same Go server, but serving its own web UI instead of connecting to a relay. Would this fragment effort or is it a natural extension?

4. **Git operations from the web UI?** Users will want to clone repos, create branches, view git log, maybe even commit/push from the web UI. How much git functionality should the web app expose vs. expecting users to manage git separately?

5. **Notification system?** When chief finishes a PRD run overnight, how does the user find out? Browser push notifications? Email? Slack webhook configured per-project? All of the above?
