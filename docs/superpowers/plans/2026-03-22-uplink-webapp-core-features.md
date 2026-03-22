# Uplink Web App: Core Features Implementation Plan (Plan 3c)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the core UI pages — device list, project detail, PRD chat, run monitoring (TUI-style), diff viewer, and file browser.

**Architecture:** Inertia/Vue pages that read cached state from the database and subscribe to Reverb channels for real-time updates. Commands to devices go through the REST API (`POST /api/devices/{id}/commands`). Ephemeral streaming (Claude output, chat responses) arrives via Reverb's `DeviceStreamEvent`.

**Tech Stack:** Vue 3, Inertia.js, Tailwind 4, Shiki (syntax highlighting), Laravel Echo (Reverb client)

**Spec:** `docs/superpowers/specs/2026-03-22-uplink-webapp-design.md`

**Prerequisite:** Plan 3a (Auth) and Plan 3b (Device Protocol) must be completed first.

---

## File Structure

```
app/Http/Controllers/
├── DeviceController.php            ← device list + detail
├── ProjectController.php           ← project detail
├── PrdController.php               ← PRD detail, chat, direct edit
├── RunController.php               ← run detail, live view, diffs
└── FileController.php              ← file browser

resources/js/
├── composables/
│   ├── useDeviceChannel.js         ← Reverb subscription for device events
│   └── useStreamingText.js         ← accumulates streamed text chunks
├── Components/
│   ├── DeviceCard.vue              ← device list item with online/offline dot
│   ├── ProjectCard.vue             ← project summary card
│   ├── PrdCard.vue                 ← PRD summary card
│   ├── StoryList.vue               ← story list with status dots (TUI-style)
│   ├── StoryDetail.vue             ← expanded story view
│   ├── ChatMessage.vue             ← single chat message (user/assistant)
│   ├── ChatInput.vue               ← auto-growing text input with send button
│   ├── LiveOutput.vue              ← terminal-style streaming output
│   ├── DiffViewer.vue              ← GitHub-style diff with file tree
│   ├── DiffFile.vue                ← single file diff (unified/side-by-side)
│   ├── FileTree.vue                ← directory tree browser
│   ├── FileViewer.vue              ← syntax-highlighted file content
│   ├── MarkdownEditor.vue          ← CodeMirror markdown editor for direct PRD editing
│   └── MarkdownPreview.vue         ← rendered markdown preview
├── Pages/
│   ├── Devices/
│   │   ├── Index.vue               ← device list
│   │   └── Show.vue                ← device detail (projects, PRDs, runs)
│   ├── Projects/
│   │   └── Show.vue                ← project detail
│   ├── Prds/
│   │   ├── Show.vue                ← PRD detail (preview + edit)
│   │   └── Chat.vue                ← PRD chat interface
│   ├── Runs/
│   │   ├── Show.vue                ← run summary (TUI-style)
│   │   ├── Live.vue                ← live Claude output stream
│   │   └── Diffs.vue               ← diff viewer
│   └── Files/
│       └── Show.vue                ← file browser + viewer

tests/
├── Feature/
│   ├── DevicePageTest.php
│   ├── PrdChatTest.php
│   ├── RunPageTest.php
│   └── FilePageTest.php
└── js/
    ├── StoryList.test.js
    ├── ChatMessage.test.js
    └── DiffViewer.test.js
```

---

### Task 1: Laravel Echo & Reverb Client Setup

**Files:**
- Modify: `resources/js/app.js`
- Create: `resources/js/composables/useDeviceChannel.js`

- [ ] **Step 1: Install Echo**

```bash
npm install laravel-echo pusher-js
```

- [ ] **Step 2: Configure Echo in app.js**

```js
// Add to resources/js/app.js
import Echo from 'laravel-echo';
import Pusher from 'pusher-js';

window.Pusher = Pusher;
window.Echo = new Echo({
    broadcaster: 'reverb',
    key: import.meta.env.VITE_REVERB_APP_KEY,
    wsHost: import.meta.env.VITE_REVERB_HOST,
    wsPort: import.meta.env.VITE_REVERB_PORT ?? 8080,
    wssPort: import.meta.env.VITE_REVERB_PORT ?? 443,
    forceTLS: (import.meta.env.VITE_REVERB_SCHEME ?? 'https') === 'https',
    enabledTransports: ['ws', 'wss'],
});
```

- [ ] **Step 3: Create useDeviceChannel composable**

```js
// resources/js/composables/useDeviceChannel.js
import { onMounted, onUnmounted } from 'vue';

export function useDeviceChannel(deviceId, handlers = {}) {
    let channel = null;

    onMounted(() => {
        channel = window.Echo.private(`device.${deviceId}`);

        if (handlers.onStateUpdated) {
            channel.listen('DeviceStateUpdated', handlers.onStateUpdated);
        }
        if (handlers.onStream) {
            channel.listen('DeviceStreamEvent', handlers.onStream);
        }
    });

    onUnmounted(() => {
        if (channel) {
            channel.stopListening('DeviceStateUpdated');
            channel.stopListening('DeviceStreamEvent');
            window.Echo.leave(`device.${deviceId}`);
        }
    });

    return { channel };
}
```

- [ ] **Step 4: Create useStreamingText composable**

```js
// resources/js/composables/useStreamingText.js
import { ref } from 'vue';

export function useStreamingText() {
    const text = ref('');
    const isStreaming = ref(false);

    function append(chunk) {
        isStreaming.value = true;
        text.value += chunk;
    }

    function reset() {
        text.value = '';
        isStreaming.value = false;
    }

    return { text, isStreaming, append, reset };
}
```

- [ ] **Step 5: Commit**

```bash
git add resources/js/
git commit -m "feat: add Laravel Echo setup and real-time composables"
```

---

### Task 2: Device List & Detail Pages

**Files:**
- Create: `app/Http/Controllers/DeviceController.php`
- Create: `resources/js/Pages/Devices/Index.vue`
- Create: `resources/js/Pages/Devices/Show.vue`
- Create: `resources/js/Components/DeviceCard.vue`

- [ ] **Step 1: Write failing test**

```php
// tests/Feature/DevicePageTest.php
<?php

use App\Models\User;
use App\Models\Device;

it('shows device list page', function () {
    $user = User::factory()->create();
    Device::create([
        'team_id' => $user->currentTeam()->id,
        'name' => 'my-vps',
        'access_token' => hash('sha256', 'tok'),
        'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
        'connected' => true,
    ]);

    $this->actingAs($user)->get('/devices')
        ->assertOk()
        ->assertInertia(fn ($page) =>
            $page->component('Devices/Index')
                ->has('devices', 1)
                ->where('devices.0.name', 'my-vps')
                ->where('devices.0.connected', true)
        );
});

it('shows device detail with projects and prds', function () {
    $user = User::factory()->create();
    $device = Device::create([
        'team_id' => $user->currentTeam()->id,
        'name' => 'my-vps',
        'access_token' => hash('sha256', 'tok'),
        'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
    ]);
    $project = $device->projects()->create([
        'external_id' => 'proj_app', 'path' => '/workspace/app', 'name' => 'app',
    ]);

    $this->actingAs($user)->get("/devices/{$device->id}")
        ->assertOk()
        ->assertInertia(fn ($page) =>
            $page->component('Devices/Show')
                ->has('device')
                ->has('projects', 1)
        );
});

it('prevents access to devices from other teams', function () {
    $user = User::factory()->create();
    $otherUser = User::factory()->create();
    $device = Device::create([
        'team_id' => $otherUser->currentTeam()->id,
        'name' => 'other-device',
        'access_token' => hash('sha256', 'tok2'),
        'refresh_token' => 'ref2',
        'token_expires_at' => now()->addHour(),
    ]);

    $this->actingAs($user)->get("/devices/{$device->id}")->assertForbidden();
});
```

- [ ] **Step 2: Implement DeviceController**

```php
// app/Http/Controllers/DeviceController.php
<?php

namespace App\Http\Controllers;

use App\Models\Device;
use Illuminate\Http\Request;
use Inertia\Inertia;

class DeviceController extends Controller
{
    public function index(Request $request)
    {
        $teamIds = $request->user()->teams->pluck('id');
        $devices = Device::whereIn('team_id', $teamIds)
            ->orderByDesc('connected')
            ->orderByDesc('last_seen_at')
            ->get();

        return Inertia::render('Devices/Index', [
            'devices' => $devices,
        ]);
    }

    public function show(Request $request, Device $device)
    {
        abort_unless($request->user()->isMemberOf($device->team), 403);

        return Inertia::render('Devices/Show', [
            'device' => $device,
            'projects' => $device->projects()->with('prds')->get(),
            'activeRuns' => $device->runs()->whereIn('status', ['running'])->with('prd')->get(),
        ]);
    }
}
```

- [ ] **Step 3: Create Vue pages**

Create `Devices/Index.vue` with device cards showing name, online/offline dot, OS/arch, chief version, last seen. Create `Devices/Show.vue` with project list, active runs, and action buttons. Follow the design tokens from Plan 3a (bg-bg-card, text-text-heading, border-border, etc.).

- [ ] **Step 4: Add routes**

```php
// routes/web.php — inside auth middleware:
use App\Http\Controllers\DeviceController;

Route::resource('devices', DeviceController::class)->only(['index', 'show']);
```

- [ ] **Step 5: Run tests**

```bash
docker compose exec app php artisan test --filter="DevicePageTest"
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add app/Http/Controllers/DeviceController.php resources/js/Pages/Devices/ resources/js/Components/ routes/web.php tests/
git commit -m "feat: add device list and detail pages"
```

---

### Task 3: PRD Chat Interface

**Files:**
- Create: `app/Http/Controllers/PrdController.php`
- Create: `resources/js/Pages/Prds/Chat.vue`
- Create: `resources/js/Components/ChatMessage.vue`
- Create: `resources/js/Components/ChatInput.vue`

This is the core creative feature — full-screen chat with Claude for PRD creation/refinement.

- [ ] **Step 1: Write failing test**

```php
// tests/Feature/PrdChatTest.php
<?php

use App\Models\User;
use App\Models\Device;
use App\Models\Project;
use App\Models\Prd;

it('shows PRD chat page with history', function () {
    $user = User::factory()->create();
    $device = Device::create([
        'team_id' => $user->currentTeam()->id, 'name' => 'test',
        'access_token' => hash('sha256', 'tok'), 'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
    ]);
    $project = $device->projects()->create([
        'external_id' => 'proj_app', 'path' => '/workspace/app', 'name' => 'app',
    ]);
    $prd = Prd::create([
        'project_id' => $project->id, 'device_id' => $device->id,
        'external_id' => 'prd_auth', 'title' => 'Auth',
        'status' => 'draft',
        'chat_history' => [
            ['role' => 'user', 'content' => 'Add OAuth', 'timestamp' => '2026-03-21T08:00:00Z'],
            ['role' => 'assistant', 'content' => 'What providers?', 'timestamp' => '2026-03-21T08:00:05Z'],
        ],
    ]);

    $this->actingAs($user)->get("/prds/{$prd->id}/chat")
        ->assertOk()
        ->assertInertia(fn ($page) =>
            $page->component('Prds/Chat')
                ->has('prd')
                ->has('chatHistory', 2)
                ->where('prd.title', 'Auth')
        );
});

it('sends a chat message via command API', function () {
    $user = User::factory()->create();
    $device = Device::create([
        'team_id' => $user->currentTeam()->id, 'name' => 'test',
        'access_token' => hash('sha256', 'tok'), 'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
    ]);
    $project = $device->projects()->create([
        'external_id' => 'proj_app', 'path' => '/workspace/app', 'name' => 'app',
    ]);
    $prd = Prd::create([
        'project_id' => $project->id, 'device_id' => $device->id,
        'external_id' => 'prd_auth', 'title' => 'Auth', 'status' => 'draft',
    ]);

    $this->actingAs($user)->postJson("/api/devices/{$device->id}/commands", [
        'type' => 'cmd.prd.message',
        'payload' => ['prd_id' => 'prd_auth', 'message' => 'Add GitHub OAuth'],
    ])->assertCreated();
});
```

- [ ] **Step 2: Implement PrdController**

```php
// app/Http/Controllers/PrdController.php
<?php

namespace App\Http\Controllers;

use App\Models\Prd;
use Illuminate\Http\Request;
use Inertia\Inertia;

class PrdController extends Controller
{
    public function show(Request $request, Prd $prd)
    {
        abort_unless($request->user()->isMemberOf($prd->device->team), 403);

        return Inertia::render('Prds/Show', [
            'prd' => $prd->load('project', 'runs'),
            'device' => $prd->device,
        ]);
    }

    public function chat(Request $request, Prd $prd)
    {
        abort_unless($request->user()->isMemberOf($prd->device->team), 403);

        return Inertia::render('Prds/Chat', [
            'prd' => $prd,
            'project' => $prd->project,
            'chatHistory' => $prd->chat_history ?? [],
            'device' => $prd->device,
        ]);
    }

    public function update(Request $request, Prd $prd)
    {
        abort_unless($request->user()->isMemberOf($prd->device->team), 403);

        // Direct edit sends cmd.prd.update to device
        // Handled via CommandService — this is just the page
        return back();
    }
}
```

- [ ] **Step 3: Create Chat.vue**

```vue
<!-- resources/js/Pages/Prds/Chat.vue -->
<script setup>
import { ref, nextTick, onMounted } from 'vue';
import { router } from '@inertiajs/vue3';
import { useDeviceChannel } from '@/composables/useDeviceChannel';
import { useStreamingText } from '@/composables/useStreamingText';
import ChatMessage from '@/Components/ChatMessage.vue';
import ChatInput from '@/Components/ChatInput.vue';

const props = defineProps({
    prd: Object,
    project: Object,
    chatHistory: Array,
    device: Object,
});

const messages = ref([...props.chatHistory]);
const chatContainer = ref(null);
const { text: streamingText, isStreaming, append, reset } = useStreamingText();

// Listen for streaming chat output
useDeviceChannel(props.device.id, {
    onStream(event) {
        if (event.eventType === 'state.prd.chat.output' && event.payload.prd_id === props.prd.external_id) {
            if (event.payload.event_type === 'assistant_text') {
                append(event.payload.text);
            }
        }
    },
    onStateUpdated() {
        // Refresh page data when PRD state changes
        router.reload({ only: ['chatHistory', 'prd'] });
    },
});

async function sendMessage(text) {
    // Add user message to local state immediately
    messages.value.push({
        role: 'user',
        content: text,
        timestamp: new Date().toISOString(),
    });

    // Reset streaming for new response
    reset();

    // Send command via API
    const type = props.prd.session_id ? 'cmd.prd.message' : 'cmd.prd.create';
    const payload = props.prd.session_id
        ? { prd_id: props.prd.external_id, message: text }
        : { project_id: props.project.external_id, message: text };

    await fetch(`/api/devices/${props.device.id}/commands`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-TOKEN': document.querySelector('meta[name="csrf-token"]')?.content,
        },
        body: JSON.stringify({ type, payload }),
    });

    await nextTick();
    scrollToBottom();
}

function scrollToBottom() {
    if (chatContainer.value) {
        chatContainer.value.scrollTop = chatContainer.value.scrollHeight;
    }
}

onMounted(scrollToBottom);
</script>

<template>
    <div class="flex flex-col h-screen bg-bg">
        <!-- Header -->
        <div class="flex items-center justify-between px-4 py-3 border-b border-border bg-bg-card">
            <div class="flex items-center gap-3">
                <a :href="`/prds/${prd.id}`" class="text-text-muted hover:text-text">&larr;</a>
                <div>
                    <h1 class="text-sm font-semibold text-text-heading">{{ prd.title }}</h1>
                    <span class="text-xs font-mono text-text-muted">{{ prd.status }}</span>
                </div>
            </div>
        </div>

        <!-- Messages -->
        <div ref="chatContainer" class="flex-1 overflow-y-auto p-4 space-y-3">
            <ChatMessage
                v-for="(msg, i) in messages"
                :key="i"
                :role="msg.role"
                :content="msg.content"
            />
            <!-- Streaming response -->
            <ChatMessage
                v-if="isStreaming"
                role="assistant"
                :content="streamingText"
                :streaming="true"
            />
        </div>

        <!-- Input -->
        <ChatInput @send="sendMessage" />
    </div>
</template>
```

- [ ] **Step 4: Create ChatMessage and ChatInput components**

```vue
<!-- resources/js/Components/ChatMessage.vue -->
<script setup>
defineProps({
    role: String,
    content: String,
    streaming: { type: Boolean, default: false },
});
</script>

<template>
    <div class="flex" :class="role === 'user' ? 'justify-end' : 'justify-start'">
        <div
            class="max-w-[85%] px-3 py-2 rounded text-sm"
            :class="role === 'user'
                ? 'bg-white/[0.06] text-text'
                : 'bg-bg-surface text-text'"
        >
            <div class="prose prose-invert prose-sm max-w-none" v-html="content" />
            <span v-if="streaming" class="inline-block w-1.5 h-4 bg-interactive animate-pulse ml-0.5" />
        </div>
    </div>
</template>
```

```vue
<!-- resources/js/Components/ChatInput.vue -->
<script setup>
import { ref } from 'vue';

const emit = defineEmits(['send']);
const text = ref('');
const textarea = ref(null);

function send() {
    if (!text.value.trim()) return;
    emit('send', text.value.trim());
    text.value = '';
    if (textarea.value) textarea.value.style.height = 'auto';
}

function autoResize(event) {
    const el = event.target;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 200) + 'px';
}

function handleKeydown(event) {
    if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        send();
    }
}
</script>

<template>
    <div class="border-t border-border bg-bg-card p-3">
        <div class="flex items-end gap-2">
            <textarea
                ref="textarea"
                v-model="text"
                @input="autoResize"
                @keydown="handleKeydown"
                placeholder="Type a message..."
                rows="1"
                class="flex-1 bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text placeholder-text-muted resize-none focus:outline-none focus:border-interactive/50"
            />
            <button
                @click="send"
                :disabled="!text.trim()"
                class="bg-interactive text-bg px-3 py-2 rounded text-sm font-medium disabled:opacity-30 hover:opacity-90 transition-opacity"
            >
                Send
            </button>
        </div>
    </div>
</template>
```

- [ ] **Step 5: Add routes**

```php
// routes/web.php — inside auth middleware:
use App\Http\Controllers\PrdController;

Route::get('/prds/{prd}', [PrdController::class, 'show'])->name('prds.show');
Route::get('/prds/{prd}/chat', [PrdController::class, 'chat'])->name('prds.chat');
```

- [ ] **Step 6: Run tests**

```bash
docker compose exec app php artisan test --filter="PrdChatTest"
```
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add app/Http/Controllers/PrdController.php resources/js/Pages/Prds/ resources/js/Components/Chat* routes/web.php tests/
git commit -m "feat: add PRD chat interface with real-time streaming"
```

---

### Task 4: Run Monitoring Page (TUI-Style)

**Files:**
- Create: `app/Http/Controllers/RunController.php`
- Create: `resources/js/Pages/Runs/Show.vue`
- Create: `resources/js/Pages/Runs/Live.vue`
- Create: `resources/js/Components/StoryList.vue`
- Create: `resources/js/Components/LiveOutput.vue`

- [ ] **Step 1: Write failing test**

```php
// tests/Feature/RunPageTest.php
<?php

use App\Models\User;
use App\Models\Device;
use App\Models\Prd;
use App\Models\Run;

it('shows run detail page with story progress', function () {
    $user = User::factory()->create();
    $device = Device::create([
        'team_id' => $user->currentTeam()->id, 'name' => 'test',
        'access_token' => hash('sha256', 'tok'), 'refresh_token' => 'ref',
        'token_expires_at' => now()->addHour(),
    ]);
    $project = $device->projects()->create([
        'external_id' => 'proj_app', 'path' => '/workspace/app', 'name' => 'app',
    ]);
    $prd = Prd::create([
        'project_id' => $project->id, 'device_id' => $device->id,
        'external_id' => 'prd_auth', 'title' => 'Auth', 'status' => 'running',
        'progress' => "## Stories\n- [x] US-001 Login\n- [ ] US-002 Register",
    ]);
    $run = Run::create([
        'prd_id' => $prd->id, 'device_id' => $device->id,
        'external_id' => 'run_001', 'status' => 'running',
        'story_index' => 1, 'started_at' => now()->subMinutes(12),
    ]);

    $this->actingAs($user)->get("/runs/{$run->id}")
        ->assertOk()
        ->assertInertia(fn ($page) =>
            $page->component('Runs/Show')
                ->has('run')
                ->has('prd')
        );
});
```

- [ ] **Step 2: Implement RunController**

```php
// app/Http/Controllers/RunController.php
<?php

namespace App\Http\Controllers;

use App\Models\Run;
use Illuminate\Http\Request;
use Inertia\Inertia;

class RunController extends Controller
{
    public function show(Request $request, Run $run)
    {
        abort_unless($request->user()->isMemberOf($run->device->team), 403);

        return Inertia::render('Runs/Show', [
            'run' => $run,
            'prd' => $run->prd,
            'device' => $run->device,
        ]);
    }

    public function live(Request $request, Run $run)
    {
        abort_unless($request->user()->isMemberOf($run->device->team), 403);

        return Inertia::render('Runs/Live', [
            'run' => $run,
            'prd' => $run->prd,
            'device' => $run->device,
        ]);
    }

    public function diffs(Request $request, Run $run)
    {
        abort_unless($request->user()->isMemberOf($run->device->team), 403);

        return Inertia::render('Runs/Diffs', [
            'run' => $run,
            'prd' => $run->prd,
            'device' => $run->device,
        ]);
    }
}
```

- [ ] **Step 3: Create StoryList component**

Parses the PRD progress markdown to extract story statuses. Shows status dots (green check / blue dot / gray circle) in a vertical list. Highlights the active story. See the TUI screenshot for reference layout.

- [ ] **Step 4: Create LiveOutput component**

Terminal-style scrolling view. Dark background (`bg-bg`), Geist Mono font, auto-scroll with pause-on-tap. Subscribes to `state.run.output` via Reverb. Shows tool usage inline with labels like "Reading file.go", "Running tests".

- [ ] **Step 5: Create Runs/Show.vue**

Summary view modeled after TUI: story list on left (sidebar on desktop, vertical list on mobile), active story detail on right, progress bar at bottom, elapsed time in header. "Live" toggle link to `/runs/{id}/live`. "View Diffs" link to `/runs/{id}/diffs`. "Stop Run" button.

- [ ] **Step 6: Add routes**

```php
// routes/web.php:
use App\Http\Controllers\RunController;

Route::get('/runs/{run}', [RunController::class, 'show'])->name('runs.show');
Route::get('/runs/{run}/live', [RunController::class, 'live'])->name('runs.live');
Route::get('/runs/{run}/diffs', [RunController::class, 'diffs'])->name('runs.diffs');
```

- [ ] **Step 7: Run tests and commit**

```bash
docker compose exec app php artisan test --filter="RunPageTest"
git add app/Http/Controllers/RunController.php resources/js/Pages/Runs/ resources/js/Components/StoryList.vue resources/js/Components/LiveOutput.vue routes/web.php tests/
git commit -m "feat: add run monitoring pages (summary + live output)"
```

---

### Task 5: Diff Viewer

**Files:**
- Create: `resources/js/Pages/Runs/Diffs.vue`
- Create: `resources/js/Components/DiffViewer.vue`
- Create: `resources/js/Components/DiffFile.vue`

- [ ] **Step 1: Install Shiki**

```bash
npm install shiki
```

- [ ] **Step 2: Create DiffViewer component**

GitHub-style diff viewer with:
- File tree sidebar (collapsible drawer on mobile)
- Unified diff (mobile) / side-by-side (desktop) toggle
- Per-story filter dropdown
- Syntax highlighting via Shiki with Tokyo Night theme
- Line numbers, +/- coloring, file status icons

The diffs data comes from the `state.diffs.response` message. On page load, the Vue page sends `cmd.diffs.get` via the command API and listens for the response on the Reverb channel. Once received, it renders the diff.

- [ ] **Step 3: Create DiffFile component**

Single file diff renderer. Takes a `DiffEntry` (file_path, diff, status) and renders it with Shiki highlighting. Supports unified and side-by-side modes.

- [ ] **Step 4: Wire into Diffs.vue page**

```vue
<!-- resources/js/Pages/Runs/Diffs.vue (structure) -->
<script setup>
import { ref, onMounted } from 'vue';
import { useDeviceChannel } from '@/composables/useDeviceChannel';
import DiffViewer from '@/Components/DiffViewer.vue';

const props = defineProps({ run: Object, prd: Object, device: Object });
const diffs = ref(null);
const loading = ref(true);

// Listen for diffs response
useDeviceChannel(props.device.id, {
    onStream(event) {
        if (event.eventType === 'state.diffs.response') {
            diffs.value = event.payload.diffs;
            loading.value = false;
        }
    },
});

// Request diffs on mount
onMounted(async () => {
    await fetch(`/api/devices/${props.device.id}/commands`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-TOKEN': document.querySelector('meta[name="csrf-token"]')?.content,
        },
        body: JSON.stringify({
            type: 'cmd.diffs.get',
            payload: { project_id: props.prd.project?.external_id },
        }),
    });
});
</script>

<template>
    <div class="h-screen bg-bg flex flex-col">
        <div class="px-4 py-3 border-b border-border bg-bg-card flex items-center gap-3">
            <a :href="`/runs/${run.id}`" class="text-text-muted hover:text-text">&larr;</a>
            <h1 class="text-sm font-semibold text-text-heading">Diffs</h1>
        </div>
        <div v-if="loading" class="flex-1 flex items-center justify-center">
            <span class="text-text-muted text-sm">Loading diffs...</span>
        </div>
        <DiffViewer v-else :diffs="diffs" />
    </div>
</template>
```

- [ ] **Step 5: Commit**

```bash
git add resources/js/Pages/Runs/Diffs.vue resources/js/Components/Diff*
git commit -m "feat: add GitHub-style diff viewer with Shiki highlighting"
```

---

### Task 6: File Browser

**Files:**
- Create: `app/Http/Controllers/FileController.php`
- Create: `resources/js/Pages/Files/Show.vue`
- Create: `resources/js/Components/FileTree.vue`
- Create: `resources/js/Components/FileViewer.vue`

- [ ] **Step 1: Implement FileController**

```php
// app/Http/Controllers/FileController.php
<?php

namespace App\Http\Controllers;

use App\Models\Device;
use Illuminate\Http\Request;
use Inertia\Inertia;

class FileController extends Controller
{
    public function show(Request $request, Device $device, string $path = '')
    {
        abort_unless($request->user()->isMemberOf($device->team), 403);

        return Inertia::render('Files/Show', [
            'device' => $device,
            'path' => $path,
            'projects' => $device->projects,
        ]);
    }
}
```

- [ ] **Step 2: Create Files/Show.vue**

Page sends `cmd.files.list` on mount and on directory navigation. Listens for `state.files.list` response. When a file is clicked, sends `cmd.file.get` and shows the content in FileViewer with Shiki highlighting. Breadcrumb navigation for the path.

- [ ] **Step 3: Add route**

```php
// routes/web.php:
use App\Http\Controllers\FileController;

Route::get('/files/{device}/{path?}', [FileController::class, 'show'])
    ->where('path', '.*')
    ->name('files.show');
```

- [ ] **Step 4: Commit**

```bash
git add app/Http/Controllers/FileController.php resources/js/Pages/Files/ resources/js/Components/File* routes/web.php
git commit -m "feat: add file browser with syntax-highlighted viewer"
```

---

### Task 7: PRD Preview & Direct Edit

**Files:**
- Create: `resources/js/Pages/Prds/Show.vue`
- Create: `resources/js/Components/MarkdownEditor.vue`
- Create: `resources/js/Components/MarkdownPreview.vue`

- [ ] **Step 1: Install CodeMirror**

```bash
npm install @codemirror/view @codemirror/state @codemirror/lang-markdown @codemirror/theme-one-dark codemirror
```

- [ ] **Step 2: Create MarkdownEditor**

CodeMirror instance configured with markdown language support and One Dark theme (closest to Tokyo Night). Save button sends `cmd.prd.update` via command API.

- [ ] **Step 3: Create Prds/Show.vue**

Shows PRD content as rendered markdown (MarkdownPreview). "Edit" button toggles to MarkdownEditor. "Chat" link goes to `/prds/{id}/chat`. "Start Run" button sends `cmd.run.start`. Shows list of runs for this PRD.

- [ ] **Step 4: Commit**

```bash
git add resources/js/Pages/Prds/Show.vue resources/js/Components/Markdown*
git commit -m "feat: add PRD preview with markdown editor and run controls"
```

---

### Task 8: Project Detail Page

**Files:**
- Create: `app/Http/Controllers/ProjectController.php`
- Create: `resources/js/Pages/Projects/Show.vue`

- [ ] **Step 1: Implement ProjectController**

```php
// app/Http/Controllers/ProjectController.php
<?php

namespace App\Http\Controllers;

use App\Models\Device;
use App\Models\Project;
use Illuminate\Http\Request;
use Inertia\Inertia;

class ProjectController extends Controller
{
    public function show(Request $request, Device $device, Project $project)
    {
        abort_unless($request->user()->isMemberOf($device->team), 403);
        abort_unless($project->device_id === $device->id, 404);

        return Inertia::render('Projects/Show', [
            'device' => $device,
            'project' => $project,
            'prds' => $project->prds()->with('runs')->get(),
        ]);
    }
}
```

Shows project name, git status (branch, clean/dirty, last commit), list of PRDs with status badges, "New PRD" button (two paths: chat or write directly).

- [ ] **Step 2: Add route**

```php
Route::get('/devices/{device}/projects/{project}', [ProjectController::class, 'show'])->name('projects.show');
```

- [ ] **Step 3: Commit**

```bash
git add app/Http/Controllers/ProjectController.php resources/js/Pages/Projects/ routes/web.php
git commit -m "feat: add project detail page with PRD list"
```

---

### Task 9: Vue Component Tests (Vitest)

**Files:**
- Create: `vitest.config.js`
- Create: `tests/js/StoryList.test.js`
- Create: `tests/js/ChatMessage.test.js`

- [ ] **Step 1: Set up Vitest**

```bash
npm install -D vitest @vue/test-utils jsdom
```

```js
// vitest.config.js
import { defineConfig } from 'vitest/config';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
    plugins: [vue()],
    test: {
        environment: 'jsdom',
        globals: true,
    },
    resolve: {
        alias: { '@': '/resources/js' },
    },
});
```

- [ ] **Step 2: Write StoryList test**

```js
// tests/js/StoryList.test.js
import { mount } from '@vue/test-utils';
import StoryList from '@/Components/StoryList.vue';

describe('StoryList', () => {
    it('renders stories with correct status indicators', () => {
        const wrapper = mount(StoryList, {
            props: {
                stories: [
                    { id: 'US-001', title: 'Login', status: 'done' },
                    { id: 'US-002', title: 'Register', status: 'in_progress' },
                    { id: 'US-003', title: 'Profile', status: 'pending' },
                ],
                activeStoryId: 'US-002',
            },
        });

        expect(wrapper.findAll('[data-testid="story-item"]')).toHaveLength(3);
        expect(wrapper.find('[data-testid="story-US-002"]').classes()).toContain('active');
    });
});
```

- [ ] **Step 3: Write ChatMessage test**

```js
// tests/js/ChatMessage.test.js
import { mount } from '@vue/test-utils';
import ChatMessage from '@/Components/ChatMessage.vue';

describe('ChatMessage', () => {
    it('renders user message on the right', () => {
        const wrapper = mount(ChatMessage, {
            props: { role: 'user', content: 'Hello' },
        });
        expect(wrapper.find('.justify-end').exists()).toBe(true);
    });

    it('renders assistant message on the left', () => {
        const wrapper = mount(ChatMessage, {
            props: { role: 'assistant', content: 'Hi there' },
        });
        expect(wrapper.find('.justify-start').exists()).toBe(true);
    });

    it('shows streaming cursor when streaming', () => {
        const wrapper = mount(ChatMessage, {
            props: { role: 'assistant', content: 'Thinking...', streaming: true },
        });
        expect(wrapper.find('.animate-pulse').exists()).toBe(true);
    });
});
```

- [ ] **Step 4: Run tests**

```bash
npx vitest run
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add vitest.config.js tests/js/
git commit -m "feat: add Vitest setup with StoryList and ChatMessage component tests"
```
