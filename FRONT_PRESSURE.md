# Feature Idea: Front Pressure

## What Chief Is (Brief Context)

Chief is a terminal-based orchestrator for autonomous AI-driven development. You write a Product Requirements Document (PRD) — first as Markdown, then converted to structured JSON — and Chief loops through the user stories in that PRD one at a time, invoking a fresh Claude Code instance for each. Each agent gets just enough context to do its one job: read progress, check codebase patterns, implement the story, run quality checks, commit, and update the PRD.

The loop is designed to run unattended. The whole point is that you can set up a 20-ticket PRD, press `S`, walk away, and come back to a finished feature branch.

## What "Back Pressure" Means in This Context

Chief already has back pressure built in. Before a story is marked as complete (`passes: true` in the PRD JSON), the agent must:

- Run typechecks, linting, and tests
- Pass all of them
- Commit only if they pass

If the quality checks fail, the story stays incomplete and the next loop iteration retries. The code cannot move forward until it is correct. This is back pressure: the downstream consequences of broken code push back against sloppy work and keep the codebase healthy.

Back pressure answers the question: *"Is the code correct?"*

## What "Front Pressure" Would Mean

Front pressure would answer a different question: *"Is the plan correct?"*

A ticket agent, while in the codebase implementing story #7 of 20, might discover something that the PRD author could not have known at planning time. Not a bug. Not a failing test. A design-level problem — a fundamental assumption in the PRD that turns out to be wrong, incomplete, or in conflict with how the codebase actually works.

Examples of what a ticket agent might surface:
- "The PRD assumes we can use library X here, but library X doesn't support Y, which we need."
- "Stories 8 through 12 are all predicated on a data model that I've just discovered is structured completely differently."
- "There's an existing implementation of this that contradicts what the PRD is asking for. One of them needs to change."

Under the current system, the agent has no mechanism to raise this. It either pushes through and produces technically-passing but semantically-wrong code, or it fails repeatedly and eventually the watchdog kills it. Either way, the unattended overnight run is derailed.

Front pressure gives the agent a voice. It can say: *"Before I proceed, someone with authority over this PRD should know this."*

## The Desired Behavior

When a ticket agent encounters a concern it believes rises to the level of a PRD-level problem, it should be able to surface that concern rather than silently proceed or fail.

That concern then gets routed to an editing loop — think of it as an automated engineering manager review. This loop runs in AFK mode (no human at the keyboard is assumed). It reads what the original agent found, considers it against the existing PRD, and makes a decision.

The editor has three options:

1. **Edit the remaining tickets.** If the concern is real but manageable, adjust the downstream stories to account for what was discovered. The work continues with a corrected plan.

2. **Dismiss the concern on the current ticket.** If the editor decides the agent's concern is not worth acting on, it notes that decision directly in the PRD on that ticket. This is critical to prevent infinite loops — the next agent working on that same story won't re-raise the same concern if it's been explicitly acknowledged and dismissed.

3. **Scrap and restart.** If the concern exposes a truly foundational problem, revert to main and start fresh. This is the nuclear option and should only happen when the alternative is building on a broken foundation.

## The Two Modes

This feature should be opt-in rather than always-on. There are two ways to run Chief:

- **Standard mode** (current behavior): Run all tickets as specified. Agents implement what they're told. Back pressure keeps the code correct.

- **Front pressure mode**: Agents are permitted to surface PRD-level concerns. If one does, an editing loop runs before the next ticket starts. The PRD may be adjusted. The run continues with an updated plan.

The reason it's opt-in: sometimes you have a mature, well-researched PRD and you want execution with no interruptions. Front pressure mode is for when you're less certain about the plan and want the agents to act as collaborators, not just implementors.

## Why This Matters

The goal is to let Chief run for hours — overnight, unattended — and arrive at a meaningful result in the morning.

Right now, back pressure handles code quality. The code that ships is correct. But if the plan was wrong at ticket 10 of 30, you get 20 more tickets of correct code implementing the wrong thing. Or worse, the agent can't reconcile the plan with reality, fails repeatedly, and the run dies at 1am.

The analogy is a software team. The product manager has a high-level vision. Engineers write a tech doc. Individual developers write the code. When a developer discovers at implementation time that a fundamental assumption was wrong, they don't just quietly write broken code or silently stop working. They raise it. The information flows back up. The plan gets corrected. Work continues.

Chief's ticket agents are those developers. Right now they have no way to raise a concern. Front pressure gives them that voice — and closes the loop between implementation-level discovery and plan-level decision-making — so the whole system can run longer, adapt to what it finds, and produce something useful rather than dying quietly in the night.
