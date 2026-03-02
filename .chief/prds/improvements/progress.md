## Codebase Patterns
- `NextStory()` returns `(*UserStory, error)` — callers must handle the error for circular dependency detection
- `NextStoryContext()` also returns `(*string, error)` for the same reason
- The `UserStory` struct uses `omitempty` for optional fields like `DependsOn` and `InProgress`
- Tests are in `prd_test.go` (not `types_test.go`) — all type-related tests live there
- The `convert_prompt.txt` uses `{{ID_PREFIX}}` and `{{PRD_FILE_PATH}}` template variables
- `embed.go` uses Go `embed` directive to compile prompts at build time
- Loop callers in `internal/loop/loop.go` use `promptBuilderForPRD()` which calls `NextStory()` and `NextStoryContext()`
- `embed.GetPrompt()` takes 6 params: prdPath, progressPath, knowledgePath, storyContext, storyID, storyTitle
- `KnowledgeWatcher` follows the same API pattern as `ProgressWatcher` (New, Start, Stop, Events)
- TUI `app.go` watchers must be initialized in constructor, started in `Init()`, stopped in `stopWatcher()`, and re-created on PRD switch
- TUI message types (e.g., `KnowledgeUpdateMsg`) must be handled in the main `Update()` switch
- `CompletedStoryRecord` optional fields use `omitempty` (e.g., `CriteriaResults`, `Attempts`) for backward-compatible JSON
- `NextStory()` accepts optional `skipIDs ...map[string]bool` variadic — backward-compatible, callers that don't need skipping just omit the argument
- `promptBuilderForPRD` in loop.go loads both PRD and knowledge.json each iteration — it's the integration point for story selection logic
- Details panel uses a two-step render: `renderDetailsPanelContent()` builds full content, `renderDetailsPanel()` clips to visible window with scroll offset
- Lipgloss `Height()` doesn't clip content — always split lines and render visible slice manually for scrollable panels
- Panel inner height = height - 2 (for border top/bottom); reserve 1 more line for scroll indicator when overflowing

## 2026-03-02 - US-001
- Implemented story dependency declaration and resolution
- Added `DependsOn []string` field to `UserStory` struct in `internal/prd/types.go`
- Updated `NextStory()` to skip stories with unsatisfied dependencies, using a `depsResolved()` helper
- Added circular dependency detection: returns error when incomplete stories remain but none are eligible
- Updated `convert_prompt.txt` to understand `dependsOn` field from markdown `**Depends On:**` lines
- Changed `NextStory()` signature from `*UserStory` to `(*UserStory, error)` and `NextStoryContext()` similarly
- Updated caller in `internal/loop/loop.go` to handle new error returns
- Added 7 new test cases: basic dependency ordering, multi-dependency, multi-dependency all satisfied, circular detection, empty dependsOn, priority tiebreaker
- Updated all existing `NextStory()` and `NextStoryContext()` tests for new error return
- Files changed: `internal/prd/types.go`, `internal/prd/prd_test.go`, `internal/loop/loop.go`, `embed/convert_prompt.txt`
- **Learnings for future iterations:**
  - The `NextStory()` signature change is a breaking API change — all callers (loop.go, tests) must be updated
  - Priority in this codebase means "lower number = higher priority" (priority 1 is picked before priority 3)
  - The `depsResolved()` helper builds a passed-map each time — acceptable for small story counts but could be optimized for large PRDs
  - The TUI doesn't call `NextStory()` directly, so TUI files didn't need changes for this story
---

## 2026-03-02 - US-002
- Implemented structured knowledge base (knowledge.json) as primary machine-readable knowledge store
- Created `internal/prd/knowledge.go` with types (`Knowledge`, `CompletedStoryRecord`), functions (`LoadKnowledge()`, `SaveKnowledge()`, `KnowledgePath()`), and `KnowledgeWatcher`
- Updated `embed/prompt.txt` to instruct agent to read knowledge.json at start and update it after story completion
- Added `{{KNOWLEDGE_PATH}}` template variable to `embed/embed.go` and `GetPrompt()` function signature
- Updated `internal/loop/loop.go` to pass knowledge path to `GetPrompt()`
- Updated TUI `app.go`: added `KnowledgeUpdateMsg`, `knowledgeWatcher`/`knowledge` fields, watcher lifecycle (init, start, stop, PRD switch)
- Updated TUI `dashboard.go`: `renderDetailsPanel` reads from knowledge.json first, falls back to progress.md
- Added 6 unit tests in `knowledge_test.go`: path derivation, missing file, round-trip, null fields, invalid JSON, empty stories
- Updated 3 existing tests in `embed_test.go` for new `GetPrompt()` signature
- Files changed: `internal/prd/knowledge.go`, `internal/prd/knowledge_test.go`, `embed/embed.go`, `embed/embed_test.go`, `embed/prompt.txt`, `internal/loop/loop.go`, `internal/tui/app.go`, `internal/tui/dashboard.go`
- **Learnings for future iterations:**
  - `embed.GetPrompt()` is a breaking API change — all callers (loop.go, embed tests) must be updated when adding template vars
  - The TUI has a 5-step watcher integration pattern: (1) message type, (2) App field, (3) constructor init, (4) Init() start + batch listener, (5) Update() handler + re-listen, (6) stopWatcher() cleanup, (7) PRD switch re-creation
  - `ProgressWatcher` watches the directory not the file (to catch creates), `KnowledgeWatcher` follows the same pattern
  - The `renderDetailsPanel` fallback logic must handle both `a.knowledge == nil` (no watcher) and `record not found` (no knowledge for this story)
---

## 2026-03-02 - US-003
- Added mandatory "Analysis Phase" to `embed/prompt.txt` as step 4 before implementation
- Analysis Phase includes: (a) read source files, (b) identify patterns/conventions, (c) list files to change, (d) think step by step about edge cases, (e) outline approach in 3-5 bullets, (f) write approach to knowledge.json before coding
- Enhanced step 1 to emphasize reading `completedStories` for architectural context
- Added "Think step by step" instruction for edge case reasoning in step 4d
- Reinforced analysis-first workflow in the Important section
- Files changed: `embed/prompt.txt`
- **Learnings for future iterations:**
  - Prompt-only stories are low-risk since they don't change Go code, but still need typecheck verification because `embed` compiles the template
  - The `approach` field already exists in `CompletedStoryRecord` — no type changes needed for this story
  - The prompt uses numbered steps with sub-steps (a, b, c) — maintain this convention for readability
---

## 2026-03-02 - US-004
- Added individual acceptance criteria verification protocol
- Added `CriteriaResult` type and `CriteriaResults []CriteriaResult` field (with `omitempty`) to `CompletedStoryRecord` in `knowledge.go`
- Updated `embed/prompt.txt`: inserted mandatory "Verification Phase" (step 6) between implementation and quality checks, with per-criterion verification instructions and fail-fast behavior
- Updated Knowledge Base schema in prompt to include `criteriaResults` example
- Updated `renderDetailsPanel` in `dashboard.go`: acceptance criteria now show ✓ (green) / ✗ (red) / • (gray) icons based on `criteriaResults` from `knowledge.json`, with evidence shown below failed criteria
- Added 2 new tests in `knowledge_test.go`: `TestLoadSaveKnowledge_CriteriaResults` (round-trip with criteria) and `TestLoadKnowledge_NoCriteriaResults` (backward compatibility)
- Files changed: `internal/prd/knowledge.go`, `internal/prd/knowledge_test.go`, `embed/prompt.txt`, `internal/tui/dashboard.go`
- **Learnings for future iterations:**
  - `CriteriaResults` uses `omitempty` so existing knowledge.json files without this field deserialize cleanly (nil slice)
  - The TUI criteria display builds a `map[string]CriteriaResult` for O(1) lookup by criterion text — this means criterion text must match exactly between prd.json and knowledge.json
  - The prompt step numbering shifted (implementation=5, verification=6, quality=7, commit=8, etc.) — future prompt changes must keep this in sync
  - The `statusPassedStyle` / `statusFailedStyle` / `MutedColor` from styles.go are reused for criteria icons — no new styles needed
---

## 2026-03-03 - US-005
- Implemented smart retry with failure analysis
- Added `Attempt` type (with `Approach`, `CriteriaResults`, `FailureAnalysis` fields) and `Attempts []Attempt` field to `CompletedStoryRecord` in `knowledge.go`
- Added `MaxAttempts = 3` constant and `ExhaustedStoryIDs()` method to `Knowledge` struct
- Made `NextStory()` accept optional `skipIDs ...map[string]bool` variadic parameter (backward-compatible) to skip exhausted stories
- Updated `promptBuilderForPRD` in `loop.go` to load knowledge.json, get exhausted story IDs, and pass them to `NextStory()`
- Added "Failure Recovery" section to `prompt.txt` (step 2) with instructions to analyze previous failed attempts, avoid repeating the same approach, and skip after 3 attempts
- Updated prompt step numbering (analysis=5, implement=6, verify=7, quality=8, commit=9, etc.)
- Updated Knowledge Base schema in prompt to include `attempts` array example
- Added attempt count display ("Attempt X/3" or "Exhausted (3/3 attempts)") to TUI details panel status line in `dashboard.go`
- Added 9 new tests: attempts round-trip, backward-compat no-attempts, ExhaustedStoryIDs, ExhaustedStoryIDs empty, NextStory skip exhausted, skip all exhausted, skip doesn't affect in-progress, nil skipIDs
- Files changed: `internal/prd/knowledge.go`, `internal/prd/knowledge_test.go`, `internal/prd/types.go`, `internal/prd/prd_test.go`, `internal/loop/loop.go`, `embed/prompt.txt`, `internal/tui/dashboard.go`
- **Learnings for future iterations:**
  - Using variadic `skipIDs ...map[string]bool` keeps the `NextStory()` API backward-compatible — existing callers (tests, direct calls) don't need changes
  - The 3-attempt skip must live in Go code (not just the prompt), because the agent is given a specific story by the loop and can't autonomously "skip to the next one"
  - `promptBuilderForPRD` in loop.go already loads the PRD each iteration, so it's the natural place to also load knowledge.json for exhausted story detection
  - InProgress stories are NOT skippable — they bypass the skip check, which is correct for interrupted story recovery
  - The prompt step numbering shifted again (now analysis=5, verify=7) — future prompt changes must keep this in sync
---

## 2026-03-03 - US-006
- Fixed details panel content overflow with proper scroll support
- Refactored `renderDetailsPanel` into two functions: `renderDetailsPanelContent` (builds full content) and `renderDetailsPanel` (clips and renders with scroll)
- Content is split into lines and clipped to the available inner height (height - 2 for borders)
- When content overflows, a scroll indicator is shown on the last line ("▼ more below" / "▲ more above" / "▲▼ X%")
- Added `J/K` (shift+j/shift+k) keys for scrolling the details panel in dashboard view
- Added `Home/End` keys for jumping to top/bottom of details panel content
- Added `detailsScrollOffset` field to `App` struct, reset to 0 whenever the selected story changes (j/k navigation, selectStoryByID, selectInProgressStory, PRD switch)
- Added helper methods: `scrollDetailsUp()`, `scrollDetailsDown()`, `scrollDetailsToTop()`, `scrollDetailsToBottom()`, `detailsPanelDimensions()`, `detailsMaxOffset()`
- Updated footer shortcuts to show "J/K: details" hint in dashboard view
- Updated help overlay with new keyboard shortcuts
- Files changed: `internal/tui/dashboard.go`, `internal/tui/app.go`, `internal/tui/help.go`
- **Learnings for future iterations:**
  - Lipgloss's `Height()` doesn't provide scroll clipping — you must manually split content into lines and render only the visible slice
  - The panel border consumes 2 lines of height (top + bottom), so inner height = height - 2
  - When showing a scroll indicator, reserve 1 line from the visible area so the indicator doesn't push content out
  - `detailsPanelDimensions()` must mirror the calculations in `renderDashboard()` and `renderStackedDashboard()` exactly
  - Story selection resets happen in multiple places: j/k navigation, `selectStoryByID()`, `selectInProgressStory()`, and PRD switch — all must reset `detailsScrollOffset`
  - `J/K` (uppercase) is cleanly separated from `j/k` (lowercase) in Bubble Tea key handling
---
