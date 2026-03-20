---
description: Deep dive into the Ralph Loop, Chief's core execution model that drives Claude to autonomously complete user stories one by one.
---

# The Ralph Loop

The Ralph Loop is Chief's core execution model: a continuous cycle that drives Claude to complete user stories one by one. It's the engine that makes autonomous development possible.

::: tip Background Reading
For the motivation and philosophy behind this approach, read the blog post [Ship Features in Your Sleep with Ralph Loops](https://larswadefalk.com/ship-features-in-your-sleep-with-ralph-loops/).
:::

## The Loop Visualized

Here's the complete Ralph Loop as a flowchart:

```
    ┌─────────────┐
    │ Start Chief │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  Show TUI   │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ Press 's'   │◀─────────────────────────────────────┐
    └──────┬──────┘                                      │
           │                                             │
           ▼                                             │
    ┌─────────────┐                                      │
    │ Read State  │                                      │
    └──────┬──────┘                                      │
           │                                             │
           ▼                                             │
    ╔═════════════╗      all complete    ┌────────────┐  │
    ║ Next Story? ║─────────────────────▶│  ✓ Done    │  │
    ╚══════╤══════╝                      └────────────┘  │
           │ story found                        ▲        │
           ▼                                    │        │
    ┌─────────────┐                             │        │
    │Build Prompt │                             │        │
    └──────┬──────┘                             │        │
           │                                    │        │
           ▼                                    │        │
    ┌─────────────┐                             │        │
    │Invoke Claude│                             │        │
    └──────┬──────┘                             │        │
           │                                    │        │
           ▼                                    │        │
    ┌─────────────┐    <chief-complete/>        │        │
    │Stream Output├────────────────────────────▶┘        │
    └──────┬──────┘                                      │
           │ session ends                                │
           ▼                                             │
    ╔═════════════╗       no            ┌────────────┐   │
    ║ Max Iters?  ║────────────────────▶│  Continue  │───┘
    ╚══════╤══════╝                     └────────────┘
           │ yes
           ▼
    ┌─────────────┐
    │  ✗ Stop     │
    └─────────────┘
```

## Before the Loop: Worktree Setup

Before the loop starts, Chief sets up the working environment. When you press `s` to start a PRD, the TUI shows a dialog offering to create an isolated worktree:

1. **Create branch** — A new branch (e.g., `chief/auth-system`) is created from your default branch
2. **Create worktree** — A git worktree is set up at `.chief/worktrees/<prd-name>/`
3. **Run setup** — If a setup command is configured (e.g., `npm install`), it runs in the worktree

This setup happens once per PRD. The loop then runs entirely within the worktree directory, isolating all file changes and commits to that branch.

You can also skip worktree creation and run in the current directory if you prefer.

## Step by Step

Each step in the loop has a specific purpose. Here's what happens in each one.

### 1. Read State

Chief reads all the files it needs to understand the current situation:

| File | What Chief Learns |
|------|-------------------|
| `prd.json` | Which stories are complete (`passes: true`), which are pending, and which is in progress |
| `progress.md` | What happened in previous iterations: learnings, patterns, and context |
| Codebase files | Current state of the code (via Claude's file reading) |

This step ensures Claude always has fresh, accurate information about what's done and what's left to do.

### 2. Select Next Story

Chief picks the next story to work on by looking at `prd.json`:

1. Find all stories where `passes: false`
2. Sort by `priority` (lowest number = highest priority)
3. Pick the first one

If a story has `inProgress: true`, Chief continues with that story instead of starting a new one. This handles cases where Claude was interrupted mid-story.

### 3. Build Prompt

Chief constructs a prompt that tells Claude exactly what to do. The prompt includes:

- **The user story**: ID, title, description, and acceptance criteria
- **Instructions**: Read the PRD, pick the next story, implement it, run checks, commit
- **Progress context**: Any patterns or learnings from `progress.md`

Here's a simplified version of what Claude receives:

```markdown
## Your Task

1. Read the PRD at `.chief/prds/your-prd/prd.json`
2. Read `progress.md` if it exists (check Codebase Patterns first)
3. Pick the highest priority story where `passes: false`
4. Mark it as `inProgress: true` in the PRD
5. Implement that single user story
6. Run quality checks (typecheck, lint, test)
7. If checks pass, commit with message: `feat: [Story ID] - [Story Title]`
8. Update the PRD to set `passes: true` and `inProgress: false`
9. Append your progress to `progress.md`
```

The prompt is embedded directly in Chief's code. There's no external template file to manage.

### 4. Invoke Claude Code

Chief runs Claude Code via the CLI, passing the constructed prompt:

```
claude --dangerously-skip-permissions --output-format stream-json
```

The flags tell Claude to:
- Skip permission prompts (Chief runs unattended)
- Output structured JSON for parsing

Claude now has full control. It can read files, write code, run tests, and commit changes, all autonomously.

### 5. Stream & Parse Output

As Claude works, it produces a stream of JSON messages. Chief parses this stream in real-time using a streaming JSON parser. This is what allows the TUI to show live progress.

Here's what the output stream looks like:

```
┌─────────────────────────────────────────────────────────────┐
│  Claude's Output Stream (stream-json format)                │
├─────────────────────────────────────────────────────────────┤
│  {"type":"text","content":"Reading prd.json..."}            │
│  {"type":"tool_use","name":"Read","input":{...}}            │
│  {"type":"text","content":"Found story US-012..."}          │
│  {"type":"tool_use","name":"Write","input":{...}}           │
│  {"type":"text","content":"Running tests..."}               │
│  {"type":"tool_use","name":"Bash","input":{...}}            │
│  {"type":"text","content":"Story complete, committing..."}  │
└─────────────────────────────────────────────────────────────┘
```

Each message contains:
- **type**: What kind of output (text, tool_use, etc.)
- **content**: The actual output or tool details

Chief parses this stream to display progress in the TUI. When Claude's session ends, Chief checks if the story was completed (by reading the updated PRD) and continues the loop.

### 6. The Completion Signal

When Claude determines that **all stories are complete**, it outputs a special marker:

```
<chief-complete/>
```

This signal tells Chief to break out of the loop early. There's no need to spawn another iteration just to discover there's nothing left to do. It's an optimization, not the primary mechanism for tracking story completion.

Individual story completion is tracked through the PRD itself (`passes: true`), not through this signal.

### 7. Continue the Loop

After each Claude session ends, Chief:

1. Increments the iteration counter
2. Checks if max iterations is reached
3. If not at limit, loops back to step 1 (Read State)

The next iteration starts fresh. Claude reads the updated PRD, sees the completed story, and picks the next one. If all stories are done, Chief stops.

## Iteration Limits

Chief has a safety limit on iterations to prevent runaway loops. When `--max-iterations` is not specified, the limit is calculated dynamically based on the number of remaining stories plus a buffer. You can also adjust the limit at runtime with `+`/`-` in the TUI.

| Scenario | What Happens |
|----------|--------------|
| Story completes normally | Iteration counter goes up by 1, loop continues |
| Story takes multiple Claude sessions | Each Claude invocation is 1 iteration |
| Limit reached | Chief stops and displays a message |

If you hit the limit, it usually means:
- A story is too complex and needs to be broken down
- Claude is stuck in a loop (check `claude.log`)
- There's an issue with the PRD format

You can adjust the limit with the `--max-iterations` flag or in your configuration.

## Post-Completion Actions

When all stories in a PRD are complete, Chief can automatically:

1. **Push the branch** — If `onComplete.push` is enabled in `.chief/config.yaml`, Chief pushes the branch to origin
2. **Create a pull request** — If `onComplete.createPR` is also enabled, Chief creates a PR via the `gh` CLI with a title and body generated from the PRD

The completion screen shows the progress of these actions with spinners, checkmarks, or error messages. On PR success, the PR URL is displayed and clickable.

If auto-actions aren't configured, the completion screen shows a hint to configure them via the Settings TUI (`,`).

You can also take manual actions from the completion screen:
- `m` — Merge the branch locally
- `c` — Clean up the worktree
- `l` — Switch to another PRD
- `q` — Quit Chief

## Why "Ralph"?

The name comes from [Ralph Wiggum loops](https://ghuntley.com/ralph/), a pattern coined by Geoffrey Huntley. The idea: instead of fighting context window limits with one long session, you run the AI in a loop. Each iteration starts fresh but reads persisted state from the previous run.

Chief's implementation was inspired by [snarktank/ralph](https://github.com/snarktank/ralph), an early proof-of-concept that demonstrated the pattern in practice.

## What's Next

- [The .chief Directory](/concepts/chief-directory): Where all this state lives
- [PRD Format](/concepts/prd-format): How to write effective user stories
- [CLI Reference](/reference/cli): Running Chief with different options
