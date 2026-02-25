## Codebase Patterns
- Provider registration and validation are centralized in `internal/agent/resolve.go`; when adding a provider, update `Resolve` switch cases and supported-provider error text together.
- User-facing provider choices are duplicated in CLI parsing/help (`cmd/chief/main.go`) and config comments (`internal/config/config.go`), so keep those enumerations in sync with resolver changes.
- Provider-specific runtime config belongs under `agent.<provider>` in `.chief/config.yaml`; enforce provider-specific validation in `agent.Resolve` so startup fails before loop execution.

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
