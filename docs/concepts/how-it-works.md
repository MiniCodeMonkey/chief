# How Chief Works

Chief is an autonomous PRD agent that transforms your product requirements into working code—without constant back-and-forth prompting.

::: tip Background
For the motivation behind Chief and a deeper exploration of autonomous coding agents, read the blog post: [Introducing Chief: Autonomous PRD Agent](https://minicodemonkey.com/blog/2025/chief)
:::

## The Core Concept

Traditional AI coding assistants require constant interaction. You prompt, Claude responds, you prompt again. It's collaborative, but it's not autonomous.

Chief takes a different approach: **define what you want upfront, then step back and watch it happen.**

You write a Product Requirements Document (PRD) describing what you want to build, broken into user stories. Chief reads the PRD, invokes Claude Code, and orchestrates the entire process—one story at a time.

## The Flow

```
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│   You    │───▶│   PRD    │───▶│  Chief   │───▶│  Claude  │───▶│   Code   │
│  Write   │    │  (.json) │    │  (Loop)  │    │  (Agent) │    │ (Commits)│
└──────────┘    └──────────┘    └──────────┘    └──────────┘    └──────────┘
```

Here's what each component does:

| Component | Role |
|-----------|------|
| **You** | Write the PRD with user stories and acceptance criteria |
| **PRD** | Machine-readable spec that defines what needs to be built |
| **Chief** | Orchestrator that manages the loop and tracks progress |
| **Claude** | AI agent that reads context, writes code, runs tests, and commits |
| **Code** | The end result—working code committed to your repository |

## One Iteration, One Story

Chief works through your PRD methodically. Each "iteration" focuses on a single user story:

1. **Read State** — Chief examines `prd.json` to find the highest-priority story where `passes: false`
2. **Build Prompt** — Constructs a prompt with instructions, the story details, and project context
3. **Invoke Claude** — Spawns Claude Code with the assembled prompt
4. **Execute** — Claude reads files, writes code, runs tests, and fixes issues until the story is complete
5. **Commit** — Claude commits the changes with a conventional commit message like `feat: [US-001] - Feature Title`
6. **Update PRD** — Marks the story as `passes: true` and records progress
7. **Repeat** — Chief checks for more incomplete stories and continues

This isolation is intentional. If something breaks, you know exactly which story caused it. Each commit represents one complete feature.

## Conventional Commits

Every completed story results in a well-formed commit:

```
feat: [US-003] - Add user authentication

- Implemented login/logout endpoints
- Added JWT token validation
- Created auth middleware
```

Your git history becomes a timeline of features, matching 1:1 with your PRD stories.

## Progress Tracking

Chief maintains a `progress.md` file in each PRD directory. After every iteration, Claude appends:

- What was implemented
- Which files changed
- Learnings for future iterations (patterns discovered, gotchas, context)

This creates institutional memory. Later iterations (and future developers) can reference this to understand decisions and avoid repeating mistakes.

## Why This Works

The autonomous approach enables things that interactive prompting can't:

- **Background execution** — SSH into a server, run `chief`, disconnect. Come back to finished features.
- **Predictable output** — Conventional commits, structured progress tracking, consistent patterns.
- **Resumable work** — Stop anytime (Ctrl+C), continue exactly where you left off later.
- **Parallel development** — Run Chief on multiple PRDs in different terminal sessions.

You define intent through the PRD. Chief handles the execution loop. Claude does the actual coding.

## Further Reading

- [The Ralph Loop](/concepts/ralph-loop) — Deep dive into the execution loop mechanics
- [PRD Format](/concepts/prd-format) — How to write effective PRDs with good user stories
- [The .chief Directory](/concepts/chief-directory) — Understanding where state is stored
