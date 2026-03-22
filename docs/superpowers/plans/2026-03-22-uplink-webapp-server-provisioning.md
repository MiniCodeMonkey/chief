# Uplink Web App: Server Provisioning Implementation Plan (Plan 3d)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the "mini Forge" server provisioning and lifecycle management feature — provision Hetzner/DigitalOcean VPS servers with Chief pre-installed, manage server lifecycle, handle SSH keys and deploy keys.

**Architecture:** Cloud provider APIs are abstracted behind a `CloudProvider` interface. Provisioning runs as a queued job with progress updates via Reverb. Each provider has its own implementation. Provisioning scripts are Debian-specific bash scripts stored in the repo.

**Tech Stack:** Laravel 13, Hetzner Cloud API, DigitalOcean API, Laravel Queues, SSH key management

**Spec:** `docs/superpowers/specs/2026-03-22-uplink-webapp-design.md` (Section 6: Server Provisioning)

**Prerequisite:** Plan 3a (Auth) and Plan 3b (Device Protocol) must be completed first. Plan 3c is NOT required — this can be built in parallel.

---

## File Structure

```
app/
├── Models/
│   ├── CloudProviderCredential.php
│   ├── ManagedServer.php
│   └── SshKey.php
├── Services/
│   ├── CloudProvider/
│   │   ├── CloudProviderInterface.php  ← abstract interface
│   │   ├── HetznerProvider.php         ← Hetzner Cloud API
│   │   ├── DigitalOceanProvider.php    ← DigitalOcean API
│   │   └── CloudProviderFactory.php    ← resolves provider by name
│   ├── ServerProvisioner.php           ← orchestrates provisioning steps
│   └── GitHubKeyService.php            ← adds deploy keys via GitHub API
├── Http/Controllers/
│   ├── ServerController.php            ← server list, detail, management
│   ├── CloudProviderController.php     ← manage API keys
│   └── SshKeyController.php            ← manage SSH keys
├── Jobs/
│   └── ProvisionServerJob.php          ← async provisioning with progress
├── Events/
│   └── ServerProvisioningProgress.php  ← real-time progress to browser
database/migrations/
├── xxxx_create_cloud_provider_credentials_table.php
├── xxxx_create_managed_servers_table.php
├── xxxx_create_ssh_keys_table.php
resources/js/Pages/Servers/
├── Index.vue                           ← server list
├── Create.vue                          ← provisioning wizard
└── Show.vue                            ← server management
scripts/
└── provision-debian.sh                 ← server provisioning script
tests/
├── Feature/
│   ├── ServerProvisioningTest.php
│   └── CloudProviderTest.php
└── Unit/
    └── ServerProvisionerTest.php
```

---

### Task 1: Migrations & Models

**Files:**
- Create: 3 migration files
- Create: `CloudProviderCredential.php`, `ManagedServer.php`, `SshKey.php`

- [ ] **Step 1: Create migrations**

```php
// cloud_provider_credentials
Schema::create('cloud_provider_credentials', function (Blueprint $table) {
    $table->id();
    $table->foreignId('team_id')->constrained()->cascadeOnDelete();
    $table->enum('provider', ['hetzner', 'digitalocean']);
    $table->text('api_key')->comment('Encrypted');
    $table->string('name');
    $table->timestamps();
});

// managed_servers
Schema::create('managed_servers', function (Blueprint $table) {
    $table->id();
    $table->foreignId('team_id')->constrained()->cascadeOnDelete();
    $table->foreignId('cloud_provider_credential_id')->constrained()->cascadeOnDelete();
    $table->string('provider_server_id')->nullable();
    $table->string('name');
    $table->string('ip_address')->nullable();
    $table->string('region')->nullable();
    $table->string('size')->nullable();
    $table->enum('status', ['provisioning', 'active', 'stopped', 'error'])->default('provisioning');
    $table->text('deploy_public_key')->nullable();
    $table->text('deploy_private_key')->nullable()->comment('Encrypted');
    $table->text('provision_log')->nullable();
    $table->timestamp('provisioned_at')->nullable();
    $table->timestamps();
});

// ssh_keys
Schema::create('ssh_keys', function (Blueprint $table) {
    $table->id();
    $table->foreignId('team_id')->constrained()->cascadeOnDelete();
    $table->string('name');
    $table->text('public_key');
    $table->timestamps();
});

// Add FK to devices table now that managed_servers exists
Schema::table('devices', function (Blueprint $table) {
    $table->foreign('managed_server_id')->references('id')->on('managed_servers')->nullOnDelete();
});
```

- [ ] **Step 2: Create models**

```php
// app/Models/CloudProviderCredential.php
class CloudProviderCredential extends Model
{
    protected $fillable = ['team_id', 'provider', 'api_key', 'name'];
    protected function casts(): array { return ['api_key' => 'encrypted']; }
    public function team(): BelongsTo { return $this->belongsTo(Team::class); }
    public function servers(): HasMany { return $this->hasMany(ManagedServer::class); }
}

// app/Models/ManagedServer.php
class ManagedServer extends Model
{
    protected $fillable = [
        'team_id', 'cloud_provider_credential_id', 'provider_server_id',
        'name', 'ip_address', 'region', 'size', 'status',
        'deploy_public_key', 'deploy_private_key', 'provision_log', 'provisioned_at',
    ];
    protected function casts(): array {
        return ['deploy_private_key' => 'encrypted', 'provisioned_at' => 'datetime'];
    }
    public function team(): BelongsTo { return $this->belongsTo(Team::class); }
    public function credential(): BelongsTo { return $this->belongsTo(CloudProviderCredential::class, 'cloud_provider_credential_id'); }
    public function device(): HasOne { return $this->hasOne(Device::class); }
}

// app/Models/SshKey.php
class SshKey extends Model
{
    protected $fillable = ['team_id', 'name', 'public_key'];
    public function team(): BelongsTo { return $this->belongsTo(Team::class); }
}
```

- [ ] **Step 3: Commit**

```bash
git add database/migrations/ app/Models/
git commit -m "feat: add cloud provider, managed server, and SSH key models"
```

---

### Task 2: Cloud Provider Interface & Implementations

**Files:**
- Create: `app/Services/CloudProvider/CloudProviderInterface.php`
- Create: `app/Services/CloudProvider/HetznerProvider.php`
- Create: `app/Services/CloudProvider/DigitalOceanProvider.php`
- Create: `app/Services/CloudProvider/CloudProviderFactory.php`

- [ ] **Step 1: Write failing test**

```php
// tests/Unit/ServerProvisionerTest.php
<?php

use App\Services\CloudProvider\CloudProviderInterface;
use App\Services\CloudProvider\CloudProviderFactory;

it('resolves hetzner provider', function () {
    $provider = CloudProviderFactory::make('hetzner', 'test-api-key');
    expect($provider)->toBeInstanceOf(CloudProviderInterface::class);
});

it('resolves digitalocean provider', function () {
    $provider = CloudProviderFactory::make('digitalocean', 'test-api-key');
    expect($provider)->toBeInstanceOf(CloudProviderInterface::class);
});
```

- [ ] **Step 2: Implement interface**

```php
// app/Services/CloudProvider/CloudProviderInterface.php
<?php

namespace App\Services\CloudProvider;

interface CloudProviderInterface
{
    /** List available server sizes with pricing */
    public function listSizes(): array;

    /** List available regions */
    public function listRegions(): array;

    /** Create a server, returns provider-specific server ID */
    public function createServer(string $name, string $region, string $size, string $sshPublicKey): array;

    /** Get server status and IP */
    public function getServer(string $serverId): array;

    /** Reboot a server */
    public function rebootServer(string $serverId): void;

    /** Resize a server */
    public function resizeServer(string $serverId, string $newSize): void;

    /** Rebuild a server (reinstall OS) */
    public function rebuildServer(string $serverId): void;

    /** Destroy a server */
    public function destroyServer(string $serverId): void;

    /** Get server metrics (CPU, RAM, disk) */
    public function getMetrics(string $serverId): array;
}
```

- [ ] **Step 3: Implement HetznerProvider**

```php
// app/Services/CloudProvider/HetznerProvider.php
<?php

namespace App\Services\CloudProvider;

use Illuminate\Support\Facades\Http;

class HetznerProvider implements CloudProviderInterface
{
    private string $baseUrl = 'https://api.hetzner.cloud/v1';

    public function __construct(private string $apiKey) {}

    public function listSizes(): array
    {
        $response = $this->request('GET', '/server_types');
        return collect($response['server_types'])->map(fn ($t) => [
            'id' => $t['name'],
            'name' => $t['name'],
            'vcpus' => $t['cores'],
            'memory' => $t['memory'],
            'disk' => $t['disk'],
            'price_monthly' => $t['prices'][0]['price_monthly']['gross'] ?? null,
        ])->all();
    }

    public function listRegions(): array
    {
        $response = $this->request('GET', '/locations');
        return collect($response['locations'])->map(fn ($l) => [
            'id' => $l['name'],
            'name' => $l['description'],
            'city' => $l['city'],
            'country' => $l['country'],
        ])->all();
    }

    public function createServer(string $name, string $region, string $size, string $sshPublicKey): array
    {
        // First, create SSH key
        $keyResponse = $this->request('POST', '/ssh_keys', [
            'name' => "chief-{$name}",
            'public_key' => $sshPublicKey,
        ]);

        $response = $this->request('POST', '/servers', [
            'name' => $name,
            'server_type' => $size,
            'location' => $region,
            'image' => 'debian-12',
            'ssh_keys' => [$keyResponse['ssh_key']['id']],
        ]);

        return [
            'server_id' => (string) $response['server']['id'],
            'ip_address' => $response['server']['public_net']['ipv4']['ip'] ?? null,
            'status' => $response['server']['status'],
        ];
    }

    public function getServer(string $serverId): array
    {
        $response = $this->request('GET', "/servers/{$serverId}");
        return [
            'server_id' => (string) $response['server']['id'],
            'ip_address' => $response['server']['public_net']['ipv4']['ip'] ?? null,
            'status' => $response['server']['status'],
        ];
    }

    public function rebootServer(string $serverId): void
    {
        $this->request('POST', "/servers/{$serverId}/actions/reboot");
    }

    public function resizeServer(string $serverId, string $newSize): void
    {
        $this->request('POST', "/servers/{$serverId}/actions/change_type", [
            'server_type' => $newSize,
            'upgrade_disk' => true,
        ]);
    }

    public function rebuildServer(string $serverId): void
    {
        $this->request('POST', "/servers/{$serverId}/actions/rebuild", [
            'image' => 'debian-12',
        ]);
    }

    public function destroyServer(string $serverId): void
    {
        $this->request('DELETE', "/servers/{$serverId}");
    }

    public function getMetrics(string $serverId): array
    {
        $response = $this->request('GET', "/servers/{$serverId}/metrics", [
            'type' => 'cpu,disk,network',
            'start' => now()->subHour()->toISOString(),
            'end' => now()->toISOString(),
        ]);
        return $response['metrics'] ?? [];
    }

    private function request(string $method, string $path, array $data = []): array
    {
        $request = Http::withToken($this->apiKey)
            ->acceptJson();

        $response = match ($method) {
            'GET' => $request->get($this->baseUrl . $path, $data),
            'POST' => $request->post($this->baseUrl . $path, $data),
            'DELETE' => $request->delete($this->baseUrl . $path),
            default => throw new \InvalidArgumentException("Unknown method: {$method}"),
        };

        $response->throw();
        return $response->json();
    }
}
```

- [ ] **Step 4: Implement DigitalOceanProvider** (same interface, DO API)

Follow the same pattern as Hetzner but using the DigitalOcean API v2 (`https://api.digitalocean.com/v2`). Map droplets to the same return format.

- [ ] **Step 5: Implement Factory**

```php
// app/Services/CloudProvider/CloudProviderFactory.php
<?php

namespace App\Services\CloudProvider;

class CloudProviderFactory
{
    public static function make(string $provider, string $apiKey): CloudProviderInterface
    {
        return match ($provider) {
            'hetzner' => new HetznerProvider($apiKey),
            'digitalocean' => new DigitalOceanProvider($apiKey),
            default => throw new \InvalidArgumentException("Unknown provider: {$provider}"),
        };
    }
}
```

- [ ] **Step 6: Run tests and commit**

```bash
docker compose exec app php artisan test --filter="ServerProvisionerTest"
git add app/Services/CloudProvider/
git commit -m "feat: add cloud provider interface with Hetzner and DigitalOcean implementations"
```

---

### Task 3: Provisioning Script

**Files:**
- Create: `scripts/provision-debian.sh`

- [ ] **Step 1: Create provisioning script**

```bash
#!/usr/bin/env bash
# scripts/provision-debian.sh
# Provisions a Debian server with Chief and Claude Code
# Arguments: $1 = access_token, $2 = uplink_url

set -euo pipefail

CHIEF_ACCESS_TOKEN="${1}"
UPLINK_URL="${2}"

echo "=== Updating system ==="
apt-get update && apt-get upgrade -y

echo "=== Installing essentials ==="
apt-get install -y git curl build-essential ufw fail2ban unzip jq

echo "=== Creating chief user ==="
useradd -m -s /bin/bash -G sudo chief
echo "chief ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/chief

echo "=== Setting up swap ==="
if [ ! -f /swapfile ]; then
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

echo "=== Configuring firewall ==="
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw --force enable

echo "=== Starting fail2ban ==="
systemctl enable fail2ban
systemctl start fail2ban

echo "=== Installing Claude Code ==="
su - chief -c 'curl -fsSL https://claude.ai/install.sh | sh'

echo "=== Installing Chief ==="
CHIEF_VERSION=$(curl -s https://api.github.com/repos/minicodemonkey/chief/releases/latest | jq -r .tag_name)
curl -fsSL "https://github.com/minicodemonkey/chief/releases/download/${CHIEF_VERSION}/chief_linux_amd64.tar.gz" | tar xz -C /usr/local/bin/

echo "=== Creating workspace ==="
su - chief -c 'mkdir -p ~/workspace'

echo "=== Generating deploy key ==="
su - chief -c 'ssh-keygen -t ed25519 -f ~/.ssh/chief_deploy_key -N "" -C "chief-deploy-key"'

echo "=== Configuring Chief credentials ==="
su - chief -c "mkdir -p ~/.chief && cat > ~/.chief/credentials.yaml << CRED
access_token: ${CHIEF_ACCESS_TOKEN}
refresh_token: \"\"
uplink_url: ${UPLINK_URL}
device_name: \$(hostname)
CRED"

echo "=== Configuring Chief uplink ==="
su - chief -c "cat > ~/.chief/config.yaml << CFG
uplink:
  enabled: true
  url: ${UPLINK_URL}
CFG"

echo "=== Setting up Chief systemd service ==="
cat > /etc/systemd/system/chief.service << SERVICE
[Unit]
Description=Chief Serve Daemon
After=network.target

[Service]
Type=simple
User=chief
WorkingDirectory=/home/chief/workspace
ExecStart=/usr/local/bin/chief serve
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable chief
systemctl start chief

echo "=== Provisioning complete ==="
```

- [ ] **Step 2: Commit**

```bash
git add scripts/provision-debian.sh
git commit -m "feat: add Debian provisioning script for Chief servers"
```

---

### Task 4: Provisioning Job & Service

**Files:**
- Create: `app/Services/ServerProvisioner.php`
- Create: `app/Jobs/ProvisionServerJob.php`
- Create: `app/Events/ServerProvisioningProgress.php`

- [ ] **Step 1: Create provisioning progress event**

```php
// app/Events/ServerProvisioningProgress.php
class ServerProvisioningProgress implements ShouldBroadcast
{
    public function __construct(
        public ManagedServer $server,
        public string $step,
        public string $message,
    ) {}

    public function broadcastOn(): array
    {
        return [new PrivateChannel('team.' . $this->server->team_id)];
    }
}
```

- [ ] **Step 2: Implement provisioning job**

```php
// app/Jobs/ProvisionServerJob.php
<?php

namespace App\Jobs;

use App\Events\ServerProvisioningProgress;
use App\Models\ManagedServer;
use App\Services\CloudProvider\CloudProviderFactory;
use App\Services\ServerProvisioner;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;

class ProvisionServerJob implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable;

    public int $timeout = 600; // 10 minutes

    public function __construct(
        private int $serverId,
        private string $sshPublicKey,
    ) {}

    public function handle(): void
    {
        $server = ManagedServer::findOrFail($this->serverId);
        $credential = $server->credential;
        $provider = CloudProviderFactory::make($credential->provider, $credential->api_key);

        $provisioner = new ServerProvisioner($provider, $server);

        try {
            $provisioner->provision($this->sshPublicKey);
        } catch (\Throwable $e) {
            $server->update(['status' => 'error', 'provision_log' => $e->getMessage()]);
            ServerProvisioningProgress::dispatch($server, 'error', $e->getMessage());
        }
    }
}
```

- [ ] **Step 3: Implement ServerProvisioner**

```php
// app/Services/ServerProvisioner.php
<?php

namespace App\Services;

use App\Events\ServerProvisioningProgress;
use App\Http\Controllers\Api\DeviceAuthController;
use App\Models\ManagedServer;
use App\Services\CloudProvider\CloudProviderInterface;
use Illuminate\Support\Facades\Http;

class ServerProvisioner
{
    public function __construct(
        private CloudProviderInterface $provider,
        private ManagedServer $server,
    ) {}

    public function provision(string $sshPublicKey): void
    {
        $this->progress('creating', 'Creating server...');

        // Create server with provider
        $result = $this->provider->createServer(
            $this->server->name,
            $this->server->region,
            $this->server->size,
            $sshPublicKey,
        );

        $this->server->update([
            'provider_server_id' => $result['server_id'],
            'ip_address' => $result['ip_address'],
        ]);

        $this->progress('booting', 'Waiting for server to boot...');

        // Poll until server is running
        $this->waitForBoot($result['server_id']);

        // Get IP if not available at creation
        $info = $this->provider->getServer($result['server_id']);
        $this->server->update(['ip_address' => $info['ip_address']]);

        $this->progress('provisioning', 'Running provisioning script...');

        // Generate device token for automated auth
        $deviceToken = $this->createDeviceToken();

        // SSH in and run provisioning script
        $this->runProvisioningScript($info['ip_address'], $deviceToken);

        $this->progress('connecting', 'Connecting to Uplink...');

        // Wait for device to connect
        $this->waitForDeviceConnection();

        $this->server->update([
            'status' => 'active',
            'provisioned_at' => now(),
        ]);

        $this->progress('done', 'Server is ready!');
    }

    private function waitForBoot(string $serverId): void
    {
        $attempts = 0;
        while ($attempts < 60) {
            $info = $this->provider->getServer($serverId);
            if ($info['status'] === 'running' && $info['ip_address']) return;
            sleep(5);
            $attempts++;
        }
        throw new \RuntimeException('Server did not boot within 5 minutes');
    }

    private function createDeviceToken(): string
    {
        // Use the provision endpoint internally
        $accessToken = \Illuminate\Support\Str::random(64);
        \App\Models\Device::create([
            'team_id' => $this->server->team_id,
            'managed_server_id' => $this->server->id,
            'name' => $this->server->name,
            'access_token' => hash('sha256', $accessToken),
            'refresh_token' => \Illuminate\Support\Str::random(64),
            'token_expires_at' => now()->addDays(90),
        ]);
        return $accessToken;
    }

    private function runProvisioningScript(string $ip, string $deviceToken): void
    {
        // Uses phpseclib for SSH (composer require phpseclib/phpseclib)
        $ssh = new \phpseclib3\Net\SSH2($ip);

        // Hetzner/DO provision with root + SSH key — key is already on server from createServer()
        // For key-based auth, use the team's SSH private key or a temporary provisioning key
        if (! $ssh->login('root')) {
            throw new \RuntimeException("SSH authentication failed for {$ip}");
        }

        $script = file_get_contents(base_path('scripts/provision-debian.sh'));
        $uplinkUrl = config('app.url');

        // Run provisioning script with arguments
        $output = $ssh->exec("bash -s -- " . escapeshellarg($deviceToken) . " " . escapeshellarg($uplinkUrl), $script);
        $exitCode = $ssh->getExitStatus();

        if ($exitCode !== 0) {
            throw new \RuntimeException("Provisioning script failed (exit {$exitCode}): " . $output);
        }

        // Read deploy key from server
        $pubKey = trim($ssh->exec('cat /home/chief/.ssh/chief_deploy_key.pub'));
        $privKey = trim($ssh->exec('cat /home/chief/.ssh/chief_deploy_key'));

        $this->server->update([
            'deploy_public_key' => $pubKey,
            'deploy_private_key' => $privKey,
        ]);

        $ssh->disconnect();
    }

    private function waitForDeviceConnection(): void
    {
        $attempts = 0;
        while ($attempts < 30) {
            $device = $this->server->device;
            if ($device && $device->connected) return;
            sleep(5);
            $attempts++;
        }
        // Non-fatal — device might connect later
    }

    private function progress(string $step, string $message): void
    {
        ServerProvisioningProgress::dispatch($this->server, $step, $message);
    }
}
```

Note: SSH uses `phpseclib/phpseclib`. Install with `composer require phpseclib/phpseclib`. The SSH key-based auth for the provisioning connection needs to use the team's provisioning key or a temporary key generated during server creation.

- [ ] **Step 4: Commit**

```bash
git add app/Services/ServerProvisioner.php app/Jobs/ app/Events/
git commit -m "feat: add server provisioning job with progress broadcasting"
```

---

### Task 5: Server Management Controllers & Pages

**Files:**
- Create: `app/Http/Controllers/ServerController.php`
- Create: `resources/js/Pages/Servers/Index.vue`
- Create: `resources/js/Pages/Servers/Create.vue`
- Create: `resources/js/Pages/Servers/Show.vue`

- [ ] **Step 1: Write failing test**

```php
// tests/Feature/ServerProvisioningTest.php
<?php

use App\Models\User;
use App\Models\CloudProviderCredential;
use App\Models\ManagedServer;

it('shows server list', function () {
    $user = User::factory()->create();
    $this->actingAs($user)->get('/servers')->assertOk()
        ->assertInertia(fn ($page) => $page->component('Servers/Index'));
});

it('shows provisioning page with regions and sizes', function () {
    $user = User::factory()->create();
    CloudProviderCredential::create([
        'team_id' => $user->currentTeam()->id,
        'provider' => 'hetzner',
        'api_key' => 'test-key',
        'name' => 'My Hetzner',
    ]);

    $this->actingAs($user)->get('/servers/create')->assertOk()
        ->assertInertia(fn ($page) => $page->component('Servers/Create')
            ->has('credentials')
        );
});

it('creates a managed server', function () {
    $user = User::factory()->create();
    $cred = CloudProviderCredential::create([
        'team_id' => $user->currentTeam()->id,
        'provider' => 'hetzner',
        'api_key' => 'test-key',
        'name' => 'My Hetzner',
    ]);

    // Mock the queue
    Queue::fake();

    $this->actingAs($user)->post('/servers', [
        'credential_id' => $cred->id,
        'name' => 'chief-worker-01',
        'region' => 'nbg1',
        'size' => 'cx22',
        'ssh_key_id' => null,
        'ssh_public_key' => 'ssh-ed25519 AAAA... user@laptop',
    ])->assertRedirect();

    expect(ManagedServer::where('name', 'chief-worker-01')->exists())->toBeTrue();
    Queue::assertPushed(\App\Jobs\ProvisionServerJob::class);
});
```

- [ ] **Step 2: Implement ServerController**

```php
// app/Http/Controllers/ServerController.php
<?php

namespace App\Http\Controllers;

use App\Jobs\ProvisionServerJob;
use App\Models\CloudProviderCredential;
use App\Models\ManagedServer;
use App\Models\SshKey;
use App\Services\CloudProvider\CloudProviderFactory;
use Illuminate\Http\Request;
use Inertia\Inertia;

class ServerController extends Controller
{
    public function index(Request $request)
    {
        $teamIds = $request->user()->teams->pluck('id');

        return Inertia::render('Servers/Index', [
            'servers' => ManagedServer::whereIn('team_id', $teamIds)
                ->with('device')
                ->orderByDesc('created_at')
                ->get(),
        ]);
    }

    public function create(Request $request)
    {
        $teamIds = $request->user()->teams->pluck('id');

        return Inertia::render('Servers/Create', [
            'credentials' => CloudProviderCredential::whereIn('team_id', $teamIds)->get(),
            'sshKeys' => SshKey::whereIn('team_id', $teamIds)->get(),
        ]);
    }

    public function store(Request $request)
    {
        $validated = $request->validate([
            'credential_id' => ['required', 'exists:cloud_provider_credentials,id'],
            'name' => ['required', 'string', 'max:255'],
            'region' => ['required', 'string'],
            'size' => ['required', 'string'],
            'ssh_key_id' => ['nullable', 'exists:ssh_keys,id'],
            'ssh_public_key' => ['required_without:ssh_key_id', 'nullable', 'string'],
        ]);

        $credential = CloudProviderCredential::findOrFail($validated['credential_id']);
        abort_unless($request->user()->isOwnerOf($credential->team), 403);

        // Resolve SSH key
        $sshPublicKey = $validated['ssh_public_key'];
        if ($validated['ssh_key_id']) {
            $sshPublicKey = SshKey::findOrFail($validated['ssh_key_id'])->public_key;
        }

        $server = ManagedServer::create([
            'team_id' => $credential->team_id,
            'cloud_provider_credential_id' => $credential->id,
            'name' => $validated['name'],
            'region' => $validated['region'],
            'size' => $validated['size'],
            'status' => 'provisioning',
        ]);

        ProvisionServerJob::dispatch($server->id, $sshPublicKey);

        return redirect("/servers/{$server->id}");
    }

    public function show(Request $request, ManagedServer $server)
    {
        abort_unless($request->user()->isMemberOf($server->team), 403);

        return Inertia::render('Servers/Show', [
            'server' => $server->load('device', 'credential'),
        ]);
    }

    public function destroy(Request $request, ManagedServer $server)
    {
        abort_unless($request->user()->isOwnerOf($server->team), 403);

        $credential = $server->credential;
        $provider = CloudProviderFactory::make($credential->provider, $credential->api_key);

        if ($server->provider_server_id) {
            $provider->destroyServer($server->provider_server_id);
        }

        $server->device?->delete();
        $server->delete();

        return redirect('/servers');
    }
}
```

- [ ] **Step 3: Add routes**

```php
// routes/web.php — inside auth middleware:
use App\Http\Controllers\ServerController;

Route::resource('servers', ServerController::class)->only(['index', 'create', 'store', 'show', 'destroy']);
```

- [ ] **Step 4: Create Vue pages**

Create `Servers/Index.vue` (server cards with status), `Servers/Create.vue` (provisioning wizard), and `Servers/Show.vue` (management page with actions). The Create page fetches regions/sizes from a new API endpoint that calls the provider.

- [ ] **Step 5: Run tests and commit**

```bash
docker compose exec app php artisan test --filter="ServerProvisioningTest"
git add app/Http/Controllers/ServerController.php app/Http/Controllers/CloudProviderController.php resources/js/Pages/Servers/ routes/web.php tests/
git commit -m "feat: add server provisioning wizard and management pages"
```

---

### Task 6: GitHub Deploy Key Integration

**Files:**
- Create: `app/Services/GitHubKeyService.php`

- [ ] **Step 1: Implement GitHubKeyService**

```php
// app/Services/GitHubKeyService.php
<?php

namespace App\Services;

use App\Models\User;
use Illuminate\Support\Facades\Http;

class GitHubKeyService
{
    /**
     * Add a public SSH key to the user's GitHub account.
     * Requires the admin:public_key scope.
     */
    public function addKey(User $user, string $title, string $publicKey): bool
    {
        if (! $user->github_token) {
            return false;
        }

        $response = Http::withToken($user->github_token)
            ->post('https://api.github.com/user/keys', [
                'title' => $title,
                'key' => $publicKey,
            ]);

        return $response->successful();
    }
}
```

- [ ] **Step 2: Add "Add to GitHub" endpoint**

```php
// Add to ServerController or create a dedicated endpoint:
public function addDeployKeyToGitHub(Request $request, ManagedServer $server)
{
    abort_unless($request->user()->isMemberOf($server->team), 403);

    if (! $server->deploy_public_key) {
        return back()->withErrors(['deploy_key' => 'No deploy key available']);
    }

    $service = app(GitHubKeyService::class);
    $success = $service->addKey(
        $request->user(),
        "Chief Deploy Key ({$server->name})",
        $server->deploy_public_key,
    );

    if (! $success) {
        return back()->withErrors(['deploy_key' => 'Failed to add key to GitHub. You may need to re-authenticate with the admin:public_key scope.']);
    }

    return back()->with('success', 'Deploy key added to GitHub');
}
```

- [ ] **Step 3: Commit**

```bash
git add app/Services/GitHubKeyService.php
git commit -m "feat: add GitHub deploy key integration for managed servers"
```

---

### Task 7: Cloud Provider API Key Management

**Files:**
- Create: `app/Http/Controllers/CloudProviderController.php`
- Create: `app/Http/Controllers/SshKeyController.php`

These are CRUD controllers for managing API keys and SSH keys in the settings area.

- [ ] **Step 1: Implement controllers**

Standard CRUD for cloud_provider_credentials and ssh_keys, scoped to team, owner-only for write operations. Add Inertia pages in Settings or inline in the Servers/Create flow.

- [ ] **Step 2: Add routes and commit**

```bash
git add app/Http/Controllers/CloudProviderController.php app/Http/Controllers/SshKeyController.php routes/web.php
git commit -m "feat: add cloud provider credential and SSH key management"
```
