# Chief Roadmap Brainstorm

Where chief goes from here. Ideas are grouped by theme, roughly ordered by potential impact within each section. Nothing is committed — this is a map of the possibility space.

---

## 1. Accessibility: Reaching Non-Terminal Users

Chief's power comes from orchestrating Claude Code, but the entry point today is `chief new` in a terminal. That's a barrier for less technical teammates (designers, PMs, founders) and for anyone who wants to draft a PRD away from their workstation.

### Web companion app

A lightweight web UI for PRD authoring only. Not a full chief replacement — just the "describe what you want" part.

- **PRD editor with live preview**: Markdown editor on the left, rendered preview on the right. Syntax-aware for the US-XXX format so it can validate structure, flag missing acceptance criteria, warn about overly large stories.
- **AI-assisted PRD writing**: Embed Claude directly in the web editor. User describes what they want conversationally, Claude generates the structured PRD. Same `init_prompt.txt` logic but via API instead of CLI.
- **Template gallery**: Curated starting points — "SaaS app", "CLI tool", "API service", "mobile app", "Chrome extension". Each comes with sensible default story structures that users customize. Lowers the blank-page problem.
- **Export to `.chief/`**: Download as a zip or push to a connected git repo. The result is a `.chief/prds/<name>/` directory ready for `chief` to run.
- **Team collaboration**: Multiple people can view/edit the same PRD. Think Google Docs for PRDs. Comments on stories, suggested edits, approval workflows.
- **Dashboard view**: Read-only view that shows progress of running chief instances. Polls a running chief process or reads `prd.json` from a remote repo to show story completion status, current iteration, etc.

**Implementation thought**: This could be a simple Next.js or Vite app. The PRD format is just markdown + JSON — no complex backend needed for the editor. The AI features would need an Anthropic API key. The progress dashboard could just poll git or read from a shared filesystem.

### Mobile app (iOS/Android or PWA)

For PRD drafting on the go. You're on a train and have an idea — open the app, describe it, Claude helps structure it into stories. When you get to your desk, the PRD is waiting.

- Could be a progressive web app (PWA) to avoid app store complexity
- Voice-to-PRD: dictate your idea, Claude transcribes and structures it
- Push notifications when chief completes a PRD back on your server

**Consideration**: This only makes sense if the web companion exists. Mobile would be a thin wrapper. Start with the web app and make it responsive.

### Desktop app (Electron/Tauri)

- Wraps the web UI + a local chief process
- One-click install for non-technical users
- Could bundle Claude Code CLI installation
- System tray icon showing chief status
- Native notifications on completion

**Consideration**: Tauri would keep it lightweight (vs Electron). But the real question is whether this adds enough over "web app + terminal" to justify the maintenance. Probably lower priority than web.

---

## 2. Sandboxed / Remote Execution

Today chief runs `claude --dangerously-skip-permissions` on your local machine. That's the #1 concern people have. There are several paths to reduce risk.

### Docker-based execution

Run each Claude iteration inside a disposable container. Chief becomes the orchestrator outside the container.

- **Per-iteration containers**: Spin up a fresh container for each story. Mount the project directory. Claude runs inside. Container is destroyed after. If Claude does something destructive, it only affects the mounted volume.
- **Pre-built images**: Provide `chief/node`, `chief/python`, `chief/go` images with common toolchains pre-installed. Users can extend with their own Dockerfile.
- **`chief run --docker`**: Single flag to switch to containerized mode. Chief handles the container lifecycle transparently.
- **Network isolation**: Containers run with no network by default. Opt-in network access for projects that need `npm install` etc.
- **Resource limits**: CPU/memory limits per container to prevent runaway processes.

**Implementation thought**: The main change is in `loop.go` — instead of `exec.Command("claude", ...)`, you'd do `exec.Command("docker", "run", ..., "claude", ...)`. The rest of the architecture stays the same. The tricky part is making the Claude CLI authentication work inside the container (mount the auth token).

### Cloud/remote execution via SSH

Run chief on a remote machine while viewing the TUI locally.

- **`chief remote <host>`**: SSH into a remote machine, run chief there, forward the TUI back. Like `ssh -t host chief`.
- **Headless mode**: `chief run --headless` outputs JSON events to stdout. A local TUI connects and renders them. Decouples execution from display.
- **Persistent sessions**: Use tmux/screen on the remote so chief survives disconnects.

**Implementation thought**: The headless mode is the real building block. Once chief can emit events to stdout and accept commands via stdin, any frontend can drive it — local TUI, web UI, remote TUI, etc. This is a clean separation that enables multiple features at once.

### Cloud-hosted execution (managed service)

A hosted version of chief where users don't need to install anything locally.

- Upload or connect a git repo
- Write/edit PRD in the web UI
- Chief runs in the cloud (managed Docker containers)
- Watch progress in the web dashboard
- Results pushed to a branch in your repo

**Consideration**: This is a big leap — it's essentially a product, not a tool. Could be a future monetization path but is a significant investment. The Docker + headless work unlocks this incrementally.

---

## 3. Smarter PRDs

The PRD generation step has huge leverage. Better PRDs lead to fewer iterations, less wasted tokens, and better code.

### Codebase-aware PRD generation

Today `chief new` doesn't know much about the existing project. Claude asks questions, but it's flying blind.

- **Auto-scan on `chief new`**: Before generating the PRD, chief runs a lightweight codebase scan — file tree, package.json/go.mod, existing patterns, test setup, linting config. This context gets injected into the init prompt.
- **Existing pattern detection**: "This project uses React + TypeScript + Tailwind + Vitest" — inform story generation so acceptance criteria match the actual stack.
- **Dependency-aware story ordering**: If the project has no test setup, automatically insert a "set up testing" story before stories that require tests.
- **Smart story sizing**: Analyze codebase complexity to estimate story size. Flag stories that are likely too large for a single Claude session and suggest splitting.

**Implementation**: Add a `scan` step to `chief new` that runs before Claude. Could use `tree`, read `package.json`, detect frameworks. Inject results as `{{CODEBASE_CONTEXT}}` in `init_prompt.txt`.

### PRD validation and linting

Catch problems before the loop starts.

- **`chief lint`**: Validate PRD structure — missing acceptance criteria, duplicate IDs, stories without clear "done" criteria, circular dependencies.
- **Story dependency graph**: Let stories declare dependencies (`dependsOn: ["US-001"]`). Chief validates the graph is acyclic and executes in dependency order.
- **Complexity scoring**: Estimate how many iterations each story might take. Warn if the total seems high.
- **"Dry run" mode**: `chief run --dry-run` — Claude reads the PRD and reports what it would do for each story without actually doing it. Helps validate the PRD makes sense before spending tokens.

### PRD from issues/tickets

Import work from existing project management tools.

- **`chief import github`**: Convert GitHub issues (with a label like `chief`) into a PRD.
- **`chief import linear`**: Same for Linear tickets.
- **`chief import clipboard`**: Paste a feature spec from anywhere, Claude converts it.
- **Sync back**: When a story completes, update the corresponding GitHub issue / Linear ticket.

### Iterative PRD refinement

- **Post-completion review**: After all stories pass, chief runs one more Claude session to review the full implementation against the original PRD. Reports gaps.
- **Auto-expand stories**: If a story fails repeatedly (>3 iterations), chief automatically suggests breaking it into smaller stories.
- **Learning from history**: Analyze past PRDs and their iteration counts to improve future story sizing.

---

## 4. Faster & Cheaper Loops

Token efficiency directly affects cost and speed. The current architecture is clean but there are optimization opportunities.

### Smarter context management

- **Selective progress.md**: Instead of appending everything, curate what goes into progress.md. Move "Codebase Patterns" to a separate file that's always read. Trim per-story entries after N iterations to keep the file from growing unbounded.
- **Focused file context**: Before each iteration, generate a "relevant files" list for the upcoming story. Pass this to Claude so it reads fewer irrelevant files.
- **Story-specific CLAUDE.md**: Generate a temporary CLAUDE.md for each story with relevant patterns, file locations, and gotchas. Delete after the story passes. This leverages Claude Code's built-in project context feature.

### Sub-agent architecture

Claude Code supports sub-agents (the Task tool). Chief could leverage this.

- **Parallel story execution**: For stories without dependencies, run multiple Claude Code sub-agents simultaneously. Chief already has the Manager for parallel PRDs — extend this to parallel stories within a PRD.
- **Specialized sub-agents**: Instead of one Claude session doing everything (implement, test, lint, commit), break it into:
  - Implementation agent: writes code
  - Review agent: reviews the implementation
  - Test agent: writes and runs tests
  - Quality agent: runs linting/formatting
  - This could catch more issues per iteration and reduce the "implement, find test failure, re-implement" cycle.
- **Research sub-agent**: Before implementing a story, a lightweight sub-agent scans the codebase and gathers context. Passes a focused summary to the implementation agent, reducing unnecessary file reads.

**Implementation thought**: This mainly affects `prompt.txt`. Instead of a single flat instruction, the prompt could instruct Claude to use sub-agents. Chief itself doesn't need to change much — it's Claude using Claude. The key is prompt engineering.

### Model selection

- **Use cheaper models for simple tasks**: PRD conversion (`convert_prompt.txt`) doesn't need a frontier model. Use Haiku or a smaller model for conversion, JSON fixing, and other mechanical tasks.
- **Configurable model per stage**: `--prd-model haiku --loop-model sonnet` or similar.
- **Auto-escalation**: Start with a cheaper model. If a story fails, retry with a more capable one.

### Caching and checkpointing

- **Git checkpoint per story**: Already happens (commits). But add the ability to revert a failed story's changes before retrying: `git reset --hard` to the pre-story commit.
- **Prompt caching**: Claude's prompt caching can reduce costs for repeated context. The PRD content and system prompt are the same across iterations — ensure they hit the cache.
- **Skip completed stories faster**: Today Claude reads the PRD and picks the next story. If chief pre-selected the story and passed it directly (instead of making Claude scan the full PRD), each iteration would start faster.

---

## 5. Observability & Debugging

When chief runs for 30+ iterations, understanding what happened is hard. Better observability helps users trust the system and debug issues.

### Token and cost tracking

- **Per-iteration token counts**: Parse Claude's stream-json output for usage data. Show tokens in/out per iteration.
- **Running cost estimate**: "This PRD has used ~$X.XX so far" in the dashboard.
- **Cost alerts**: Warn if a single story is consuming unusually many tokens.

### Better progress visualization

- **Gantt-style timeline**: Show a timeline of story execution. Which stories took how many iterations, where retries happened, idle time between iterations.
- **Diff viewer**: For each story, show the cumulative git diff. "Here's what story US-003 changed."
- **Token sparkline**: Small inline chart next to each story showing token usage over iterations.

### Structured logging

- **Replace claude.log with structured events**: Instead of raw Claude output, emit structured JSON events: `{"type": "story_start", "story": "US-003", "timestamp": "..."}`. Makes it parseable and queryable.
- **Event export**: `chief export-log --format json` for integration with external tools.
- **Log rotation**: Auto-rotate logs after N MB to prevent unbounded growth.

### Error analysis

- **Failure classification**: When a story fails, classify why — test failure, lint error, compilation error, Claude confused, context limit hit. Show this in the dashboard.
- **Suggested fixes**: If a story keeps failing, suggest common remedies: "This story has failed 3 times on test errors. Consider splitting it or adding test setup as a prerequisite story."

---

## 6. Workflow & Integration

Chief is a standalone tool today. Integrating it into existing development workflows would increase its utility.

### CI/CD integration

- **GitHub Action**: `uses: minicodemonkey/chief-action@v1` — run chief in CI. Useful for automated feature implementation from issue templates.
- **PR creation**: After chief completes, automatically create a PR with the changes and a summary of what was implemented.
- **Review integration**: When a PR from chief gets review comments, chief could pick them up and address them (new iteration with review feedback as context).

### Git workflow improvements

- **Automatic branch creation**: Instead of warning about main branch, always create a feature branch: `chief/<prd-name>`.
- **Squash option**: `chief run --squash` — after completion, squash all story commits into one clean commit.
- **Interactive rebase**: `chief run --rebase` — after completion, clean up the commit history.
- **PR description generation**: Auto-generate a PR description from the PRD + progress.md.

### IDE integration

- **VS Code extension**: Show chief status in the sidebar. View stories, see progress, start/stop loops. Essentially the TUI dashboard in VS Code.
- **JetBrains plugin**: Same for IntelliJ/WebStorm users.
- **File annotations**: In the editor, annotate files with which story last modified them.

### Webhook/notification integration

- **Slack notifications**: Post to a channel when chief starts, completes, or fails.
- **Webhook support**: `chief run --webhook https://...` — POST events to a URL.
- **Discord integration**: Bot that reports chief status.
- **Email summary**: Send a completion email with what was implemented.

---

## 7. PRD Management & Organization

As people use chief for more projects, managing PRDs becomes important.

### PRD archiving and history

- **`chief archive <name>`**: Move completed PRDs to an archive directory. Keep `.chief/prds/` clean.
- **PRD versioning**: Track versions of a PRD as it's edited. Git-based — each edit is a commit to the PRD file.
- **Completion reports**: When a PRD finishes, generate a summary report: what was built, how many iterations, token usage, time elapsed.

### PRD composition

- **Parent/child PRDs**: Large projects could have a top-level PRD that references sub-PRDs. Chief orchestrates them in order.
- **Shared stories**: Common setup stories (test framework, CI config) that can be referenced across PRDs.
- **PRD templates**: Save a completed PRD as a template for similar future projects.

### Multi-project management

- **Global dashboard**: `chief dashboard` — shows all projects with active PRDs across your machine.
- **Project registry**: Track which directories have `.chief/` folders.

---

## 8. Quality & Reliability

Making chief and the code it produces more reliable.

### Smarter quality checks

- **Configurable quality gates**: Let users define what "passes" means per project. Not just "tests pass" but custom scripts: `chief.yaml` with `quality_checks: ["npm test", "npm run lint", "npm run typecheck"]`.
- **Screenshot testing**: For frontend stories, take a screenshot after implementation and include it in progress.md. Visual regression detection.
- **Security scanning**: Run a lightweight security scan after each story. Flag if Claude introduced a known vulnerability pattern.

### Rollback and recovery

- **`chief rollback <story>`**: Revert a specific story's changes if they're problematic.
- **Automatic rollback on failure**: If a story fails quality checks, automatically revert its changes before the next iteration attempts it again.
- **State snapshot**: Before each iteration, snapshot the full state (git sha, prd.json, progress.md) so any point can be restored.

### Test infrastructure

- **Better test coverage for chief itself**: Add integration tests that run mini PRDs and verify the output.
- **Replay mode**: Record a chief run and replay it for testing without hitting Claude's API.

---

## 9. Community & Ecosystem

### PRD sharing

- **PRD registry**: A place to share and discover PRDs. "Here's a PRD that adds authentication to a Next.js app" — others can fork and customize.
- **PRD marketplace**: Curated, high-quality PRDs that reliably produce good results.

### Plugin system

- **Hook points**: Before/after PRD creation, before/after each iteration, before/after each story. Users can run custom scripts at these points.
- **Custom prompts**: Allow users to override `prompt.txt`, `init_prompt.txt`, etc. with project-specific versions. For example, a project could add domain-specific instructions.
- **MCP (Model Context Protocol) integration**: Let users add MCP servers that Claude can use during iterations. Database access, API calls, custom tools.

---

## Priority Assessment

Mapping these ideas by impact vs. effort:

### High impact, moderate effort (do first)
- **Docker-based execution** — addresses the #1 concern (safety)
- **Codebase-aware PRD generation** — makes PRDs better with minimal architecture change
- **Token/cost tracking** — gives users confidence and control
- **Headless mode** — unlocks remote execution, web UI, and clean architecture

### High impact, high effort (plan carefully)
- **Web companion for PRD authoring** — broadens audience significantly
- **Sub-agent architecture** — could dramatically improve per-iteration quality
- **CI/CD integration (GitHub Action)** — unlocks automation workflows

### Moderate impact, low effort (quick wins)
- **PRD validation/linting** — catches errors before wasting tokens
- **Configurable quality gates** — chief.yaml with custom check commands
- **Story-specific CLAUDE.md** — better per-story context with minimal code change
- **Selective progress.md** — cap file size, separate patterns from history
- **Auto-branch creation** — small UX improvement
- **Cheaper model for conversion** — use Haiku for prd.md to prd.json

### Lower priority (future exploration)
- Mobile app (wait for web app)
- Desktop app (questionable ROI)
- Cloud-hosted managed service (big investment)
- PRD registry / marketplace (needs community first)
- IDE plugins (nice-to-have)

---

## Open Questions

1. **Should the web UI be a separate project or part of this repo?** Separate keeps chief simple. Monorepo keeps everything coordinated.

2. **How much should chief know about Claude Code internals?** Today it treats Claude as a black box (pass prompt, get output). Should it do more — like pre-selecting files, managing context windows, or orchestrating sub-agents directly?

3. **What's the right model for configuration?** Zero-config is a strength. But Docker, quality gates, model selection, and webhooks need config. A `chief.yaml` in the project root? CLI flags only? Both?

4. **Should chief support non-Claude models?** The architecture could work with any coding agent that accepts a prompt and outputs to stdout. Is there value in model-agnostic execution, or does tight Claude integration matter more?

5. **What's the monetization path?** Chief is MIT-licensed. Cloud-hosted execution, premium PRD templates, team collaboration, and enterprise features (SSO, audit logs, managed execution) are all options. Does the project want to stay fully open source, or is a commercial layer planned?

6. **How do we measure PRD quality?** We know a PRD is "good" if it completes in few iterations with correct code. Can we build a feedback loop that improves PRD generation based on execution outcomes?
