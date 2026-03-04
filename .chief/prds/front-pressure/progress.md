## Codebase Patterns
- GetPrompt() in embed/embed.go appends optional sections to the base template string — good pattern for conditional prompt additions without template placeholder complexity
- Config structs follow the pattern: nested struct type + field on Config with yaml tag matching camelCase config key
- Tests use `t.TempDir()` for isolated file system operations
- All config types are in `internal/config/config.go` (single file)
- Test package is `package config` (same package, not `_test`)
- Parser uses `extractStoryID(text, startTag, endTag)` to extract content between XML-like tags; reuse this for new tag types
- Parser test file is `package loop` (same package, not `_test`)
- Multiline text in stream-json test fixtures must use `\n` escape sequences in JSON strings (literal newlines break JSON parsing)
- app.go does NOT import lipgloss by default — add it if adding render methods to app.go; alternatively add render helpers to log.go or other TUI files that already import lipgloss
- TUI log viewer uses AddEvent() switch to filter which events get displayed; new event types must be added to this switch to appear in the log
- centerModal(content, width, height) is defined in internal/tui/completion.go — reuse it for completion-like overlay screens

---

## 2026-03-04 - US-001
- What was implemented: Added `FrontPressureConfig` struct with `Enabled bool` field to `internal/config/config.go`. Added `FrontPressure FrontPressureConfig` field on the `Config` struct with `yaml:"frontPressure"` tag.
- Files changed:
  - `internal/config/config.go` - added `FrontPressureConfig` type and `FrontPressure` field on `Config`
  - `internal/config/config_test.go` - added `TestFrontPressureMarshalUnmarshal` and `TestFrontPressureDefaultsToFalse` tests
- **Learnings for future iterations:**
  - Config follows a consistent pattern: each feature area gets its own `XxxConfig` struct with yaml tags
  - The `Default()` function returns `&Config{}` which gives zero-values (false for bool, empty for string)
  - Tests use temp dirs and Save/Load functions for round-trip testing
  - The `prd.json` may have an `inProgress: true` field added by the chief orchestrator - safe to remove when setting `passes: true`
---

## 2026-03-04 - US-002
- What was implemented: Added `DismissedConcerns []string` field to `UserStory` in `internal/prd/types.go` with `json:"dismissedConcerns,omitempty"` tag. Added three tests to `prd_test.go` covering the new field.
- Files changed:
  - `internal/prd/types.go` - added `DismissedConcerns []string` field to `UserStory` struct
  - `internal/prd/prd_test.go` - added `TestDismissedConcerns_EmptyOmittedFromJSON`, `TestDismissedConcerns_RoundTrip`, `TestDismissedConcerns_LegacyPRDDeserializesWithEmptySlice`
- **Learnings for future iterations:**
  - `prd_test.go` uses `package prd` (same package), not `package prd_test`
  - The `omitempty` tag on a `[]string` field causes nil/empty slices to be omitted from JSON - verified via `map[string]interface{}` inspection
  - Legacy PRDs without new fields deserialize cleanly due to Go's zero-value initialization (nil slice for `[]string`)
  - Test file already imports `encoding/json`, `os`, `path/filepath` - no new imports needed for these tests
---

## 2026-03-04 - US-003
- What was implemented: Added `EventFrontPressure` constant to `EventType` iota in `internal/loop/parser.go`. Added `"FrontPressure"` case to `String()` method. Added detection of `<front-pressure>...</front-pressure>` tags in `parseAssistantMessage()` using the existing `extractStoryID()` helper. Added four new tests to `parser_test.go` and added `EventFrontPressure` to the `TestEventTypeString` table.
- Files changed:
  - `internal/loop/parser.go` - added `EventFrontPressure` constant, `String()` case, and tag detection in `parseAssistantMessage()`
  - `internal/loop/parser_test.go` - added `EventFrontPressure` to string table, added `TestParseLineFrontPressurePresent`, `TestParseLineFrontPressureAbsent`, `TestParseLineFrontPressureMalformed`, `TestParseLineFrontPressureMultiline`
- **Learnings for future iterations:**
  - `extractStoryID()` is a general-purpose tag extractor - reuse it for any `<tag>content</tag>` pattern
  - The concern detection is placed BEFORE the `ralph-status` check in `parseAssistantMessage()` - order matters since first match wins
  - `extractStoryID()` already calls `strings.TrimSpace()` on the extracted content - no need to trim again at the call site
  - Stream-json test fixtures with multiline text must encode newlines as `\n` (JSON escape), not literal newlines
---

## 2026-03-04 - US-005
- What was implemented: Created `embed/fp_editor_prompt.txt` with template variables `{{PRD_PATH}}`, `{{STORY_ID}}`, `{{CONCERN_TEXT}}`, `{{DISMISSED_CONCERNS}}`. Added `GetFPEditorPrompt(prdPath, storyID, concern, dismissedConcerns string) string` to `embed/embed.go`. Added four tests covering substitution, decision tags, guidance content, and non-empty output.
- Files changed:
  - `embed/fp_editor_prompt.txt` - new file with editor prompt template
  - `embed/embed.go` - added `fpEditorPromptTemplate` embed var and `GetFPEditorPrompt()` function
  - `embed/embed_test.go` - added `TestGetFPEditorPrompt_SubstitutesAllPlaceholders`, `TestGetFPEditorPrompt_ContainsDecisionTags`, `TestGetFPEditorPrompt_ContainsGuidance`, `TestGetFPEditorPrompt_NotEmpty`
- **Learnings for future iterations:**
  - `GetFPEditorPrompt` takes `dismissedConcerns` as a pre-formatted `string` (not `[]string`) — callers must format the list before calling
  - The prompt template uses `{{PLACEHOLDER}}` style (same as other prompts), not Go template syntax
  - The `<fp-decision>` tag examples in the prompt template are used verbatim by `FrontPressureEditor.Review()` in US-006 — keep them consistent
---

## 2026-03-04 - US-004
- What was implemented: Extended `GetPrompt()` in `embed/embed.go` to accept `frontPressureEnabled bool` and `dismissedConcerns []string`. When `frontPressureEnabled=true`, a "## Front Pressure" section is appended explaining the tag and constraints. When dismissed concerns are also provided, a "## Previously Dismissed Concerns" section is appended. When `frontPressureEnabled=false`, output is identical to before (no regression). Updated the one caller in `internal/loop/loop.go` to pass `false, nil`. Added three new tests covering all three scenarios.
- Files changed:
  - `embed/embed.go` - extended `GetPrompt()` signature and appended conditional sections
  - `embed/embed_test.go` - updated existing call sites (3) to pass new parameters; added `TestGetPrompt_FrontPressureDisabled`, `TestGetPrompt_FrontPressureEnabled`, `TestGetPrompt_FrontPressureEnabledWithDismissedConcerns`
  - `internal/loop/loop.go` - updated `embed.GetPrompt()` call to pass `false, nil`
- **Learnings for future iterations:**
  - Appending optional sections to a prompt after template substitution is cleaner than adding placeholders to the template for optional/conditional content
  - When extending a function signature, always grep for all callers before changing the signature to avoid missing any
  - The `prd.json` `inProgress: true` field is added by the chief orchestrator and should be removed when setting `passes: true`
---

## 2026-03-04 - US-006
- What was implemented: Created `FrontPressureEditor` struct in `internal/loop/fp_editor.go`. Added `FPDecision` enum with `FPDecisionEdit`, `FPDecisionDismiss`, `FPDecisionScrap`. Implemented `Review()` method that calls `ClaudeRunner` with the editor prompt and parses `<fp-decision>` tag from output. Default `ClaudeRunner` runs `claude --dangerously-skip-permissions -p <prompt> --output-format stream-json` and collects all assistant text using `ParseLine()`. Missing/unrecognized decision tags default to `FPDecisionDismiss`.
- Files changed:
  - `internal/loop/fp_editor.go` - new file with `FPDecision` type, `FrontPressureEditor` struct, `NewFrontPressureEditor()`, `Review()`, and `defaultClaudeRunner()`
  - `internal/loop/fp_editor_test.go` - new file with 5 tests covering: edit decision, dismiss decision, scrap decision, no decision tag (defaults to dismiss), prompt contains concern and story ID
- **Learnings for future iterations:**
  - `FrontPressureEditor.ClaudeRunner` is a function field (not an interface), making it easy to inject a fake in tests without defining a separate interface type
  - The `defaultClaudeRunner` reuses `ParseLine()` from the same package to extract assistant text from stream-json — consistent with how the loop processes Claude output
  - The `Review()` method uses `extractStoryID()` (same package) to parse `<fp-decision>` tags — same pattern as parsing `<front-pressure>` and `<ralph-status>` tags
  - `NewFrontPressureEditor()` sets `ClaudeRunner` to `defaultClaudeRunner` via `e.defaultClaudeRunner` (method value) — tests bypass this by constructing `&FrontPressureEditor{ClaudeRunner: fakeRunner}` directly
---

## 2026-03-04 - US-008
- What was implemented: Wired front pressure through Manager and TUI. Manager.Start() now calls instance.Loop.SetFrontPressure(true, NewFrontPressureEditor()) when config.FrontPressure.Enabled is true. Added StateFrontPressure to AppState enum. Added ViewFrontPressureScrap to ViewMode. Added event handling for EventFrontPressure (yellow log entry), EventFrontPressureResolved (green log entry), and EventFrontPressureScrap (transitions to scrap screen). Added renderFrontPressureScrapView() showing a centered modal with explanation and hint. Updated LogViewer.AddEvent() and renderEntry() to handle the two new display events.
- Files changed:
  - `internal/loop/manager.go` - Manager.Start() calls SetFrontPressure if config enabled
  - `internal/tui/app.go` - StateFrontPressure state, ViewFrontPressureScrap view, event handling, renderFrontPressureScrapView(), lipgloss import added
  - `internal/tui/log.go` - AddEvent() filter and renderEntry() dispatch updated; renderFrontPressure() and renderFrontPressureResolved() helpers added
- **Learnings for future iterations:**
  - app.go does NOT import lipgloss by default — must add it explicitly when adding render methods that use lipgloss styles
  - LogViewer.AddEvent() has a switch that acts as a filter; new events must be added to the case list or they are silently dropped
  - centerModal() from completion.go is the right utility for centered modal overlays — it's available across the tui package
  - TUI render methods on *App can be called from the value-receiver View() method because Go auto-dereferences addressable copies
  - EventFrontPressureScrap should NOT be added to the log filter (it transitions to a new view instead of logging)
---

## 2026-03-04 - US-009
- What was implemented: Added comprehensive unit tests for parser and PRD type changes. Fixed a race condition in `TestLoop_WatchdogKillsHungProcess` (watchdog goroutine could send to `l.events` while the test was closing it). Added `EventFrontPressureResolved` and `EventFrontPressureScrap` to the `TestEventTypeString` table. Added `TestParseLineFrontPressureAtStart`, `TestParseLineFrontPressureMidText`, and `TestParseLineStandardEventsUnaffectedByFrontPressure` tests. All prd_test.go DismissedConcerns tests were already present from US-002.
- Files changed:
  - `internal/loop/loop_test.go` - fixed race in `TestLoop_WatchdogKillsHungProcess` by waiting for watchdog goroutine to exit before closing `l.events`
  - `internal/loop/parser_test.go` - added `EventFrontPressureResolved`/`EventFrontPressureScrap` to string table; added `TestParseLineFrontPressureAtStart`, `TestParseLineFrontPressureMidText`, `TestParseLineStandardEventsUnaffectedByFrontPressure`
- **Learnings for future iterations:**
  - The `TestLoop_WatchdogKillsHungProcess` had a race: `close(watchdogDone)` signals the watchdog to stop but does NOT block until it exits; wrap `runWatchdog` in a goroutine that closes a separate `watchdogExited` channel and wait on that before closing `l.events`
  - When adding new EventType constants to an iota, always add them to the `TestEventTypeString` table — it's easy to miss new constants added in later stories
  - Parser tests for tag positioning: "tag at start" = no text before the opening tag; "tag at end" = text before the opening tag; "tag mid-text" = text both before and after the closing tag
  - `go test ./internal/loop/... ./internal/prd/... -race` is the required command to verify US-009; without `-race`, race conditions go undetected
---

## 2026-03-04 - US-007
- What was implemented: Added front pressure integration to the Loop. Loop struct got `frontPressureEnabled`, `frontPressureEditor`, `pendingConcern`, and `currentStoryID` fields. Added `SetFrontPressure()` method. Modified `processOutput()` to capture concern text when FP is enabled. Added `handleFrontPressure()` method that loads PRD dismissed concerns, calls the editor, and emits `EventFrontPressureResolved` or `EventFrontPressureScrap`. Modified `Run()` to clear pending concern, capture current story ID before each iteration, call `handleFrontPressure()` after iteration, and return early if a scrap decision stopped the loop. Added two new event types: `EventFrontPressureResolved` and `EventFrontPressureScrap`.
- Files changed:
  - `internal/loop/parser.go` - added `EventFrontPressureResolved` and `EventFrontPressureScrap` constants with String() cases
  - `internal/loop/loop.go` - added FP fields to Loop struct, `SetFrontPressure()`, `handleFrontPressure()`, modified `processOutput()` and `Run()`
  - `internal/loop/loop_test.go` - added `TestFrontPressure_Disabled`, `TestFrontPressure_Enabled_Edit`, `TestFrontPressure_Enabled_Dismiss`, `TestFrontPressure_Enabled_Scrap`
- **Learnings for future iterations:**
  - `handleFrontPressure()` is called in `Run()` AFTER `runIterationWithRetry()` returns — not during streaming
  - The loop captures `currentStoryID` before each iteration by loading the PRD (cheap operation); this is important because after the iteration runs the story may already be marked complete
  - When FP is disabled (default), `EventFrontPressure` is still emitted to the channel — only the editor call is skipped
  - The scrap path sets `l.stopped = true` then the Run() loop checks `l.stopped` right after `handleFrontPressure()` and returns nil
  - Tests for FP directly call `l.handleFrontPressure(ctx)` after `l.processOutput(r)` — no need to run the full `Run()` loop for unit testing
  - `strings` import was needed in loop.go for `strings.Join(dismissedConcerns, "\n")`
---
