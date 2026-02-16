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
- `runManager.startEventMonitor(ctx)` subscribes to engine events for cross-cutting concerns like quota detection, progress streaming, and output forwarding
- `sendStateSnapshot`, `handleMessage`, and `serveShutdown` now accept `*runManager` parameter
- `runInfo` tracks `startTime` and `storyID` for progress messages — `storyID` is updated on `EventStoryStarted`
- `handleEvent()` routes engine events to `sendRunProgress()`, `sendRunComplete()`, `sendClaudeOutput()` based on event type
- Settings handlers in `internal/cmd/settings.go`: `handleGetSettings`, `handleUpdateSettings` — use `config.Load()`/`config.Save()` with project path
- Config uses `*bool` for optional booleans (e.g., `AutoCommit`) and `Effective*()` methods to provide defaults
- Per-story logs: `storyLogger` in `internal/cmd/logs.go` writes to `.chief/prds/<id>/logs/<story-id>.log`; `handleGetLogs` handler retrieves them
- `runManager.loggers` map tracks per-run story loggers; `writeStoryLog()` writes during event handling; loggers cleaned up on `cleanup()`/`stopAll()`
- Per-story diffs: `handleGetDiff` in `internal/cmd/diffs.go` uses `git log --grep <storyID>` to find commits, then `git show` for diff/files; proactive diffs sent on `EventStoryCompleted`
- Clone/create: `handleCloneRepo`/`handleCreateProject` in `internal/cmd/clone.go`; clone runs async in goroutine; `scanner.WorkspacePath()` exposes workspace dir
- Version check/update: `internal/update` package for GitHub Releases API check + binary download; `internal/cmd/update.go` for `RunUpdate()` command
- `update.CheckForUpdate(version, opts)` returns `CheckResult` with `UpdateAvailable` bool; `update.PerformUpdate(version, opts)` does full download+replace
- Startup version check runs non-blocking in `PersistentPreRun`; serve mode checks every 24h and sends `update_available` WS message
- `ServeOptions.ReleasesURL` allows testing the periodic version checker with a mock server

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

## 2026-02-15 - US-017
- **What was implemented:** Run progress streaming — `run_progress`, `run_complete`, and `claude_output` messages sent over WebSocket during active Ralph loop runs
- **Files changed:**
  - `internal/cmd/runs.go` - Extended `startEventMonitor` to handle all engine event types; added `handleEvent()` router, `sendRunProgress()`, `sendRunComplete()`, `sendClaudeOutput()` methods; added `startTime` and `storyID` tracking to `runInfo`
  - `internal/cmd/runs_test.go` - Added 5 tests: `HandleEventRunProgress` (all event types with nil client), `HandleEventUnknownRun`, `HandleEventStoryTracking`, `SendRunComplete`, `RunProgressStreaming` (integration test with mock claude)
  - `.chief/prds/uplink/prd.json` - Updated US-017 status
- **Learnings for future iterations:**
  - `startEventMonitor` was extended (not replaced) — the event loop now handles all event types via `handleEvent()`, not just quota exhaustion
  - `runInfo` tracks `storyID` (updated on `EventStoryStarted`) so that `sendRunProgress` and `sendClaudeOutput` can include it even for events that don't carry a story ID
  - `sendRunComplete` loads the PRD from disk to calculate pass/fail counts — same pattern as other PRD readers in the codebase
  - All send methods guard against nil client, so the run manager can be used in tests without a WebSocket connection
  - Mock claude scripts that output stream-json format are useful for integration testing: `echo '{"type":"system","subtype":"init"}'` triggers `EventIterationStart`
  - `time.Since(info.startTime).Round(time.Second).String()` gives human-readable durations like "5m0s" for the `run_complete` message
---

## 2026-02-15 - US-018
- **What was implemented:** Project settings via WebSocket — `get_settings` and `update_settings` handlers
- **Files changed:**
  - `internal/config/config.go` - Added `MaxIterations`, `AutoCommit` (`*bool`), `CommitPrefix`, `ClaudeModel`, `TestCommand` fields to `Config` struct; added `EffectiveMaxIterations()` and `EffectiveAutoCommit()` helper methods; added `DefaultMaxIterations` constant
  - `internal/config/config_test.go` - Added `TestSaveAndLoadSettingsFields` and `TestEffectiveDefaults` tests
  - `internal/cmd/settings.go` - New file with `handleGetSettings()` and `handleUpdateSettings()` handlers; loads/saves config via `config.Load()`/`config.Save()`; validates `max_iterations >= 1`; partial update support via pointer fields
  - `internal/cmd/settings_test.go` - 7 integration tests: defaults, project not found (get/update), existing config, full update, partial update preserving existing, invalid max_iterations
  - `internal/cmd/serve.go` - Added routing for `get_settings` and `update_settings` messages in `handleMessage()`
  - `.chief/prds/uplink/prd.json` - Updated US-018 status
- **Learnings for future iterations:**
  - Config fields use `*bool` for `AutoCommit` so `false` is distinguishable from "not set" — `EffectiveAutoCommit()` returns `true` when nil
  - `config.Load()` returns `Default()` when config file doesn't exist — no error, just zero values with `Effective*()` providing defaults
  - Settings handlers reuse `projectFinder` interface (same pattern as sessions and runs)
  - `handleUpdateSettings` does load→merge→save pattern: loads existing config, applies only non-nil fields from request, saves back
  - YAML tags use `omitempty` to avoid writing zero values to config file
---

## 2026-02-15 - US-019
- **What was implemented:** Per-story logging during Ralph loop runs and `get_logs` handler
- **Files changed:**
  - `internal/cmd/logs.go` - New `storyLogger` struct for writing per-story log files, `handleGetLogs` handler for retrieving logs via WebSocket, `readLogFile`/`readMostRecentLog`/`sendLogLines` helper functions
  - `internal/cmd/logs_test.go` - 15 tests: story logger write/read, empty story ID, overwrite on new run, line limit, nonexistent files, most recent log, run manager integration, serve integration tests for get_logs (with story ID, without story ID, project not found, PRD not found, line limit), end-to-end logging integration
  - `internal/cmd/runs.go` - Added `loggers` map to `runManager`, `writeStoryLog()` method, logger creation in `startRun()`, logger cleanup in `cleanup()`/`stopAll()`, story log writing in `handleEvent()` for AssistantText/ToolStart/ToolResult/Error events
  - `internal/cmd/serve.go` - Added `get_logs` message routing in `handleMessage()`
  - `.chief/prds/uplink/prd.json` - Updated US-019 status
- **Learnings for future iterations:**
  - Per-story logs are stored at `.chief/prds/<id>/logs/<story-id>.log` — separate from the main `claude.log` which logs all raw output
  - `newStoryLogger()` removes the entire `logs/` directory on creation (V1 simplicity: starting a new run overwrites previous logs)
  - `storyLogger` lazily opens files on first write for each story ID — avoids creating empty log files
  - `readMostRecentLog()` uses file modification time to find the most recently active story's log
  - `readLogFile()` returns empty slice (not error) for nonexistent files — graceful handling of missing logs
  - The `runManager.loggers` map is keyed by the same `runKey(project, prdID)` as the `runs` map
  - Story log writing happens in `handleEvent()` alongside WebSocket message sending — they are parallel operations
  - `handleGetLogs` follows the same `projectFinder` + error handling pattern as settings/sessions handlers
---

## 2026-02-15 - US-020
- **What was implemented:** Per-story diff generation and retrieval via `get_diff` WebSocket handler, plus proactive diff sending on story completion
- **Files changed:**
  - `internal/cmd/diffs.go` - New file with `handleGetDiff` handler, `getStoryDiff()`, `findStoryCommit()`, `getCommitDiff()`, `getCommitFiles()`, `sendDiffMessage()` functions
  - `internal/cmd/diffs_test.go` - 11 tests: getStoryDiff success/no-commit/multiple-files, findStoryCommit most-recent, sendDiffMessage nil-safety, runManager sendStoryDiff, serve integration tests (get_diff success, project not found, PRD not found, no commit)
  - `internal/cmd/runs.go` - Added `sendStoryDiff()` method to `runManager` for proactive diff on `EventStoryCompleted`; added call in `handleEvent()`
  - `internal/cmd/serve.go` - Added `get_diff` message routing in `handleMessage()`
- **Learnings for future iterations:**
  - Story commits follow `feat: <story-id> - <title>` pattern — use `git log --grep <storyID> -1` to find the most recent matching commit
  - `git show --format= --patch <hash>` gives the diff without commit metadata; `--name-only` gives the file list
  - `sendStoryDiff` derives project path from `prdPath` by walking 4 levels up: `prd.json → <id> → prds → .chief → project`
  - `getStoryDiff` is shared between the `handleGetDiff` handler (on-demand) and `sendStoryDiff` (proactive on story completion)
  - `createGitRepoWithStoryCommit()` test helper creates a git repo with a commit matching the story pattern for diff testing
---

## 2026-02-15 - US-021
- **What was implemented:** Git clone and project creation via WebSocket — `clone_repo` and `create_project` handlers
- **Files changed:**
  - `internal/cmd/clone.go` - New file with `handleCloneRepo` (async git clone with progress streaming), `handleCreateProject` (directory creation with optional git init), `inferDirName()`, `runClone()`, `scanGitProgress()`, `sendCloneProgress()`, `sendCloneComplete()` functions
  - `internal/cmd/clone_test.go` - 15 tests: inferDirName, clone success, custom directory name, directory already exists, invalid URL, create project success, create with git init, already exists, empty name, scanGitProgress splitter, percent pattern parsing, nil client safety
  - `internal/cmd/serve.go` - Added `clone_repo` and `create_project` message routing in `handleMessage()`
  - `internal/workspace/scanner.go` - Added `WorkspacePath()` method to expose workspace directory path
  - `.chief/prds/uplink/prd.json` - Updated US-021 status
- **Learnings for future iterations:**
  - Git clone writes progress to stderr, not stdout — use `StderrPipe()` to capture progress
  - Git clone uses `\r` for in-place progress updates — custom `scanGitProgress` splitter handles both `\r` and `\n`
  - Clone runs in a goroutine to avoid blocking the message loop — sends `clone_progress` and `clone_complete` messages asynchronously
  - `inferDirName()` handles both HTTPS URLs and SSH-style URLs (git@github.com:user/repo.git)
  - After clone/create, `scanner.ScanAndUpdate()` is called to make the new project immediately discoverable
  - `create_project` with `git_init: true` sends `project_state` (project is discoverable); without git_init sends `project_list`
  - `WorkspacePath()` was added to `Scanner` to expose the workspace path for clone/create operations
---

## 2026-02-15 - US-022
- **What was implemented:** Version check against GitHub Releases API and `chief update` self-update command
- **Files changed:**
  - `internal/update/update.go` - New package with `CheckForUpdate()`, `PerformUpdate()`, `CompareVersions()`, version normalization, asset finding, download/checksum verification, atomic binary replacement
  - `internal/update/update_test.go` - 19 tests: version check (update available, already latest, dev version, API error, bad JSON), version normalization, version comparison, asset finding (match, no match, no checksum), write permission check, download to temp, checksum verification (success, mismatch), perform update (already latest, full flow), version with v-prefix
  - `internal/cmd/update.go` - `RunUpdate(UpdateOptions)` command, `CheckVersionOnStartup()` (non-blocking goroutine for interactive CLI), `CheckVersionForServe()` for serve mode
  - `internal/cmd/update_test.go` - 6 tests: already latest, API error, serve version check (update available, no update, API failure, dev version)
  - `internal/cmd/serve.go` - Added `runVersionChecker()` goroutine (checks every 24h), `checkAndNotify()` helper that sends `update_available` over WebSocket, added `ReleasesURL` to `ServeOptions` for testing
  - `cmd/chief/main.go` - Added `update` subcommand, `PersistentPreRun` with non-blocking startup version check (skipped for update/serve/version commands), updated help text
- **Learnings for future iterations:**
  - `PerformUpdate()` accepts `currentVersion` as parameter (not discovered from binary) — version is set via ldflags at build time and passed through
  - Asset naming convention: `chief-<GOOS>-<GOARCH>` for binary, `.sha256` suffix for checksum
  - Checksum file format: `"hash  filename"` — use `strings.Fields()` to parse
  - `os.Executable()` + `filepath.EvalSymlinks()` to get the real binary path for replacement
  - Write permission check: try `os.CreateTemp` in the target directory, immediately clean up
  - `PersistentPreRun` on root Cobra command runs before all subcommands — use command name to skip specific commands
  - Startup version check runs in a goroutine (non-blocking) — print message asynchronously; may appear after other output
  - Serve version checker: immediate check on startup + `time.NewTicker(24 * time.Hour)` for periodic checks
  - `update.Options.ReleasesURL` field allows tests to point at mock server (same pattern as `auth.BaseURL`)
- `handleMessage` returns `bool` — `true` signals the serve loop to exit cleanly (for remote update); all other handlers return `false`
- Setup token login: `chief login --setup-token <token>` exchanges via `POST /oauth/device/exchange`; cloud-init uses `CHIEF_SETUP_TOKEN` env var and a one-shot systemd service
- Rate limiting: `ws.NewRateLimiter()` checks in main event loop before `handleMessage()`; `Reset()` on reconnect; ping is exempt; expensive ops (clone_repo, start_run, new_prd) have per-type 2/minute limits
---

## 2026-02-15 - US-023
- **What was implemented:** Remote update via WebSocket — `trigger_update` handler that downloads and installs latest binary, sends confirmation, and exits cleanly for systemd restart
- **Files changed:**
  - `internal/cmd/remote_update.go` - New `handleTriggerUpdate()` function: checks for update, downloads/installs if available, sends `update_available` confirmation or `UPDATE_FAILED` error, returns bool indicating whether serve should exit
  - `internal/cmd/remote_update_test.go` - 4 tests: already latest (unit), API error (unit), serve integration already latest, serve integration API error
  - `internal/cmd/serve.go` - Added `trigger_update` routing in `handleMessage()`, changed `handleMessage` to return bool for exit signaling, main event loop handles exit cleanly after successful update
  - `.chief/prds/uplink/prd.json` - Updated US-023 status
- **Learnings for future iterations:**
  - `handleMessage` now returns a `bool` — returning `true` signals the serve loop to exit cleanly (used for remote update)
  - `handleTriggerUpdate` returns `true` only on successful update; errors and "already latest" return `false`
  - Avoided `os.Exit(0)` in handler — instead, the serve loop performs clean shutdown and returns `nil` error, allowing systemd `Restart=always` to pick up the new binary
  - Integration tests that need `ReleasesURL` cannot use `serveTestHelper` (it doesn't expose that field) — write them manually with the same hello/welcome/state_snapshot pattern
  - Permission errors from `update.PerformUpdate` contain "Permission denied" text — matched via `strings.Contains` to send a descriptive `UPDATE_FAILED` error
---

## 2026-02-15 - US-024
- **What was implemented:** Systemd service unit file and cloud-init setup script for VPS deployment
- **Files changed:**
  - `deploy/chief.service` - Systemd unit file with `ConditionPathExists` for credentials, `Type=simple`, `Restart=always`, `RestartSec=5`, `After=network-online.target`, security hardening directives
  - `deploy/cloud-init.sh` - Idempotent cloud-init script that creates `chief` user, installs Chief binary (via existing `install.sh`), installs Claude Code CLI (via npm), creates workspace directory, writes and enables systemd service (but does NOT start it), prints post-deploy instructions
- **Learnings for future iterations:**
  - Deployment files live in `deploy/` directory at project root
  - Systemd `ConditionPathExists` prevents service from start/restart-looping before auth — service won't start until credentials file exists
  - Cloud-init script reuses the existing `install.sh` for binary installation (via `CHIEF_INSTALL_DIR` env var)
  - Service is `enabled` but not `started` — user must first run `chief login` and authenticate Claude Code before starting
  - Script handles multiple distros for Node.js installation (apt/dnf/yum)
  - `ProtectSystem=strict` + `ReadWritePaths=/home/chief` limits write access to only the chief home directory
---

## 2026-02-15 - US-025
- **What was implemented:** One-time setup token for automated Chief authentication during VPS provisioning
- **Files changed:**
  - `internal/cmd/login.go` - Added `SetupToken` field to `LoginOptions`, `exchangeSetupToken()` function that calls `POST /oauth/device/exchange` with the token and device name, returns credentials on success or falls back to manual login instructions on failure
  - `internal/cmd/login_test.go` - Added 5 tests: setup token success, invalid token, expired token, server error, default device name with setup token
  - `cmd/chief/main.go` - Added `--setup-token` flag to the `login` subcommand
  - `deploy/cloud-init.sh` - Added `handle_setup_token()` function: writes token to `/tmp/chief-setup-token`, creates one-shot `chief-setup.service` that runs `chief login --setup-token` and starts the main service on success; updated usage docs and post-deploy instructions for token mode
- **Learnings for future iterations:**
  - Setup token flow is much simpler than device OAuth — single HTTP POST, no polling, no browser interaction
  - `exchangeSetupToken()` is called early in `RunLogin()` (before the "already logged in" check) since it's non-interactive
  - The one-shot systemd service (`chief-setup.service`) chains `chief login` → `rm token file` → `systemctl start chief` in a single ExecStart
  - `ExecStartPost` cleans up the token file even if the login fails, ensuring the single-use token doesn't persist
  - Cloud-init passes the token via `CHIEF_SETUP_TOKEN` environment variable — safer than command-line args which appear in process listings
---

## 2026-02-16 - US-026
- **What was implemented:** WebSocket rate limiting with token bucket algorithm and per-type expensive operation limiting
- **Files changed:**
  - `internal/ws/ratelimit.go` - New `RateLimiter` struct with global token bucket (30 burst, 10/sec sustained), per-type `expensiveTracker` for expensive ops (2/minute for `clone_repo`, `start_run`, `new_prd`), ping exemption, `Reset()` for reconnection, `FormatRetryAfter()` helper
  - `internal/ws/ratelimit_test.go` - 19 tests: burst allowance, burst exhaustion, token refill, ping exemption, expensive ops limiting, independent expensive trackers, window expiry, reset, expensive-consumes-global, concurrent access, FormatRetryAfter, IsExpensiveType, IsExemptType, tokenBucket retryAfter, expensiveTracker retryAfter
  - `internal/cmd/serve.go` - Created `rateLimiter` before WebSocket client, `Reset()` on reconnect, rate limit check in main event loop before `handleMessage()`, sends `RATE_LIMITED` error with retry-after hint
  - `internal/cmd/serve_test.go` - 3 integration tests: global rate limit exhaustion, ping exemption during rate limiting, expensive operation limiting
- **Learnings for future iterations:**
  - Token bucket is a good fit for global rate limiting: allows bursts while enforcing sustained rate
  - Expensive operations need separate per-type tracking with a sliding window (not token bucket) since the limit is per-minute, not per-second
  - Rate limit check is done in the main event loop (before `handleMessage`) rather than inside `handleMessage` — cleaner separation of concerns
  - `rateLimiter.Reset()` is called in the `WithOnReconnect` callback alongside `sendStateSnapshot` — rate limiter state resets on reconnection
  - Pre-existing race conditions exist in `runManager` tests (unrelated to rate limiting) — these fail with `-race` flag but pass without it
---
