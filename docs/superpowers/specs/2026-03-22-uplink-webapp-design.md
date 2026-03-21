# Chief Uplink — Web Application Design

## Overview

Chief Uplink is a web application at uplink.chiefloop.com that provides remote management and control of Chief instances. It serves as the central hub between Chief CLI daemons (`chief serve`) and browser clients, handling authentication, state caching, real-time relay, server provisioning, and team management.

The web app does zero AI work. All Claude Code execution happens on the user's machine or VPS. Chief connects outbound to the web app (like Plex or Tailscale), so no port forwarding or firewall configuration is needed.

This is an open source project. It is hosted at uplink.chiefloop.com but can be self-hosted.

## Tech Stack

- **Backend:** Laravel 13, Octane (FrankenPHP)
- **Frontend:** Vue 3, Inertia.js, Tailwind 4
- **Real-time (browser):** Laravel Reverb
- **Real-time (devices):** Raw WebSocket on `/ws/device` via Octane
- **Database:** MariaDB
- **Auth:** Custom (GitHub OAuth + email/password), Laravel Sanctum for API tokens
- **Code highlighting:** Shiki (Tokyo Night theme)
- **Typography:** Geist Sans + Geist Mono
- **Tests:** Pest (unit/integration), Vitest (Vue components), Playwright (browser)
- **CI:** GitHub Actions

## Architecture

```
Laravel 13 (Octane/FrankenPHP)
├── Inertia.js → Vue 3 SPA (all pages)
├── Reverb → Browser WebSocket (real-time updates)
├── /ws/device → Raw WebSocket (device protocol)
├── /api/* → REST API (browser commands, OAuth device flow endpoints)
├── Custom Auth (GitHub OAuth + email/password)
├── Teams (Owner/Member roles)
├── Server Provisioning (Hetzner/DO API)
└── MariaDB (state cache + app data)
```

### Key Layers

- **Auth layer:** Custom GitHub OAuth + email/password. No Laravel starter kit. Sanctum for API token management. GitHub OAuth includes incremental scope authorization for features like deploy key management.
- **Team layer:** Users belong to teams. All resources (devices, servers, projects, PRDs) are scoped to a team. Simple roles: Owner (full control) and Member (view everything, create PRDs, start runs, but can't manage devices/team/servers).
- **Device layer:** Raw WebSocket endpoint at `/ws/device`. `DeviceConnectionManager` tracks live connections in memory, validates against DB. Separate from Reverb — different protocol, different handler.
- **Broadcast layer:** Reverb for browser push. Private channels per team (`private-team.{teamId}`) for cross-device updates and per device (`private-device.{deviceId}`) for device-specific state. Note: the protocol spec references `private-user.{userId}` — this is superseded by team-scoped channels since teams are the access boundary.
- **Provisioning layer:** Hetzner Cloud and DigitalOcean API integration. Provisioning scripts for Debian. Server lifecycle management (reboot, resize, rebuild, destroy).

### Device Protocol

The device WebSocket protocol is defined in the separate protocol spec (`2026-03-21-uplink-network-protocol-design.md`). Key points relevant to the web app:

- Chief is the sole owner of truth. The database is a read-only cache.
- State flows one way: chief → server cache → browser.
- Commands flow the opposite: browser → REST API → server → device WebSocket → chief.
- Ephemeral messages (streaming output) are relayed to Reverb without storing.
- On device connect, chief sends `state.sync` which replaces the entire cache for that device.
- After `state.sync`, server drains any `pending_commands` (oldest first).

## Visual Design

### Design Tokens

**Typography:**
- UI font: Geist Sans (negative letter-spacing: -0.02em on headings)
- Mono font: Geist Mono (for data, metrics, status labels, code)

**Colors (Dark Mode — default):**
- Background: `#0a0a0f` (main), `#12131a` (card/panel), `#1a1b26` (elevated surface)
- Text: `#f0f6fc` (headings), `#c9d1d9` (body), `#8b949e` (secondary), `#484f58` (muted)
- Interactive: `#E5E5E5` (primary buttons, links — white-on-dark Vercel style)
- Borders: `#1a1b26` (1px solid)

**Colors (Light Mode):**
- Background: `#ffffff` (main), `#f6f8fa` (card/panel), `#ffffff` (elevated surface)
- Text: `#1f2328` (headings), `#424a53` (body), `#656d76` (secondary), `#8b949e` (muted)
- Interactive: `#1a1a1a` (primary buttons — dark-on-light, inverted Vercel style)
- Borders: `#d1d9e0` (1px solid)

**Shared across themes:**
- Brand mark: `#1D3F6A` (logo only, not for UI elements)
- Status: `#3fb950` (success/online), `#58a6ff` (in progress/info), `#f0b429` (running/warning), `#f85149` (error/offline)

**Theme switching:** Respects system preference (`prefers-color-scheme`) by default. User can override in settings (dark/light/system). Stored in localStorage + `users.theme_preference` in DB.

**Code highlighting:** Tokyo Night (dark mode), GitHub Light (light mode) via Shiki.

**Spacing & Shape:**
- Border radius: 4px (sharp, not rounded)
- Progress bars: 3px height
- Status dots: 6px diameter

**Branding:** Use existing Chief logo and mark assets from `assets/` directory. Colors and visual identity consistent with the VitePress docs site at chiefloop.com. Logo adapts to theme (navy mark on light, light mark on dark).

### Design Principles

- Mobile-first, dark mode default with light mode support
- Developer tool aesthetic (Linear/Vercel)
- Minimal, focused, single-purpose screens
- Monospace for data, sans-serif for prose
- No emojis in UI (use status dots and icons)

## Navigation

### Mobile (Bottom Tab Bar)

| Tab | Content |
|-----|---------|
| Home | Resume last context or activity feed |
| Devices | Device list → projects → PRDs → runs |
| Servers | Managed VPS list, provisioning |
| Settings | Account, team, preferences |

### Desktop (>768px)

Bottom tabs become a left sidebar with the same structure. More room for labels and secondary nav.

### Resume Last Context

When the user opens the app, they're returned to the last page they visited (stored in `users.last_visited_url`). If no previous visit, show an activity feed with recent events across all devices.

### Real-time Indicators

- Green/red dot on devices (online/offline)
- Animated pulse on actively running PRDs
- Toast notifications for run completions and errors
- Browser push notifications when tab is not active (on run completion)

## Pages & Features

### Authentication

**Login page:** GitHub OAuth button (primary) + email/password form (secondary). No starter kit — custom implementation.

**Registration:** Email, name, password, optional GitHub link. Creates a default team for the user (Owner role).

**GitHub OAuth scopes:** Basic profile at login. `admin:public_key` scope requested incrementally when the user first provisions a server and wants to auto-add a deploy key to GitHub.

### Empty States

Every empty state is instructional, not just a "nothing here" message.

**No devices connected (Home/Devices):**
- "Connect your first Chief server"
- Two cards: "Run locally" (shows `chief login` + `chief serve` terminal commands to copy) and "Launch a cloud server" (links to /servers/create)
- Brief explainer: "Chief runs on your machine — Uplink lets you control it from here"

**Device connected, no projects:**
- "This workspace is empty"
- "Clone a repo" button (triggers `cmd.project.clone`)
- Shows the workspace path on the device

**Project with no PRDs:**
- "Create your first PRD"
- Two paths: "Chat with Claude" (opens chat) and "Write it yourself" (opens editor)
- Brief explainer about what PRDs are

**PRD with no runs:**
- Shows PRD content preview
- Prominent "Start Run" button
- "Chief will work through each story autonomously"

**No managed servers:**
- "Launch a Chief server in the cloud"
- Provider cards (Hetzner, DigitalOcean) with logos
- "Bring your own API key"

**No team members:**
- "Invite your team"
- Invite form right in the empty state

### PRD Chat Interface

Full-screen chat, mobile-first. Modeled after iMessage.

**Layout:**
- Top bar: PRD name, back button, status badge (draft/ready), overflow menu
- Chat area: scrollable messages. User messages right-aligned (subtle white background `#ffffff10`), Claude responses left-aligned with markdown rendering (dark surface `#1a1b26`)
- Bottom: text input with send button, auto-grows for multi-line
- Claude's responses stream in token-by-token via Reverb (from `state.prd.chat.output`)

**Conversation flow:**
1. User taps "New PRD" → chat opens with system message "Describe what you want to build"
2. User types → sends `cmd.prd.create`
3. Claude responds with clarifying questions (streamed live)
4. Back and forth until Claude writes the `prd.md` file
5. Chat shows "PRD Created" card with "View PRD" button
6. User can continue refining via `cmd.prd.message`

**PRD Preview:** Accessible via button in chat. Rendered markdown with story list.

**Direct editing:** "Edit" button opens a CodeMirror markdown editor. Save sends `cmd.prd.update`. No Claude involved. Users can also create PRDs directly via the editor without chat.

**On desktop:** Chat stays full-width (not split) for consistent conversational feel.

### Run Monitoring

Modeled after the Chief TUI. Two modes.

**Summary view (default):**
- Top bar: PRD name, run status badge, elapsed time
- Story list: vertical list with status indicators (green check = done, blue dot = in progress, gray circle = pending, red x = failed)
- Active story card: expanded view showing title, priority, description, acceptance criteria, progress notes, iteration count
- Progress bar at bottom: "3/7 stories" with percentage
- Action buttons: Stop Run, View Diffs, View Log

**Live view (toggle):**
- Terminal-style scrolling view of Claude's output
- Dark background, Geist Mono, Tokyo Night syntax highlighting
- Auto-scrolls, tap to pause
- Tool usage shown inline: "Reading file.go", "Running tests"
- Ephemeral — streams via Reverb from `state.run.output`

**On desktop:** Story list becomes sidebar (like TUI), active story + live view in main panel.

**Notifications:** Browser push notification on `state.run.completed` when tab is not active. Summary card shows stories completed, time taken, result.

### Diff Viewer

GitHub-style code review experience.

**Layout:**
- File tree (collapsible on mobile): changed files with status icons (added/modified/deleted/renamed) and line counts (+12/-3)
- Diff panel: syntax-highlighted via Shiki
- View modes: unified (default on mobile), side-by-side (default on desktop)
- Per-story filtering: dropdown to show only changes from a specific story

**Mobile:** File tree as collapsible drawer at top. Unified diff only. Swipe between files. Pinch to zoom.

**Data flow:** Browser REST request → server sends `cmd.diffs.get` → chief responds with `state.diffs.response` → server caches (encrypted) and returns to browser.

### File Browser

- Directory tree navigation (from `cmd.files.list` → `state.files.list`)
- File content viewer with Shiki syntax highlighting (from `cmd.file.get` → `state.file.response`)
- Breadcrumb navigation for path
- Not cached — fetched on demand each time

### Server Provisioning & Management (Mini Forge)

#### Provisioning Flow

1. **Choose provider:** Hetzner Cloud or DigitalOcean. User enters API key (stored encrypted per team in `cloud_provider_credentials`).
2. **Configure:** Fetches available plans/regions from provider API. User picks region (with latency hints), size (vCPU/RAM/disk/price), server name, SSH key (paste, select saved, or generate new).
3. **Provision:** One click. Progress screen shows steps: Creating server → Waiting for boot → Running provisioning script → Installing Chief → Connecting to Uplink → Done.
4. **Server ready:** Redirects to server detail. Device appears in Devices tab automatically.

#### Provisioning Script (Debian)

- Updates system, installs essentials (git, curl, build-essential)
- Installs Claude Code CLI
- Installs Chief binary
- Creates `chief` user (non-root, sudo access)
- Configures `chief serve` as systemd service
- Runs `chief login` automatically using a pre-generated device token (no user SSH needed for chief auth)
- Generates SSH deploy key (`~/.ssh/chief_deploy_key`) for git operations
- Sets up swap, ufw firewall, fail2ban
- Creates workspace at `~/workspace` (`/home/chief/workspace`)
- Adds user's SSH public key to `~/.ssh/authorized_keys`

#### SSH Key Management

**User SSH key:** During provisioning, user can paste a public key, select from saved keys, or generate a new pair (private key shown once for download). Added to server for SSH access.

**Deploy key for Git:** After provisioning, the web UI shows the generated deploy public key with a "Copy" button. If user authenticated via GitHub OAuth, a one-click "Add to GitHub" button calls the GitHub API (using incremental `admin:public_key` scope) to add the key directly.

#### Server Detail Page (Management)

- Status card: online/offline, uptime, IP address, provider, region, size
- Resource monitoring: CPU, RAM, disk usage (polled from cloud provider API — Hetzner Metrics / DO Monitoring)
- Actions: Reboot, Resize, Rebuild, Destroy (with confirmation modals)
- Service management: Start/Stop/Restart chief systemd service, view status
- Server logs: system logs and chief service logs
- SSH access info: IP, username, copyable SSH command
- Update Chief: one-click update to latest version
- Deploy key: view/copy public key, add to GitHub button

### Team Management

- **Team settings page:** team name, member list with roles
- **Invite flow:** enter email → sends invitation with token → recipient creates account or logs in → joins team
- **Roles:** Owner (full control including device/server/team management) and Member (view all, create PRDs, start runs)
- **All resources scoped to team:** devices, servers, projects, PRDs, runs

### Landing Page

Not a sales pitch — an explanation. Free service.

**Structure:**
- Hero: "Remote control for Chief" — one-sentence description, mobile UI screenshot
- What it is: 3-4 short paragraphs. "Chief runs on your machine. Uplink lets you manage it from anywhere."
- How it works: 3-step visual (Install Chief → Run `chief serve` → Open Uplink)
- Features: icon grid showing key capabilities with clear descriptions
- Self-hosting: brief section noting open source, link to README
- Sign up / Sign in: buttons at top and bottom

**Tone:** matter-of-fact, developer-to-developer. No "supercharge" or "10x" language.

**Branding:** Uses Chief logo from `assets/logo.svg`, consistent with chiefloop.com docs site.

## Database Schema

```sql
-- Users & Auth
users (id, name, email, password?, github_id?, github_token?, avatar_url?, last_visited_url?, theme_preference enum[dark,light,system] default system, created_at, updated_at)

-- Teams
teams (id, name, owner_id FK users, created_at, updated_at)
team_user (team_id, user_id, role enum[owner,member], created_at)
team_invitations (id, team_id, email, token, accepted_at?, created_at, updated_at)

-- Cloud Providers & Servers
cloud_provider_credentials (id, team_id, provider enum[hetzner,digitalocean], api_key encrypted, name, created_at, updated_at)
managed_servers (id, team_id, cloud_provider_credential_id, provider_server_id, name, ip_address, region, size, status enum[provisioning,active,stopped,error], deploy_public_key, deploy_private_key encrypted, provisioned_at?, created_at, updated_at)
ssh_keys (id, team_id, name, public_key, created_at)

-- Devices (from chief serve)
devices (id, team_id, managed_server_id? FK managed_servers, name, os, arch, chief_version, access_token hashed, refresh_token encrypted, last_seen_at, connected bool, created_at, updated_at)
device_codes (id, device_code, user_code, user_id?, team_id, expires_at, approved_at?, created_at)

-- Cached State (from device protocol)
projects (id, device_id, path, name, git_remote, git_branch, git_status, last_commit_hash, last_commit_message, last_commit_at, created_at, updated_at)
prds (id, project_id, device_id, title, status, content encrypted, progress encrypted, chat_history encrypted, session_id, created_at, updated_at)
runs (id, prd_id, device_id, status, result, error_message, story_index, started_at, completed_at, created_at, updated_at)
pending_commands (id, device_id, user_id, type, payload encrypted, status enum[pending,delivered,failed], created_at, delivered_at?)
cached_diffs (id, project_id, story_id?, diffs encrypted, fetched_at, created_at, updated_at)
```

**Key relationships:**
- Everything is team-scoped
- A device belongs to a team, optionally linked to a managed_server
- Projects/PRDs/runs cascade from devices
- Content fields (PRD body, progress, chat history, diffs) encrypted at rest
- Metadata (status, timestamps, names) plaintext for querying

## Testing Strategy

### Unit Tests (Pest)

- Models, services, DTOs
- Protocol message validation (against contract JSON schemas)
- DeviceConnectionManager
- Server provisioning logic
- Auth flows (GitHub OAuth, email/password, token refresh)
- Team authorization (Owner vs Member permissions)

### Integration Tests (Pest)

- REST API endpoints (authenticated, authorized, team-scoped)
- Device WebSocket message handling (connect, state.sync, commands, ack/error)
- Reverb broadcasting (events fire correctly on state changes)
- GitHub OAuth flow with mocked GitHub API
- Cloud provider API integration with mocked Hetzner/DO APIs
- Command queue (pending → delivered on reconnect)

### Browser Tests (Playwright)

- Critical user flows: login, connect device, create PRD via chat, start run, view diffs
- Mobile viewport testing (375px width)
- Empty state screens render correctly
- Real-time updates (WebSocket events reflected in UI)
- Server provisioning flow

### Vue Component Tests (Vitest)

- Individual component rendering and interaction
- Chat message rendering (markdown, streaming)
- Story list state management
- Diff viewer file navigation

### CI (GitHub Actions)

- PHP unit + integration tests (Pest)
- JS/Vue component tests (Vitest)
- Playwright browser tests
- Laravel Pint (PHP code style)
- ESLint + TypeScript checking
- Contract schema validation (PHP validates against same `contract/schemas/` as Go)

## Inertia Page Structure

```
/login                           → Login (GitHub OAuth + email)
/register                        → Registration
/                                → Home (resume last context or activity feed)
/devices                         → Device list with online/offline status
/devices/{id}                    → Device detail (projects, PRDs, runs)
/devices/{id}/projects/{pid}     → Project detail (PRDs, git status)
/prds/{id}                       → PRD detail (preview, edit, start run)
/prds/{id}/chat                  → PRD chat interface
/runs/{id}                       → Run detail (TUI-style summary)
/runs/{id}/live                  → Live Claude output stream
/runs/{id}/diffs                 → GitHub-style diff viewer
/files/{deviceId}/{path*}        → File browser
/servers                         → Managed servers list
/servers/create                  → Provision new server
/servers/{id}                    → Server management
/settings                        → User settings
/settings/team                   → Team management (invite, roles)
```

## README & Self-Hosting

The project README should cover:
- What Uplink is (brief description)
- Screenshots of key screens
- Quick start for the hosted version (sign up, install chief, connect)
- Self-hosting instructions: Docker Compose setup, environment variables, database setup, Reverb configuration, reverse proxy (Caddy/nginx)
- Development setup: clone, install, migrate, seed, run
- Contributing guidelines
- License

## Out of Scope (Separate Projects)

- **Ansible provisioner** for uplink.chiefloop.com infrastructure (separate repo — Docker, MariaDB, Redis, backups, firewalls, SSL, deployment)
- **One-click VPS deployment** of the Uplink web app itself (Hetzner/DO for the uplink server, not chief workers)
- **Billing/plans** — free for now, add later
- **Mobile native app** — browser-based PWA is sufficient
