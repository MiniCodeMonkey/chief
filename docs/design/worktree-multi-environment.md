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

### Decision 3: Worktree Lifecycle Management

**New Commands:**

```bash
# Create a new worktree with PRD
chief worktree new feature-c --prd "Add payment processing"

# List all worktrees and their PRD status
chief worktree list

# Switch to a worktree (opens new shell or changes directory)
chief worktree switch feature-a

# Remove a worktree (with confirmation)
chief worktree remove feature-b

# Start dev environment for current worktree
chief env start

# Stop dev environment
chief env stop

# Show allocated ports
chief env ports
```

**Automatic Worktree Creation Flow:**

```
User: chief new "Add dark mode"

Chief: Creating PRD "add-dark-mode"...

       Would you like to create a dedicated worktree for this PRD?
       This allows parallel development with isolated environments.

       [Y] Yes, create worktree  [N] No, use current directory

User: Y

Chief: Creating worktree at ../project-add-dark-mode...
       Creating branch: feature/add-dark-mode
       Initializing environment profile (port offset: +200)

       Ready! To start working:
         cd ../project-add-dark-mode && chief start
```

---

## UX/UI Design

### TUI Changes

#### 1. Worktree Indicator in Status Bar

```
┌─ Chief ─────────────────────────────────────────────────────────────┐
│  ◉ feature-a [Running]  ○ feature-b [Paused]  ○ main [Ready]       │
│  ↳ Worktree: ~/project-feature-a  Ports: 3100, 5532, 6479          │
├─────────────────────────────────────────────────────────────────────┤
```

#### 2. Dashboard Multi-Worktree View

```
┌─ Dashboard ─────────────────────────────────────────────────────────┐
│                                                                     │
│  ACTIVE WORKTREES                                                   │
│  ───────────────                                                    │
│                                                                     │
│  ● feature-a          ███████████░░░░  73% (8/11 stories)          │
│    ~/project-feature-a                                              │
│    Env: Running (ports 3100, 5532)    Iteration: 12                │
│                                                                     │
│  ◐ feature-b          █████░░░░░░░░░░  36% (4/11 stories)          │
│    ~/project-feature-b                                              │
│    Env: Running (ports 3200, 5632)    Iteration: 5 [Paused]        │
│                                                                     │
│  ○ main               ░░░░░░░░░░░░░░░  Ready                       │
│    ~/project                                                        │
│    Env: Stopped                                                     │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [W] Switch Worktree  [E] Env Control  [N] New Worktree  [?] Help  │
└─────────────────────────────────────────────────────────────────────┘
```

#### 3. Environment Control Panel (New View)

```
┌─ Environment: feature-a ────────────────────────────────────────────┐
│                                                                     │
│  STATUS: Running                        UPTIME: 2h 34m             │
│                                                                     │
│  SERVICES                                                           │
│  ────────                                                           │
│  ● postgres    5532:5432    healthy     postgres:15-alpine         │
│  ● redis       6479:6379    healthy     redis:7-alpine             │
│  ● app         3100:3000    healthy     node:20                    │
│  ● worker      -            running     node:20                    │
│                                                                     │
│  PORT MAPPINGS                                                      │
│  ─────────────                                                      │
│  Base offset: +100                                                  │
│  App URL: http://localhost:3100                                     │
│  Database: postgres://localhost:5532/app                           │
│                                                                     │
│  LOGS (app)                                                         │
│  ──────────                                                         │
│  [2024-01-15 10:23:45] Server listening on port 3000               │
│  [2024-01-15 10:23:46] Connected to database                       │
│  [2024-01-15 10:24:12] GET /api/users 200 45ms                     │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [S] Start  [P] Stop  [R] Restart  [L] Logs  [←] Back              │
└─────────────────────────────────────────────────────────────────────┘
```

#### 4. Worktree Picker

```
┌─ Select Worktree ───────────────────────────────────────────────────┐
│                                                                     │
│  > ● feature-a     Running    8/11 stories    ~/project-feature-a  │
│    ◐ feature-b     Paused     4/11 stories    ~/project-feature-b  │
│    ○ main          Ready      -               ~/project             │
│    + Create new worktree...                                         │
│                                                                     │
│  ↑/↓ Navigate  Enter Select  N New  D Delete  Esc Cancel           │
└─────────────────────────────────────────────────────────────────────┘
```

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
- Track allocated offsets in `.chief/config.json`
- Release offsets when worktrees are deleted
- Warn when approaching limit (40+ worktrees)
- Allow manual offset override: `chief worktree new --port-offset 5000`

### 2. Orphaned Environments

**Problem**: User deletes worktree directory manually, leaving docker containers running.

**Mitigation**:
- Use unique container names: `chief-{worktree-id}-{service}`
- Periodic cleanup check: detect orphaned containers
- `chief env cleanup` command to stop all Chief-managed containers

```go
// Container naming convention
containerName := fmt.Sprintf("chief-%s-%s", worktreeID[:8], serviceName)
```

### 3. Shared Database State

**Problem**: Multiple worktrees might need different database schemas/data.

**Mitigation**:
- Each worktree gets its own database volume
- Volume naming: `chief-{worktree-id}-{service}-data`
- Option to seed from main: `chief env start --seed-from main`

### 4. Resource Limits

**Problem**: Running 5+ full dev environments simultaneously could exhaust RAM/CPU.

**Mitigation**:
- Display resource usage in TUI
- Warn when starting environments: "4 environments already running (8GB RAM used)"
- `chief env pause` to stop containers without removing state
- Auto-pause inactive worktrees after configurable timeout

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
1. Add worktree detection (`git worktree list`)
2. Add `.chief/config.json` support
3. TUI shows current worktree info (read-only)

### Phase 2: Environment Isolation
1. Implement port allocation
2. Docker compose override generation
3. `chief env start/stop/status` commands
4. Environment panel in TUI

### Phase 3: Worktree Management
1. `chief worktree new/list/remove` commands
2. PRD-worktree binding
3. Worktree picker in TUI
4. Cross-worktree dashboard view

### Phase 4: Polish
1. Auto-pause inactive environments
2. Resource usage monitoring
3. Environment seeding
4. Windows testing & fixes

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

### Journey 1: First-Time Worktree User

```
$ cd my-project
$ chief new "Add user authentication"

Creating PRD "add-user-authentication"...

💡 Tip: You can work on multiple PRDs simultaneously using worktrees.
   Each worktree gets isolated ports so you can run parallel dev environments.

Would you like to create a dedicated worktree? [Y/n] y

Where should the worktree be created?
  [1] ../my-project-add-user-authentication (recommended)
  [2] ./worktrees/add-user-authentication
  [3] Custom path...

> 1

Creating worktree...
✓ Created branch: feature/add-user-authentication
✓ Created worktree: ../my-project-add-user-authentication
✓ Allocated ports: app:3100, postgres:5532, redis:6479
✓ Generated docker-compose.override.yml

Ready! To start working:

  cd ../my-project-add-user-authentication
  chief start

Or continue here and Chief will prompt you to switch.
```

### Journey 2: Checking Status Across Worktrees

```
$ chief worktree list

WORKTREES
─────────

● main (current)
  Path: ~/projects/my-project
  PRD: (none active)
  Env: stopped

● add-user-authentication
  Path: ~/projects/my-project-add-user-authentication
  PRD: add-user-authentication (4/11 stories, 36%)
  Env: running (ports 3100, 5532, 6479)
  Loop: paused at iteration 7

● fix-payment-bug
  Path: ~/projects/my-project-fix-payment-bug
  PRD: fix-payment-bug (9/10 stories, 90%)
  Env: running (ports 3200, 5632, 6579)
  Loop: running, iteration 15

Total: 3 worktrees, 2 environments running
```

### Journey 3: Switching Context

```
$ chief worktree switch fix-payment-bug

Switching to worktree: fix-payment-bug
Path: ~/projects/my-project-fix-payment-bug

Environment is already running (ports 3200, 5632, 6579)
PRD: fix-payment-bug (90% complete)
Loop: running at iteration 15

Opening TUI...
```
