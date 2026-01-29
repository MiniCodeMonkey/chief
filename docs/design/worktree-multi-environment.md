# Git Worktree & Multi-Environment Support for Chief

## Executive Summary

This document outlines the architecture and UX design for adding git worktree support to Chief, enabling simultaneous work on multiple PRDs across isolated development environments. The key challenge is ensuring each worktree can run its own dev stack (e.g., docker-compose) without port collisions or state interference.

---

## Problem Statement

Currently, Chief operates within a single working directory. Users who want to work on multiple PRDs simultaneously must:
1. Manually manage separate git worktrees
2. Manually configure unique ports for each dev environment
3. Context-switch between terminal sessions
4. Risk state collisions when running parallel Ralph loops

**Goal**: Enable Chief to orchestrate multiple worktrees with fully isolated dev environments, allowing the Ralph loop to test and QA each PRD independently.

---

## Architecture Overview

### Current State

```
project/
├── .chief/
│   └── prds/
│       ├── feature-a/
│       │   ├── prd.md
│       │   ├── prd.json
│       │   └── log.txt
│       └── feature-b/
│           └── ...
├── src/
└── ...
```

### Proposed State

```
project/                          # Main worktree
├── .chief/
│   ├── config.json              # NEW: Global config
│   └── prds/
│       └── feature-a/           # PRD tied to main worktree
│           ├── prd.md
│           ├── prd.json
│           ├── log.txt
│           └── worktree.json    # NEW: Worktree binding
│
├── .git/
│   └── worktrees/
│       └── feature-b/           # Git's worktree metadata

project-feature-b/                # Linked worktree (sibling directory)
├── .chief/
│   └── prds/
│       └── feature-b/           # PRD isolated to this worktree
│           ├── prd.md
│           ├── prd.json
│           ├── log.txt
│           ├── worktree.json
│           └── env.json         # NEW: Environment config
├── src/
└── ...
```

---

## Core Design Decisions

### Decision 1: PRD-Worktree Binding Strategy

**Options Considered:**

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| A. Shared PRDs | All worktrees share `.chief/` from main | Simple, unified view | State collisions, complex syncing |
| B. Independent PRDs | Each worktree has its own `.chief/` | Full isolation, simple | No cross-worktree visibility |
| C. **Hybrid** | Coordinator in main, execution in worktrees | Best of both worlds | More complexity |

**Recommendation: Option B (Independent PRDs)** with a lightweight coordinator mode.

Rationale:
- Matches git worktree mental model (each is independent)
- Avoids complex state synchronization
- Chief's current `.chief/prds/` structure already supports this
- Users can run `chief` in any worktree independently

### Decision 2: Development Environment Isolation

**The Port Collision Problem:**

When running `docker-compose up` in multiple worktrees, services clash on ports:
```
Worktree A: postgres:5432, redis:6379, app:3000
Worktree B: postgres:5432 ❌ CONFLICT
```

**Proposed Solution: Dynamic Port Allocation with Environment Profiles**

```go
// internal/env/profile.go
type EnvironmentProfile struct {
    WorktreeID   string            `json:"worktreeId"`
    PRDName      string            `json:"prdName"`
    PortOffset   int               `json:"portOffset"`   // e.g., 0, 100, 200
    Ports        map[string]int    `json:"ports"`        // service -> port
    EnvVars      map[string]string `json:"envVars"`      // injected env vars
    ComposeFile  string            `json:"composeFile"`  // generated compose override
}
```

**Port Allocation Strategy:**

```
Base ports (from docker-compose.yml):
  postgres: 5432
  redis: 6379
  app: 3000

Worktree 0 (main):     5432, 6379, 3000
Worktree 1 (feature-a): 5532, 6479, 3100  (+100 offset)
Worktree 2 (feature-b): 5632, 6579, 3200  (+200 offset)
```

**Implementation Approach:**

1. **Auto-detect compose files**: Scan for `docker-compose.yml`, `compose.yml`, `docker-compose.*.yml`
2. **Parse port mappings**: Extract all `ports:` definitions
3. **Generate override file**: Create `.chief/docker-compose.override.yml` with offset ports
4. **Inject environment**: Set `COMPOSE_FILE` to include the override
5. **Track allocations**: Store in `.chief/env.json` to prevent conflicts

```yaml
# .chief/docker-compose.override.yml (auto-generated)
version: "3.8"
services:
  postgres:
    ports:
      - "5532:5432"  # Host port offset, container port unchanged
  redis:
    ports:
      - "6479:6379"
  app:
    ports:
      - "3100:3000"
    environment:
      - DATABASE_URL=postgres://localhost:5532/app
      - REDIS_URL=redis://localhost:6479
```

### Decision 3: TUI-First Worktree Management

**Design Philosophy**: Chief is a TUI-first application. Rather than adding CLI subcommands
(`chief worktree new/list/switch`), all worktree operations are integrated directly into
the existing TUI flows.

**Key Integration Points:**

1. **PRD Creation Flow** - When creating a new PRD, offer worktree creation inline
2. **Dashboard View** - Shows all worktrees with their PRD status at a glance
3. **Picker Enhancement** - The existing PRD picker becomes a worktree-aware picker
4. **Hotkeys** - Quick actions for environment control without leaving dashboard

**No New CLI Commands** - Everything happens in the TUI:
- Creating worktrees: Part of new PRD flow or via `[N]` from dashboard
- Listing worktrees: The dashboard IS the list
- Switching: Select from picker, Chief spawns new terminal in that worktree
- Environment control: Inline in dashboard with `[E]` hotkey

---

## UX/UI Design

### Design Philosophy: TUI-Native Worktree Management

Chief is fundamentally a TUI application. Rather than bolting on CLI subcommands, worktree
management is woven into the existing TUI patterns:

- **Dashboard IS the worktree list** - No separate "worktree list" command needed
- **PRD creation includes worktree creation** - Natural flow, not a separate step
- **Picker is worktree-aware** - Shows worktrees, not just PRDs
- **Environment control via hotkeys** - No context switching to terminal

### TUI Flow Diagrams

#### Flow 1: New PRD with Worktree (from Dashboard)

```
┌─ Dashboard ─────────────────────────────────────────────────────────┐
│                                                                     │
│  PRDs                                                               │
│  ────                                                               │
│  ● feature-a          ███████████░░░░  73% (8/11)     Running      │
│                                                                     │
│                                                                     │
│                                                                     │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [N] New PRD    [S] Start/Stop    [L] Logs    [E] Env    [?] Help  │
└─────────────────────────────────────────────────────────────────────┘

User presses [N]
        ↓

┌─ New PRD ───────────────────────────────────────────────────────────┐
│                                                                     │
│  What would you like to build?                                      │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ Add user authentication with OAuth support                   │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  Create in dedicated worktree?                                      │
│                                                                     │
│    ● Yes - Isolated branch & environment (recommended)              │
│    ○ No  - Use current directory                                    │
│                                                                     │
│  Worktree location:                                                 │
│    ../my-project-add-user-auth                                      │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  Enter: Create    Tab: Toggle worktree    Esc: Cancel              │
└─────────────────────────────────────────────────────────────────────┘

User presses Enter
        ↓

┌─ Creating... ───────────────────────────────────────────────────────┐
│                                                                     │
│  ✓ Generated PRD from description                                   │
│  ✓ Created branch: feature/add-user-auth                           │
│  ✓ Created worktree: ../my-project-add-user-auth                   │
│  ● Detecting environment...                                         │
│    Found: docker-compose.yml                                        │
│  ✓ Allocated ports (offset +100): app:3100, db:5532, redis:6479    │
│  ✓ Generated docker-compose.override.yml                           │
│                                                                     │
│  Ready! Opening in new terminal...                                  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

        ↓ (spawns new terminal in worktree, runs `chief`)

┌─ Dashboard ─────────────────────────────────────────────────────────┐
│  ~/my-project-add-user-auth                    Ports: 3100 5532    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  PRDs                                                               │
│  ────                                                               │
│  ● add-user-auth      ░░░░░░░░░░░░░░░   0% (0/7)        Ready      │
│                                                                     │
│  Environment: Stopped                                               │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [S] Start    [E] Start Env    [L] Logs    [?] Help                │
└─────────────────────────────────────────────────────────────────────┘
```

#### Flow 2: Multi-Worktree Dashboard (Coordinator Mode)

When running `chief --all` or pressing `[A]` for "All Worktrees":

```
┌─ All Worktrees ─────────────────────────────────────────────────────┐
│                                                                     │
│  WORKTREES (3)                                              [A]ll  │
│  ─────────────                                                      │
│                                                                     │
│  ┌─ main ─────────────────────────────────────────────────────────┐│
│  │  ~/projects/my-project                                Stopped  ││
│  │  No active PRD                                                 ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
│  ┌─ feature-a ◉ ──────────────────────────────────────────────────┐│
│  │  ~/projects/my-project-feature-a              Ports: 3100 5532 ││
│  │  add-auth         ███████████░░░░  73% (8/11)   Iter 12  ▶ RUN ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
│  ┌─ feature-b ◐ ──────────────────────────────────────────────────┐│
│  │  ~/projects/my-project-feature-b              Ports: 3200 5632 ││
│  │  fix-payments     █████░░░░░░░░░░  36% (4/11)   Iter 5   ⏸ PAU ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  ↑/↓ Select   Enter: Focus   [N] New   [E] Env All   [?] Help     │
└─────────────────────────────────────────────────────────────────────┘

User selects feature-a and presses Enter
        ↓
(Spawns terminal in that worktree OR shows focused view below)

┌─ feature-a ─────────────────────────────────────────────────────────┐
│  ~/my-project-feature-a                        Ports: 3100 5532    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  add-user-auth       ███████████░░░░  73% (8/11 stories)           │
│                                                                     │
│  Current: US-004 "Password reset flow"                              │
│  Iteration: 12                                                      │
│                                                                     │
│  ┌─ Log ──────────────────────────────────────────────────────────┐│
│  │ [12:34:56] Testing password reset email...                     ││
│  │ [12:35:02] ✓ Email sent successfully                           ││
│  │ [12:35:03] Testing reset token validation...                   ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [S] Stop   [P] Pause   [L] Full Log   [A] All Worktrees   [?]    │
└─────────────────────────────────────────────────────────────────────┘
```

#### Flow 3: Inline Environment Control

No separate "environment panel" - environment status and controls are integrated
directly into the dashboard:

```
┌─ Dashboard ─────────────────────────────────────────────────────────┐
│  ~/my-project-feature-a                                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  add-user-auth       ███████████░░░░  73% (8/11)      ▶ Running    │
│                                                                     │
│  ┌─ Environment ──────────────────────────────────────────────────┐│
│  │  ● app        localhost:3100  ✓ healthy                        ││
│  │  ● postgres   localhost:5532  ✓ healthy                        ││
│  │  ● redis      localhost:6479  ✓ healthy                        ││
│  │                                                    [E] Control ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
│  ┌─ Current Story ────────────────────────────────────────────────┐│
│  │  US-004: Password reset flow                                   ││
│  │  Iteration 12 • Started 2m ago                                 ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [S] Stop   [P] Pause   [E] Env   [L] Logs   [A] All   [?] Help   │
└─────────────────────────────────────────────────────────────────────┘

User presses [E]
        ↓

┌─ Environment Control ───────────────────────────────────────────────┐
│                                                                     │
│  Environment: Running (2h 34m)                     Offset: +100    │
│                                                                     │
│  SERVICES                                                           │
│  ────────                                                           │
│  [1] ● app        3100:3000    ✓ healthy     node:20              │
│  [2] ● postgres   5532:5432    ✓ healthy     postgres:15-alpine   │
│  [3] ● redis      6479:6379    ✓ healthy     redis:7-alpine       │
│  [4] ● worker     -            ✓ running     node:20              │
│                                                                     │
│  QUICK ACTIONS                                                      │
│  ─────────────                                                      │
│  [R] Restart All    [D] Down (stop & remove)    [U] Up (start)    │
│  [L] Logs (all)     [1-4] Logs (service)        [O] Open in browser│
│                                                                     │
│  URLs                                                               │
│  ────                                                               │
│  App:      http://localhost:3100                                   │
│  Database: postgres://localhost:5532/app                           │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  Esc: Back    R: Restart    D: Down    L: Logs                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### Flow 4: Worktree Picker (Enhanced PRD Picker)

The existing PRD picker becomes worktree-aware. Accessed via Tab or when
multiple worktrees exist:

```
┌─ Select Worktree ───────────────────────────────────────────────────┐
│                                                                     │
│  > ┌──────────────────────────────────────────────────────────────┐│
│    │ ● feature-a        ▶ Running                                 ││
│    │   add-user-auth    ███████████░░░  73%        Ports: 3100   ││
│    └──────────────────────────────────────────────────────────────┘│
│                                                                     │
│    ┌──────────────────────────────────────────────────────────────┐│
│    │ ◐ feature-b        ⏸ Paused                                  ││
│    │   fix-payments     █████░░░░░░░░░  36%        Ports: 3200   ││
│    └──────────────────────────────────────────────────────────────┘│
│                                                                     │
│    ┌──────────────────────────────────────────────────────────────┐│
│    │ ○ main             Stopped                                   ││
│    │   (no active PRD)                                            ││
│    └──────────────────────────────────────────────────────────────┘│
│                                                                     │
│    ┌──────────────────────────────────────────────────────────────┐│
│    │ + Create new worktree...                                     ││
│    └──────────────────────────────────────────────────────────────┘│
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  ↑/↓ Navigate   Enter: Open in terminal   Del: Remove   Esc: Back │
└─────────────────────────────────────────────────────────────────────┘
```

### Hotkey Reference

| Key | Context | Action |
|-----|---------|--------|
| `N` | Dashboard | New PRD (with worktree option) |
| `A` | Dashboard | Show all worktrees (coordinator mode) |
| `E` | Dashboard | Environment control panel |
| `Tab` | Dashboard | Worktree picker |
| `S` | Dashboard | Start/Stop Ralph loop |
| `P` | Dashboard | Pause Ralph loop |
| `L` | Dashboard | View full log |
| `O` | Env Panel | Open app in browser |
| `R` | Env Panel | Restart all services |
| `D` | Env Panel | Down (stop & remove containers) |
| `U` | Env Panel | Up (start containers) |
| `1-9` | Env Panel | View logs for specific service |

### Terminal Spawning Behavior

When selecting a different worktree, Chief needs to open it. Options:

**Option A: Spawn New Terminal (Recommended)**
- Uses `$TERMINAL` env var or detects (iTerm2, Terminal.app, gnome-terminal, etc.)
- Spawns: `$TERMINAL -e "cd /path/to/worktree && chief"`
- Current Chief instance stays open
- User has multiple terminals, one per worktree

**Option B: Print Instructions**
- Shows: "Run `cd ../my-project-feature-a && chief` in another terminal"
- Less magical, more explicit
- Good fallback when terminal detection fails

**Option C: tmux/Screen Integration**
- If running in tmux, create new window: `tmux new-window -c /path/to/worktree chief`
- Best UX for tmux users
- Detect via `$TMUX` env var

---

## Implementation Details

### New Package Structure

```
internal/
├── worktree/
│   ├── worktree.go      # Core worktree operations
│   ├── detector.go      # Find existing worktrees
│   ├── coordinator.go   # Cross-worktree communication
│   └── types.go         # Data structures
│
├── env/
│   ├── profile.go       # Environment profiles
│   ├── ports.go         # Port allocation
│   ├── compose.go       # Docker compose integration
│   ├── detector.go      # Detect compose files, Dockerfile, etc.
│   └── manager.go       # Start/stop environments
```

### Key Data Structures

```go
// internal/worktree/types.go
package worktree

type Worktree struct {
    ID          string    `json:"id"`          // Unique identifier
    Path        string    `json:"path"`        // Absolute path to worktree
    Branch      string    `json:"branch"`      // Git branch name
    PRDName     string    `json:"prdName"`     // Associated PRD (if any)
    IsMain      bool      `json:"isMain"`      // Is this the main worktree?
    CreatedAt   time.Time `json:"createdAt"`
    PortOffset  int       `json:"portOffset"`  // Allocated port offset
}

type WorktreeBinding struct {
    WorktreeID  string `json:"worktreeId"`
    PRDName     string `json:"prdName"`
    BoundAt     time.Time `json:"boundAt"`
}
```

```go
// internal/env/profile.go
package env

type EnvironmentType string

const (
    EnvTypeDockerCompose EnvironmentType = "docker-compose"
    EnvTypeDocker        EnvironmentType = "docker"
    EnvTypeNative        EnvironmentType = "native"
    EnvTypeCustom        EnvironmentType = "custom"
)

type EnvironmentProfile struct {
    Type         EnvironmentType   `json:"type"`
    WorktreeID   string            `json:"worktreeId"`
    PortOffset   int               `json:"portOffset"`
    PortMappings map[string]Port   `json:"portMappings"`
    EnvVars      map[string]string `json:"envVars"`
    Status       EnvStatus         `json:"status"`
    StartedAt    *time.Time        `json:"startedAt,omitempty"`
}

type Port struct {
    Service       string `json:"service"`
    ContainerPort int    `json:"containerPort"`
    HostPort      int    `json:"hostPort"`      // Original
    MappedPort    int    `json:"mappedPort"`    // With offset applied
}

type EnvStatus string

const (
    EnvStatusStopped  EnvStatus = "stopped"
    EnvStatusStarting EnvStatus = "starting"
    EnvStatusRunning  EnvStatus = "running"
    EnvStatusError    EnvStatus = "error"
)
```

### Port Allocation Algorithm

```go
// internal/env/ports.go
package env

const (
    PortOffsetIncrement = 100
    MaxWorktrees        = 50  // Supports offsets 0-4900
)

type PortAllocator struct {
    configPath string
    mu         sync.Mutex
}

// AllocateOffset finds the next available port offset
func (pa *PortAllocator) AllocateOffset() (int, error) {
    pa.mu.Lock()
    defer pa.mu.Unlock()

    allocated := pa.loadAllocatedOffsets()

    for offset := 0; offset < MaxWorktrees*PortOffsetIncrement; offset += PortOffsetIncrement {
        if !allocated[offset] {
            pa.saveOffset(offset)
            return offset, nil
        }
    }

    return 0, fmt.Errorf("no available port offsets (max %d worktrees)", MaxWorktrees)
}

// ApplyOffset transforms a base port with the worktree's offset
func ApplyOffset(basePort, offset int) int {
    return basePort + offset
}
```

### Docker Compose Integration

```go
// internal/env/compose.go
package env

// GenerateOverride creates a docker-compose.override.yml with offset ports
func GenerateOverride(composePath string, offset int) (string, error) {
    // 1. Parse original compose file
    config, err := parseComposeFile(composePath)
    if err != nil {
        return "", err
    }

    // 2. Extract and transform port mappings
    override := ComposeOverride{
        Version:  config.Version,
        Services: make(map[string]ServiceOverride),
    }

    for name, service := range config.Services {
        if len(service.Ports) > 0 {
            override.Services[name] = ServiceOverride{
                Ports: transformPorts(service.Ports, offset),
            }
        }
    }

    // 3. Generate environment variables for service discovery
    // (so app can find postgres at the new port)

    // 4. Write override file
    overridePath := filepath.Join(filepath.Dir(composePath), ".chief", "docker-compose.override.yml")
    return overridePath, writeComposeOverride(overridePath, override)
}
```

### Claude Prompt Injection for Environment Awareness

When Ralph executes tests, Chief should inject environment context:

```go
// internal/loop/loop.go (modified)

func (l *Loop) buildPrompt(story *prd.UserStory) string {
    prompt := l.basePrompt

    // Inject environment context if available
    if l.envProfile != nil {
        prompt += fmt.Sprintf(`

## Development Environment

This PRD is running in worktree: %s
Port mappings for this environment:
%s

When running tests or accessing services, use these ports:
- App: http://localhost:%d
- Database: postgres://localhost:%d/app
- Redis: redis://localhost:%d

Do NOT use hardcoded default ports (3000, 5432, 6379) as they may conflict with other worktrees.
`,
            l.envProfile.WorktreeID,
            formatPortMappings(l.envProfile.PortMappings),
            l.envProfile.PortMappings["app"].MappedPort,
            l.envProfile.PortMappings["postgres"].MappedPort,
            l.envProfile.PortMappings["redis"].MappedPort,
        )
    }

    return prompt
}
```

---

## Gotchas & Edge Cases

### 1. Port Exhaustion

**Problem**: Running many worktrees could exhaust available ports.

**Mitigation**:
- Track allocated offsets in `.chief/config.json` (in main worktree)
- Release offsets when worktrees are deleted via TUI
- Show warning in environment panel when approaching limit (40+ worktrees)
- Advanced: Allow manual offset override in new PRD dialog (hidden behind "Advanced" toggle)

### 2. Orphaned Environments

**Problem**: User deletes worktree directory manually, leaving docker containers running.

**Mitigation**:
- Use unique container names: `chief-{worktree-id}-{service}`
- In coordinator view (`[A]`), detect and show orphaned containers
- "Clean up orphaned environments" option in coordinator view
- On startup, check for orphaned Chief containers and offer to stop them

```go
// Container naming convention
containerName := fmt.Sprintf("chief-%s-%s", worktreeID[:8], serviceName)
```

### 3. Shared Database State

**Problem**: Multiple worktrees might need different database schemas/data.

**Mitigation**:
- Each worktree gets its own database volume
- Volume naming: `chief-{worktree-id}-{service}-data`
- In environment panel: "Seed from..." option to copy data from another worktree's volume

### 4. Resource Limits

**Problem**: Running 5+ full dev environments simultaneously could exhaust RAM/CPU.

**Mitigation**:
- Display resource usage in coordinator view footer
- Warn in environment panel when starting: "4 environments already running (~8GB RAM)"
- "Pause" option in environment panel to stop containers without removing volumes
- Auto-pause inactive worktrees after configurable timeout (shown in settings/config)

### 5. Network Conflicts

**Problem**: Docker networks might conflict across worktrees.

**Mitigation**:
- Create isolated networks per worktree: `chief-{worktree-id}-network`
- Inter-service communication uses container names, not localhost

### 6. File Watching Across Worktrees

**Problem**: fsnotify might have issues with symlinked files in worktrees.

**Mitigation**:
- Use absolute paths resolved through `filepath.EvalSymlinks()`
- Test worktree scenarios explicitly

### 7. Git Operations During Tests

**Problem**: Ralph might try to checkout branches, disrupting the worktree.

**Mitigation**:
- Inject prompt guidance: "You are in a dedicated worktree. Do not switch branches."
- Detect and warn if branch changes unexpectedly
- Consider `git worktree lock` during active loops

### 8. Compose File Variations

**Problem**: Projects use different compose file names and structures.

**Mitigation**:
- Auto-detect: `docker-compose.yml`, `compose.yml`, `docker-compose.yaml`, `compose.yaml`
- Support `COMPOSE_FILE` env var
- Allow explicit config: `chief.yaml` with `compose_file: docker/dev.yml`

### 9. Non-Docker Environments

**Problem**: Not all projects use Docker.

**Mitigation**:
- Detect environment type automatically
- Support native processes with port allocation
- Extensible `EnvironmentManager` interface

```go
type EnvironmentManager interface {
    Detect(projectPath string) (EnvironmentType, error)
    Start(profile *EnvironmentProfile) error
    Stop(profile *EnvironmentProfile) error
    Status(profile *EnvironmentProfile) (EnvStatus, error)
    Ports(profile *EnvironmentProfile) ([]Port, error)
}
```

### 10. Windows Compatibility

**Problem**: Git worktrees and Docker behave differently on Windows.

**Mitigation**:
- Use `filepath` package consistently
- Test on Windows with WSL2 Docker
- Document Windows-specific limitations

---

## Configuration

### New Configuration File: `.chief/config.json`

```json
{
  "version": 1,
  "worktrees": {
    "autoCreate": true,
    "defaultLocation": "../",
    "branchPrefix": "feature/"
  },
  "environment": {
    "type": "auto",
    "portOffsetIncrement": 100,
    "autoStart": false,
    "pauseInactiveAfter": "30m"
  },
  "allocatedOffsets": {
    "abc123": 0,
    "def456": 100,
    "ghi789": 200
  }
}
```

### Per-PRD Environment Config: `.chief/prds/{name}/env.json`

```json
{
  "worktreeId": "def456",
  "portOffset": 100,
  "portMappings": {
    "app": {"container": 3000, "host": 3100},
    "postgres": {"container": 5432, "host": 5532},
    "redis": {"container": 6379, "host": 6479}
  },
  "envVars": {
    "DATABASE_URL": "postgres://localhost:5532/app",
    "REDIS_URL": "redis://localhost:6479",
    "APP_URL": "http://localhost:3100"
  },
  "composeOverride": ".chief/docker-compose.override.yml",
  "status": "running",
  "startedAt": "2024-01-15T10:00:00Z"
}
```

---

## Migration Path

### Phase 1: Foundation (Non-Breaking)
1. Add worktree detection (`git worktree list` parsing)
2. Add `.chief/config.json` support
3. Show worktree path in dashboard header (read-only awareness)
4. Store port offset in PRD metadata

### Phase 2: Environment Integration
1. Auto-detect docker-compose/compose.yml
2. Implement port allocation algorithm
3. Generate docker-compose.override.yml
4. Add environment status widget to dashboard
5. Add `[E]` hotkey for environment control panel
6. Inject environment context into Ralph prompts

### Phase 3: Worktree Creation Flow
1. Enhance `[N]` new PRD flow with worktree toggle
2. Implement `git worktree add` integration
3. Terminal spawning for new worktree (iTerm2, gnome-terminal, tmux)
4. Enhance PRD picker to show worktrees

### Phase 4: Coordinator Mode
1. Add `[A]` all-worktrees view
2. Cross-worktree status aggregation (read from sibling `.chief/` dirs)
3. Quick-switch between worktrees
4. Global environment controls (stop all, resource usage)

### Phase 5: Polish
1. Auto-pause inactive environments (configurable timeout)
2. Orphaned container cleanup
3. Environment seeding from main worktree
4. Windows/WSL2 testing & edge cases

---

## Success Metrics

1. **Isolation**: Two worktrees running simultaneously with zero port conflicts
2. **UX**: Creating a new worktree + PRD in < 30 seconds
3. **Reliability**: Ralph can run tests in each worktree without interference
4. **Resource efficiency**: Pausing inactive environments reduces memory usage
5. **Discoverability**: Users can see all worktree status from any worktree

---

## Open Questions

1. **Should Chief support a "coordinator mode"** that shows all worktrees from a single TUI instance?
   - Pro: Single pane of glass
   - Con: Complexity, potential for cross-worktree interference

2. **How should secrets/env files be handled across worktrees?**
   - Copy on worktree creation?
   - Symlink to main?
   - User responsibility?

3. **Should we support Kubernetes/Skaffold** as an environment type?
   - Namespace-per-worktree instead of port offsets
   - More complex but better for k8s-native projects

4. **What about monorepos with multiple services?**
   - Each service might have its own compose file
   - Need to detect and orchestrate multiple environments

---

## Appendix: User Journeys

### Journey 1: First-Time Worktree User (All in TUI)

```
User runs: chief

┌─ Dashboard ─────────────────────────────────────────────────────────┐
│  ~/my-project                                                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  No PRDs yet. Press [N] to create one.                             │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

User presses [N]

┌─ New PRD ───────────────────────────────────────────────────────────┐
│                                                                     │
│  What would you like to build?                                      │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ Add user authentication with OAuth support_                  │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ ● Create in dedicated worktree (isolated branch & env)      │   │
│  │ ○ Use current directory                                     │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  Location: ../my-project-add-user-auth                             │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

User presses Enter

┌─ Creating ──────────────────────────────────────────────────────────┐
│                                                                     │
│  ✓ Generated PRD: 7 user stories                                   │
│  ✓ Created branch: feature/add-user-auth                           │
│  ✓ Created worktree: ../my-project-add-user-auth                   │
│  ✓ Detected: docker-compose.yml                                    │
│  ✓ Allocated ports: app→3100, postgres→5532, redis→6479           │
│                                                                     │
│  Opening new terminal...                                            │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

(New terminal opens in worktree with Chief running)

┌─ Dashboard ─────────────────────────────────────────────────────────┐
│  ~/my-project-add-user-auth                    Ports: 3100 5532    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  add-user-auth       ░░░░░░░░░░░░░░░   0% (0/7)          Ready     │
│                                                                     │
│  Environment: Stopped                                               │
│  Press [E] to start, or [S] to start loop (will auto-start env)   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Journey 2: Checking All Worktrees (Coordinator View)

```
User runs: chief (in any worktree)
User presses [A] for "All Worktrees"

┌─ All Worktrees ─────────────────────────────────────────────────────┐
│                                                                     │
│  ┌─ main ───────────────────────────────────────────── Stopped ───┐│
│  │  ~/projects/my-project                                         ││
│  │  (no active PRD)                                               ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
│  ┌─ feature-a ─────────────────────────────── Ports: 3100 5532 ───┐│
│  │  ~/projects/my-project-add-user-auth                           ││
│  │  add-user-auth    ███████░░░░░░░░  55% (4/7)    ▶ Running i12  ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
│> ┌─ feature-b ─────────────────────────────── Ports: 3200 5632 ───┐│
│  │  ~/projects/my-project-fix-payments                            ││
│  │  fix-payments     ████████████░░░  90% (9/10)   ⏸ Paused  i15  ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
│  Resources: 2 envs running • ~4.2 GB RAM • 12% CPU                 │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  ↑↓ Select    Enter: Open terminal    [N] New    [E] Stop all env │
└─────────────────────────────────────────────────────────────────────┘
```

### Journey 3: Quick Context Switch

```
User is in feature-a worktree, presses Tab

┌─ Switch Worktree ───────────────────────────────────────────────────┐
│                                                                     │
│    main              (no PRD)                         Stopped      │
│  > feature-a ●       add-user-auth      55%          Running       │
│    feature-b ◐       fix-payments       90%          Paused        │
│    + New worktree...                                               │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

User selects feature-b, presses Enter

(New terminal spawns OR existing terminal is focused)

┌─ Dashboard ─────────────────────────────────────────────────────────┐
│  ~/my-project-fix-payments                     Ports: 3200 5632    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  fix-payments        ████████████░░░  90% (9/10)      ⏸ Paused     │
│                                                                     │
│  ┌─ Environment ────────────────────────────────────── Running ───┐│
│  │  ● app       :3200   ● postgres  :5632   ● redis   :6579      ││
│  └────────────────────────────────────────────────────────────────┘│
│                                                                     │
│  Last: US-009 "Refund processing" - Testing refund edge cases...  │
│  Press [S] to resume                                               │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Journey 4: Environment Troubleshooting

```
User notices tests failing, suspects database issue
User presses [E] for environment control

┌─ Environment ───────────────────────────────────────────────────────┐
│  Status: Running (45m)                            Port offset: +100│
│                                                                     │
│  SERVICES                                                           │
│  ────────                                                           │
│  [1] ● app         3100:3000    healthy      2m ago               │
│  [2] ● postgres    5532:5432    healthy      45m ago              │
│  [3] ● redis       6479:6379    unhealthy    Connection refused   │
│  [4]   worker      -            stopped      Exited (1)           │
│                                                                     │
│  ⚠ redis is unhealthy - this may cause test failures              │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [3] View redis logs    [R] Restart all    [D] Down    Esc: Back  │
└─────────────────────────────────────────────────────────────────────┘

User presses [3]

┌─ Logs: redis ───────────────────────────────────────────────────────┐
│                                                                     │
│  # oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo                   │
│  * Running mode=standalone, port=6379                              │
│  # Warning: Could not create server TCP listening socket           │
│  # *:6379: bind: Address already in use                            │
│  # Redis will now exit                                             │
│                                                                     │
│  (Port conflict detected - likely another redis on 6379)           │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [R] Restart service    [K] Kill port 6379    Esc: Back           │
└─────────────────────────────────────────────────────────────────────┘
```
