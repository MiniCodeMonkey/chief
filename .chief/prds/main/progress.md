## Codebase Patterns
- Provider registration and validation are centralized in `internal/agent/resolve.go`; when adding a provider, update `Resolve` switch cases and supported-provider error text together.
- User-facing provider choices are duplicated in CLI parsing/help (`cmd/chief/main.go`) and config comments (`internal/config/config.go`), so keep those enumerations in sync with resolver changes.
- Provider-specific runtime config belongs under `agent.<provider>` in `.chief/config.yaml`; enforce provider-specific validation in `agent.Resolve` so startup fails before loop execution.
- Provider execution contract tests are implemented as executable mock CLI scripts in integration tests, which is the preferred way to assert real argv/stdin handling through `loop.Run`.
- Runtime provider failures should be represented as `loop.ExecutionError` with explicit `Kind` values and remediation text, then propagated through `EventError.Text` for consistent TUI/user-facing error rendering.
- User-facing error classifiers (for example `internal/loop/execution_error.go`) should have dedicated unit tests covering every error-kind mapping and provider-specific remediation branch.

## 2026-02-25 10:39:54 CET - US-001
- Implemented first-class provider registration for `opencode` by adding `OpenCodeProvider`, wiring it into provider resolution, and updating CLI/provider validation strings to include `opencode`.
- Files changed: `internal/agent/opencode.go`, `internal/agent/opencode_test.go`, `internal/agent/resolve.go`, `internal/agent/resolve_test.go`, `cmd/chief/main.go`, `internal/config/config.go`, `.chief/prds/main/prd.json`, `.chief/prds/main/progress.md`.
- **Learnings for future iterations:**
  - New providers must implement `loop.Provider` in `internal/agent` and include provider-specific unit tests mirroring `claude`/`codex` coverage.
  - Provider resolution precedence is flag -> env -> config -> default, so tests should verify provider support across all three override layers.
  - CLI validation/help text currently hardcodes supported providers in several places, so provider additions require coordinated updates to avoid inconsistent UX.
---

## 2026-02-25 10:44:58 CET - US-002
- Implemented OpenCode runtime configuration support via `agent.opencode` settings (`cliPath`, `requiredEnv`), provider-specific CLI path precedence, and pre-execution validation for invalid/missing required environment variables with actionable error messages.
- Files changed: `internal/config/config.go`, `internal/config/config_test.go`, `internal/agent/resolve.go`, `internal/agent/resolve_test.go`, `docs/reference/configuration.md`, `docs/troubleshooting/common-issues.md`, `README.md`, `.chief/prds/main/prd.json`, `.chief/prds/main/progress.md`.
- **Learnings for future iterations:**
  - Prefer provider-specific config overrides (for example `agent.opencode.cliPath`) while keeping shared fallback fields (like `agent.cliPath`) to avoid breaking existing setups.
  - Validate provider-specific configuration during provider resolution, not during loop runtime, so users get immediate actionable failures.
  - Required environment variable lists should be validated for both syntax and presence to catch typos and missing shell state early.
---

## 2026-02-25 10:49:40 CET - US-003
- Implemented OpenCode execution integration coverage using real process execution via mock `opencode` scripts to validate successful runs, non-zero exit failures, and context-canceled runs.
- Files changed: `internal/agent/opencode_integration_test.go`, `.chief/prds/main/prd.json`, `.chief/prds/main/progress.md`.
- **Learnings for future iterations:**
  - Use `loop.NewLoopWithWorkDir` in provider integration tests to verify CLI process behavior end-to-end instead of only unit-testing command builders.
  - Keep retry disabled in failure-path integration tests (`DisableRetry`) so expected error assertions remain deterministic and fast.
  - Validate provider command contracts inside test scripts (argv shape plus stdin capture) to catch regressions in invocation format early.
---

## 2026-02-25 10:55:50 CET - US-004
- Implemented explicit execution error-state mapping for provider runtime failures (`missing_binary`, `timeout`, `non_zero_exit`, `process_failure`) with remediation guidance and labeled stderr summaries, and surfaced those messages consistently through loop events/TUI log rendering.
- Files changed: `internal/loop/execution_error.go`, `internal/loop/loop.go`, `internal/tui/log.go`, `internal/agent/opencode_integration_test.go`, `.chief/prds/main/prd.json`, `.chief/prds/main/progress.md`.
- **Learnings for future iterations:**
  - Capture stderr while streaming to logs so failures can include concise `stderr:` context without losing full raw logs.
  - Preserve explicit timeout classification (`ExecutionErrorKindTimeout`) instead of collapsing deadline errors to plain `context.DeadlineExceeded` if user-facing state mapping is required.
  - When emitting `EventError`, always populate `Text` (or derive it from `Err`) so TUI history panels do not degrade to generic fallback messages.
---

## 2026-02-25 11:00:37 CET - US-005
- Added reliability regression coverage for execution failure classification/remediation in `internal/loop/execution_error_test.go` and documented a release validation checklist for OpenCode install/config/run/failure checks.
- Files changed: `internal/loop/execution_error_test.go`, `docs/reference/opencode-reliability-checklist.md`, `docs/.vitepress/config.ts`, `docs/reference/configuration.md`, `docs/troubleshooting/common-issues.md`, `.chief/prds/main/prd.json`, `.chief/prds/main/progress.md`.
- **Learnings for future iterations:**
  - Use focused unit tests for error mapping helpers so provider-specific remediation text changes are caught before integration tests.
  - Keep release-validation docs in the Reference section and link them from both configuration and troubleshooting pages so QA flows are discoverable.
  - For reliability stories, validate both targeted provider suites and full-project `go test ./...`/`go vet ./...` to catch cross-provider regressions.
---
