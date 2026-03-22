# Uplink Web App: Device Protocol & Real-time Implementation Plan (Plan 3b)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the device WebSocket endpoint, state caching from protocol messages, command queue, Reverb broadcasting to browsers, and the device OAuth API endpoints.

**Architecture:** A raw WebSocket endpoint at `/ws/device` runs under Octane. `DeviceConnectionManager` tracks live connections in memory. Incoming state messages are validated against contract JSON schemas and upserted into the database. Commands from the browser flow through `pending_commands` and are forwarded over the device WebSocket. State changes trigger Reverb broadcasts to connected browsers.

**Tech Stack:** Laravel 13 Octane, WebSockets (Ratchet or raw Swoole), Laravel Reverb, Pest

**Spec:** `docs/superpowers/specs/2026-03-22-uplink-webapp-design.md` and `docs/superpowers/specs/2026-03-21-uplink-network-protocol-design.md`

**Prerequisite:** Plan 3a (Scaffolding & Auth) must be completed first.

---

## File Structure

```
app/
├── Models/
│   ├── Device.php
│   ├── Project.php
│   ├── Prd.php
│   ├── Run.php
│   └── PendingCommand.php
├── Services/
│   ├── DeviceConnectionManager.php     ← tracks live WebSocket connections in memory
│   ├── DeviceProtocol/
│   │   ├── MessageValidator.php        ← validates against contract JSON schemas
│   │   ├── StateHandler.php            ← processes state.* messages → DB upserts
│   │   ├── ControlHandler.php          ← processes ack/error → updates pending_commands
│   │   └── MessageRouter.php           ← routes messages by type to handlers
│   └── CommandService.php              ← creates pending_commands, forwards to device
├── Http/
│   ├── Controllers/
│   │   └── Api/
│   │       ├── DeviceAuthController.php    ← device OAuth endpoints (request, verify, provision)
│   │       └── DeviceCommandController.php ← browser → command → device
│   └── Middleware/
│       └── AuthenticateDevice.php          ← validates device bearer token on WS
├── Events/
│   ├── DeviceConnected.php
│   ├── DeviceDisconnected.php
│   ├── DeviceStateUpdated.php          ← generic: project/PRD/run state changed
│   └── DeviceStreamEvent.php           ← ephemeral: run output, chat output, logs
├── WebSocket/
│   └── DeviceWebSocketHandler.php      ← Octane WebSocket handler for /ws/device
database/
└── migrations/
    ├── xxxx_create_devices_table.php
    ├── xxxx_create_projects_table.php
    ├── xxxx_create_prds_table.php
    ├── xxxx_create_runs_table.php
    └── xxxx_create_pending_commands_table.php
contract/
└── schemas/                            ← copied/symlinked from chief repo
tests/
├── Feature/
│   ├── DeviceAuthTest.php
│   ├── DeviceWebSocketTest.php
│   ├── StateHandlerTest.php
│   └── CommandServiceTest.php
└── Unit/
    ├── MessageValidatorTest.php
    └── DeviceConnectionManagerTest.php
```

---

### Task 1: Device-related Migrations

**Files:**
- Create: 5 migration files for devices, projects, prds, runs, pending_commands

- [ ] **Step 1: Create devices migration**

```php
Schema::create('devices', function (Blueprint $table) {
    $table->id();
    $table->foreignId('team_id')->constrained()->cascadeOnDelete();
    $table->unsignedBigInteger('managed_server_id')->nullable(); // FK added in Plan 3d after managed_servers table exists
    $table->string('name');
    $table->string('os')->nullable();
    $table->string('arch')->nullable();
    $table->string('chief_version')->nullable();
    $table->string('access_token', 64)->unique();
    $table->string('refresh_token_hash', 64)->unique();
    $table->timestamp('token_expires_at');
    $table->timestamp('last_seen_at')->nullable();
    $table->boolean('connected')->default(false);
    $table->timestamps();

    $table->index('team_id');
});
```

- [ ] **Step 2: Create projects migration**

```php
Schema::create('projects', function (Blueprint $table) {
    $table->id();
    $table->foreignId('device_id')->constrained()->cascadeOnDelete();
    $table->string('external_id')->comment('ID from chief, e.g. proj_myapp');
    $table->string('path');
    $table->string('name');
    $table->string('git_remote')->nullable();
    $table->string('git_branch')->nullable();
    $table->string('git_status')->nullable();
    $table->string('last_commit_hash')->nullable();
    $table->string('last_commit_message')->nullable();
    $table->timestamp('last_commit_at')->nullable();
    $table->timestamps();

    $table->unique(['device_id', 'external_id']);
});
```

- [ ] **Step 3: Create prds migration**

```php
Schema::create('prds', function (Blueprint $table) {
    $table->id();
    $table->foreignId('project_id')->constrained()->cascadeOnDelete();
    $table->foreignId('device_id')->constrained()->cascadeOnDelete();
    $table->string('external_id')->comment('ID from chief, e.g. prd_auth');
    $table->string('title');
    $table->string('status')->default('draft');
    $table->text('content')->nullable()->comment('Encrypted');
    $table->text('progress')->nullable()->comment('Encrypted');
    $table->text('chat_history')->nullable()->comment('Encrypted');
    $table->string('session_id')->nullable()->comment('Claude Code session ID');
    $table->timestamps();

    $table->unique(['device_id', 'external_id']);
});
```

- [ ] **Step 4: Create runs migration**

```php
Schema::create('runs', function (Blueprint $table) {
    $table->id();
    $table->foreignId('prd_id')->constrained()->cascadeOnDelete();
    $table->foreignId('device_id')->constrained()->cascadeOnDelete();
    $table->string('external_id')->comment('ID from chief');
    $table->string('status')->default('running');
    $table->string('result')->nullable();
    $table->text('error_message')->nullable();
    $table->integer('story_index')->default(0);
    $table->timestamp('started_at')->nullable();
    $table->timestamp('completed_at')->nullable();
    $table->timestamps();

    $table->unique(['device_id', 'external_id']);
});
```

- [ ] **Step 5: Create pending_commands migration**

```php
Schema::create('pending_commands', function (Blueprint $table) {
    $table->id();
    $table->foreignId('device_id')->constrained()->cascadeOnDelete();
    $table->foreignId('user_id')->nullable()->constrained()->nullOnDelete();
    $table->string('message_id', 64)->unique()->comment('Protocol message ID');
    $table->string('type')->comment('e.g. cmd.run.start');
    $table->text('payload')->comment('Encrypted JSON');
    $table->enum('status', ['pending', 'delivered', 'failed'])->default('pending');
    $table->timestamp('delivered_at')->nullable();
    $table->timestamps();

    $table->index(['device_id', 'status']);
});
```

- [ ] **Step 6: Run migrations**

```bash
docker compose exec app php artisan migrate
```

- [ ] **Step 7: Commit**

```bash
git add database/migrations/
git commit -m "feat: add device, project, prd, run, and pending_commands migrations"
```

---

### Task 2: Eloquent Models

**Files:**
- Create: `app/Models/Device.php`, `Project.php`, `Prd.php`, `Run.php`, `PendingCommand.php`

- [ ] **Step 1: Write failing test**

```php
// tests/Feature/DeviceAuthTest.php (initial model test)
it('creates a device belonging to a team', function () {
    $user = User::factory()->create();
    $team = $user->currentTeam();

    $device = Device::create([
        'team_id' => $team->id,
        'name' => 'test-server',
        'access_token' => hash('sha256', 'test-token'),
        'refresh_token' => 'encrypted-refresh',
        'token_expires_at' => now()->addHour(),
    ]);

    expect($device->team->id)->toBe($team->id);
    expect($team->devices)->toHaveCount(1);
});
```

- [ ] **Step 2: Implement Device model**

```php
// app/Models/Device.php
<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Device extends Model
{
    protected $fillable = [
        'team_id', 'managed_server_id', 'name', 'os', 'arch',
        'chief_version', 'access_token', 'refresh_token_hash',
        'token_expires_at', 'last_seen_at', 'connected',
    ];

    protected function casts(): array
    {
        return [
            'token_expires_at' => 'datetime',
            'last_seen_at' => 'datetime',
            'connected' => 'boolean',
        ];
    }

    public function team(): BelongsTo { return $this->belongsTo(Team::class); }
    public function projects(): HasMany { return $this->hasMany(Project::class); }
    public function prds(): HasMany { return $this->hasMany(Prd::class); }
    public function runs(): HasMany { return $this->hasMany(Run::class); }
    public function pendingCommands(): HasMany { return $this->hasMany(PendingCommand::class); }

    public static function findByToken(string $token): ?self
    {
        return static::where('access_token', hash('sha256', $token))->first();
    }
}
```

- [ ] **Step 3: Implement Project, Prd, Run, PendingCommand models**

```php
// app/Models/Project.php
class Project extends Model
{
    protected $fillable = [
        'device_id', 'external_id', 'path', 'name', 'git_remote',
        'git_branch', 'git_status', 'last_commit_hash',
        'last_commit_message', 'last_commit_at',
    ];

    protected function casts(): array
    {
        return ['last_commit_at' => 'datetime'];
    }

    public function device(): BelongsTo { return $this->belongsTo(Device::class); }
    public function prds(): HasMany { return $this->hasMany(Prd::class); }
}

// app/Models/Prd.php
class Prd extends Model
{
    protected $fillable = [
        'project_id', 'device_id', 'external_id', 'title', 'status',
        'content', 'progress', 'chat_history', 'session_id',
    ];

    protected function casts(): array
    {
        return [
            'content' => 'encrypted',
            'progress' => 'encrypted',
            'chat_history' => 'encrypted:array',
        ];
    }

    public function project(): BelongsTo { return $this->belongsTo(Project::class); }
    public function device(): BelongsTo { return $this->belongsTo(Device::class); }
    public function runs(): HasMany { return $this->hasMany(Run::class); }
}

// app/Models/Run.php
class Run extends Model
{
    protected $fillable = [
        'prd_id', 'device_id', 'external_id', 'status', 'result',
        'error_message', 'story_index', 'started_at', 'completed_at',
    ];

    protected function casts(): array
    {
        return [
            'started_at' => 'datetime',
            'completed_at' => 'datetime',
        ];
    }

    public function prd(): BelongsTo { return $this->belongsTo(Prd::class); }
    public function device(): BelongsTo { return $this->belongsTo(Device::class); }
}

// app/Models/PendingCommand.php
class PendingCommand extends Model
{
    protected $fillable = [
        'device_id', 'user_id', 'message_id', 'type',
        'payload', 'status', 'delivered_at',
    ];

    protected function casts(): array
    {
        return [
            'payload' => 'encrypted:array',
            'delivered_at' => 'datetime',
        ];
    }

    public function device(): BelongsTo { return $this->belongsTo(Device::class); }
    public function scopePending($query) { return $query->where('status', 'pending'); }
}
```

- [ ] **Step 4: Add `devices()` relationship to Team model**

```php
// app/Models/Team.php — add:
public function devices(): HasMany { return $this->hasMany(Device::class); }
```

- [ ] **Step 5: Run tests**

```bash
docker compose exec app php artisan test --filter="creates a device"
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/Models/
git commit -m "feat: add Device, Project, Prd, Run, PendingCommand models"
```

---

### Task 3: Device OAuth API Endpoints

**Files:**
- Create: `app/Http/Controllers/Api/DeviceAuthController.php`
- Create: `tests/Feature/DeviceAuthTest.php`

These are the REST endpoints that `chief login` talks to.

- [ ] **Step 1: Write failing tests**

```php
// tests/Feature/DeviceAuthTest.php
<?php

use App\Models\User;
use App\Models\Device;

it('issues a device code', function () {
    $this->postJson('/api/auth/device/request', [
        'device_name' => 'my-laptop',
    ])->assertOk()->assertJsonStructure([
        'device_code', 'user_code', 'verify_url', 'expires_in', 'interval',
    ]);
});

it('returns pending when device code not yet approved', function () {
    $response = $this->postJson('/api/auth/device/request', ['device_name' => 'test']);
    $deviceCode = $response->json('device_code');

    $this->postJson('/api/auth/device/verify', [
        'device_code' => $deviceCode,
    ])->assertStatus(202)->assertJson(['status' => 'pending']);
});

it('returns tokens when device code is approved', function () {
    $user = User::factory()->create();
    $team = $user->currentTeam();

    $response = $this->postJson('/api/auth/device/request', ['device_name' => 'test']);
    $deviceCode = $response->json('device_code');
    $userCode = $response->json('user_code');

    // Simulate user approval
    DB::table('device_codes')
        ->where('device_code', $deviceCode)
        ->update(['user_id' => $user->id, 'team_id' => $team->id, 'approved_at' => now()]);

    $this->postJson('/api/auth/device/verify', [
        'device_code' => $deviceCode,
    ])->assertOk()->assertJsonStructure([
        'access_token', 'refresh_token', 'expires_in', 'device_id',
    ]);

    // Device should be created
    expect(Device::where('team_id', $team->id)->count())->toBe(1);
});

it('provisions a device token directly (for VPS automation)', function () {
    $user = User::factory()->create();
    $team = $user->currentTeam();

    $this->actingAs($user)->postJson('/api/auth/device/provision', [
        'team_id' => $team->id,
        'server_name' => 'chief-worker-01',
    ])->assertOk()->assertJsonStructure([
        'access_token', 'refresh_token', 'expires_in', 'device_id',
    ]);

    expect(Device::where('team_id', $team->id)->where('name', 'chief-worker-01')->count())->toBe(1);
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker compose exec app php artisan test --filter="DeviceAuthTest"
```

- [ ] **Step 3: Implement DeviceAuthController**

```php
// app/Http/Controllers/Api/DeviceAuthController.php
<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\Device;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Str;

class DeviceAuthController extends Controller
{
    public function request(Request $request)
    {
        $request->validate(['device_name' => ['required', 'string']]);

        $deviceCode = Str::random(40);
        $userCode = strtoupper(Str::random(4) . '-' . Str::random(4));

        DB::table('device_codes')->insert([
            'device_code' => $deviceCode,
            'user_code' => $userCode,
            'device_name' => $request->device_name,
            'expires_at' => now()->addMinutes(15),
            'created_at' => now(),
            'updated_at' => now(),
        ]);

        return response()->json([
            'device_code' => $deviceCode,
            'user_code' => $userCode,
            'verify_url' => config('app.url') . '/activate',
            'expires_in' => 900,
            'interval' => 5,
        ]);
    }

    public function verify(Request $request)
    {
        $request->validate(['device_code' => ['required', 'string']]);

        $code = DB::table('device_codes')
            ->where('device_code', $request->device_code)
            ->first();

        if (! $code || now()->isAfter($code->expires_at)) {
            return response()->json(['error' => 'expired'], 410);
        }

        if (! $code->approved_at) {
            return response()->json(['status' => 'pending'], 202);
        }

        // Create device with tokens
        $accessToken = Str::random(64);
        $refreshToken = Str::random(64);

        $device = Device::create([
            'team_id' => $code->team_id,
            'name' => $code->device_name ?? 'unknown',
            'access_token' => hash('sha256', $accessToken),
            'refresh_token_hash' => hash('sha256', $refreshToken),
            'token_expires_at' => now()->addDays(30),
        ]);

        // Clean up used code
        DB::table('device_codes')->where('id', $code->id)->delete();

        return response()->json([
            'access_token' => $accessToken,
            'refresh_token' => $refreshToken,
            'expires_in' => 30 * 24 * 3600,
            'device_id' => 'dev_' . $device->id,
        ]);
    }

    public function provision(Request $request)
    {
        $request->validate([
            'team_id' => ['required', 'exists:teams,id'],
            'server_name' => ['required', 'string'],
        ]);

        // Verify user belongs to team and is owner
        $team = $request->user()->teams()
            ->where('team_id', $request->team_id)
            ->wherePivot('role', 'owner')
            ->firstOrFail();

        $accessToken = Str::random(64);
        $refreshToken = Str::random(64);

        $device = Device::create([
            'team_id' => $team->id,
            'name' => $request->server_name,
            'access_token' => hash('sha256', $accessToken),
            'refresh_token_hash' => hash('sha256', $refreshToken),
            'token_expires_at' => now()->addDays(90),
        ]);

        return response()->json([
            'access_token' => $accessToken,
            'refresh_token' => $refreshToken,
            'expires_in' => 90 * 24 * 3600,
            'device_id' => 'dev_' . $device->id,
        ]);
    }

    public function refresh(Request $request)
    {
        $request->validate(['refresh_token' => ['required', 'string']]);

        // Refresh tokens are hashed like access tokens for DB lookup
        $device = Device::where('refresh_token_hash', hash('sha256', $request->refresh_token))->first();

        if (! $device) {
            return response()->json(['error' => 'invalid_token'], 401);
        }

        $newAccessToken = Str::random(64);
        $newRefreshToken = Str::random(64);
        $device->update([
            'access_token' => hash('sha256', $newAccessToken),
            'refresh_token_hash' => hash('sha256', $newRefreshToken),
            'token_expires_at' => now()->addDays(30),
        ]);

        return response()->json([
            'access_token' => $newAccessToken,
            'refresh_token' => $device->refresh_token,
            'expires_in' => 30 * 24 * 3600,
        ]);
    }

    public function revoke(Request $request, int $deviceId)
    {
        $device = Device::findOrFail($deviceId);

        // Must belong to user's team
        $request->user()->teams()->where('team_id', $device->team_id)->firstOrFail();

        $device->delete();

        return response()->noContent();
    }
}
```

- [ ] **Step 4: Add API routes**

```php
// routes/api.php
use App\Http\Controllers\Api\DeviceAuthController;

Route::prefix('auth/device')->group(function () {
    Route::post('request', [DeviceAuthController::class, 'request']);
    Route::post('verify', [DeviceAuthController::class, 'verify']);
    Route::post('refresh', [DeviceAuthController::class, 'refresh']);
});

Route::middleware('auth:sanctum')->group(function () {
    Route::post('auth/device/provision', [DeviceAuthController::class, 'provision']);
    Route::delete('auth/device/{deviceId}', [DeviceAuthController::class, 'revoke']);
});
```

- [ ] **Step 5: Run tests**

```bash
docker compose exec app php artisan test --filter="DeviceAuthTest"
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add app/Http/Controllers/Api/ routes/api.php tests/
git commit -m "feat: add device OAuth API endpoints (request, verify, provision, refresh, revoke)"
```

---

### Task 4: Contract Schema Validation (PHP)

**Files:**
- Create: `app/Services/DeviceProtocol/MessageValidator.php`
- Create: `tests/Unit/MessageValidatorTest.php`
- Symlink or copy: `contract/schemas/` from chief repo

- [ ] **Step 1: Install JSON Schema validator**

```bash
composer require opis/json-schema
```

- [ ] **Step 2: Set up contract schemas**

```bash
# Option A: git submodule
git submodule add https://github.com/minicodemonkey/chief.git contract-source
ln -s contract-source/contract contract

# Option B: copy schemas manually and track in this repo
mkdir -p contract/schemas
# Copy schemas from chief repo
```

- [ ] **Step 3: Write failing test**

```php
// tests/Unit/MessageValidatorTest.php
<?php

use App\Services\DeviceProtocol\MessageValidator;

it('validates a valid state.sync envelope', function () {
    $validator = new MessageValidator(base_path('contract/schemas'));

    $message = json_decode(file_get_contents(base_path('contract/fixtures/state/sync.valid.json')), true);

    expect($validator->validate($message))->toBeTrue();
});

it('rejects an invalid envelope', function () {
    $validator = new MessageValidator(base_path('contract/schemas'));

    $message = ['type' => 'invalid', 'payload' => []]; // missing id, device_id, timestamp

    expect($validator->validate($message))->toBeFalse();
    expect($validator->errors())->not->toBeEmpty();
});
```

- [ ] **Step 4: Implement MessageValidator**

```php
// app/Services/DeviceProtocol/MessageValidator.php
<?php

namespace App\Services\DeviceProtocol;

use Opis\JsonSchema\Validator;
use Opis\JsonSchema\Errors\ErrorFormatter;

class MessageValidator
{
    private Validator $validator;
    private string $schemasDir;
    private array $lastErrors = [];

    public function __construct(string $schemasDir)
    {
        $this->schemasDir = $schemasDir;
        $this->validator = new Validator();
        $this->validator->resolver()->registerPrefix(
            'file://' . realpath($schemasDir) . '/',
            realpath($schemasDir) . '/'
        );
    }

    public function validate(array $message): bool
    {
        $this->lastErrors = [];

        // Validate envelope
        $envelopeSchema = 'file://' . realpath($this->schemasDir) . '/envelope.json';
        $result = $this->validator->validate(
            json_decode(json_encode($message)),
            $envelopeSchema
        );

        if (! $result->isValid()) {
            $this->lastErrors = (new ErrorFormatter())->format($result->error());
            return false;
        }

        return true;
    }

    public function errors(): array
    {
        return $this->lastErrors;
    }
}
```

- [ ] **Step 5: Run tests**

```bash
docker compose exec app php artisan test --filter="MessageValidatorTest"
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/Services/DeviceProtocol/MessageValidator.php tests/Unit/ contract/
git commit -m "feat: add contract schema validation for device protocol messages"
```

---

### Task 5: State Handler (state.* messages → DB)

**Files:**
- Create: `app/Services/DeviceProtocol/StateHandler.php`
- Create: `tests/Feature/StateHandlerTest.php`

- [ ] **Step 1: Write failing test**

```php
// tests/Feature/StateHandlerTest.php
<?php

use App\Models\Device;
use App\Models\User;
use App\Models\Project;
use App\Models\Prd;
use App\Services\DeviceProtocol\StateHandler;

it('processes state.sync and creates projects and prds', function () {
    $user = User::factory()->create();
    $device = Device::create([
        'team_id' => $user->currentTeam()->id,
        'name' => 'test',
        'access_token' => hash('sha256', 'tok'),
        'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
    ]);

    $handler = new StateHandler();
    $handler->handleSync($device, [
        'device' => ['name' => 'test-vps', 'os' => 'linux', 'arch' => 'amd64', 'chief_version' => '0.5.0'],
        'projects' => [[
            'id' => 'proj_myapp', 'path' => '/home/chief/workspace/myapp',
            'name' => 'myapp', 'git_branch' => 'main', 'git_status' => 'clean',
        ]],
        'prds' => [[
            'id' => 'prd_auth', 'project_id' => 'proj_myapp', 'title' => 'Auth',
            'status' => 'ready', 'content' => '# Auth PRD',
            'chat_history' => [['role' => 'user', 'content' => 'Add OAuth', 'timestamp' => '2026-03-21T08:00:00Z']],
            'session_id' => 'sess_abc123',
        ]],
        'runs' => [],
    ]);

    expect($device->fresh()->chief_version)->toBe('0.5.0');
    expect($device->projects)->toHaveCount(1);
    expect($device->prds)->toHaveCount(1);
    expect($device->prds->first()->session_id)->toBe('sess_abc123');
});

it('replaces entire device cache on sync', function () {
    $user = User::factory()->create();
    $device = Device::create([
        'team_id' => $user->currentTeam()->id,
        'name' => 'test',
        'access_token' => hash('sha256', 'tok'),
        'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
    ]);

    // Existing stale data
    $device->projects()->create(['external_id' => 'proj_old', 'path' => '/old', 'name' => 'old']);

    $handler = new StateHandler();
    $handler->handleSync($device, [
        'device' => ['name' => 'test', 'os' => 'linux', 'arch' => 'amd64', 'chief_version' => '0.5.0'],
        'projects' => [['id' => 'proj_new', 'path' => '/new', 'name' => 'new']],
        'prds' => [],
        'runs' => [],
    ]);

    expect($device->projects()->count())->toBe(1);
    expect($device->projects->first()->external_id)->toBe('proj_new');
});
```

- [ ] **Step 2: Implement StateHandler**

```php
// app/Services/DeviceProtocol/StateHandler.php
<?php

namespace App\Services\DeviceProtocol;

use App\Events\DeviceStateUpdated;
use App\Models\Device;
use App\Models\Prd;
use App\Models\Project;
use App\Models\Run;
use Illuminate\Support\Facades\DB;

class StateHandler
{
    public function handleSync(Device $device, array $payload): void
    {
        DB::transaction(function () use ($device, $payload) {
            // Update device info
            $device->update([
                'name' => $payload['device']['name'] ?? $device->name,
                'os' => $payload['device']['os'] ?? null,
                'arch' => $payload['device']['arch'] ?? null,
                'chief_version' => $payload['device']['chief_version'] ?? null,
                'last_seen_at' => now(),
            ]);

            // Replace all cached state for this device
            $device->runs()->delete();
            $device->prds()->delete();
            $device->projects()->delete();

            // Re-create projects
            $projectMap = [];
            foreach ($payload['projects'] ?? [] as $proj) {
                $project = $device->projects()->create([
                    'external_id' => $proj['id'],
                    'path' => $proj['path'],
                    'name' => $proj['name'],
                    'git_remote' => $proj['git_remote'] ?? null,
                    'git_branch' => $proj['git_branch'] ?? null,
                    'git_status' => $proj['git_status'] ?? null,
                    'last_commit_hash' => $proj['last_commit']['hash'] ?? null,
                    'last_commit_message' => $proj['last_commit']['message'] ?? null,
                    'last_commit_at' => $proj['last_commit']['timestamp'] ?? null,
                ]);
                $projectMap[$proj['id']] = $project->id;
            }

            // Re-create PRDs
            $prdMap = [];
            foreach ($payload['prds'] ?? [] as $prd) {
                $projectId = $projectMap[$prd['project_id']] ?? null;
                if (! $projectId) continue;

                $created = $device->prds()->create([
                    'project_id' => $projectId,
                    'external_id' => $prd['id'],
                    'title' => $prd['title'],
                    'status' => $prd['status'],
                    'content' => $prd['content'] ?? null,
                    'progress' => $prd['progress'] ?? null,
                    'chat_history' => $prd['chat_history'] ?? null,
                    'session_id' => $prd['session_id'] ?? null,
                ]);
                $prdMap[$prd['id']] = $created->id;
            }

            // Re-create runs
            foreach ($payload['runs'] ?? [] as $run) {
                $prdId = $prdMap[$run['prd_id']] ?? null;
                if (! $prdId) continue;

                $device->runs()->create([
                    'prd_id' => $prdId,
                    'external_id' => $run['id'],
                    'status' => $run['status'],
                    'result' => $run['result'] ?? null,
                    'error_message' => $run['error_message'] ?? null,
                    'story_index' => $run['story_index'] ?? 0,
                    'started_at' => $run['started_at'] ?? null,
                    'completed_at' => $run['completed_at'] ?? null,
                ]);
            }
        });

        DeviceStateUpdated::dispatch($device);
    }

    public function handlePrdUpdated(Device $device, array $payload): void
    {
        $prd = $device->prds()->where('external_id', $payload['prd']['id'])->first();
        if (! $prd) return;

        $prd->update([
            'title' => $payload['prd']['title'] ?? $prd->title,
            'status' => $payload['prd']['status'] ?? $prd->status,
            'content' => $payload['prd']['content'] ?? $prd->content,
            'progress' => $payload['prd']['progress'] ?? $prd->progress,
            'chat_history' => $payload['prd']['chat_history'] ?? $prd->chat_history,
            'session_id' => $payload['prd']['session_id'] ?? $prd->session_id,
        ]);

        DeviceStateUpdated::dispatch($device);
    }

    public function handleProjectsUpdated(Device $device, array $payload): void
    {
        DB::transaction(function () use ($device, $payload) {
            // Only update projects — do NOT touch PRDs or runs
            $existingIds = $device->projects()->pluck('external_id')->all();
            $newIds = collect($payload['projects'] ?? [])->pluck('id')->all();

            // Delete removed projects (cascades to their PRDs/runs via FK)
            $device->projects()->whereNotIn('external_id', $newIds)->delete();

            // Upsert projects
            foreach ($payload['projects'] ?? [] as $proj) {
                $device->projects()->updateOrCreate(
                    ['external_id' => $proj['id']],
                    [
                        'path' => $proj['path'],
                        'name' => $proj['name'],
                        'git_remote' => $proj['git_remote'] ?? null,
                        'git_branch' => $proj['git_branch'] ?? null,
                        'git_status' => $proj['git_status'] ?? null,
                        'last_commit_hash' => $proj['last_commit']['hash'] ?? null,
                        'last_commit_message' => $proj['last_commit']['message'] ?? null,
                        'last_commit_at' => $proj['last_commit']['timestamp'] ?? null,
                    ]
                );
            }
        });

        DeviceStateUpdated::dispatch($device);
    }

    public function handleRunProgress(Device $device, array $payload): void
    {
        $run = $device->runs()->where('external_id', $payload['run']['id'])->first();
        if (! $run) return;

        $run->update([
            'status' => $payload['run']['status'] ?? $run->status,
            'result' => $payload['run']['result'] ?? $run->result,
            'error_message' => $payload['run']['error_message'] ?? $run->error_message,
            'story_index' => $payload['run']['story_index'] ?? $run->story_index,
            'completed_at' => $payload['run']['completed_at'] ?? $run->completed_at,
        ]);

        DeviceStateUpdated::dispatch($device);
    }
}
```

- [ ] **Step 3: Create events**

```php
// app/Events/DeviceStateUpdated.php
class DeviceStateUpdated implements ShouldBroadcast
{
    public function __construct(public Device $device) {}

    public function broadcastOn(): array
    {
        return [
            new PrivateChannel('team.' . $this->device->team_id),
            new PrivateChannel('device.' . $this->device->id),
        ];
    }
}

// app/Events/DeviceStreamEvent.php (ephemeral, not stored)
class DeviceStreamEvent implements ShouldBroadcast
{
    public function __construct(
        public Device $device,
        public string $eventType,
        public array $payload,
    ) {}

    public function broadcastOn(): array
    {
        return [new PrivateChannel('device.' . $this->device->id)];
    }
}
```

- [ ] **Step 4: Run tests**

```bash
docker compose exec app php artisan test --filter="StateHandlerTest"
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add app/Services/DeviceProtocol/StateHandler.php app/Events/ tests/
git commit -m "feat: add state handler for device protocol messages"
```

---

### Task 6: Message Router & WebSocket Handler

**Files:**
- Create: `app/Services/DeviceProtocol/MessageRouter.php`
- Create: `app/WebSocket/DeviceWebSocketHandler.php`
- Create: `app/Services/DeviceConnectionManager.php`

- [ ] **Step 1: Implement MessageRouter**

Routes incoming device messages to the appropriate handler by type prefix.

```php
// app/Services/DeviceProtocol/MessageRouter.php
<?php

namespace App\Services\DeviceProtocol;

use App\Events\DeviceStreamEvent;
use App\Models\Device;
use App\Models\PendingCommand;

class MessageRouter
{
    public function __construct(
        private StateHandler $stateHandler,
    ) {}

    public function route(Device $device, array $message): void
    {
        $type = $message['type'] ?? '';
        $payload = $message['payload'] ?? [];

        match (true) {
            $type === 'state.sync' => $this->stateHandler->handleSync($device, $payload),
            str_starts_with($type, 'state.prd.created'),
            str_starts_with($type, 'state.prd.updated') => $this->stateHandler->handlePrdUpdated($device, $payload),
            str_starts_with($type, 'state.prd.deleted') => $device->prds()->where('external_id', $payload['prd_id'] ?? '')->delete(),
            str_starts_with($type, 'state.run.') => $this->stateHandler->handleRunProgress($device, $payload),
            str_starts_with($type, 'state.projects.updated') => $this->stateHandler->handleProjectsUpdated($device, $payload),
            $type === 'state.device.heartbeat' => $device->update(['last_seen_at' => now()]),

            // Ephemeral messages — relay to Reverb, don't store
            in_array($type, ['state.run.output', 'state.prd.chat.output', 'state.log.output', 'state.project.clone.progress'])
                => DeviceStreamEvent::dispatch($device, $type, $payload),

            // Response messages — relay to Reverb for waiting browser, cache if applicable
            in_array($type, ['state.diffs.response', 'state.files.list', 'state.file.response', 'state.log.response', 'state.settings.updated'])
                => DeviceStreamEvent::dispatch($device, $type, $payload),

            // Ack/Error — update pending_commands
            $type === 'ack' => PendingCommand::where('message_id', $payload['ref_id'] ?? '')->update(['status' => 'delivered', 'delivered_at' => now()]),
            $type === 'error' => PendingCommand::where('message_id', $payload['ref_id'] ?? '')->update(['status' => 'failed']),

            default => null, // unknown message type, ignore
        };
    }
}
```

- [ ] **Step 2: Implement DeviceConnectionManager**

```php
// app/Services/DeviceConnectionManager.php
<?php

namespace App\Services;

use App\Events\DeviceConnected;
use App\Events\DeviceDisconnected;
use App\Models\Device;

class DeviceConnectionManager
{
    /** @var array<int, mixed> device_id => WebSocket connection */
    private array $connections = [];

    public function register(Device $device, mixed $connection): void
    {
        $this->connections[$device->id] = $connection;
        $device->update(['connected' => true, 'last_seen_at' => now()]);
        DeviceConnected::dispatch($device);
    }

    public function unregister(Device $device): void
    {
        unset($this->connections[$device->id]);
        $device->update(['connected' => false]);
        DeviceDisconnected::dispatch($device);
    }

    public function getConnection(int $deviceId): mixed
    {
        return $this->connections[$deviceId] ?? null;
    }

    public function isConnected(int $deviceId): bool
    {
        return isset($this->connections[$deviceId]);
    }

    public function sendToDevice(int $deviceId, array $message): bool
    {
        $conn = $this->getConnection($deviceId);
        if (! $conn) return false;

        $conn->send(json_encode($message));
        return true;
    }
}
```

- [ ] **Step 3: Implement WebSocket handler**

The exact WebSocket implementation depends on the Octane driver (FrankenPHP vs Swoole). This provides the handler logic; wiring into Octane's WebSocket support is implementation-specific.

```php
// app/WebSocket/DeviceWebSocketHandler.php
<?php

namespace App\WebSocket;

use App\Models\Device;
use App\Services\DeviceConnectionManager;
use App\Services\DeviceProtocol\MessageRouter;
use App\Services\DeviceProtocol\MessageValidator;
use Illuminate\Support\Facades\Log;

class DeviceWebSocketHandler
{
    public function __construct(
        private DeviceConnectionManager $connections,
        private MessageRouter $router,
        private MessageValidator $validator,
    ) {}

    public function onOpen(mixed $connection, string $authHeader): void
    {
        // Extract bearer token
        $token = str_replace('Bearer ', '', $authHeader);
        $device = Device::findByToken($token);

        if (! $device) {
            $connection->close(4001, 'Unauthorized');
            return;
        }

        // Store device reference on connection for later use
        $connection->deviceId = $device->id;
        $this->connections->register($device, $connection);

        // Send welcome
        $connection->send(json_encode([
            'type' => 'welcome',
            'id' => 'msg_welcome_' . uniqid(),
            'device_id' => 'server',
            'timestamp' => now()->toISOString(),
            'payload' => [
                'session_id' => uniqid('sess_'),
                'server_version' => config('app.version', '1.0.0'),
                'capabilities' => ['state_sync', 'commands', 'streaming'],
            ],
        ]));

        // Drain pending commands after device sends state.sync
        // (handled in onMessage after processing state.sync)
    }

    public function onMessage(mixed $connection, string $data): void
    {
        $message = json_decode($data, true);

        if (! $message || ! isset($message['type'])) {
            Log::warning('Device WS: invalid message received');
            return;
        }

        $device = Device::find($connection->deviceId);
        if (! $device) return;

        // Validate against contract schema (non-blocking, just log)
        if (! $this->validator->validate($message)) {
            Log::warning('Device WS: schema validation failed', [
                'type' => $message['type'],
                'errors' => $this->validator->errors(),
            ]);
        }

        // Route to handler
        $this->router->route($device, $message);

        // After state.sync, drain pending commands
        if ($message['type'] === 'state.sync') {
            $this->drainPendingCommands($device);
        }
    }

    public function onClose(mixed $connection): void
    {
        $device = Device::find($connection->deviceId ?? 0);
        if ($device) {
            $this->connections->unregister($device);
        }
    }

    private function drainPendingCommands(Device $device): void
    {
        $pending = $device->pendingCommands()
            ->pending()
            ->orderBy('created_at')
            ->get();

        foreach ($pending as $command) {
            $this->connections->sendToDevice($device->id, [
                'type' => $command->type,
                'id' => $command->message_id,
                'device_id' => 'server',
                'timestamp' => now()->toISOString(),
                'payload' => $command->payload,
            ]);
        }
    }
}
```

- [ ] **Step 4: Commit**

```bash
git add app/Services/ app/WebSocket/ app/Events/
git commit -m "feat: add message router, connection manager, and WebSocket handler"
```

---

### Task 7: Command Service (Browser → Device)

**Files:**
- Create: `app/Http/Controllers/Api/DeviceCommandController.php`
- Create: `app/Services/CommandService.php`
- Create: `tests/Feature/CommandServiceTest.php`

- [ ] **Step 1: Write failing test**

```php
// tests/Feature/CommandServiceTest.php
<?php

use App\Models\User;
use App\Models\Device;
use App\Models\PendingCommand;
use App\Services\CommandService;

it('creates a pending command', function () {
    $user = User::factory()->create();
    $device = Device::create([
        'team_id' => $user->currentTeam()->id,
        'name' => 'test',
        'access_token' => hash('sha256', 'tok'),
        'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
    ]);

    $service = new CommandService(app(\App\Services\DeviceConnectionManager::class));
    $command = $service->send($device, $user, 'cmd.run.start', ['prd_id' => 'prd_001']);

    expect($command)->toBeInstanceOf(PendingCommand::class);
    expect($command->type)->toBe('cmd.run.start');
    expect($command->status)->toBe('pending');
});

it('sends command via REST API', function () {
    $user = User::factory()->create();
    $device = Device::create([
        'team_id' => $user->currentTeam()->id,
        'name' => 'test',
        'access_token' => hash('sha256', 'tok'),
        'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
    ]);

    $this->actingAs($user)->postJson("/api/devices/{$device->id}/commands", [
        'type' => 'cmd.run.start',
        'payload' => ['prd_id' => 'prd_001'],
    ])->assertCreated()->assertJsonStructure(['id', 'message_id', 'status']);
});
```

- [ ] **Step 2: Implement CommandService**

```php
// app/Services/CommandService.php
<?php

namespace App\Services;

use App\Models\Device;
use App\Models\PendingCommand;
use App\Models\User;
use Illuminate\Support\Str;

class CommandService
{
    public function __construct(private DeviceConnectionManager $connections) {}

    public function send(Device $device, User $user, string $type, array $payload): PendingCommand
    {
        $messageId = 'msg_' . Str::random(12);

        $command = PendingCommand::create([
            'device_id' => $device->id,
            'user_id' => $user->id,
            'message_id' => $messageId,
            'type' => $type,
            'payload' => $payload,
        ]);

        // Try to send immediately if device is connected
        if ($this->connections->isConnected($device->id)) {
            $sent = $this->connections->sendToDevice($device->id, [
                'type' => $type,
                'id' => $messageId,
                'device_id' => 'server',
                'timestamp' => now()->toISOString(),
                'payload' => $payload,
            ]);

            if (! $sent) {
                // Connection dropped, command stays pending for reconnect
            }
        }

        return $command;
    }
}
```

- [ ] **Step 3: Implement DeviceCommandController**

```php
// app/Http/Controllers/Api/DeviceCommandController.php
<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\Device;
use App\Services\CommandService;
use Illuminate\Http\Request;

class DeviceCommandController extends Controller
{
    public function __construct(private CommandService $commandService) {}

    public function store(Request $request, int $deviceId)
    {
        $request->validate([
            'type' => ['required', 'string', 'starts_with:cmd.'],
            'payload' => ['required', 'array'],
        ]);

        $device = Device::findOrFail($deviceId);

        // Verify user has access to this device's team
        $request->user()->teams()->where('team_id', $device->team_id)->firstOrFail();

        $command = $this->commandService->send(
            $device,
            $request->user(),
            $request->type,
            $request->payload,
        );

        return response()->json([
            'id' => $command->id,
            'message_id' => $command->message_id,
            'status' => $command->status,
        ], 201);
    }
}
```

- [ ] **Step 4: Add route**

```php
// routes/api.php — inside auth:sanctum middleware group:
use App\Http\Controllers\Api\DeviceCommandController;

Route::post('devices/{deviceId}/commands', [DeviceCommandController::class, 'store']);
```

- [ ] **Step 5: Run tests**

```bash
docker compose exec app php artisan test --filter="CommandServiceTest"
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add app/Services/CommandService.php app/Http/Controllers/Api/DeviceCommandController.php routes/api.php tests/
git commit -m "feat: add command service for browser → device command flow"
```

---

### Task 8: Reverb Channel Authorization

**Files:**
- Modify: `routes/channels.php`

- [ ] **Step 1: Define private channel authorization**

```php
// routes/channels.php
<?php

use App\Models\Device;
use Illuminate\Support\Facades\Broadcast;

Broadcast::channel('team.{teamId}', function ($user, $teamId) {
    return $user->teams()->where('team_id', $teamId)->exists();
});

Broadcast::channel('device.{deviceId}', function ($user, $deviceId) {
    $device = Device::find($deviceId);
    return $device && $user->teams()->where('team_id', $device->team_id)->exists();
});
```

- [ ] **Step 2: Commit**

```bash
git add routes/channels.php
git commit -m "feat: add Reverb channel authorization for team and device channels"
```
