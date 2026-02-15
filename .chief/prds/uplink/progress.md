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
- Message types defined as `Type*` constants in `internal/ws/messages.go` (e.g., `TypeStartRun`, `TypeGetProject`)
- Error codes defined as `ErrCode*` constants in `internal/ws/messages.go` (e.g., `ErrCodeProjectNotFound`)
- Use `ws.NewDispatcher()` to create a message router; `Register(type, handler)` to add handlers; `Dispatch(msg)` to route
- Use pointer fields (`*int`, `*bool`, `*string`) for optional/partial update messages to distinguish "not set" from zero values
- Serve command uses `ServeOptions.Ctx` (context) for testability — tests cancel ctx to stop serve loop
- For handshake error tests (incompatible/auth_failed), call `srv.CloseClientConnections()` to prevent ws reconnection loops
- Cancel context before `client.Close()` to avoid race where readLoop reconnects during Close()
- Workspace scanner: `internal/workspace` package; `workspace.New(dir, wsClient)` creates scanner; `scanner.Run(ctx)` runs periodic scan loop; `scanner.Projects()` returns current list
- `ws.Client.Send()` accepts `interface{}` — pass typed message structs directly, no need to marshal separately
- Scanner now supports `SetClient()` for deferred client setup and `FindProject(name)` for single-project lookup
- State snapshot is sent after handshake AND on reconnect; use `sendStateSnapshot(client, scanner)` helper
- `sendError(client, code, message, requestID)` helper sends typed error messages over WebSocket
- `serveTestHelper(t, workspacePath, serverFn)` encapsulates WS test boilerplate (hello/welcome/state_snapshot/cancel)
- After handshake, server always receives a `state_snapshot` message — existing tests must account for this
- `sessionManager` in `internal/cmd/session.go` manages Claude PRD sessions: `newPRD()`, `sendMessage()`, `closeSession()`, `killAll()`, `activeSessions()`
- Mock `claude` binary in tests: write shell script to temp dir, prepend to PATH via `t.Setenv("PATH", dir+":"+origPath)`
- `projectFinder` interface allows testing handlers without real `workspace.Scanner`
- Session timeout: `sessionManager` has configurable `timeout`, `warningThresholds`, and `checkInterval` fields — set to small values in tests for speed
- `expireSession()` closes stdin, waits 2s grace period, then force-kills; sends `session_expired` after process exits
- `runManager` in `internal/cmd/runs.go` manages Ralph loop runs: `startRun()`, `pauseRun()`, `resumeRun()`, `stopRun()`, `activeRuns()`, `stopAll()`
- Run control uses `engine.Engine` — resume works by calling `engine.Start()` again (creates fresh Loop picking up next unfinished story)
- `runKey(project, prdID)` creates engine registration key as `"project/prdID"`
- Quota errors detected via `loop.IsQuotaError(text)` — checks stderr/exit error for patterns like "rate limit", "quota", "429"
- Quota errors bypass retry logic and set `LoopStatePaused` (not `LoopStateError`) for resumability
- `runManager.startEventMonitor(ctx)` subscribes to engine events for cross-cutting concerns like quota detection
- `sendStateSnapshot`, `handleMessage`, and `serveShutdown` now accept `*runManager` parameter

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

## 2026-02-15 - US-008
- **What was implemented:** Strongly-typed message definitions, error codes, and message dispatcher for the WebSocket protocol
- **Files changed:**
  - `internal/ws/messages.go` - All message type structs for the protocol catalog (server→web app, web app→server, bidirectional), type constants, error code constants
  - `internal/ws/dispatcher.go` - `Dispatcher` struct with `Register()`, `Unregister()`, `Dispatch()` for routing messages by type to handlers
  - `internal/ws/messages_test.go` - 28 serialization/deserialization round-trip tests covering all message types, optional field omission, partial updates
  - `internal/ws/dispatcher_test.go` - 7 tests: register/dispatch, unknown type ignored, unregister, handler replacement, raw JSON passthrough, concurrent access, multiple handlers
- **Learnings for future iterations:**
  - Use `*int`, `*bool`, `*string` pointer fields for partial/optional updates (e.g., `UpdateSettingsMessage`) — allows distinguishing "not provided" from zero values
  - `json.RawMessage` on the `Message.Raw` field preserves the full original JSON, so dispatcher handlers can unmarshal into the specific message type
  - Error codes are string constants (`ErrCodeProjectNotFound`) not enums — easier to serialize and forward-compatible
  - Type constants (`TypeStartRun`, `TypeGetProject`, etc.) centralize string literals and prevent typos
  - Dispatcher uses `sync.RWMutex` for safe concurrent access — handlers can be registered/unregistered while dispatching
  - Unknown message types are logged and ignored (forward compatibility per spec)
  - `interface{}` is used for `PRDContentMessage.State` since the PRD state is a flexible JSON object
---

## 2026-02-15 - US-009
- **What was implemented:** Basic `chief serve` command — headless daemon that connects to chiefloop.com via WebSocket
- **Files changed:**
  - `internal/cmd/serve.go` - `RunServe(ServeOptions)` with workspace validation, credential check, token refresh, WebSocket connection, protocol handshake, signal handling, ping/pong handling, clean shutdown
  - `internal/cmd/serve_test.go` - 9 tests: workspace validation (nonexistent, file-not-dir), not-logged-in, successful connect+handshake, device name override, incompatible version, auth failure, log file output, ping/pong, token refresh
  - `cmd/chief/main.go` - Added `serve` subcommand with `--workspace`, `--name`, `--log-file` flags; updated help text
  - `.chief/prds/uplink/prd.json` - Updated US-009 status
- **Learnings for future iterations:**
  - `ServeOptions.Ctx` (context.Context) is the key testing mechanism — tests create a context, cancel it from the mock server handler after handshake completes, which causes RunServe to exit cleanly
  - For error-path tests (incompatible/auth_failed), the ws.Client reconnection behavior creates issues — use `srv.CloseClientConnections()` in the server handler to prevent reconnection loops during Close()
  - Must cancel context BEFORE calling `client.Close()` to avoid race where readLoop reconnects and Close() gets a stale conn reference
  - The serve command passes `version` from build-time `Version` var in main.go through `ServeOptions.Version` to the handshake
  - Device name defaults to credential's device name, overridable via `--name` flag
  - Log output defaults to stdout; `--log-file` redirects `log.SetOutput()` to a file
---

## 2026-02-15 - US-010
- **What was implemented:** Workspace scanner that discovers git repositories in the workspace directory
- **Files changed:**
  - `internal/workspace/scanner.go` - New `Scanner` struct with `Scan()`, `ScanAndUpdate()`, `Run()`, `Projects()` methods. Scans one level deep for `.git/` dirs, gathers branch/commit/PRD info, sends `project_list` over WebSocket on changes, re-scans every 60s
  - `internal/workspace/scanner_test.go` - 12 tests: discover repos, detect .chief, discover PRDs, multiple projects, empty workspace, permission errors, detect add/remove, periodic scanning with WebSocket, context cancellation, projectsEqual, branch detection
  - `internal/cmd/serve.go` - Integrated scanner: starts `workspace.New(opts.Workspace, client).Run(ctx)` in a goroutine after handshake
  - `.chief/prds/uplink/prd.json` - Updated US-010 status
- **Learnings for future iterations:**
  - `ws.Client.Send()` accepts `interface{}` and marshals it internally — pass the message struct directly, don't double-marshal
  - `os.Stat()` doesn't require read permission on a file, only traverse on parent — to test permission errors, remove perms on the parent directory
  - `sendProjectList()` must guard against nil client (scanner can be used standalone in tests)
  - `projectsEqual()` compares by building a name→project map — handles different ordering between scans
  - Git info gathered via `git rev-parse --abbrev-ref HEAD` (branch) and `git log -1 --format=%H%n%s%n%an%n%aI` (commit hash, message, author, ISO timestamp)
  - PRD completion status formatted as `"passed/total"` (e.g., `"2/3"`)
  - Scanner uses `time.NewTicker` for periodic scans; tests set `scanner.interval` to small values for speed
- File watcher: `workspace.NewWatcher(dir, scanner, client)` creates watcher; `watcher.Activate(name)` enables deep watching; `watcher.Run(ctx)` runs event loop
- `fsnotify` does NOT recurse into subdirectories — must explicitly `Add()` each subdirectory (e.g., each PRD dir inside `.chief/prds/`)
- Watcher `Activate()` is called by serve command when `get_project` message is received (or run started, session opened)
- `watcher.inactiveTimeout` can be set to small values in tests for fast inactivity cleanup testing
---

## 2026-02-15 - US-011
- **What was implemented:** Selective file watching using `fsnotify` for workspace root and active project deep watchers
- **Files changed:**
  - `internal/workspace/watcher.go` - New `Watcher` struct with `Activate()`, `Run()`, `Close()`, inactivity cleanup, deep watcher setup/teardown for `.chief/`, `.chief/prds/` (+ subdirs), `.git/`. Sends `project_state` updates on file changes.
  - `internal/workspace/watcher_test.go` - 9 tests: workspace root changes, activate project, unknown project, activity refresh, inactivity cleanup, PRD change sends project_state, git HEAD change sends project_state, context cancellation, no deep watchers for inactive projects
  - `internal/cmd/serve.go` - Integrated watcher: creates `NewWatcher()` after scanner, passes to `handleMessage()` and `serveShutdown()`, activates project on `get_project` message
  - `.chief/prds/uplink/prd.json` - Updated US-011 status
- **Learnings for future iterations:**
  - `fsnotify` watches individual directories, not recursive trees — must add each subdir explicitly (e.g., `.chief/prds/feature/`)
  - `activeProject` struct tracks `watching` bool to prevent duplicate watcher setup
  - Inactivity cleanup runs on a 1-minute ticker; tests override `inactiveTimeout` to milliseconds
  - `handleMessage()` now accepts `*workspace.Watcher` to activate projects on `get_project`
  - `serveShutdown()` now accepts `*workspace.Watcher` to close it during shutdown
  - For git HEAD changes, `fsnotify` sees `HEAD.lock` operations — matching `strings.Contains(subPath, "HEAD")` catches both direct HEAD writes and lock-based updates
---

## 2026-02-15 - US-012
- **What was implemented:** State snapshot on connect/reconnect, `list_projects`, `get_project`, and `get_prd` handlers with proper error responses
- **Files changed:**
  - `internal/cmd/serve.go` - Added `sendStateSnapshot()`, `sendError()`, `handleListProjects()`, `handleGetProject()`, `handleGetPRD()` functions; refactored `RunServe` to create scanner before WebSocket connect for immediate scan availability; `handleMessage` now routes `list_projects`, `get_project`, `get_prd` messages; state snapshot sent after handshake and on reconnect via `WithOnReconnect` callback
  - `internal/cmd/serve_test.go` - Added 7 new tests: `StateSnapshotOnConnect`, `ListProjects`, `GetProject`, `GetProjectNotFound`, `GetPRD`, `GetPRDNotFound`, `GetPRDProjectNotFound`; added `createGitRepo()` helper and `serveTestHelper()` for cleaner test setup; updated PingPong test to handle state_snapshot message
  - `internal/workspace/scanner.go` - Added `SetClient()` to set client after creation and `FindProject()` for single-project lookup by name
- **Learnings for future iterations:**
  - Scanner is now created before WebSocket client (with nil client), then `SetClient()` is called after client creation — this allows initial scan to populate project list before handshake
  - `sendStateSnapshot()` is reused for both initial handshake and reconnect (via `WithOnReconnect` callback)
  - `sendError()` utility includes `request_id` field to help clients correlate errors to their requests
  - `get_prd` reads both `prd.md` (markdown content, optional) and `prd.json` (state) — content can be empty if prd.md doesn't exist yet
  - Test helper `serveTestHelper()` encapsulates the boilerplate: reads hello, sends welcome, reads state_snapshot, then calls custom server function
  - Existing PingPong test needed updating because state_snapshot is now sent before pong — the server must read it first
  - `FindProject()` uses read lock and linear scan — fine for typical workspace sizes (dozens of projects)
---

## 2026-02-15 - US-013
- **What was implemented:** Interactive Claude PRD sessions over WebSocket — spawn, stream output, send messages, and close sessions
- **Files changed:**
  - `internal/cmd/session.go` - New `sessionManager` struct managing Claude PRD sessions: `newPRD()` spawns `claude` with init_prompt, `sendMessage()` writes to stdin, `closeSession()` with save/kill options, `killAll()` for shutdown, `activeSessions()` for state snapshots, auto-conversion of prd.md→prd.json on session end
  - `internal/cmd/session_test.go` - 10 tests: new_prd (real and mock claude), project not found, prd_message session not found, close_prd_session session not found, mock claude lifecycle (spawn→message→close), save close, active sessions tracking, send message with echo verification, close errors, duplicate session prevention
  - `internal/cmd/serve.go` - Integrated `sessionManager`: created after WS client, passed to `handleMessage()`, `serveShutdown()`, `sendStateSnapshot()`; added routing for `new_prd`, `prd_message`, `close_prd_session` message types; state snapshot now includes active sessions
  - `.chief/prds/uplink/prd.json` - Updated US-013 status
- **Learnings for future iterations:**
  - Claude interactive sessions use positional arg (not `-p` flag): `exec.Command("claude", prompt)` — the `-p` flag is for non-interactive/print mode
  - Use shell script mocks in tests: write `#!/bin/sh` script to temp dir, prepend to `PATH` with `t.Setenv("PATH", dir+":"+origPath)` — this allows testing process lifecycle without real claude binary
  - `projectFinder` interface extracted for testability — `*workspace.Scanner` satisfies it implicitly
  - `sessionManager` uses `done` channel per session for synchronization: `close(sess.done)` signals process exit, `<-sess.done` blocks in `closeSession()` and `killAll()`
  - Auto-conversion after session: scans all PRD dirs in project for `prd.NeedsConversion()` — Claude may create new PRD dirs during session
  - `serveTestHelper` reads state_snapshot automatically after handshake — all test server functions receive conn after this step
---

## 2026-02-15 - US-014
- **What was implemented:** Session timeout with warnings — sessions automatically expire after 30 minutes of inactivity with warnings at 20, 25, and 29 minutes
- **Files changed:**
  - `internal/cmd/session.go` - Added `lastActive`/`activeMu` fields to `claudeSession`, `resetActivity()`/`inactiveDuration()` methods, `runTimeoutChecker()` goroutine with configurable check interval, `sendTimeoutWarning()`, `expireSession()` (saves state, kills process, sends `session_expired`), `sendMessage()` now resets inactivity timer, `killAll()` stops timeout checker
  - `internal/cmd/session_test.go` - Added 5 tests: `TimeoutExpiration` (session expires and sends `session_expired`), `TimeoutWarnings` (warnings sent at correct thresholds), `TimeoutResetOnMessage` (prd_message resets timer), `IndependentTimers` (concurrent sessions have separate timers), `TimeoutCheckerGoroutineSafe` (concurrent session creation/messaging while timeout checker runs)
- **Learnings for future iterations:**
  - Timeout checker uses a `stopTimeout` channel (closed by `killAll()`) to cleanly stop the goroutine
  - Warning thresholds and check intervals are configurable on `sessionManager` for fast testing — production uses 30s check interval with 20/25/29 minute thresholds
  - `expireSession()` gives Claude a 2-second grace period to finish writing after closing stdin before force-killing
  - `sentWarnings` map in the checker prevents duplicate warnings — cleaned up when sessions are removed
  - Direct `activeMu.Lock()` and `lastActive` manipulation in tests allows simulating time passage without real waits
  - The process wait goroutine (from `newPRD`) handles cleanup (sends `claude_output done=true`, auto-converts, removes from sessions map); `expireSession` waits for `<-sess.done` so both paths are coordinated
---

## 2026-02-15 - US-015
- **What was implemented:** Run control handlers for start_run, pause_run, resume_run, stop_run via WebSocket
- **Files changed:**
  - `internal/cmd/runs.go` - New `runManager` struct wrapping `engine.Engine` with `startRun()`, `pauseRun()`, `resumeRun()`, `stopRun()`, `activeRuns()`, `stopAll()` methods; handler functions `handleStartRun`, `handlePauseRun`, `handleResumeRun`, `handleStopRun`; `activator` interface for testability
  - `internal/cmd/runs_test.go` - 11 tests: start_run routing, project not found, pause/resume/stop not active errors, run manager unit tests (start+already active, pause/resume, stop, active runs, multiple concurrent projects, state string conversion)
  - `internal/cmd/serve.go` - Integrated engine and run manager: creates `engine.New(5)` and `newRunManager()`, passes to `handleMessage()`, `sendStateSnapshot()`, `serveShutdown()`; routes `start_run`/`pause_run`/`resume_run`/`stop_run` messages; state snapshot now includes active runs; shutdown stops all runs
- **Learnings for future iterations:**
  - Resume is implemented by calling `engine.Start()` again after a paused loop — `Manager.Start()` creates a fresh `Loop` that picks up from the next unfinished story in prd.json
  - `runKey(project, prdID)` creates engine registration key as `"project/prdID"` to support multiple PRDs per project
  - Error strings like `"RUN_ALREADY_ACTIVE"` and `"RUN_NOT_ACTIVE"` are used as sentinel error messages matched in handlers
  - `sendStateSnapshot`, `handleMessage`, and `serveShutdown` now accept `*runManager` parameter — existing tests continued to work because `serveTestHelper` uses these functions indirectly through `RunServe`
  - `activator` interface allows testing `handleStartRun` without a real `workspace.Watcher`
---

## 2026-02-15 - US-016
- **What was implemented:** Quota detection and auto-pause for Ralph loop runs
- **Files changed:**
  - `internal/loop/parser.go` - Added `IsQuotaError()` function with quota/rate-limit pattern matching, `ErrQuotaExhausted` sentinel error, `EventQuotaExhausted` event type
  - `internal/loop/loop.go` - Capture stderr into buffer during `runIteration()`, check for quota patterns on non-zero exit; skip retries for quota errors; emit `EventQuotaExhausted` instead of `EventError`
  - `internal/loop/manager.go` - Set `LoopStatePaused` (not `LoopStateError`) when quota exhaustion is detected, making the run resumable
  - `internal/cmd/runs.go` - Added `startEventMonitor()` goroutine that subscribes to engine events and detects `EventQuotaExhausted`; `handleQuotaExhausted()` sends `run_paused` with `reason: "quota_exhausted"` and `quota_exhausted` message over WebSocket
  - `internal/cmd/serve.go` - Wired up `runs.startEventMonitor(ctx)` after run manager creation
  - `internal/loop/parser_test.go` - Added `TestIsQuotaError` with 16 test cases for pattern matching
  - `internal/cmd/runs_test.go` - Added 5 tests: `HandleQuotaExhausted`, `HandleQuotaExhaustedUnknownRun`, `EventMonitorQuotaDetection`, `QuotaExhaustedWebSocket` (integration test with mock claude), `IsQuotaErrorIntegration`
- **Learnings for future iterations:**
  - Quota errors are detected by checking stderr content and exit code error text against known patterns ("rate limit", "quota", "429", "too many requests", "resource_exhausted", "overloaded")
  - Quota errors bypass retry logic entirely — `runIterationWithRetry` returns immediately with `ErrQuotaExhausted` instead of retrying
  - The manager sets `LoopStatePaused` for quota errors (not `LoopStateError`) so the run can be resumed by the user
  - `runManager.startEventMonitor()` subscribes to engine events and runs in a goroutine — it watches for `EventQuotaExhausted` across all runs
  - When sending WS messages from `handleQuotaExhausted`, must guard against nil client (run manager can be used without a client in tests)
  - `logAndCaptureStream` captures stderr into a `bytes.Buffer` while still logging it — used instead of `logStream` for the stderr pipe
  - Mock claude scripts for quota tests: `echo "rate limit exceeded" >&2; exit 1` simulates quota exhaustion
---
