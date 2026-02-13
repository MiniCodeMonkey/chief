# Chief Roadmap Brainstorm

Where chief goes from here. Ideas are grouped by theme, roughly ordered by potential impact within each section. Nothing is committed — this is a map of the possibility space.

---

## Architectural Principle: Claude Code CLI Only

Chief's core identity is as an orchestrator of the **Claude Code CLI**. It does not call the Anthropic API directly — everything goes through `claude`. This is intentional and should remain a hard constraint:

- **Simplicity**: Chief doesn't manage API keys, token counting, model routing, or streaming. Claude Code handles all of that.
- **Feature leverage**: Every improvement to Claude Code (new tools, better context management, sub-agents, prompt caching) benefits chief for free.
- **Auth delegation**: Users authenticate with Claude Code once. Chief never touches credentials.
- **Permission model**: Claude Code's permission system (and `--dangerously-skip-permissions`) is the trust boundary. Chief doesn't add its own.

This constraint has real implications for features like the web app and cloud execution — see those sections for how to work within it.

---

## Community Requests (GitHub Issues)

Two open issues from the community that should inform priorities:

### [#4] In-TUI diff viewer (requested by @ddaan)

> "Whenever I open up my terminal where chief is cooking, I would like to see the results of the diff, and the notes of what was implemented."

This is the first real feature request and it's a great one. The user wants a lazygit-style diff view integrated into the TUI — see what changed per story, see the implementation notes, all without leaving chief. This maps directly to the **Observability** theme below. See section 5 for the full design.

### [#3] Issue management systems as provider (requested by @kasperhartwich)

> "You should consider support different ticket systems as a provider you can configure."

Users want to pull stories from GitHub Issues, Linear, Jira, etc. instead of writing PRDs from scratch. This maps to the **Smarter PRDs > PRD from issues/tickets** section below. The pluggable provider model is a natural extension of chief's architecture.

---

## 1. Accessibility: Reaching Non-Terminal Users

Chief's power comes from orchestrating Claude Code, but the entry point today is `chief new` in a terminal. That's a barrier for less technical teammates (designers, PMs, founders) and for anyone who wants to draft a PRD away from their workstation.

### Web companion app

A web UI for PRD authoring and progress monitoring. The key constraint: **chief only uses Claude Code CLI, never the Anthropic API directly.** This shapes the architecture significantly.

**Tier 1: No AI needed (pure editor + dashboard)**

The simplest version doesn't need AI at all:

- **PRD markdown editor with live preview**: Markdown editor on the left, rendered preview on the right. Syntax-aware for the US-XXX format — validates structure, flags missing acceptance criteria, warns about overly large stories. This is just a specialized markdown editor. No Claude needed.
- **Template gallery**: Curated starting points — "SaaS app", "CLI tool", "API service", "mobile app". Pre-written PRD skeletons that users fill in. No AI, just good defaults.
- **Export to `.chief/`**: Download as a zip or push to a connected git repo. The result is a `.chief/prds/<name>/` directory ready for `chief` to run.
- **Progress dashboard**: Read-only view showing progress of running chief instances. Reads `prd.json` from a git repo or shared filesystem. Shows story completion, current iteration, elapsed time. No Claude needed — just reads JSON.
- **Team collaboration**: Multiple people view/edit the same PRD. Comments on stories, suggested edits. Google Docs for PRDs.

This tier is a pure web app. No API keys, no backend AI calls. A PM or designer could use it to write PRDs that a developer then runs with `chief`.

**Tier 2: AI-assisted PRD writing (needs Claude Code access)**

To get the interactive `chief new` experience in the browser, we need Claude Code somewhere. Options:

- **Option A: Claude OAuth / connect to Claude Code on the web.** Claude Code is available in the browser (claude.ai/code). If Claude offers an OAuth flow or embed mechanism, the web app could initiate a Claude Code session with the `init_prompt.txt` pre-loaded. The user's own Claude account handles auth and billing. Chief's web app is just the launcher.
- **Option B: Local bridge.** Run a lightweight `chief serve` process locally that exposes a WebSocket API. The web app connects to `localhost:PORT`. When the user asks for AI-assisted PRD writing, the bridge spawns `claude -p "..." --output-format stream-json` and streams results back to the browser. Claude Code CLI handles auth. This is the cleanest fit with the "CLI only" principle — the web app is just a pretty frontend for the same CLI.
- **Option C: Bring your own API key (escape hatch).** Users provide an Anthropic API key in the web app. The backend calls Claude's API directly for PRD generation only. This breaks the "CLI only" principle but might be the pragmatic choice for a hosted web app where there's no local CLI to shell out to. Should be a last resort.

**Recommendation**: Start with Tier 1 (no AI, pure editor + dashboard). It's useful immediately and doesn't require solving the auth problem. Add Tier 2 via the local bridge (`chief serve`) since it preserves the CLI-only architecture. Explore Claude OAuth as it becomes available.

### Mobile app (iOS/Android or PWA)

For PRD drafting on the go. You're on a train and have an idea — open the app, describe it, structure it into stories. When you get to your desk, the PRD is waiting.

- Best as a progressive web app (PWA) wrapping the Tier 1 web editor — avoids app store complexity
- AI features would require the bridge or OAuth approaches from above
- Push notifications when chief completes a PRD back on your server

**Consideration**: This only makes sense if the web companion exists. Mobile would be a thin responsive wrapper. Start with the web app.

### Desktop app (Tauri)

- Wraps the web UI + runs `chief serve` locally for the bridge
- One-click install for non-technical users
- Could bundle Claude Code CLI installation
- System tray icon showing chief status
- Native notifications on completion
- **Key advantage over web-only**: Has direct access to the local filesystem and Claude Code CLI, so the bridge is built in

**Consideration**: Tauri keeps it lightweight. The real question is whether this adds enough over "web app + terminal" to justify the maintenance. But it's the cleanest path to AI-assisted PRD writing since the local CLI is right there.

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
- Chief runs in the cloud (managed Docker containers with Claude Code CLI installed)
- Watch progress in the web dashboard
- Results pushed to a branch in your repo

**Auth challenge**: Claude Code CLI needs to be authenticated in the cloud containers. Options: (a) Claude OAuth flow where users authorize the service to act on their behalf, (b) users provide their own API key that gets passed to the container's Claude Code config, (c) a service-level Anthropic API key with usage-based billing. All options keep chief using the CLI — the key just configures Claude Code's auth, not chief's.

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

### Issue management systems as providers — [GH #3]

Import work from existing project management tools. Rather than hardcoding each integration, design a pluggable provider model.

- **Provider interface**: Define a clean interface — `ListIssues()`, `GetIssue(id)`, `UpdateIssue(id, status)`. Each provider implements this.
- **GitHub Issues provider**: `chief import --from github --label chief` — pulls issues with a specific label, Claude converts them into a PRD. Each issue becomes one or more user stories.
- **Linear provider**: `chief import --from linear --project X` — same flow for Linear. Linear's richer metadata (estimates, cycles) could inform story priority and sizing.
- **Jira provider**: Enterprise users often live in Jira. A Jira provider would widen chief's reach significantly.
- **Clipboard/URL provider**: `chief import --from url <spec-url>` or `chief import --from clipboard` — paste a feature spec from anywhere, Claude converts it.
- **Bidirectional sync**: When a story completes, update the corresponding issue/ticket. Close GitHub issues, move Linear tickets to "Done", transition Jira tickets.
- **Continuous sync mode**: Watch for new issues with a label and automatically create/update PRD stories. Chief becomes a continuously-fed work queue.

**Implementation thought**: The provider interface could live in `internal/provider/`. Each provider is a separate file. The import command uses Claude (similar to `convert_prompt.txt`) to transform issue content into the PRD markdown format. Config could be in `chief.yaml` or CLI flags: `provider: github` with `label: chief`.

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

Claude Code CLI supports `--model`, so chief can control this without touching the API directly.

- **Use cheaper models for simple tasks**: PRD conversion (`convert_prompt.txt`) doesn't need a frontier model. Pass `--model claude-haiku-4-5-20251001` for conversion, JSON fixing, and other mechanical tasks. Easy change in `generator.go`.
- **Configurable model per stage**: `--prd-model haiku --loop-model sonnet` or similar. Chief maps these to `claude --model <id>` flags.
- **Auto-escalation**: Start with a cheaper model. If a story fails, retry with `--model` set to a more capable one. Chief already has retry logic in `loop.go` — extend it to escalate the model on retry.

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

### In-TUI diff viewer — [GH #4]

This is the most requested feature. Users want to see what chief is doing without leaving the TUI.

- **Per-story diff view**: New view mode (`d` key) that shows the cumulative git diff for the selected story. Uses the conventional commit format (`feat: [US-XXX] - Title`) to identify which commits belong to which story.
- **Live diff during execution**: While a story is in progress, show the working tree diff (uncommitted changes). Updates in real-time as Claude writes files.
- **Implementation notes panel**: Alongside the diff, show the corresponding progress.md entry for that story — what was implemented, files changed, learnings.
- **Lazygit-style navigation**: File list on the left, diff on the right. `j`/`k` to navigate files, `enter` to expand/collapse, syntax highlighting via chroma (already a dependency).
- **Summary view**: A compact view showing stats per story: files changed, lines added/removed, which files were touched.

**Implementation thought**: The git integration already exists in `internal/git/`. The main work is a new TUI view component. Could use `git diff <pre-story-sha>...<post-story-sha>` per story, or `git show <commit>` for each story commit. The commit message convention makes this straightforward to parse.

### Other progress visualization

- **Gantt-style timeline**: Show a timeline of story execution. Which stories took how many iterations, where retries happened, idle time between iterations.
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

Mapping these ideas by impact vs. effort. Community requests ([GH #3], [GH #4]) are weighted higher because they represent validated demand.

### High impact, moderate effort (do first)
- **In-TUI diff viewer** [GH #4] — most requested feature, builds trust by showing what chief is doing, moderate TUI work with existing git infra
- **Docker-based execution** — addresses the #1 concern (safety)
- **Codebase-aware PRD generation** — makes PRDs better with minimal architecture change
- **Token/cost tracking** — gives users confidence and control
- **Headless mode** — unlocks remote execution, web UI, and clean architecture

### High impact, high effort (plan carefully)
- **Issue management providers** [GH #3] — pluggable provider model is a real architecture decision, but GitHub Issues alone would be a great start
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

4. **What's the best auth model for the web app?** The CLI-only principle means the web app can't just call the Anthropic API. The local bridge (`chief serve`) works for local use. For hosted/cloud use, Claude OAuth would be ideal if available. Is "bring your own API key" an acceptable escape hatch for a hosted version, or does it violate the principle too much?

5. **What's the monetization path?** Chief is MIT-licensed. Cloud-hosted execution, premium PRD templates, team collaboration, and enterprise features (SSO, audit logs, managed execution) are all options. Does the project want to stay fully open source, or is a commercial layer planned?

6. **How do we measure PRD quality?** We know a PRD is "good" if it completes in few iterations with correct code. Can we build a feedback loop that improves PRD generation based on execution outcomes?
