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
- Shared engine: `internal/engine` package wraps `loop.Manager` with fan-out event subscription via `Subscribe()`
- TUI uses `a.eng` (engine) not `a.manager` — all loop operations go through engine
- `engine.New(maxIter)` creates engine; `engine.Subscribe()` returns `(<-chan ManagerEvent, unsubFunc)`
- Tests using App struct directly must use `eng` field (not `manager`) and create `engine.New()` instances
- WebSocket client: `internal/ws` package with `ws.New(url, opts...)`, `Connect(ctx)`, `Send(msg)`, `Receive()`, `Close()`
- Use `gorilla/websocket` library for WebSocket connections
- WebSocket test pattern: use `httptest.NewServer` + `websocket.Upgrader` for mock servers; `wsURL()` helper to convert HTTP to WS URL
- `WithOnReconnect(fn)` option allows serve command to re-send state snapshot on reconnect
- Protocol handshake: `client.Handshake(accessToken, version, deviceName)` after `Connect()` — sends `hello`, waits for `welcome`/`incompatible`/`auth_failed`
- UUID generation: `ws.newUUID()` uses `crypto/rand` (no external dependency); `ws.NewMessage(type)` creates envelope with UUID + ISO8601 timestamp
- Handshake errors: `*ErrIncompatible` (version mismatch, do NOT retry), `ErrAuthFailed` (deauthorized), `ErrHandshakeTimeout` (10s timeout)

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

## 2026-02-15 - US-005
- **What was implemented:** Extracted shared engine from TUI into `internal/engine` package
- **Files changed:**
  - `internal/engine/engine.go` - New `Engine` struct wrapping `loop.Manager` with fan-out event subscription (`Subscribe()`) for multiple consumers
  - `internal/engine/engine_test.go` - 25 tests covering: creation, register/unregister, subscribe/unsubscribe, fan-out events, concurrent access, shutdown, worktree info, config, PRD loading
  - `internal/tui/app.go` - Replaced `manager *loop.Manager` with `eng *engine.Engine`; added `eventCh` and `unsubFn` fields; TUI now subscribes to engine events via `Subscribe()`; added `NewAppWithEngine()` for sharing engine with serve command
  - `internal/tui/dashboard.go` - Updated `a.manager` → `a.eng` references
  - `internal/tui/layout_test.go` - Updated tests to create `engine.Engine` instead of `loop.Manager` directly; added `newTestEngine()` and `newTestEngineWithWorktree()` helpers
- **Learnings for future iterations:**
  - The `loop.Manager` already had the right abstraction (channels, start/stop/pause, events). The engine adds fan-out subscription on top.
  - Fan-out uses non-blocking sends (`select { case ch <- event: default: }`) to avoid slow consumers blocking the pipeline
  - `Subscribe()` returns a cleanup function — must be called to avoid resource leaks
  - `engine.Shutdown()` stops the forwarding goroutine; `engine.StopAll()` only stops loops
  - TabBar and PRDPicker still use `loop.Manager` directly (via `eng.Manager()`) since they only need read-only state queries
  - When tests create `App` struct literals, use `eng` field (not `manager`) and pass `engine.New()` instances
---

## 2026-02-15 - US-006
- **What was implemented:** WebSocket client with automatic reconnection in `internal/ws` package
- **Files changed:**
  - `internal/ws/client.go` - `Client` struct with `Connect(ctx)`, `Send(msg)`, `Receive()`, `Close()` API; exponential backoff + jitter reconnection (1s→60s max); ping/pong handler; `WithOnReconnect` callback option; context-based cancellation
  - `internal/ws/client_test.go` - 10 tests: connect/send/receive, graceful close, send-when-disconnected, reconnect-on-server-close, context cancellation, ping/pong, backoff calculation, default URL, channel buffer, multiple messages
  - `go.mod` / `go.sum` - Added `github.com/gorilla/websocket` dependency
  - `.chief/prds/uplink/prd.json` - Updated US-006 status
- **Learnings for future iterations:**
  - `gorilla/websocket` is the standard Go WebSocket library — `DefaultDialer.Dial()` for connecting, `ReadMessage()`/`WriteMessage()` for I/O
  - Ping/pong: set `SetPingHandler` to auto-respond with pong via `WriteControl(PongMessage, ...)`; also set `SetPongHandler` (even empty) to prevent default pong from interfering
  - Reconnection loop lives in `readLoop` — on read error, close old conn, dial new one, set up handlers again, call `onRecon` callback
  - Backoff with jitter formula: `base * 2^(attempt-1) * rand(0.5, 1.5)`, capped at max
  - Test pattern for WebSocket: `httptest.NewServer` with `websocket.Upgrader` in handler, convert URL with `strings.TrimPrefix(s.URL, "http")` → `"ws" + ...`
  - `atomic.Int32` useful for tracking connection counts in reconnection tests
  - Message struct uses `json.RawMessage` for `Raw` field to preserve the full original message for downstream consumers
---

## 2026-02-15 - US-007
- **What was implemented:** Protocol handshake for WebSocket connections with authentication and version compatibility verification
- **Files changed:**
  - `internal/ws/handshake.go` - `Handshake()` method on `Client`, hello/welcome/incompatible/auth_failed message types, `newUUID()` v4 generator, `NewMessage()` envelope helper, `ErrIncompatible`/`ErrAuthFailed`/`ErrHandshakeTimeout` error types
  - `internal/ws/handshake_test.go` - 8 tests: success, incompatible version, auth failure, timeout, correct hello contents verification, connection closed during handshake, UUID format, NewMessage helper
  - `.chief/prds/uplink/prd.json` - Updated US-007 status
- **Learnings for future iterations:**
  - UUID v4 can be generated with `crypto/rand` + bit manipulation (set version=4, variant=RFC4122) — no need for external `google/uuid` dependency
  - Handshake uses `client.Receive()` channel with `time.NewTimer` for timeout — clean select-based pattern
  - `*ErrIncompatible` is a struct error (not sentinel) so it can carry the server's message; `ErrAuthFailed` and `ErrHandshakeTimeout` are sentinels
  - Handshake must be called after `Connect()` — it sends the hello message and blocks until response or timeout
  - When connection closes during handshake, the readLoop reconnects but handshake still times out (correct behavior — caller should retry)
  - `runtime.GOOS` and `runtime.GOARCH` provide OS/architecture info for the hello message
---
