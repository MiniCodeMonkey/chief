# Git Worktree Support for Chief

## Context

Chief already supports running multiple PRDs in parallel via the Loop Manager. However, all PRDs share the same working directory and git state. Parallel Claude instances can conflict: editing the same files, producing interleaved commits, and stepping on each other's branches. Git worktrees solve this by giving each PRD its own isolated checkout on its own branch.

---

## UX Design

### How It Works (User's Perspective)

1. User creates PRDs as normal: `chief new auth`, `chief new payments`
2. When pressing `s` to start a PRD, a dialog offers worktree creation
3. Each PRD's Claude instance works in its own isolated directory on its own branch
4. When a PRD completes, its branch has all the commits ready for merge/PR
5. User merges branches at their leisure

### Worktree Location

Worktrees live under `.chief/worktrees/<prd-name>/`. Since `.chief/` is already gitignored, this keeps everything contained and invisible to git in the main repo.

```
.chief/
  config.yaml               # Project-level config (YAML)
  prds/
    auth/prd.json            # PRD state (always in main repo)
    payments/prd.json
  worktrees/
    auth/                    # Full repo checkout on chief/auth branch
    payments/                # Full repo checkout on chief/payments branch
```

### Start Dialog (Enhanced Branch Warning)

When pressing `s` to start a PRD, the dialog appears contextually:

**On a protected branch (main/master):**

```
You are on the main branch.

> Create worktree + branch (Recommended)    chief/auth in .chief/worktrees/auth/
  Create branch only                        chief/auth (stay in current directory)
  Continue on main                          (not recommended)

[Enter] Confirm   [j/k] Navigate   [e] Edit branch name
```

**Another PRD already running in same directory:**

```
Another PRD (payments) is already running in this directory.

> Create worktree (Recommended)              chief/auth in .chief/worktrees/auth/
  Run in same directory                      (may cause file conflicts)

[Enter] Confirm   [j/k] Navigate
```

**No conflicts (not protected, nothing else running):**

```
How should this PRD run?

> Run in current directory (Recommended)     Use the current working directory
  Create worktree + branch                   chief/auth in .chief/worktrees/auth/

[Enter] Confirm   [j/k] Navigate   [e] Edit branch name
```

### Tab Bar - Branch Info

```
 auth [chief/auth] > 3/8    payments [chief/payments] > 1/5    + New
```

### Dashboard Header - Worktree Path

With worktree:

```
 chief  auth  [Running]  iter 3  2m 14s
 branch: chief/auth  dir: .chief/worktrees/auth/
```

Without worktree (running in main repo):

```
 chief  auth  [Running]  iter 3  2m 14s
 branch: chief/auth  dir: ./ (current directory)
```

### PRD Completion - Fully Automated

When a PRD completes, chief automatically runs whatever post-completion actions are configured. The user can walk away from the computer and chief handles everything.

**With push + PR enabled (typical):**

```
PRD Complete!  auth  8/8 stories

Branch 'chief/auth' has 8 commits.

Pushing chief/auth -> origin/chief/auth...       Done
Creating pull request...                          Done
PR #42: feat(auth): JWT authentication system     https://github.com/user/repo/pull/42

[m] Merge locally   [c] Clean worktree   [l] Switch PRD   [q] Quit
```

**With nothing configured:**

```
PRD Complete!  auth  8/8 stories

Branch chief/auth has 8 commits.
Configure auto-push and PR in settings (,)

[m] Merge locally   [c] Clean worktree   [l] Switch PRD   [q] Quit
```

Push and PR creation are config-only - not manual actions. If the user wants to push or create a PR, they configure it in settings and it happens automatically on every PRD completion.

### Picker - Worktree Status + Actions

```
PRDs

> auth           8/8  Complete   chief/auth      .chief/worktrees/auth/
  payments       1/5  Running    chief/payments  .chief/worktrees/payments/
  main           0/3  Ready      (current directory)

[Enter] Select   [s] Start   [n] New   [m] Merge   [c] Clean
```

Picker actions:
- `n` - Create a new PRD (same flow as `chief new` - launches Claude Code for interactive PRD creation)
- `e` - Edit the selected PRD (same flow as `chief edit`)
- `s` - Start the selected PRD
- `m` - Merge selected PRD's branch into current branch (only for completed PRDs with worktrees)
- `c` - Clean selected PRD's worktree + optionally delete branch

### Creating PRDs

PRDs can be created two ways - both invoke the same flow (launch Claude Code with the PRD-creation prompt):

1. **From the CLI:** `chief new [name]` - creates a PRD and exits
2. **From the TUI:** Press `n` in the picker - creates a PRD and returns to the TUI

Editing works the same way: `chief edit [name]` from CLI, or `e` on a selected PRD in the picker.

### CLI Commands

CLI stays minimal - only the core trio:

```
chief new [name]              # Create a new PRD
chief edit [name]             # Edit an existing PRD
chief list                    # List all PRDs with progress
```

All worktree management (merge, clean, listing) and settings editing are **TUI-only** via the picker, completion screen, and `,` keybinding. This keeps the CLI surface minimal and pushes users toward the TUI where they get full context.

### Status command enhanced

The existing `chief status` command gains worktree/branch info:

```
$ chief status auth
Project: Auth System
Branch:  chief/auth
Worktree: .chief/worktrees/auth/
Progress: 8/8 stories complete (100%)
```

### Chief Config File (.chief/config.yaml)

YAML format for readability:

```yaml
worktree:
  setup: "npm install"

onComplete:
  push: true
  createPR: true
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `worktree.setup` | string | `""` | Shell command to run after creating a worktree (e.g., `npm install`) |
| `onComplete.push` | bool | `false` | Auto-push the PRD branch to origin when all stories complete |
| `onComplete.createPR` | bool | `false` | Auto-create a PR after pushing (requires `push: true` and `gh` CLI) |

### First-Run Config Prompt

Part of the first-time setup flow (gitignore -> PRD name -> config). Three steps:

**Step 3: Post-completion settings**
- Push branch to remote? (y/n)
- Auto-create a pull request? (y/n)

**Step 4: Worktree setup command**
- Option A: "Let Claude figure it out" (Recommended) - chief invokes Claude Code with a one-shot prompt to analyze the project and detect the right setup commands (npm install, go mod download, pip install, etc.), then writes the result to config
- Option B: Enter manually - text input for the command
- Option C: Skip

Answers saved to `.chief/config.yaml`. On subsequent runs, these prompts are skipped.

### `gh` CLI Validation

When the user enables "Automatically create a pull request" - either during onboarding (Step 3) or via the Settings TUI - chief immediately validates:

1. Is `gh` in PATH? (`which gh`)
2. Is `gh` authenticated? (`gh auth status`)

If either check fails, show a graceful error **before saving the config**:

```
+----------------------------------------------------------+
|                                                          |
|  ! GitHub CLI Required                                   |
|  --------------------------------------------------------|
|                                                          |
|  Auto-creating pull requests requires the GitHub CLI     |
|  (gh) to be installed and authenticated.                 |
|                                                          |
|  Install:  https://cli.github.com                        |
|  Then run: gh auth login                                 |
|                                                          |
|  > Continue without PR creation                          |
|    Try again                                             |
|                                                          |
|  --------------------------------------------------------|
|  Up/Down: Navigate  Enter: Select                        |
|                                                          |
+----------------------------------------------------------+
```

Same validation applies in the Settings TUI when toggling `createPR` to `Yes`. If `gh` isn't available, show the error and revert the toggle.

### Auto-Detect Setup Command (Claude Prompt)

When the user selects "Let Claude figure it out", chief runs Claude Code with a one-shot prompt:

```
Analyze this project and determine what commands need to run to install
dependencies and set up a working development environment from a fresh
git checkout (e.g., after creating a git worktree).

Look at package managers, lock files, README, build tools, etc. Return ONLY the
shell command(s) needed, joined with &&. Examples:
- "npm install"
- "go mod download"
- "pip install -r requirements.txt"
- "npm install && npx prisma generate"

If no setup is needed, return "none".
Do not explain, just return the command.
```

The result is written directly to `worktree.setup` in `.chief/config.yaml`.

### Settings TUI

Accessible via `,` keybinding from any TUI view. Allows editing all config values at any time.

```
+----------------------------------------------------------+
|                                                          |
|  Settings                         .chief/config.yaml     |
|  --------------------------------------------------------|
|                                                          |
|  Worktree                                                |
|  > Setup command          npm install                    |
|                                                          |
|  On Complete                                             |
|    Push to remote          Yes                           |
|    Create pull request     Yes                           |
|                                                          |
|  --------------------------------------------------------|
|  Config: .chief/config.yaml                              |
|  --------------------------------------------------------|
|  Enter: Edit  Up/Down: Navigate  Esc: Close              |
|                                                          |
+----------------------------------------------------------+
```

Editing a boolean toggles it (with `gh` validation on `createPR`). Editing a string opens an inline text input. Changes are saved immediately to `.chief/config.yaml`.

### PR Creation Behavior

When `onComplete.createPR` is enabled, chief automatically creates a PR after pushing:

- **Branch:** Already on `chief/<prd-name>`
- **Commits:** Follow conventional commits from the agent prompt (`feat: [Story ID] - [Story Title]`)
- **PR title:** Derived from the PRD project name, following conventional commits format (e.g., `feat(auth): JWT authentication system`)
- **PR body:**
  - Summary section: high-level description from PRD
  - Changes section: outline of what was implemented (derived from completed user stories)
  - No test plan checklist
  - No mention of Claude or Claude Code
- **Screenshots:** If the PRD involves UI changes, use playwright to capture screenshots and include them
- **Command:** `gh pr create --title "..." --body "..."`

Chief generates the PR content by reading the PRD description and completed story titles, rather than invoking Claude again - keeping it fast and deterministic.

---

## Transparency Principles

Every automated action should show *what* is happening, *where* it's happening, and *what directory/branch/path* is involved. Never hide the mechanics. The process should be slick and subtle but still transparent and clear so it doesn't appear magical or confusing.

### Specific Transparency Requirements

**Tab bar** - Show branch name per tab:
```
 auth [chief/auth] > 3/8    payments [chief/payments] > 1/5
```

**Dashboard header** - Show working directory and branch:
```
 chief  auth  [Running]  iter 3  2m 14s
 branch: chief/auth  dir: .chief/worktrees/auth/
```

**Worktree setup spinner** - Show each step as it happens with paths:
```
 ✓ Created branch 'chief/auth' from 'main'
 ✓ Created worktree at .chief/worktrees/auth/
 * Running setup: npm install...
```

**Completion screen** - Show what auto-actions ran and their results with full context:
```
 Branch 'chief/auth' has 8 commits.
 ✓ Pushed chief/auth -> origin/chief/auth
 ✓ PR #42: feat(auth): JWT authentication system
   https://github.com/user/repo/pull/42
```

**Picker** - Show worktree path, branch, and working directory for every PRD:
```
 > auth           8/8  Complete   chief/auth      .chief/worktrees/auth/
   payments       1/5  Running    chief/payments  .chief/worktrees/payments/
   main           0/3  Ready      (current directory)
```

**Start dialog** - Always show the full path of where Claude will work:
```
 > Create worktree + branch 'chief/auth'
   Claude will work in: .chief/worktrees/auth/
   Branch created from: main

   Create branch only 'chief/auth'
   Claude will work in: ./ (current directory)
```

**Settings TUI** - Show config file path in header:
```
 Settings                                .chief/config.yaml
```

**Clean confirmation** - Show exact paths being removed:
```
 Remove worktree for 'auth'?

 Worktree: .chief/worktrees/auth/  (will be deleted)
 Branch:   chief/auth              (will be deleted)
```

**Merge result** - Show what happened:
```
 ✓ Merged chief/auth into main (fast-forward)
 8 commits applied.
```

**Merge conflict** - Show exact instructions:
```
 x Could not merge 'chief/auth' into 'main'.
 Conflicting files:
   src/auth/handler.go
   src/middleware/jwt.go
 Resolve in your terminal:
   cd /Users/you/project
   git merge chief/auth
```

**Auto-action errors** - Inline, actionable, never blocking:
```
 ✓ Pushed chief/auth to origin
 x PR creation failed: gh not found
   Install: https://cli.github.com
```

---

## Design Decisions

1. **Worktree branches always start from main/master** - not from current HEAD. This ensures each PRD has a clean base regardless of what branch the user happens to be on.
2. **Config prompt is part of the first-time setup flow** - The existing flow is: gitignore -> PRD name. Extended to: gitignore -> PRD name -> post-completion config -> worktree setup. One consolidated onboarding.
3. **Push and PR are config-only, not manual actions** - Once configured, they fire automatically on every PRD completion. The goal is "start and walk away." No `p` or `r` keybindings in the TUI. If the user wants to change the behavior, they use the settings TUI (`,` key).
4. **Claude auto-detects worktree setup commands** - During onboarding, the user can let Claude analyze the project to figure out what setup commands are needed. This is a one-shot Claude invocation that writes the result to config.
5. **Config is YAML** - `.chief/config.yaml` for readability and easy manual editing.
6. **Agent prompt doesn't need worktree awareness** - Claude just sees a normal git repo checkout. The `{{PRD_PATH}}` is absolute so it works from any CWD.
7. **`gh` CLI is validated eagerly** - When the user enables `createPR`, chief checks for `gh` immediately (during onboarding and in settings) rather than waiting for completion to fail. Errors are graceful with actionable instructions.
8. **Merge conflicts show error in TUI** - The TUI can't do interactive merge resolution. When merge fails, show the error output and instruct the user to resolve in their terminal.
9. **Clean on running PRD is blocked** - The `c` keybinding is disabled (grayed out) for running PRDs. User must stop first.
10. **Settings TUI for post-onboarding changes** - Accessible via `,` from any view. All config values editable with immediate save.
11. **No new CLI subcommands** - Merge, clean, worktree listing, and settings are TUI-only. The CLI stays minimal: `new`, `edit`, `list`. This pushes users toward the TUI where they get full context and avoids duplicating UI surfaces.
12. **Transparency over magic** - Every TUI surface shows paths, branches, directories, and running processes. Users should always know what's happening and where.

---

## TUI Mockups

All mockups follow the existing modal pattern: centered rounded-border modals, `>` selection indicator, divider lines, footer with keybinding hints.

### First-Time Setup (Extended)

The existing flow adds two new steps after PRD name entry:

**Step 3: Post-Completion Config**

```
+----------------------------------------------------------+
|                                                          |
|  ✓ Added .chief to .gitignore                            |
|  ✓ PRD name: main                                        |
|                                                          |
|  Post-Completion Settings                                |
|  --------------------------------------------------------|
|                                                          |
|  When a PRD finishes all its stories:                    |
|                                                          |
|  Push branch to remote?                                  |
|  > Yes  (Recommended)                                    |
|    No                                                    |
|                                                          |
|  Automatically create a pull request?                    |
|  > Yes  (Recommended)                                    |
|    No                                                    |
|                                                          |
|  You can change these later with , or chief settings     |
|                                                          |
|  --------------------------------------------------------|
|  Up/Down: Navigate  Tab: Next field  Enter: Confirm      |
|                                                          |
+----------------------------------------------------------+
```

When "Yes" is selected for PR creation, chief runs `gh auth status` before proceeding. If `gh` is not available or not authenticated, the `gh` CLI Required error dialog is shown (see `gh` CLI Validation section above).

**Step 4: Worktree Setup Command**

```
+----------------------------------------------------------+
|                                                          |
|  ✓ Push on complete                                      |
|  ✓ Create PR on complete                                 |
|                                                          |
|  Worktree Setup Command                                  |
|  --------------------------------------------------------|
|                                                          |
|  Command to run after creating a git worktree            |
|  (e.g., install dependencies)                            |
|                                                          |
|  > Let Claude figure it out  (Recommended)               |
|    Enter manually                                        |
|    Skip                                                  |
|                                                          |
|  --------------------------------------------------------|
|  Up/Down: Navigate  Enter: Select                        |
|                                                          |
+----------------------------------------------------------+
```

**Step 4b: Claude auto-detecting setup (after selecting "Let Claude figure it out")**

```
+----------------------------------------------------------+
|                                                          |
|  Worktree Setup Command                                  |
|  --------------------------------------------------------|
|                                                          |
|  * Analyzing project for setup commands...               |
|                                                          |
+----------------------------------------------------------+
```

**Step 4c: Claude result**

```
+----------------------------------------------------------+
|                                                          |
|  Worktree Setup Command                                  |
|  --------------------------------------------------------|
|                                                          |
|  Detected: npm install && npx prisma generate            |
|                                                          |
|  > Use this command  (Recommended)                       |
|    Edit                                                  |
|    Skip                                                  |
|                                                          |
|  --------------------------------------------------------|
|  Up/Down: Navigate  Enter: Select                        |
|                                                          |
+----------------------------------------------------------+
```

**Step 4 (manual entry, if selected):**

```
+----------------------------------------------------------+
|                                                          |
|  Worktree Setup Command                                  |
|  --------------------------------------------------------|
|                                                          |
|  Command to run after creating a worktree:               |
|                                                          |
|  +----------------------------------------------+       |
|  | npm install_                                  |       |
|  +----------------------------------------------+       |
|                                                          |
|  --------------------------------------------------------|
|  Enter: Confirm  Esc: Back                               |
|                                                          |
+----------------------------------------------------------+
```

### Enhanced Branch Warning Dialog (Worktree Option)

**On protected branch:**

```
+----------------------------------------------------------+
|                                                          |
|  ! Protected Branch Warning                              |
|  --------------------------------------------------------|
|                                                          |
|  You are on the 'main' branch.                           |
|  Starting the loop will make changes to this branch.     |
|                                                          |
|  > Create worktree + branch 'chief/auth'  (Recommended) |
|    Claude will work in: .chief/worktrees/auth/           |
|    Branch created from: main                             |
|                                                          |
|    Create branch only 'chief/auth'                       |
|    Claude will work in: ./ (current directory)           |
|                                                          |
|    Continue on 'main'                                    |
|    Not recommended for production branches                |
|                                                          |
|    Cancel                                                |
|                                                          |
|  --------------------------------------------------------|
|  Up/Down: Navigate  Enter: Select  e: Edit branch name   |
|                                                          |
+----------------------------------------------------------+
```

**Another PRD already running:**

```
+----------------------------------------------------------+
|                                                          |
|  ! Parallel Execution                                    |
|  --------------------------------------------------------|
|                                                          |
|  PRD 'payments' is already running in this directory.    |
|  Running another PRD here may cause file conflicts.      |
|                                                          |
|  > Create worktree + branch 'chief/auth'  (Recommended) |
|    Claude will work in: .chief/worktrees/auth/           |
|                                                          |
|    Run in same directory                                 |
|    May cause conflicts with running PRD                  |
|                                                          |
|    Cancel                                                |
|                                                          |
|  --------------------------------------------------------|
|  Up/Down: Navigate  Enter: Select  e: Edit branch name   |
|                                                          |
+----------------------------------------------------------+
```

### Worktree Setup Spinner

Shown after selecting "Create worktree" in the branch dialog:

```
+----------------------------------------------------------+
|                                                          |
|  Setting Up Worktree                                     |
|  --------------------------------------------------------|
|                                                          |
|  ✓ Created branch 'chief/auth' from 'main'              |
|  ✓ Created worktree at .chief/worktrees/auth/            |
|  * Running setup: npm install...                         |
|                                                          |
|  --------------------------------------------------------|
|  This may take a moment.  Esc: Cancel                    |
|                                                          |
+----------------------------------------------------------+
```

After setup completes (briefly shown before loop starts):

```
+----------------------------------------------------------+
|                                                          |
|  Setting Up Worktree                                     |
|  --------------------------------------------------------|
|                                                          |
|  ✓ Created branch 'chief/auth' from 'main'              |
|  ✓ Created worktree at .chief/worktrees/auth/            |
|  ✓ Setup complete: npm install                           |
|                                                          |
|  Starting loop...                                        |
|                                                          |
+----------------------------------------------------------+
```

### Completion Screen

**Auto-push + PR in progress:**

```
+----------------------------------------------------------+
|                                                          |
|  ✓ PRD Complete!  auth  8/8 stories                      |
|  --------------------------------------------------------|
|                                                          |
|  Branch 'chief/auth' has 8 commits.                      |
|                                                          |
|  ✓ Pushed chief/auth -> origin/chief/auth                |
|  * Creating pull request...                              |
|                                                          |
+----------------------------------------------------------+
```

**Auto-actions finished:**

```
+----------------------------------------------------------+
|                                                          |
|  ✓ PRD Complete!  auth  8/8 stories                      |
|  --------------------------------------------------------|
|                                                          |
|  Branch 'chief/auth' has 8 commits.                      |
|                                                          |
|  ✓ Pushed chief/auth -> origin/chief/auth                |
|  ✓ PR #42: feat(auth): JWT authentication system         |
|    https://github.com/user/repo/pull/42                  |
|                                                          |
|  --------------------------------------------------------|
|  m: Merge locally  c: Clean worktree  l: Switch PRD      |
|  q: Quit                                                 |
|                                                          |
+----------------------------------------------------------+
```

**No auto-actions configured:**

```
+----------------------------------------------------------+
|                                                          |
|  ✓ PRD Complete!  auth  8/8 stories                      |
|  --------------------------------------------------------|
|                                                          |
|  Branch 'chief/auth' has 8 commits.                      |
|  Configure auto-push and PR in settings (,)              |
|                                                          |
|  --------------------------------------------------------|
|  m: Merge locally  c: Clean worktree  l: Switch PRD      |
|  q: Quit                                                 |
|                                                          |
+----------------------------------------------------------+
```

**Auto-action error (shown inline, non-blocking):**

```
+----------------------------------------------------------+
|                                                          |
|  ✓ PRD Complete!  auth  8/8 stories                      |
|  --------------------------------------------------------|
|                                                          |
|  Branch 'chief/auth' has 8 commits.                      |
|                                                          |
|  ✓ Pushed chief/auth -> origin/chief/auth                |
|  x PR creation failed: gh not found                      |
|    Install: https://cli.github.com                       |
|                                                          |
|  --------------------------------------------------------|
|  m: Merge locally  c: Clean worktree  l: Switch PRD      |
|  q: Quit                                                 |
|                                                          |
+----------------------------------------------------------+
```

### Merge Conflict Error

When `m` (merge) encounters conflicts:

```
+----------------------------------------------------------+
|                                                          |
|  x Merge Conflict                                        |
|  --------------------------------------------------------|
|                                                          |
|  Could not merge 'chief/auth' into 'main'.               |
|                                                          |
|  Conflicting files:                                      |
|    src/auth/handler.go                                   |
|    src/middleware/jwt.go                                  |
|                                                          |
|  Resolve conflicts in your terminal:                     |
|    cd /path/to/project                                   |
|    git merge chief/auth                                  |
|    # resolve conflicts, then git commit                  |
|                                                          |
|  --------------------------------------------------------|
|  Enter/Esc: Dismiss                                      |
|                                                          |
+----------------------------------------------------------+
```

### Clean Confirmation Dialog

When `c` (clean) is pressed for a completed PRD:

```
+----------------------------------------------------------+
|                                                          |
|  Clean Worktree                                          |
|  --------------------------------------------------------|
|                                                          |
|  Remove worktree for 'auth'?                             |
|                                                          |
|  Worktree: .chief/worktrees/auth/  (will be deleted)     |
|  Branch:   chief/auth              (will be deleted)     |
|                                                          |
|  > Remove worktree + delete branch  (Recommended)        |
|    Remove worktree only (keep branch)                    |
|    Cancel                                                |
|                                                          |
|  --------------------------------------------------------|
|  Up/Down: Navigate  Enter: Confirm                       |
|                                                          |
+----------------------------------------------------------+
```

---

## Technical Gotchas

### 1. `.chief/` is gitignored and won't exist in worktrees

Since `.chief/` is gitignored, worktree checkouts won't have it. PRD files (`prd.json`, `progress.md`, `claude.log`) all live in `.chief/prds/<name>/` in the main repo only. This works because:
- The agent prompt uses `{{PRD_PATH}}` as an absolute path - Claude reads `prd.json` regardless of CWD
- The PRD watcher watches files in the main repo's `.chief/` directory

### 2. Claude's working directory must change

Currently: `l.claudeCmd.Dir = filepath.Dir(l.prdPath)` (sets CWD to `.chief/prds/<name>/`)
Must become: `l.claudeCmd.Dir = l.workDir` where `workDir` is the worktree path (or the project root for non-worktree PRDs)

### 3. Dependencies must be installed per worktree

Each worktree is a fresh checkout with no `node_modules/`, build artifacts, etc. Solved by the `worktree.setup` config in `.chief/config.yaml`. During onboarding, Claude can auto-detect the right command by analyzing the project's package managers and lock files.

### 4. Disk space

Each worktree is a full source checkout (git objects are shared via the `.git` dir). Large repos add up. Mitigated by `c` (clean) in the TUI after merge.

### 5. Git lock contention

Concurrent git operations across worktrees can occasionally hit lock files. Git worktrees are designed for this, but rapid concurrent commits could race. The Ralph loop's sequential model (one commit per iteration) makes this unlikely.

### 6. Branch uniqueness (a feature, not a bug)

Git enforces each worktree must be on a unique branch. Two worktrees cannot share a branch. This prevents two PRDs from stomping on each other's state.

### 7. Orphaned worktrees on crash

If chief crashes, worktrees remain on disk. Need:
- `c` keybinding in picker for manual removal
- Startup detection of orphaned worktrees
- Under the hood: `git worktree prune` cleans git's internal tracking

### 8. Submodules

Worktrees don't auto-init submodules. The setup command would need `git submodule update --init` if the project uses them.

### 9. Stale worktree paths

If `.chief/worktrees/<name>` already exists from a previous run:
- Check if it's a valid worktree on the expected branch -> reuse it
- Otherwise remove and recreate

### 10. Worktree inside the main repo's gitignored dir

Placing worktrees at `.chief/worktrees/` means they're inside the main repo's tree but gitignored. Git handles this fine - the main repo won't track worktree contents. The worktree itself has its own index and HEAD pointing at the shared `.git` via a `.git` file (not directory).

### 11. CLAUDE.md and project config

If the project has a `CLAUDE.md` or other config at the repo root, the worktree will have its own copy (from the branch checkout). This is correct behavior - Claude should see the project config.

### 12. Merging may produce conflicts

If two PRDs modify overlapping files, merging their branches will conflict. Chief should report this clearly but not try to auto-resolve.

---

## High-Level Implementation

### Files to Create/Modify

| File | Change |
|------|--------|
| `internal/git/worktree.go` | **New** - Worktree CRUD primitives |
| `internal/git/push.go` | **New** - Push branch, create PR via `gh`, CheckGHCLI validation |
| `internal/config/config.go` | **New** - Load/save `.chief/config.yaml` (YAML) |
| `internal/loop/loop.go` | Add `workDir` field, use it for `cmd.Dir` |
| `internal/loop/manager.go` | LoopInstance gets WorktreeDir, Branch fields; post-completion hooks |
| `internal/tui/app.go` | Enhanced start dialog, worktree creation flow, completion actions, `,` keybinding |
| `internal/tui/branch_warning.go` | Add worktree option to dialog with path transparency |
| `internal/tui/first_time_setup.go` | Extend with post-completion config + worktree setup steps + `gh` validation |
| `internal/tui/dashboard.go` | Show branch + worktree dir in header |
| `internal/tui/tabbar.go` | Show `[branch-name]` per tab |
| `internal/tui/picker.go` | Show worktree path/branch + `m`/`c` keybindings |
| `internal/tui/completion.go` | **New** - Completion screen with auto-action progress |
| `internal/tui/settings.go` | **New** - Settings editor overlay (`,` keybinding) with `gh` validation |
| `embed/detect_setup_prompt.txt` | **New** - Prompt for Claude to auto-detect worktree setup commands |
| `cmd/chief/main.go` | Load config on startup (no new subcommands) |

### Step 1: Git Worktree Primitives

`internal/git/worktree.go` - Shell out to git commands:

```go
type Worktree struct {
    Path     string // Filesystem path
    Branch   string // Branch checked out
    HEAD     string // Current commit SHA
    Prunable bool
}

// Finds the default branch (main or master)
func GetDefaultBranch(repoDir string) (string, error)
// Creates branch from default branch (main/master), then creates worktree
func CreateWorktree(repoDir, worktreePath, branch string) error
// git worktree remove <path>
func RemoveWorktree(repoDir, worktreePath string) error
// git worktree list --porcelain
func ListWorktrees(repoDir string) ([]Worktree, error)
// Check if path is a valid worktree
func IsWorktree(path string) bool
// Standard path: .chief/worktrees/<prd-name>
func WorktreePathForPRD(baseDir, prdName string) string
// git worktree prune
func PruneWorktrees(repoDir string) error
// Merge a branch into current branch, returns conflict file list on failure
func MergeBranch(repoDir, branch string) ([]string, error)
```

### Step 2: Loop Changes

`internal/loop/loop.go` - Add `workDir` to Loop:

```go
type Loop struct {
    prdPath  string // Path to prd.json (for reading PRD state)
    workDir  string // Working directory for Claude (worktree or project root)
    // ... rest unchanged
}
```

In `runIteration`, change line 256:
```go
// Before:
l.claudeCmd.Dir = filepath.Dir(l.prdPath)
// After:
l.claudeCmd.Dir = l.workDir
```

Add factory function:
```go
func NewLoopWithWorkDir(prdPath, workDir string, maxIter int) *Loop
```

`NewLoopWithEmbeddedPrompt` continues to work for non-worktree PRDs, deriving `workDir` from `prdPath` as today (or better: default to the project root).

### Step 3: Manager Changes

`internal/loop/manager.go` - Extend LoopInstance:

```go
type LoopInstance struct {
    Name        string
    PRDPath     string
    WorktreeDir string // Empty string = no worktree, use main repo
    Branch      string // Branch name (e.g., "chief/auth")
    // ... rest unchanged
}
```

New method:
```go
func (m *Manager) RegisterWithWorktree(name, prdPath, worktreeDir, branch string) error
```

In `Start()`, pass `workDir` when creating the Loop:
```go
workDir := m.baseDir // default: project root
if instance.WorktreeDir != "" {
    workDir = instance.WorktreeDir
}
instance.Loop = NewLoopWithWorkDir(instance.PRDPath, workDir, prompt, m.maxIter)
```

### Step 4: Config System (internal/config/config.go)

Uses `gopkg.in/yaml.v3` for YAML parsing.

```go
type Config struct {
    Worktree   WorktreeConfig   `yaml:"worktree"`
    OnComplete OnCompleteConfig `yaml:"onComplete"`
}

type WorktreeConfig struct {
    Setup string `yaml:"setup"` // e.g., "npm install"
}

type OnCompleteConfig struct {
    Push     bool `yaml:"push"`     // Auto-push branch to origin
    CreatePR bool `yaml:"createPR"` // Auto-create PR after push
}

func Load(baseDir string) (*Config, error)     // Reads .chief/config.yaml
func Save(baseDir string, cfg *Config) error   // Writes .chief/config.yaml
func Exists(baseDir string) bool               // Check if config.yaml exists
func Default() *Config                         // Returns config with zero-value defaults
```

### Step 5: Git Push + PR + Validation (internal/git/push.go)

```go
// Check if gh CLI is installed and authenticated
func CheckGHCLI() (installed bool, authenticated bool, err error)

// Push a branch to origin
func PushBranch(dir, branch string) error

// Create PR using gh CLI, returns PR URL
func CreatePR(dir, branch, title, body string) (string, error)

// Generate PR title from PRD (conventional commits format)
func PRTitleFromPRD(p *prd.PRD) string

// Generate PR body from PRD (summary + changes from stories, no test plan, no Claude mentions)
func PRBodyFromPRD(p *prd.PRD) string
```

`CheckGHCLI()` runs `gh auth status` and parses the exit code:
- Exit 0: installed and authenticated
- Exit non-zero or command not found: not ready

PR generation reads the PRD directly - no Claude invocation needed:
- Title: `feat(<prd-name>): <PRD project name>` (conventional commits)
- Body: `## Summary\n<PRD description>\n\n## Changes\n<bullet list of completed story titles>`

### Step 6: TUI Integration

**`internal/tui/first_time_setup.go`** - Extended with two new steps:

The existing flow (gitignore -> PRD name) gains two additional steps:
- `StepPostCompletion` - Two yes/no toggles: push to remote, create PR. Uses same `>` selection pattern as gitignore step. When PR is selected, runs `CheckGHCLI()` and shows error dialog if validation fails.
- `StepWorktreeSetup` - Three options: "Let Claude figure it out" / "Enter manually" / "Skip". If Claude is selected, runs a one-shot Claude Code invocation to analyze the project and detect setup commands. Shows spinner while Claude runs, then presents result for confirmation/editing.

`FirstTimeSetupResult` gains new fields: `PushOnComplete bool`, `CreatePROnComplete bool`, `WorktreeSetup string`. After setup completes, `main.go` saves these to `.chief/config.yaml`.

**`internal/tui/app.go`** - Enhanced start flow:

The `startLoop()` method becomes:
1. Check context: protected branch? another PRD running in same directory?
2. Show enhanced dialog with worktree option (see mockups above)
3. If worktree selected:
   a. Create branch `chief/<name>` from default branch (main/master)
   b. Create worktree at `.chief/worktrees/<name>/` on that branch
   c. Run setup command if configured (show spinner: "Setting up worktree...")
   d. Register instance with worktree path
4. Start the loop
5. On completion: run auto-actions from config (push, create PR)

Add `,` keybinding handler to show settings overlay from any view.

**`internal/tui/completion.go`** - Completion screen:

When a PRD completes (`EventComplete` received):
1. Automatically run configured actions (push, create PR) - show progress inline
2. Display results (success or error for each auto-action) with full paths and URLs
3. Show remaining manual action keybindings:
   - `m` - Merge branch locally
   - `c` - Clean worktree + delete branch
   - `l` - Switch to another PRD
   - `q` - Quit

**`internal/tui/settings.go`** - New - Settings editor overlay:

Modal view accessible via `,` from any view. Renders all config values as an editable list. Booleans toggle on Enter (with `gh` validation on `createPR` toggle). Strings open inline text input. Saves to `.chief/config.yaml` on every change. Shows config file path in header.

**`internal/tui/branch_warning.go`** - Add "Create worktree + branch" as first option. Each option shows where Claude will work (path transparency).

**`internal/tui/dashboard.go`** - Add branch/worktree directory line to header section.

**`internal/tui/tabbar.go`** - Show `[branch-name]` next to PRD name in tabs.

**`internal/tui/picker.go`** - Show worktree path and branch in PRD entries. Action keybindings for completed PRDs:
- `m` - Merge selected PRD's branch
- `c` - Clean selected PRD's worktree

---

## Documentation Updates

All docs under `docs/` need updates to reflect the new features.

| Doc File | Changes |
|----------|---------|
| `docs/reference/cli.md` | Keep `new`, `edit`, `list` commands. Add keyboard shortcuts for new TUI features: `,` (settings), `m` (merge), `c` (clean). Update TUI keyboard shortcuts section with worktree-related keybindings. |
| `docs/reference/configuration.md` | **Major rewrite.** Document `.chief/config.yaml` format, all config keys (`worktree.setup`, `onComplete.push`, `onComplete.createPR`), the Settings TUI (`,` key), first-time setup flow, and Claude auto-detect for setup commands. Replaces the "No Global Config" section. |
| `docs/concepts/chief-directory.md` | Add `worktrees/` subdirectory and `config.yaml` to the directory structure diagram. Explain worktree layout (`.chief/worktrees/<name>/`), that worktrees are full checkouts sharing git objects. Add `config.yaml` to the file explanations section. |
| `docs/concepts/how-it-works.md` | Add section on git worktrees for parallel PRD execution. Update the execution loop to mention worktree isolation. Add note about auto-push and PR creation on completion. |
| `docs/concepts/ralph-loop.md` | Update the loop flowchart to show the optional worktree creation before "Press 's'" and the post-completion actions (push, PR) after "Done". Add section on working directory isolation. |
| `docs/guide/quick-start.md` | Add coverage of the first-time setup config prompts (push/PR/worktree setup). Mention the settings TUI. Update keyboard controls table with new keybindings (`,`, `m`, `c`). |
| `docs/guide/installation.md` | Add optional prerequisite: `gh` CLI for auto-PR creation with link to https://cli.github.com. |
| `docs/troubleshooting/common-issues.md` | Add sections: "Worktree Setup Failed", "PR Creation Failed (gh not found)", "Orphaned Worktrees", "Merge Conflicts". |
| `docs/troubleshooting/faq.md` | Add: "How do worktrees work?", "Can I run multiple PRDs in parallel safely?", "How do I merge a completed PRD?", "How do I clean up worktrees?", "What is .chief/config.yaml?". |
| `docs/adr/0007-git-worktree-isolation.md` | **New** - ADR documenting the decision to use git worktrees for parallel PRD isolation, alternatives considered (separate clones, docker, single-branch parallel), and trade-offs. |

### Sidebar Config

`docs/.vitepress/config.ts` - No structural changes needed. Existing sidebar sections cover all updated pages. The new ADR is auto-discovered by the ADR index page.

---

## Verification

1. **Single PRD, no worktree:** `chief test` -> press `s` -> choose "current directory" -> works as today
2. **Single PRD, with worktree:** `chief test` -> press `s` -> choose "create worktree" -> Claude works in `.chief/worktrees/test/`
3. **Two parallel PRDs:** Start auth, then start payments -> second gets worktree dialog emphasizing isolation -> both run independently
4. **Worktree reuse:** Stop a PRD, restart it -> detects existing worktree, reuses it
5. **First-run config:** Delete `.chief/config.yaml` -> start chief -> prompted for push/PR/setup preferences -> config saved as YAML
6. **Claude auto-detect setup:** During onboarding, select "Let Claude figure it out" -> Claude analyzes project -> detected command shown for confirmation -> saved to config
7. **`gh` validation (onboarding):** Enable PR creation without `gh` installed -> error dialog shown -> option to continue without or retry
8. **`gh` validation (settings):** Toggle createPR to Yes without `gh` -> error shown, toggle reverts
9. **Auto-push + PR:** Configure push + PR -> complete a PRD -> branch pushed + PR created automatically without user interaction
10. **Auto-action errors:** Configure PR but no `gh` installed -> complete a PRD -> shows error inline on completion screen, doesn't block
11. **Settings TUI:** Press `,` -> settings overlay opens -> toggle a boolean -> verify `config.yaml` updated -> press Esc -> back to dashboard
12. **Picker actions:** Open picker -> select completed PRD -> press `m` to merge, `c` to clean -> verify each works
13. **Crash recovery:** Kill chief -> restart -> detect orphaned worktrees
14. **Conflict merge:** Two PRDs edit same file -> merge reports conflict clearly with file list and terminal instructions
15. **PR format:** Verify PR title follows conventional commits, body has summary + changes, no Claude mentions
16. **Walk-away test:** Configure push + PR, start a PRD, leave -> PRD completes, pushes, creates PR all unattended
17. **Transparency check:** At every stage, verify the TUI shows what directory, branch, and path is involved - nothing should feel hidden or magical
18. **No new CLI commands:** Verify `chief merge`, `chief clean`, `chief worktrees`, `chief settings` do NOT exist as subcommands
19. **Docs accuracy:** All documentation pages reflect the new features, config format, and keyboard shortcuts accurately
