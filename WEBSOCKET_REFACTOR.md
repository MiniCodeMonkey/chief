# WebSocket Refactor: Managed Reverb Compatibility

## Problem

Laravel Cloud's managed Reverb is a separate service that only runs the standard Pusher protocol routes (`/app/{appKey}`, `/up`, etc.). Our custom `/ws/server` endpoint (registered via `ChiefReverbFactory`) never gets loaded because it lives in our application code, not in the managed Reverb cluster. This means the chief CLI can't connect.

## Solution

Keep managed Reverb for browser broadcasting (it already works). Adapt the chief CLI to:

1. **Send data to the server via HTTP POST** (replaces WebSocket sends)
2. **Receive commands via Reverb's Pusher protocol** (subscribes to a private channel as a standard Pusher client)

No infrastructure changes required — managed Reverb stays as-is.

## Architecture: Before vs After

### Before (custom WebSocket)

```
Chief CLI ──WebSocket /ws/server──→ ChiefServerController ──broadcast──→ Browser
    ↑                                                                       │
    └──── WebSocket (sendToDevice) ←── CommandRelayController ←── HTTP POST─┘
```

- Single persistent WebSocket for all bidirectional communication
- Custom hello/welcome handshake for auth
- In-memory connection tracking (ServerConnectionManager)
- ChiefReverbFactory adds custom route to Reverb server

### After (HTTP + Pusher channel)

```
Chief CLI ──HTTP POST /api/device/messages──→ MessageIngestionController ──broadcast──→ Browser
    ↑                                                                                    │
    └──── Reverb private channel (Pusher protocol) ←── broadcast ←── CommandRelayController ←─┘
```

- CLI sends data via HTTP POST (batched)
- CLI receives commands by subscribing to `private-chief-server.{deviceId}` on managed Reverb
- Auth via OAuth access token (existing) on both HTTP and channel subscription
- No custom Reverb routes, no in-memory connection tracking

---

## Detailed Changes

### Phase 1: New HTTP ingestion endpoint (chief-uplink)

Create a new HTTP API endpoint that accepts messages from chief CLIs — the replacement for the WebSocket receive path.

#### 1.1 New controller: `MessageIngestionController`

**File:** `app/Http/Controllers/Api/MessageIngestionController.php`

Accepts batched messages from the CLI via HTTP POST. Replaces `ChiefServerController::handleMessage()`.

```
POST /api/device/messages
Authorization: Bearer {access_token}
Content-Type: application/json

{
    "messages": [
        {"type": "state_snapshot", "id": "...", "timestamp": "...", ...},
        {"type": "claude_output", "id": "...", "timestamp": "...", ...},
        ...
    ]
}
```

Responsibilities:
- Validate OAuth access token (reuse `DeviceOAuthController::validateAccessToken()`)
- Check device not revoked
- For each message:
  - If `project_state` → update `CachedProjectState` (same as current `handleProjectState()`)
  - If bufferable → buffer via `WebSocketMessageBuffer`
  - Broadcast to browser via `ChiefMessageReceived` event
- Return acknowledgment with any pending server-side state (e.g., new session_id)

Response:
```json
{
    "accepted": 5,
    "session_id": "uuid"
}
```

#### 1.2 New controller: `DevicePresenceController`

**File:** `app/Http/Controllers/Api/DevicePresenceController.php`

Handles explicit connect/disconnect lifecycle — replaces the WebSocket open/close events.

```
POST /api/device/connect
Authorization: Bearer {access_token}
Content-Type: application/json

{
    "chief_version": "0.5.0",
    "device_name": "sierra",
    "os": "darwin",
    "arch": "arm64",
    "protocol_version": 1
}
```

Responsibilities:
- Validate access token, check device not revoked
- Update device metadata (chief_version, os, arch, device_name, is_online, last_connected_at)
- Generate session_id for message buffering
- Mark device reconnected in buffer
- Dispatch `DeviceConnected` event
- Return welcome response (same fields as current WebSocket welcome)

```json
{
    "type": "welcome",
    "protocol_version": 1,
    "device_id": 42,
    "session_id": "uuid",
    "reverb": {
        "key": "app-key",
        "host": "ws-xxx-reverb.laravel.cloud",
        "port": 443,
        "scheme": "https"
    }
}
```

The `reverb` block tells the CLI where to connect as a Pusher client.

```
POST /api/device/disconnect
Authorization: Bearer {access_token}
```

Responsibilities:
- Mark device offline, dispatch `DeviceDisconnected`
- Start buffer grace period

#### 1.3 New middleware: `AuthenticateDevice`

**File:** `app/Http/Middleware/AuthenticateDevice.php`

Extracts and validates the OAuth access token from the `Authorization: Bearer` header. Sets `$request->attributes->set('device_id', ...)` and `$request->attributes->set('user_id', ...)` for downstream controllers. Reuses `DeviceOAuthController::validateAccessToken()`.

Apply to all `/api/device/*` routes.

#### 1.4 Channel auth for CLI devices

**File:** `routes/channels.php`

Add authorization for the new CLI channel:

```php
Broadcast::channel('chief-server.{deviceId}', function ($user, $deviceId) {
    // CLI authenticates channel subscription using its OAuth token.
    // The token is passed as the auth token in the Pusher subscription.
    // We need a custom auth endpoint for this — see 1.5.
    return $user->deviceAuthorizations()
        ->where('id', $deviceId)
        ->whereNull('revoked_at')
        ->exists();
});
```

#### 1.5 Custom Pusher auth endpoint for CLI

The chief CLI authenticates via OAuth access tokens, not Laravel sessions. We need a custom broadcasting auth endpoint that accepts Bearer tokens.

**File:** `app/Http/Controllers/Api/DeviceBroadcastAuthController.php`

```
POST /api/device/broadcasting/auth
Authorization: Bearer {access_token}
Content-Type: application/json

{"socket_id": "...", "channel_name": "private-chief-server.42"}
```

This endpoint:
- Validates the access token
- Checks the device owns the requested channel
- Returns the Pusher auth signature (same format as Laravel's standard broadcast auth)

#### 1.6 Refactor `CommandRelayController`

**File:** `app/Http/Controllers/Api/CommandRelayController.php`

Currently calls `$this->connectionManager->sendToDevice()` which sends via in-memory WebSocket. Change to broadcast the command via Reverb to the CLI's channel:

```php
// Before:
$sent = $this->connectionManager->sendToDevice($deviceId, $message);

// After:
broadcast(new ChiefCommandDispatched($deviceId, $userId, $message));
```

New event: `ChiefCommandDispatched` broadcasts on `private-chief-server.{deviceId}` with event name `chief.command`.

The `isDeviceOnline` check changes from in-memory lookup to checking `DeviceAuthorization.is_online` in the database.

#### 1.7 Refactor `ServerConnectionManager`

The in-memory connection tracking (`$connections`, `$deviceToConnection`, `$connectionObjects`) is no longer needed. The class simplifies to a stateless service that:

- Delegates to `WebSocketMessageBuffer` for buffering
- Checks device online status via database
- No longer stores Connection objects or session IDs in memory (session IDs stored in Redis or on the DeviceAuthorization model)

Alternatively, this class can be removed entirely and its responsibilities distributed to the new controllers.

#### 1.8 New event: `ChiefCommandDispatched`

**File:** `app/Events/ChiefCommandDispatched.php`

```php
class ChiefCommandDispatched implements ShouldBroadcast
{
    public function __construct(
        public readonly int $deviceId,
        public readonly int $userId,
        public readonly array $command,
    ) {}

    public function broadcastOn(): array
    {
        return [new Channel("private-chief-server.{$this->deviceId}")];
    }

    public function broadcastAs(): string
    {
        return 'chief.command';
    }

    public function broadcastWith(): array
    {
        return $this->command;
    }
}
```

#### 1.9 New route: `DeviceHeartbeatController`

The CLI needs to periodically confirm it's still alive, since we no longer have a persistent WebSocket connection to detect disconnects.

```
POST /api/device/heartbeat
Authorization: Bearer {access_token}
```

Called every 30-60 seconds by the CLI. Updates `last_heartbeat_at` on the device. A scheduled job marks devices as offline if no heartbeat received within 2 minutes.

---

### Phase 2: CLI changes (chief — Go)

#### 2.1 New HTTP client: `internal/uplink/client.go`

Replaces the WebSocket client for sending data to the server. Handles:

- `POST /api/device/connect` — on startup
- `POST /api/device/messages` — batched message sending
- `POST /api/device/heartbeat` — periodic keepalive
- `POST /api/device/disconnect` — on shutdown
- OAuth token refresh (existing logic, but applied to HTTP headers)
- Retry with exponential backoff on failure

#### 2.2 Message batcher: `internal/uplink/batcher.go`

Batches outgoing messages to reduce HTTP request volume. Key design:

- Collects messages in a buffer
- Flushes when: buffer reaches N messages (e.g., 20), OR time threshold elapsed (e.g., 200ms), OR a priority message arrives (e.g., `run_complete`)
- Each flush sends a single `POST /api/device/messages` with the batch
- Priority messages (user-visible state changes) flush immediately
- Streaming messages (`claude_output`) batch on the 200ms timer

Categories:
- **Immediate flush:** `run_complete`, `run_paused`, `error`, `clone_complete`, `session_expired`, `quota_exhausted`
- **Batched (200ms):** `claude_output`, `prd_output`, `run_progress`, `clone_progress`
- **Low priority (1s):** `state_snapshot`, `project_state`, `project_list`, `settings`, `log_lines`

#### 2.3 Pusher client: `internal/uplink/pusher.go`

Subscribes to `private-chief-server.{deviceId}` on managed Reverb to receive commands from the browser. Uses a Go Pusher client library (e.g., `pusher/pusher-websocket-go` or a lightweight implementation).

Responsibilities:
- Connect to Reverb as a Pusher client using the app key and host from the connect response
- Authenticate the private channel via `POST /api/device/broadcasting/auth`
- Listen for `chief.command` events
- Route received commands to the existing message dispatcher

The existing `Dispatcher` pattern (type-based routing) stays the same — commands arrive through a different transport but are dispatched identically.

#### 2.4 Refactor `internal/cmd/serve.go`

Replace `ws.Client` usage with the new uplink client:

```go
// Before:
client = ws.New(wsURL, ws.WithOnReconnect(func() { ... }))
client.Connect(ctx)
client.Handshake(creds.AccessToken, version, deviceName)
// ... main loop reads from client.Receive()

// After:
uplink := uplink.New(baseURL, creds.AccessToken, uplink.WithOnReconnect(func() { ... }))
uplink.Connect(ctx)  // POST /api/device/connect + subscribe to Pusher channel
// ... main loop reads from uplink.Receive() (same interface, different transport)
```

The `Send()` method now enqueues into the batcher instead of writing to WebSocket. The `Receive()` channel is fed by the Pusher client instead of WebSocket reads.

#### 2.5 Handshake changes

The current WebSocket handshake (hello → welcome) becomes an HTTP request:

```go
// Before:
client.Handshake(accessToken, version, deviceName)

// After:
welcome, err := httpClient.Connect(ConnectRequest{
    ChiefVersion: version,
    DeviceName:   deviceName,
    OS:           runtime.GOOS,
    Arch:         runtime.GOARCH,
})
// welcome contains device_id, session_id, reverb config
```

#### 2.6 Reconnection logic

Two reconnection paths:

1. **HTTP failures:** Retry with exponential backoff (same as current WebSocket retry). If the server is unreachable, buffer messages locally and flush on reconnect.
2. **Pusher disconnection:** The Pusher client library handles reconnection automatically. On reconnect, re-authenticate the channel.

On any reconnection, re-send state snapshot via HTTP POST (same as current `onRecon` callback).

#### 2.7 Heartbeat

Add a periodic heartbeat goroutine that calls `POST /api/device/heartbeat` every 30 seconds. If the heartbeat fails, trigger reconnection logic.

#### 2.8 Graceful shutdown

On SIGTERM/SIGINT:
1. Stop accepting new commands from Pusher
2. Flush message batcher
3. Call `POST /api/device/disconnect`
4. Disconnect Pusher client
5. Kill Claude sessions and runs (existing logic)

---

### Phase 3: Remove old WebSocket infrastructure (chief-uplink)

After the new system is working:

#### 3.1 Delete files
- `app/WebSocket/ChiefReverbFactory.php`
- `app/WebSocket/ChiefServerController.php`
- `app/Console/Commands/StartReverbServer.php`

#### 3.2 Simplify `WebSocketServiceProvider`
- Remove the `StartServer::class` → `StartReverbServer::class` container binding
- Remove `ServerConnectionManager` singleton if fully replaced
- Keep `PrdSessionManager` singleton

#### 3.3 Remove Reverb dependency from `ServerConnectionManager`
- Remove `use Laravel\Reverb\Servers\Reverb\Connection;`
- Remove `$connectionObjects` array and all methods that reference it
- Or delete the class entirely if all responsibilities moved to new controllers

#### 3.4 Clean up tests
- Update `tests/Feature/WebSocket/MessageRelayTest.php` → test HTTP endpoints
- Add tests for `MessageIngestionController`, `DevicePresenceController`, `DeviceBroadcastAuthController`

---

### Phase 4: Frontend changes (chief-uplink — minimal)

The browser-side code requires almost no changes.

#### 4.1 `CommandRelayController` response

The `isDeviceOnline` check changes from in-memory to database, but the HTTP response contract stays the same. Frontend `useCommandRelay.ts` is unchanged.

#### 4.2 `useChiefMessages.ts` — unchanged

Still subscribes to `private-device.{deviceId}` and listens for `chief.message` events. The messages arrive via the same Reverb channel — only the server-side path that triggers the broadcast changes (HTTP controller instead of WebSocket controller).

#### 4.3 `useEcho.ts` — unchanged

No changes to Echo setup or connection management.

#### 4.4 `echo.ts` — unchanged

Still uses `broadcaster: 'reverb'` with managed Reverb config.

---

## Message Flow Comparison

### CLI → Browser (e.g., `claude_output` streaming)

**Before:**
1. CLI sends `claude_output` JSON over WebSocket
2. `ChiefServerController::handleMessage()` receives it
3. Buffers message via `WebSocketMessageBuffer`
4. Dispatches `ChiefMessageReceived` broadcast event
5. Reverb delivers to browser on `private-device.{deviceId}`

**After:**
1. CLI enqueues `claude_output` into message batcher
2. Batcher flushes batch via `POST /api/device/messages`
3. `MessageIngestionController` receives batch
4. For each message: buffer + dispatch `ChiefMessageReceived`
5. Reverb delivers to browser on `private-device.{deviceId}` (same as before)

### Browser → CLI (e.g., `start_run` command)

**Before:**
1. Browser calls `POST /ws/command/{deviceId}` with `{type: "start_run", payload: {...}}`
2. `CommandRelayController::send()` validates request
3. Calls `ServerConnectionManager::sendToDevice()` → writes to in-memory WebSocket connection
4. CLI receives `start_run` in `readLoop()` → dispatches to handler

**After:**
1. Browser calls `POST /ws/command/{deviceId}` with `{type: "start_run", payload: {...}}` (same)
2. `CommandRelayController::send()` validates request (same)
3. Dispatches `ChiefCommandDispatched` broadcast event on `private-chief-server.{deviceId}`
4. Reverb delivers to CLI's Pusher subscription → dispatches to handler

---

## Device Lifecycle

### Connect

1. CLI calls `POST /api/device/connect` with metadata + access token
2. Server validates token, updates device record, generates session_id
3. Server dispatches `DeviceConnected` event to browser
4. Server returns welcome response with Reverb config
5. CLI connects to Reverb as Pusher client, subscribes to `private-chief-server.{deviceId}`
6. CLI sends initial `state_snapshot` via `POST /api/device/messages`

### Steady state

- CLI sends messages in batches via `POST /api/device/messages` (every 200ms or on priority)
- CLI sends heartbeat via `POST /api/device/heartbeat` (every 30s)
- CLI receives commands via Pusher channel subscription
- Browser sends commands via `POST /ws/command/{deviceId}` (unchanged)
- Browser receives messages via `private-device.{deviceId}` channel (unchanged)

### Disconnect

**Graceful (CLI shutdown):**
1. CLI calls `POST /api/device/disconnect`
2. Server marks device offline, starts buffer grace period
3. Server dispatches `DeviceDisconnected` event

**Ungraceful (network failure, crash):**
1. Heartbeat stops arriving
2. Scheduled job detects stale heartbeat (>2 min)
3. Marks device offline, starts buffer grace period
4. Dispatches `DeviceDisconnected` event

---

## Risks and Mitigations

### Latency from HTTP batching
**Risk:** 200ms batch window adds latency to `claude_output` streaming.
**Mitigation:** 200ms is imperceptible for terminal-like output. Priority messages flush immediately. Can tune batch window down to 100ms if needed.

### HTTP overhead vs WebSocket
**Risk:** More HTTP requests than a single persistent connection.
**Mitigation:** Batching reduces request count significantly. A typical streaming session generates ~5 HTTP requests/second (vs thousands of individual WebSocket frames). HTTP/2 connection reuse minimizes TCP overhead.

### Pusher message size limits
**Risk:** Managed Reverb (Pusher protocol) may have message size limits for channel events.
**Mitigation:** Commands from browser → CLI are small (typically <1KB). The large data flow (CLI → server) goes via HTTP, not Pusher channels. Reverb's default max message size is 10KB, and commands never approach this.

### Heartbeat-based disconnect detection
**Risk:** Up to 2 minutes to detect a crashed CLI (vs instant WebSocket close detection).
**Mitigation:** Acceptable for the use case — the browser already debounces disconnect events by 2 seconds. The "offline" indicator updates within 2 minutes, which is fine. Can reduce heartbeat interval to 15s and detection to 45s if needed.

### Authentication on Pusher channel
**Risk:** The CLI uses OAuth tokens, not Laravel sessions, for auth. Pusher channel auth requires a custom endpoint.
**Mitigation:** `DeviceBroadcastAuthController` provides a standard Pusher auth response using the CLI's OAuth token. The Pusher client library supports custom auth endpoints.

---

## Implementation Order

1. **Phase 1.1–1.3:** New HTTP endpoints + middleware (can deploy independently, no breaking changes)
2. **Phase 1.4–1.5:** Channel auth for CLI (deploy with Phase 1)
3. **Phase 1.8–1.9:** New event + heartbeat (deploy with Phase 1)
4. **Phase 2.1–2.3:** New Go uplink client, batcher, Pusher client
5. **Phase 2.4–2.8:** Refactor serve command to use new client
6. **Phase 1.6–1.7:** Refactor CommandRelayController to broadcast instead of direct WebSocket send
7. **Test end-to-end with both old and new CLI versions**
8. **Phase 3:** Remove old WebSocket infrastructure after confirming new system works
9. **Phase 4:** Any minor frontend adjustments

Total estimate: ~15 new/modified files across both repos.
