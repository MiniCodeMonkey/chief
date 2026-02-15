## Codebase Patterns
- CLI entry point is `cmd/chief/main.go`, command implementations are in `internal/cmd/`
- Each command handler accepts a typed Options struct (e.g., `cmd.NewOptions`, `cmd.EditOptions`)
- TUI is built on Bubble Tea (`github.com/charmbracelet/bubbletea`)
- Configuration uses YAML via `gopkg.in/yaml.v3`, stored in `.chief/config.yaml`
- PRD files live in `.chief/prds/<name>/prd.json`
- Build requires `pkg-config` and `libasound2-dev` for the audio notification library (`oto`)
- Go version 1.24.0 is required (per go.mod)
- Commit style follows conventional commits: `feat:`, `fix:`, `chore:` prefixes
- Use `cobra.ArbitraryArgs` on root command to accept positional PRD name/path
- Cobra is now the CLI framework (`github.com/spf13/cobra`)
- Auth credentials stored at `~/.chief/credentials.yaml` via `internal/auth` package
- Use `t.Setenv("HOME", dir)` to redirect home directory in tests (auto-cleaned up)
- Use `httptest.NewServer` for mocking HTTP endpoints; pass base URL via Options struct
- New commands: add `newXxxCmd()` in `main.go`, `RunXxx(XxxOptions)` in `internal/cmd/xxx.go`
- Token refresh is mutex-protected in `internal/auth` — use `auth.RefreshToken(baseURL)` for thread-safe refresh
- `auth.RevokeDevice(accessToken, baseURL)` handles server-side device revocation
- Logout gracefully handles revocation failure (warns, still deletes local creds)

---

## 2026-02-15 - US-001
- **What was implemented:** Migrated CLI from manual flag parsing to Cobra framework
- **Files changed:**
  - `cmd/chief/main.go` - Replaced switch-based dispatch and manual flag parsing with Cobra command tree
  - `go.mod` / `go.sum` - Added `github.com/spf13/cobra` dependency
- **Learnings for future iterations:**
  - Cobra's `SetUsageTemplate` propagates to child commands; use `SetHelpFunc` with a parent check to customize only root help
  - Set `SilenceErrors: true` and `SilenceUsage: true` on root for clean error output, but remember to print the error in `main()` yourself
  - `cobra.ArbitraryArgs` allows positional args on root while still having subcommands
  - The `wiggum` Easter egg command is marked `Hidden: true` so it doesn't appear in help
  - Future subcommands (login, logout, serve, update) can be added via `rootCmd.AddCommand()`
---

## 2026-02-15 - US-002
- **What was implemented:** Credential storage module in `internal/auth` package
- **Files changed:**
  - `internal/auth/auth.go` - `Credentials` struct, `LoadCredentials()`, `SaveCredentials()`, `DeleteCredentials()`, `IsExpired()`, `IsNearExpiry()` methods
  - `internal/auth/auth_test.go` - 11 unit tests covering full save/load/delete cycle, permissions, atomic writes, expiry logic
- **Learnings for future iterations:**
  - Credentials use `~/.chief/credentials.yaml` (user home, NOT project dir) — different from project config which is relative to `baseDir`
  - `os.UserHomeDir()` is used for home directory; override with `t.Setenv("HOME", dir)` in tests
  - `t.Setenv()` automatically restores on cleanup — no need for manual defer
  - Atomic write pattern: `os.CreateTemp` in same dir → write → `os.Rename` — ensures no partial writes
  - File permissions must be `0600` for credentials (not `0644` like config)
  - `LoadCredentials()` returns `ErrNotLoggedIn` (not a default) when file is missing — this differs from config's pattern of returning defaults
  - `gopkg.in/yaml.v3` handles `time.Time` natively — no custom marshaling needed
---

## 2026-02-15 - US-003
- **What was implemented:** Device OAuth login command (`chief login`)
- **Files changed:**
  - `internal/cmd/login.go` - `RunLogin()` with device OAuth flow: request device code, display URL/code, open browser, poll for token, save credentials
  - `internal/cmd/login_test.go` - 6 tests using `httptest` mock server: success flow, device code error, authorization denied, default device name, already-logged-in decline, browser open safety
  - `cmd/chief/main.go` - Added `login` subcommand with `--name` flag, updated help text
- **Learnings for future iterations:**
  - Use `httptest.NewServer` for mocking HTTP endpoints in tests — no need for interface abstraction
  - The `LoginOptions.BaseURL` field allows tests to point at the mock server (defaults to `https://chiefloop.com`)
  - `os.Pipe()` + `os.Stdin` override is the pattern for testing interactive stdin prompts
  - `sync/atomic.Int32` for tracking poll count across goroutines in tests
  - Poll-based tests are slow (5s per poll interval) — keep poll count low in tests
  - `openBrowser()` is best-effort and uses `xdg-open` on Linux, `open` on macOS
  - Login command follows existing pattern: `LoginOptions` struct → `RunLogin()` function
---

## 2026-02-15 - US-004
- **What was implemented:** `chief logout` command and automatic token refresh with thread safety
- **Files changed:**
  - `internal/auth/auth.go` - Added `RefreshToken()` (mutex-protected), `RevokeDevice()`, `ErrSessionExpired`, `refreshResponse` struct
  - `internal/auth/auth_test.go` - Added 7 tests: refresh success, not-near-expiry skip, session expired, not-logged-in, thread safety (concurrent goroutines), revoke success, revoke server error
  - `internal/cmd/logout.go` - New `RunLogout(LogoutOptions)` with revocation endpoint call, graceful handling of revocation failure, credential deletion
  - `internal/cmd/logout_test.go` - 3 tests: success with revocation, not-logged-in, revocation fails but still deletes credentials
  - `cmd/chief/main.go` - Added `logout` subcommand, updated help text
  - `.chief/prds/uplink/prd.json` - Updated US-004 status
- **Learnings for future iterations:**
  - `RefreshToken()` uses a global `sync.Mutex` to prevent concurrent refresh attempts — after acquiring the lock, it re-checks `IsNearExpiry()` in case another goroutine already refreshed
  - Logout follows the pattern: try server-side revocation, warn on failure, always delete local credentials
  - `RevokeDevice()` and `RefreshToken()` accept a `baseURL` parameter for testability (same pattern as login)
  - The `defaultBaseURL` constant was moved to `internal/auth` since both auth and cmd packages need it
  - Thread-safety test verifies only 1 actual HTTP call is made when 5 goroutines call `RefreshToken()` concurrently
---
