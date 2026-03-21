# Uplink Web App: Scaffolding & Auth Implementation Plan (Plan 3a)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up the Laravel 13 project with Docker Compose, custom authentication (GitHub OAuth + email/password), teams with Owner/Member roles, and the base layout with mobile-first navigation.

**Architecture:** A new Laravel 13 project (`chief-uplink`) with Octane/FrankenPHP, Vue 3/Inertia, and Tailwind 4. Custom auth without starter kits. Every user gets a default team on registration. All resources will be team-scoped. Mobile-first bottom tab navigation with desktop sidebar adaptation.

**Tech Stack:** Laravel 13, Octane (FrankenPHP), Vue 3, Inertia.js, Tailwind 4, MariaDB, Redis, Pest, Vitest, Geist font

**Spec:** `docs/superpowers/specs/2026-03-22-uplink-webapp-design.md`

**Prerequisite:** None — this is the first plan for the web app (separate repo: `chief-uplink`)

---

## File Structure

```
chief-uplink/
├── docker-compose.yml                          ← Dev environment (MariaDB, Redis, Mailpit)
├── docker-compose.prod.yml                     ← Production environment
├── Dockerfile                                  ← FrankenPHP-based image
├── .env.example                                ← Environment template
├── app/
│   ├── Models/
│   │   ├── User.php                            ← User model (GitHub fields, theme, last_visited_url)
│   │   ├── Team.php                            ← Team model
│   │   └── TeamInvitation.php                  ← Team invitation model
│   ├── Http/
│   │   ├── Controllers/
│   │   │   ├── Auth/
│   │   │   │   ├── LoginController.php         ← Email/password login
│   │   │   │   ├── RegisterController.php      ← Registration + default team creation
│   │   │   │   ├── GitHubController.php        ← GitHub OAuth flow
│   │   │   │   └── LogoutController.php        ← Logout
│   │   │   ├── DashboardController.php         ← Home / resume last context
│   │   │   └── Settings/
│   │   │       ├── ProfileController.php       ← User settings
│   │   │       └── TeamController.php          ← Team management, invitations
│   │   └── Middleware/
│   │       ├── TrackLastVisitedUrl.php          ← Stores last URL for resume-context
│   │       └── EnsureTeamAccess.php             ← Authorizes team-scoped resources
│   ├── Policies/
│   │   └── TeamPolicy.php                      ← Owner vs Member authorization
│   └── Services/
│       └── TeamService.php                     ← Team creation, invitation, member management
├── database/
│   └── migrations/
│       ├── xxxx_create_users_table.php
│       ├── xxxx_create_teams_table.php
│       ├── xxxx_create_team_user_table.php
│       └── xxxx_create_team_invitations_table.php
├── resources/
│   ├── js/
│   │   ├── app.js                              ← Inertia + Vue setup
│   │   ├── Layouts/
│   │   │   └── AppLayout.vue                   ← Main layout (bottom tabs mobile, sidebar desktop)
│   │   ├── Components/
│   │   │   ├── BottomNav.vue                   ← Mobile bottom tab navigation
│   │   │   ├── Sidebar.vue                     ← Desktop sidebar navigation
│   │   │   └── ThemeProvider.vue               ← Dark/light theme management
│   │   └── Pages/
│   │       ├── Auth/
│   │       │   ├── Login.vue                   ← Login page
│   │       │   └── Register.vue                ← Registration page
│   │       ├── Dashboard.vue                   ← Home / resume context
│   │       └── Settings/
│   │           ├── Profile.vue                 ← User settings (theme, etc.)
│   │           └── Team.vue                    ← Team management
│   └── css/
│       └── app.css                             ← Tailwind imports + Geist font + theme tokens
├── routes/
│   ├── web.php                                 ← Inertia routes
│   └── auth.php                                ← Auth routes (login, register, GitHub OAuth)
└── tests/
    ├── Feature/
    │   ├── Auth/
    │   │   ├── LoginTest.php
    │   │   ├── RegisterTest.php
    │   │   └── GitHubAuthTest.php
    │   ├── TeamTest.php
    │   └── DashboardTest.php
    └── Unit/
        └── Services/
            └── TeamServiceTest.php
```

---

### Task 1: Project Scaffolding & Docker Compose

**Files:**
- Create: `chief-uplink/` (new Laravel 13 project)
- Create: `docker-compose.yml`
- Create: `Dockerfile`
- Create: `.env.example`

- [ ] **Step 1: Create new Laravel 13 project**

```bash
composer create-project laravel/laravel chief-uplink
cd chief-uplink
```

- [ ] **Step 2: Install core dependencies**

```bash
composer require laravel/octane laravel/reverb laravel/sanctum inertiajs/inertia-laravel
composer require --dev pestphp/pest pestphp/pest-plugin-laravel
npm install @inertiajs/vue3 vue @vitejs/plugin-vue tailwindcss @tailwindcss/vite
```

- [ ] **Step 3: Create Dockerfile**

```dockerfile
# Dockerfile
FROM dunglas/frankenphp:latest-php8.4-alpine

RUN install-php-extensions \
    pdo_mysql \
    redis \
    pcntl \
    intl \
    zip \
    bcmath

COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

RUN apk add --no-cache nodejs npm

WORKDIR /var/www/html

COPY composer.json composer.lock ./
RUN composer install --no-dev --no-scripts --no-autoloader

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN composer dump-autoload --optimize
RUN npm run build

EXPOSE 8000 8080

CMD ["php", "artisan", "octane:frankenphp", "--host=0.0.0.0", "--port=8000"]
```

- [ ] **Step 4: Create docker-compose.yml**

```yaml
# docker-compose.yml
services:
  app:
    build: .
    ports:
      - "8000:8000"
      - "8080:8080"
    volumes:
      - .:/var/www/html
      - /var/www/html/vendor
      - /var/www/html/node_modules
    depends_on:
      - mariadb
      - redis
    environment:
      APP_ENV: local
      APP_DEBUG: "true"
      APP_KEY: ${APP_KEY}
      APP_URL: http://localhost:8000
      DB_CONNECTION: mysql
      DB_HOST: mariadb
      DB_PORT: 3306
      DB_DATABASE: uplink
      DB_USERNAME: uplink
      DB_PASSWORD: secret
      REDIS_HOST: redis
      REVERB_APP_ID: uplink-local
      REVERB_APP_KEY: local-key
      REVERB_APP_SECRET: local-secret
      REVERB_HOST: 0.0.0.0
      REVERB_PORT: 8080
    command: php artisan octane:frankenphp --host=0.0.0.0 --port=8000 --watch

  mariadb:
    image: mariadb:11
    ports:
      - "3306:3306"
    environment:
      MYSQL_DATABASE: uplink
      MYSQL_USER: uplink
      MYSQL_PASSWORD: secret
      MYSQL_ROOT_PASSWORD: secret
    volumes:
      - mariadb_data:/var/lib/mysql

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  mailpit:
    image: axllent/mailpit
    ports:
      - "1025:1025"
      - "8025:8025"

volumes:
  mariadb_data:
```

- [ ] **Step 5: Configure .env.example**

```bash
APP_NAME="Chief Uplink"
APP_ENV=local
APP_DEBUG=true
APP_URL=http://localhost:8000

DB_CONNECTION=mysql
DB_HOST=mariadb
DB_PORT=3306
DB_DATABASE=uplink
DB_USERNAME=uplink
DB_PASSWORD=secret

REDIS_HOST=redis
REDIS_PORT=6379

REVERB_APP_ID=uplink-local
REVERB_APP_KEY=local-key
REVERB_APP_SECRET=local-secret
REVERB_HOST=0.0.0.0
REVERB_PORT=8080

GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
GITHUB_REDIRECT_URI=http://localhost:8000/auth/github/callback

MAIL_MAILER=smtp
MAIL_HOST=mailpit
MAIL_PORT=1025
```

- [ ] **Step 6: Verify Docker Compose starts**

```bash
docker compose up -d
docker compose exec app php artisan key:generate
docker compose exec app php artisan migrate
```
Expected: all services running, migrations succeed

- [ ] **Step 7: Commit**

```bash
git init
git add .
git commit -m "feat: scaffold Laravel 13 project with Docker Compose"
```

---

### Task 2: Tailwind 4, Geist Font, Theme Tokens

**Files:**
- Create: `resources/css/app.css`
- Modify: `vite.config.js`
- Modify: `resources/views/app.blade.php`

- [ ] **Step 1: Configure Vite with Tailwind 4 and Vue**

```js
// vite.config.js
import { defineConfig } from 'vite';
import laravel from 'laravel-vite-plugin';
import vue from '@vitejs/plugin-vue';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
    plugins: [
        laravel({
            input: ['resources/css/app.css', 'resources/js/app.js'],
            refresh: true,
        }),
        vue({
            template: {
                transformAssetUrls: {
                    base: null,
                    includeAbsolute: false,
                },
            },
        }),
        tailwindcss(),
    ],
});
```

- [ ] **Step 2: Create CSS with theme tokens**

```css
/* resources/css/app.css */
@import "tailwindcss";

@theme {
    /* Typography */
    --font-sans: 'Geist', system-ui, -apple-system, sans-serif;
    --font-mono: 'Geist Mono', ui-monospace, SFMono-Regular, monospace;

    /* Dark theme colors (default) */
    --color-bg: #0a0a0f;
    --color-bg-card: #12131a;
    --color-bg-surface: #1a1b26;
    --color-text: #c9d1d9;
    --color-text-heading: #f0f6fc;
    --color-text-secondary: #8b949e;
    --color-text-muted: #484f58;
    --color-border: #1a1b26;
    --color-interactive: #E5E5E5;
    --color-brand: #1D3F6A;

    /* Status colors */
    --color-success: #3fb950;
    --color-info: #58a6ff;
    --color-warning: #f0b429;
    --color-error: #f85149;
}

/* Light mode overrides */
@media (prefers-color-scheme: light) {
    :root:not([data-theme="dark"]) {
        --color-bg: #ffffff;
        --color-bg-card: #f6f8fa;
        --color-bg-surface: #ffffff;
        --color-text: #424a53;
        --color-text-heading: #1f2328;
        --color-text-secondary: #656d76;
        --color-text-muted: #8b949e;
        --color-border: #d1d9e0;
        --color-interactive: #1a1a1a;
    }
}

[data-theme="dark"] {
    --color-bg: #0a0a0f;
    --color-bg-card: #12131a;
    --color-bg-surface: #1a1b26;
    --color-text: #c9d1d9;
    --color-text-heading: #f0f6fc;
    --color-text-secondary: #8b949e;
    --color-text-muted: #484f58;
    --color-border: #1a1b26;
    --color-interactive: #E5E5E5;
}

[data-theme="light"] {
    --color-bg: #ffffff;
    --color-bg-card: #f6f8fa;
    --color-bg-surface: #ffffff;
    --color-text: #424a53;
    --color-text-heading: #1f2328;
    --color-text-secondary: #656d76;
    --color-text-muted: #8b949e;
    --color-border: #d1d9e0;
    --color-interactive: #1a1a1a;
}

body {
    @apply bg-bg text-text font-sans antialiased;
    letter-spacing: -0.01em;
}

h1, h2, h3, h4 {
    @apply text-text-heading;
    letter-spacing: -0.02em;
}
```

- [ ] **Step 3: Update Blade template with Geist font**

```html
<!-- resources/views/app.blade.php -->
<!DOCTYPE html>
<html lang="{{ str_replace('_', '-', app()->getLocale()) }}" data-theme="{{ auth()->user()?->theme_preference ?? 'system' }}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title inertia>{{ config('app.name') }}</title>
    <link rel="preconnect" href="https://cdn.jsdelivr.net" crossorigin>
    <link href="https://cdn.jsdelivr.net/npm/geist@1/dist/fonts/geist-sans/style.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/geist@1/dist/fonts/geist-mono/style.css" rel="stylesheet">
    @vite(['resources/css/app.css', 'resources/js/app.js'])
    @inertiaHead
</head>
<body>
    @inertia
</body>
</html>
```

- [ ] **Step 4: Commit**

```bash
git add resources/css/app.css vite.config.js resources/views/app.blade.php
git commit -m "feat: add Tailwind 4 theme tokens with Geist font and dark/light mode"
```

---

### Task 3: Inertia + Vue Setup

**Files:**
- Create: `resources/js/app.js`
- Create: `resources/js/Layouts/AppLayout.vue`
- Create: `resources/js/Pages/Dashboard.vue`

- [ ] **Step 1: Configure Inertia with Vue**

```js
// resources/js/app.js
import { createApp, h } from 'vue';
import { createInertiaApp } from '@inertiajs/vue3';
import AppLayout from './Layouts/AppLayout.vue';

createInertiaApp({
    title: (title) => title ? `${title} - Chief Uplink` : 'Chief Uplink',
    resolve: (name) => {
        const pages = import.meta.glob('./Pages/**/*.vue', { eager: true });
        const page = pages[`./Pages/${name}.vue`];
        page.default.layout = page.default.layout || AppLayout;
        return page;
    },
    setup({ el, App, props, plugin }) {
        createApp({ render: () => h(App, props) })
            .use(plugin)
            .mount(el);
    },
});
```

- [ ] **Step 2: Create AppLayout with mobile bottom tabs + desktop sidebar**

```vue
<!-- resources/js/Layouts/AppLayout.vue -->
<script setup>
import { Link, usePage } from '@inertiajs/vue3';
import { computed } from 'vue';

const page = usePage();
const currentPath = computed(() => page.url);

const navItems = [
    { name: 'Home', href: '/', icon: 'home' },
    { name: 'Devices', href: '/devices', icon: 'server' },
    { name: 'Servers', href: '/servers', icon: 'cloud' },
    { name: 'Settings', href: '/settings', icon: 'settings' },
];

function isActive(href) {
    if (href === '/') return currentPath.value === '/';
    return currentPath.value.startsWith(href);
}
</script>

<template>
    <div class="min-h-screen bg-bg flex">
        <!-- Desktop Sidebar (hidden on mobile) -->
        <aside class="hidden md:flex md:w-56 md:flex-col md:border-r md:border-border bg-bg-card">
            <div class="p-4 border-b border-border">
                <span class="text-text-heading font-semibold text-sm tracking-tight">Chief Uplink</span>
            </div>
            <nav class="flex-1 p-2 space-y-0.5">
                <Link
                    v-for="item in navItems"
                    :key="item.name"
                    :href="item.href"
                    class="flex items-center gap-2 px-3 py-2 rounded text-sm transition-colors"
                    :class="isActive(item.href)
                        ? 'bg-bg-surface text-text-heading font-medium'
                        : 'text-text-secondary hover:text-text hover:bg-bg-surface/50'"
                >
                    {{ item.name }}
                </Link>
            </nav>
        </aside>

        <!-- Main content -->
        <main class="flex-1 pb-16 md:pb-0">
            <slot />
        </main>

        <!-- Mobile Bottom Tabs (hidden on desktop) -->
        <nav class="md:hidden fixed bottom-0 left-0 right-0 bg-bg-card border-t border-border flex justify-around py-2 z-50">
            <Link
                v-for="item in navItems"
                :key="item.name"
                :href="item.href"
                class="flex flex-col items-center gap-0.5 px-3 py-1 text-xs transition-colors"
                :class="isActive(item.href) ? 'text-text-heading' : 'text-text-muted'"
            >
                <span class="text-base">{{ item.name }}</span>
            </Link>
        </nav>
    </div>
</template>
```

- [ ] **Step 3: Create Dashboard placeholder**

```vue
<!-- resources/js/Pages/Dashboard.vue -->
<script setup>
defineProps({
    lastVisitedUrl: String,
});
</script>

<template>
    <div class="p-4 md:p-8">
        <h1 class="text-xl font-semibold mb-2">Welcome to Chief Uplink</h1>
        <p class="text-text-secondary text-sm">Your remote control for Chief.</p>
    </div>
</template>
```

- [ ] **Step 4: Add dashboard route**

```php
// routes/web.php
use App\Http\Controllers\DashboardController;
use Illuminate\Support\Facades\Route;

Route::middleware(['auth'])->group(function () {
    Route::get('/', [DashboardController::class, 'index'])->name('dashboard');
});
```

```php
// app/Http/Controllers/DashboardController.php
<?php

namespace App\Http\Controllers;

use Inertia\Inertia;

class DashboardController extends Controller
{
    public function index()
    {
        $user = auth()->user();

        if ($user->last_visited_url && $user->last_visited_url !== '/') {
            return redirect($user->last_visited_url);
        }

        return Inertia::render('Dashboard');
    }
}
```

- [ ] **Step 5: Verify page loads in browser**

```bash
docker compose exec app php artisan serve &
npm run dev
# Visit http://localhost:8000
```
Expected: Dashboard page renders with Geist font and dark theme

- [ ] **Step 6: Commit**

```bash
git add resources/js/ app/Http/Controllers/DashboardController.php routes/web.php
git commit -m "feat: add Inertia/Vue setup with AppLayout and Dashboard"
```

---

### Task 4: User Model & Migrations

**Files:**
- Modify: `app/Models/User.php`
- Modify: `database/migrations/xxxx_create_users_table.php`
- Create: `database/migrations/xxxx_create_teams_table.php`
- Create: `database/migrations/xxxx_create_team_user_table.php`
- Create: `database/migrations/xxxx_create_team_invitations_table.php`

- [ ] **Step 1: Write test for user creation with default team**

```php
// tests/Feature/Auth/RegisterTest.php
<?php

use App\Models\User;
use App\Models\Team;

it('creates a default team when a user registers', function () {
    $user = User::factory()->create([
        'name' => 'Test User',
        'email' => 'test@example.com',
    ]);

    expect($user->teams)->toHaveCount(1);
    expect($user->currentTeam()()->name)->toBe("Test User's Team");
    expect($user->teams->first()->pivot->role)->toBe('owner');
});

it('returns the current team', function () {
    $user = User::factory()->create();
    $team = $user->currentTeam();

    expect($team)->toBeInstanceOf(Team::class);
    expect($team->owner_id)->toBe($user->id);
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
docker compose exec app php artisan test --filter="creates a default team"
```
Expected: FAIL (Team model doesn't exist)

- [ ] **Step 3: Update users migration**

```php
// database/migrations/0001_01_01_000000_create_users_table.php
// Modify the up() method's Schema::create('users') call to include:
Schema::create('users', function (Blueprint $table) {
    $table->id();
    $table->string('name');
    $table->string('email')->unique();
    $table->string('password')->nullable(); // nullable for GitHub-only users
    $table->string('github_id')->nullable()->unique();
    $table->text('github_token')->nullable();
    $table->string('avatar_url')->nullable();
    $table->string('last_visited_url')->nullable();
    $table->enum('theme_preference', ['dark', 'light', 'system'])->default('system');
    $table->rememberToken();
    $table->timestamps();
});
```

- [ ] **Step 4: Create teams migration**

```php
// database/migrations/xxxx_create_teams_table.php
Schema::create('teams', function (Blueprint $table) {
    $table->id();
    $table->string('name');
    $table->foreignId('owner_id')->constrained('users')->cascadeOnDelete();
    $table->timestamps();
});
```

- [ ] **Step 5: Create team_user pivot migration**

```php
// database/migrations/xxxx_create_team_user_table.php
Schema::create('team_user', function (Blueprint $table) {
    $table->foreignId('team_id')->constrained()->cascadeOnDelete();
    $table->foreignId('user_id')->constrained()->cascadeOnDelete();
    $table->enum('role', ['owner', 'member'])->default('member');
    $table->timestamps();

    $table->primary(['team_id', 'user_id']);
});
```

- [ ] **Step 6: Create team_invitations migration**

```php
// database/migrations/xxxx_create_team_invitations_table.php
Schema::create('team_invitations', function (Blueprint $table) {
    $table->id();
    $table->foreignId('team_id')->constrained()->cascadeOnDelete();
    $table->string('email');
    $table->string('token', 64)->unique();
    $table->timestamp('accepted_at')->nullable();
    $table->timestamps();
});
```

- [ ] **Step 7: Update User model**

```php
// app/Models/User.php
<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;

class User extends Authenticatable
{
    use HasFactory, Notifiable;

    protected $fillable = [
        'name', 'email', 'password',
        'github_id', 'github_token', 'avatar_url',
        'last_visited_url', 'theme_preference',
    ];

    protected $hidden = ['password', 'remember_token', 'github_token'];

    protected function casts(): array
    {
        return [
            'password' => 'hashed',
            'github_token' => 'encrypted',
        ];
    }

    public function teams()
    {
        return $this->belongsToMany(Team::class)->withPivot('role')->withTimestamps();
    }

    public function ownedTeams()
    {
        return $this->hasMany(Team::class, 'owner_id');
    }

    public function currentTeam(): Team
    {
        return $this->teams()->first() ?? $this->createDefaultTeam();
    }

    public function isOwnerOf(Team $team): bool
    {
        return $this->teams()->where('team_id', $team->id)->wherePivot('role', 'owner')->exists();
    }

    public function isMemberOf(Team $team): bool
    {
        return $this->teams()->where('team_id', $team->id)->exists();
    }

    private function createDefaultTeam(): Team
    {
        $team = Team::create([
            'name' => "{$this->name}'s Team",
            'owner_id' => $this->id,
        ]);

        $team->users()->attach($this->id, ['role' => 'owner']);

        return $team;
    }
}
```

- [ ] **Step 8: Create Team model**

```php
// app/Models/Team.php
<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\BelongsToMany;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Team extends Model
{
    protected $fillable = ['name', 'owner_id'];

    public function owner(): BelongsTo
    {
        return $this->belongsTo(User::class, 'owner_id');
    }

    public function users(): BelongsToMany
    {
        return $this->belongsToMany(User::class)->withPivot('role')->withTimestamps();
    }

    public function invitations(): HasMany
    {
        return $this->hasMany(TeamInvitation::class);
    }
}
```

- [ ] **Step 9: Create TeamInvitation model**

```php
// app/Models/TeamInvitation.php
<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class TeamInvitation extends Model
{
    protected $fillable = ['team_id', 'email', 'token', 'accepted_at'];

    protected function casts(): array
    {
        return ['accepted_at' => 'datetime'];
    }

    public function team(): BelongsTo
    {
        return $this->belongsTo(Team::class);
    }

    public function isPending(): bool
    {
        return is_null($this->accepted_at);
    }
}
```

- [ ] **Step 10: Update UserFactory to create default team**

```php
// database/factories/UserFactory.php
// Add an afterCreating callback:
public function configure(): static
{
    return $this->afterCreating(function (User $user) {
        $user->currentTeam()(); // triggers default team creation
    });
}
```

- [ ] **Step 11: Run migrations and tests**

```bash
docker compose exec app php artisan migrate:fresh
docker compose exec app php artisan test --filter="RegisterTest"
```
Expected: PASS

- [ ] **Step 12: Commit**

```bash
git add app/Models/ database/migrations/ database/factories/ tests/
git commit -m "feat: add User, Team, TeamInvitation models with default team creation"
```

---

### Task 5: Email/Password Authentication

**Files:**
- Create: `app/Http/Controllers/Auth/LoginController.php`
- Create: `app/Http/Controllers/Auth/RegisterController.php`
- Create: `app/Http/Controllers/Auth/LogoutController.php`
- Create: `resources/js/Pages/Auth/Login.vue`
- Create: `resources/js/Pages/Auth/Register.vue`
- Create: `routes/auth.php`
- Create: `tests/Feature/Auth/LoginTest.php`

- [ ] **Step 1: Write failing tests**

```php
// tests/Feature/Auth/LoginTest.php
<?php

use App\Models\User;

it('shows the login page', function () {
    $this->get('/login')->assertOk()->assertInertia(fn ($page) =>
        $page->component('Auth/Login')
    );
});

it('logs in with valid credentials', function () {
    $user = User::factory()->create([
        'password' => bcrypt('password123'),
    ]);

    $this->post('/login', [
        'email' => $user->email,
        'password' => 'password123',
    ])->assertRedirect('/');

    $this->assertAuthenticatedAs($user);
});

it('rejects invalid credentials', function () {
    $user = User::factory()->create([
        'password' => bcrypt('password123'),
    ]);

    $this->post('/login', [
        'email' => $user->email,
        'password' => 'wrong',
    ])->assertSessionHasErrors('email');

    $this->assertGuest();
});

it('registers a new user', function () {
    $this->post('/register', [
        'name' => 'New User',
        'email' => 'new@example.com',
        'password' => 'password123',
        'password_confirmation' => 'password123',
    ])->assertRedirect('/');

    $this->assertAuthenticated();
    expect(User::where('email', 'new@example.com')->exists())->toBeTrue();
});

it('logs out', function () {
    $user = User::factory()->create();
    $this->actingAs($user)->post('/logout')->assertRedirect('/login');
    $this->assertGuest();
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker compose exec app php artisan test --filter="LoginTest"
```

- [ ] **Step 3: Implement controllers**

```php
// app/Http/Controllers/Auth/LoginController.php
<?php

namespace App\Http\Controllers\Auth;

use App\Http\Controllers\Controller;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;
use Inertia\Inertia;

class LoginController extends Controller
{
    public function show()
    {
        return Inertia::render('Auth/Login');
    }

    public function store(Request $request)
    {
        $credentials = $request->validate([
            'email' => ['required', 'email'],
            'password' => ['required'],
        ]);

        if (! Auth::attempt($credentials, $request->boolean('remember'))) {
            return back()->withErrors([
                'email' => 'These credentials do not match our records.',
            ])->onlyInput('email');
        }

        $request->session()->regenerate();

        return redirect()->intended('/');
    }
}
```

```php
// app/Http/Controllers/Auth/RegisterController.php
<?php

namespace App\Http\Controllers\Auth;

use App\Http\Controllers\Controller;
use App\Models\User;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;
use Illuminate\Validation\Rules\Password;
use Inertia\Inertia;

class RegisterController extends Controller
{
    public function show()
    {
        return Inertia::render('Auth/Register');
    }

    public function store(Request $request)
    {
        $validated = $request->validate([
            'name' => ['required', 'string', 'max:255'],
            'email' => ['required', 'email', 'max:255', 'unique:users'],
            'password' => ['required', 'confirmed', Password::defaults()],
        ]);

        $user = User::create($validated);

        Auth::login($user);

        return redirect('/');
    }
}
```

```php
// app/Http/Controllers/Auth/LogoutController.php
<?php

namespace App\Http\Controllers\Auth;

use App\Http\Controllers\Controller;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;

class LogoutController extends Controller
{
    public function store(Request $request)
    {
        Auth::logout();
        $request->session()->invalidate();
        $request->session()->regenerateToken();

        return redirect('/login');
    }
}
```

- [ ] **Step 4: Create auth routes**

```php
// routes/auth.php
<?php

use App\Http\Controllers\Auth\LoginController;
use App\Http\Controllers\Auth\RegisterController;
use App\Http\Controllers\Auth\LogoutController;
use Illuminate\Support\Facades\Route;

Route::middleware('guest')->group(function () {
    Route::get('/login', [LoginController::class, 'show'])->name('login');
    Route::post('/login', [LoginController::class, 'store']);
    Route::get('/register', [RegisterController::class, 'show'])->name('register');
    Route::post('/register', [RegisterController::class, 'store']);
});

Route::middleware('auth')->group(function () {
    Route::post('/logout', [LogoutController::class, 'store'])->name('logout');
});
```

Register the auth routes file in `bootstrap/app.php` inside the `withRouting` call:

```php
->withRouting(
    web: __DIR__.'/../routes/web.php',
    commands: __DIR__.'/../routes/console.php',
    health: '/up',
    then: function () {
        Route::middleware('web')->group(base_path('routes/auth.php'));
    },
)
```

Add `use Illuminate\Support\Facades\Route;` to the imports in `bootstrap/app.php`.

- [ ] **Step 5: Create Login.vue**

```vue
<!-- resources/js/Pages/Auth/Login.vue -->
<script setup>
import { useForm, Link } from '@inertiajs/vue3';

defineOptions({ layout: null }); // no app layout for auth pages

const form = useForm({
    email: '',
    password: '',
    remember: false,
});

function submit() {
    form.post('/login');
}
</script>

<template>
    <div class="min-h-screen bg-bg flex items-center justify-center p-4">
        <div class="w-full max-w-sm">
            <h1 class="text-xl font-semibold text-text-heading mb-1 text-center">Chief Uplink</h1>
            <p class="text-text-secondary text-sm mb-8 text-center">Sign in to your account</p>

            <a href="/auth/github" class="flex items-center justify-center gap-2 w-full bg-interactive text-bg font-medium text-sm py-2.5 rounded hover:opacity-90 transition-opacity mb-4">
                Sign in with GitHub
            </a>

            <div class="flex items-center gap-3 mb-4">
                <div class="flex-1 h-px bg-border"></div>
                <span class="text-text-muted text-xs">or</span>
                <div class="flex-1 h-px bg-border"></div>
            </div>

            <form @submit.prevent="submit" class="space-y-3">
                <div>
                    <input
                        v-model="form.email"
                        type="email"
                        placeholder="Email"
                        class="w-full bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text placeholder-text-muted focus:outline-none focus:border-interactive/50"
                    />
                    <p v-if="form.errors.email" class="text-error text-xs mt-1">{{ form.errors.email }}</p>
                </div>
                <div>
                    <input
                        v-model="form.password"
                        type="password"
                        placeholder="Password"
                        class="w-full bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text placeholder-text-muted focus:outline-none focus:border-interactive/50"
                    />
                </div>
                <button
                    type="submit"
                    :disabled="form.processing"
                    class="w-full bg-interactive text-bg font-medium text-sm py-2.5 rounded hover:opacity-90 transition-opacity disabled:opacity-50"
                >
                    Sign in
                </button>
            </form>

            <p class="text-text-muted text-xs text-center mt-6">
                Don't have an account? <Link href="/register" class="text-text underline underline-offset-2">Sign up</Link>
            </p>
        </div>
    </div>
</template>
```

- [ ] **Step 6: Create Register.vue**

```vue
<!-- resources/js/Pages/Auth/Register.vue -->
<script setup>
import { useForm, Link } from '@inertiajs/vue3';

defineOptions({ layout: null });

const form = useForm({
    name: '',
    email: '',
    password: '',
    password_confirmation: '',
});

function submit() {
    form.post('/register');
}
</script>

<template>
    <div class="min-h-screen bg-bg flex items-center justify-center p-4">
        <div class="w-full max-w-sm">
            <h1 class="text-xl font-semibold text-text-heading mb-1 text-center">Create Account</h1>
            <p class="text-text-secondary text-sm mb-8 text-center">Get started with Chief Uplink</p>

            <a href="/auth/github" class="flex items-center justify-center gap-2 w-full bg-interactive text-bg font-medium text-sm py-2.5 rounded hover:opacity-90 transition-opacity mb-4">
                Sign up with GitHub
            </a>

            <div class="flex items-center gap-3 mb-4">
                <div class="flex-1 h-px bg-border"></div>
                <span class="text-text-muted text-xs">or</span>
                <div class="flex-1 h-px bg-border"></div>
            </div>

            <form @submit.prevent="submit" class="space-y-3">
                <div>
                    <input v-model="form.name" type="text" placeholder="Name"
                        class="w-full bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text placeholder-text-muted focus:outline-none focus:border-interactive/50" />
                    <p v-if="form.errors.name" class="text-error text-xs mt-1">{{ form.errors.name }}</p>
                </div>
                <div>
                    <input v-model="form.email" type="email" placeholder="Email"
                        class="w-full bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text placeholder-text-muted focus:outline-none focus:border-interactive/50" />
                    <p v-if="form.errors.email" class="text-error text-xs mt-1">{{ form.errors.email }}</p>
                </div>
                <div>
                    <input v-model="form.password" type="password" placeholder="Password"
                        class="w-full bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text placeholder-text-muted focus:outline-none focus:border-interactive/50" />
                    <p v-if="form.errors.password" class="text-error text-xs mt-1">{{ form.errors.password }}</p>
                </div>
                <div>
                    <input v-model="form.password_confirmation" type="password" placeholder="Confirm Password"
                        class="w-full bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text placeholder-text-muted focus:outline-none focus:border-interactive/50" />
                </div>
                <button type="submit" :disabled="form.processing"
                    class="w-full bg-interactive text-bg font-medium text-sm py-2.5 rounded hover:opacity-90 transition-opacity disabled:opacity-50">
                    Create Account
                </button>
            </form>

            <p class="text-text-muted text-xs text-center mt-6">
                Already have an account? <Link href="/login" class="text-text underline underline-offset-2">Sign in</Link>
            </p>
        </div>
    </div>
</template>
```

- [ ] **Step 7: Run tests**

```bash
docker compose exec app php artisan test --filter="LoginTest"
```
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add app/Http/Controllers/Auth/ resources/js/Pages/Auth/ routes/auth.php tests/
git commit -m "feat: add custom email/password authentication"
```

---

### Task 6: GitHub OAuth

**Files:**
- Create: `app/Http/Controllers/Auth/GitHubController.php`
- Create: `tests/Feature/Auth/GitHubAuthTest.php`

- [ ] **Step 1: Install Socialite**

```bash
composer require laravel/socialite
```

- [ ] **Step 2: Write failing test**

```php
// tests/Feature/Auth/GitHubAuthTest.php
<?php

use App\Models\User;
use Laravel\Socialite\Facades\Socialite;
use Laravel\Socialite\Two\User as SocialiteUser;

it('redirects to GitHub for authentication', function () {
    $this->get('/auth/github')->assertRedirect();
});

it('creates a new user from GitHub callback', function () {
    $githubUser = new SocialiteUser();
    $githubUser->id = '12345';
    $githubUser->name = 'GitHub User';
    $githubUser->email = 'github@example.com';
    $githubUser->avatar = 'https://example.com/avatar.jpg';
    $githubUser->token = 'github-token';

    Socialite::shouldReceive('driver->user')->andReturn($githubUser);

    $this->get('/auth/github/callback')->assertRedirect('/');

    $this->assertAuthenticated();
    $user = User::where('github_id', '12345')->first();
    expect($user)->not->toBeNull();
    expect($user->name)->toBe('GitHub User');
    expect($user->teams)->toHaveCount(1);
});

it('logs in existing user from GitHub callback', function () {
    $existing = User::factory()->create([
        'github_id' => '12345',
        'email' => 'github@example.com',
    ]);

    $githubUser = new SocialiteUser();
    $githubUser->id = '12345';
    $githubUser->name = 'GitHub User';
    $githubUser->email = 'github@example.com';
    $githubUser->avatar = 'https://example.com/avatar.jpg';
    $githubUser->token = 'new-github-token';

    Socialite::shouldReceive('driver->user')->andReturn($githubUser);

    $this->get('/auth/github/callback')->assertRedirect('/');

    $this->assertAuthenticatedAs($existing);
    expect($existing->fresh()->github_token)->not->toBe('github-token');
});
```

- [ ] **Step 3: Run test to verify it fails**

```bash
docker compose exec app php artisan test --filter="GitHubAuthTest"
```

- [ ] **Step 4: Implement GitHubController**

```php
// app/Http/Controllers/Auth/GitHubController.php
<?php

namespace App\Http\Controllers\Auth;

use App\Http\Controllers\Controller;
use App\Models\User;
use Illuminate\Support\Facades\Auth;
use Laravel\Socialite\Facades\Socialite;

class GitHubController extends Controller
{
    public function redirect()
    {
        return Socialite::driver('github')->redirect();
    }

    public function callback()
    {
        $githubUser = Socialite::driver('github')->user();

        $user = User::where('github_id', $githubUser->id)->first();

        if ($user) {
            $user->update([
                'github_token' => $githubUser->token,
                'avatar_url' => $githubUser->avatar,
            ]);
        } else {
            $user = User::where('email', $githubUser->email)->first();

            if ($user) {
                $user->update([
                    'github_id' => $githubUser->id,
                    'github_token' => $githubUser->token,
                    'avatar_url' => $githubUser->avatar,
                ]);
            } else {
                $user = User::create([
                    'name' => $githubUser->name ?? $githubUser->nickname,
                    'email' => $githubUser->email,
                    'github_id' => $githubUser->id,
                    'github_token' => $githubUser->token,
                    'avatar_url' => $githubUser->avatar,
                ]);
            }
        }

        Auth::login($user, remember: true);

        return redirect('/');
    }
}
```

- [ ] **Step 5: Add GitHub routes**

Add to `routes/auth.php` inside the guest middleware group:

```php
Route::get('/auth/github', [GitHubController::class, 'redirect'])->name('github.redirect');
Route::get('/auth/github/callback', [GitHubController::class, 'callback'])->name('github.callback');
```

Add the `use` statement:
```php
use App\Http\Controllers\Auth\GitHubController;
```

- [ ] **Step 6: Configure Socialite**

Add to `config/services.php`:

```php
'github' => [
    'client_id' => env('GITHUB_CLIENT_ID'),
    'client_secret' => env('GITHUB_CLIENT_SECRET'),
    'redirect' => env('GITHUB_REDIRECT_URI', '/auth/github/callback'),
],
```

- [ ] **Step 7: Run tests**

```bash
docker compose exec app php artisan test --filter="GitHubAuthTest"
```
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add app/Http/Controllers/Auth/GitHubController.php routes/auth.php config/services.php tests/
git commit -m "feat: add GitHub OAuth authentication"
```

---

### Task 7: TrackLastVisitedUrl Middleware

**Files:**
- Create: `app/Http/Middleware/TrackLastVisitedUrl.php`
- Create: `tests/Feature/DashboardTest.php`

- [ ] **Step 1: Write failing test**

```php
// tests/Feature/DashboardTest.php
<?php

use App\Models\User;

it('redirects to last visited URL on dashboard load', function () {
    $user = User::factory()->create([
        'last_visited_url' => '/devices',
    ]);

    $this->actingAs($user)->get('/')->assertRedirect('/devices');
});

it('shows dashboard when no last visited URL', function () {
    $user = User::factory()->create([
        'last_visited_url' => null,
    ]);

    $this->actingAs($user)->get('/')->assertOk()->assertInertia(fn ($page) =>
        $page->component('Dashboard')
    );
});

it('tracks the last visited URL', function () {
    $user = User::factory()->create();

    $this->actingAs($user)->get('/settings');

    expect($user->fresh()->last_visited_url)->toBe('/settings');
});
```

- [ ] **Step 2: Implement middleware**

```php
// app/Http/Middleware/TrackLastVisitedUrl.php
<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;

class TrackLastVisitedUrl
{
    private array $excluded = ['', 'login', 'register', 'logout', 'activate'];

    public function handle(Request $request, Closure $next)
    {
        $response = $next($request);

        if ($request->user()
            && $request->isMethod('GET')
            && ! $request->ajax()
            && ! in_array($request->path(), $this->excluded)
            && $response->isSuccessful()
        ) {
            $request->user()->update([
                'last_visited_url' => '/' . ltrim($request->path(), '/'),
            ]);
        }

        return $response;
    }
}
```

- [ ] **Step 3: Register middleware**

Add to `bootstrap/app.php` in the web middleware group:

```php
->withMiddleware(function (Middleware $middleware) {
    $middleware->web(append: [
        \App\Http\Middleware\TrackLastVisitedUrl::class,
    ]);
})
```

- [ ] **Step 4: Run tests**

```bash
docker compose exec app php artisan test --filter="DashboardTest"
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add app/Http/Middleware/ tests/Feature/DashboardTest.php bootstrap/app.php
git commit -m "feat: add TrackLastVisitedUrl middleware for resume-context"
```

---

### Task 8: Team Management & Authorization

**Files:**
- Create: `app/Services/TeamService.php`
- Create: `app/Policies/TeamPolicy.php`
- Create: `app/Http/Middleware/EnsureTeamAccess.php`
- Create: `app/Http/Controllers/Settings/TeamController.php`
- Create: `resources/js/Pages/Settings/Team.vue`
- Create: `tests/Unit/Services/TeamServiceTest.php`
- Create: `tests/Feature/TeamTest.php`

- [ ] **Step 1: Write failing unit test for TeamService**

```php
// tests/Unit/Services/TeamServiceTest.php
<?php

use App\Models\User;
use App\Models\Team;
use App\Models\TeamInvitation;
use App\Services\TeamService;

it('creates an invitation', function () {
    $service = new TeamService();
    $user = User::factory()->create();
    $team = $user->currentTeam();

    $invitation = $service->invite($team, 'invitee@example.com');

    expect($invitation)->toBeInstanceOf(TeamInvitation::class);
    expect($invitation->email)->toBe('invitee@example.com');
    expect($invitation->token)->toHaveLength(64);
    expect($invitation->isPending())->toBeTrue();
});

it('accepts an invitation', function () {
    $service = new TeamService();
    $owner = User::factory()->create();
    $team = $owner->currentTeam;
    $invitation = $service->invite($team, 'invitee@example.com');

    $invitee = User::factory()->create(['email' => 'invitee@example.com']);
    $service->acceptInvitation($invitation, $invitee);

    expect($invitee->isMemberOf($team))->toBeTrue();
    expect($invitation->fresh()->isPending())->toBeFalse();
});

it('removes a member', function () {
    $service = new TeamService();
    $owner = User::factory()->create();
    $team = $owner->currentTeam;
    $member = User::factory()->create();
    $team->users()->attach($member->id, ['role' => 'member']);

    $service->removeMember($team, $member);

    expect($member->isMemberOf($team))->toBeFalse();
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
docker compose exec app php artisan test --filter="TeamServiceTest"
```

- [ ] **Step 3: Implement TeamService**

```php
// app/Services/TeamService.php
<?php

namespace App\Services;

use App\Models\Team;
use App\Models\TeamInvitation;
use App\Models\User;
use Illuminate\Support\Str;

class TeamService
{
    public function invite(Team $team, string $email): TeamInvitation
    {
        return $team->invitations()->create([
            'email' => $email,
            'token' => Str::random(64),
        ]);
    }

    public function acceptInvitation(TeamInvitation $invitation, User $user): void
    {
        $invitation->team->users()->attach($user->id, ['role' => 'member']);
        $invitation->update(['accepted_at' => now()]);
    }

    public function removeMember(Team $team, User $user): void
    {
        $team->users()->detach($user->id);
    }
}
```

- [ ] **Step 4: Run unit tests**

```bash
docker compose exec app php artisan test --filter="TeamServiceTest"
```
Expected: all PASS

- [ ] **Step 5: Write feature test for team management**

```php
// tests/Feature/TeamTest.php
<?php

use App\Models\User;

it('shows team settings page', function () {
    $user = User::factory()->create();

    $this->actingAs($user)->get('/settings/team')->assertOk()->assertInertia(fn ($page) =>
        $page->component('Settings/Team')
            ->has('team')
            ->has('members')
    );
});

it('allows owner to invite a member', function () {
    $user = User::factory()->create();

    $this->actingAs($user)->post('/settings/team/invite', [
        'email' => 'newmember@example.com',
    ])->assertRedirect();

    $this->assertDatabaseHas('team_invitations', [
        'email' => 'newmember@example.com',
    ]);
});

it('prevents member from managing the team they do not own', function () {
    $owner = User::factory()->create();
    $team = $owner->currentTeam();

    // Create member without their own default team for this test
    $member = User::create([
        'name' => 'Member',
        'email' => 'member@example.com',
        'password' => bcrypt('password'),
    ]);
    $team->users()->attach($member->id, ['role' => 'member']);

    // Member's currentTeam() returns the shared team (their only team)
    $this->actingAs($member)->post('/settings/team/invite', [
        'email' => 'another@example.com',
    ])->assertForbidden();
});
```

- [ ] **Step 6: Implement TeamPolicy**

```php
// app/Policies/TeamPolicy.php
<?php

namespace App\Policies;

use App\Models\Team;
use App\Models\User;

class TeamPolicy
{
    public function manage(User $user, Team $team): bool
    {
        return $user->isOwnerOf($team);
    }

    public function view(User $user, Team $team): bool
    {
        return $user->isMemberOf($team);
    }
}
```

Register in `app/Providers/AppServiceProvider.php`:
```php
use Illuminate\Support\Facades\Gate;
use App\Models\Team;
use App\Policies\TeamPolicy;

public function boot(): void
{
    Gate::policy(Team::class, TeamPolicy::class);
}
```

- [ ] **Step 7: Implement TeamController**

```php
// app/Http/Controllers/Settings/TeamController.php
<?php

namespace App\Http\Controllers\Settings;

use App\Http\Controllers\Controller;
use App\Services\TeamService;
use Illuminate\Http\Request;
use Inertia\Inertia;

class TeamController extends Controller
{
    public function __construct(private TeamService $teamService) {}

    public function show(Request $request)
    {
        $team = $request->user()->currentTeam;

        return Inertia::render('Settings/Team', [
            'team' => $team,
            'members' => $team->users->map(fn ($user) => [
                'id' => $user->id,
                'name' => $user->name,
                'email' => $user->email,
                'avatar_url' => $user->avatar_url,
                'role' => $user->pivot->role,
            ]),
            'invitations' => $team->invitations()->whereNull('accepted_at')->get(),
        ]);
    }

    public function invite(Request $request)
    {
        $team = $request->user()->currentTeam;
        $this->authorize('manage', $team);

        $request->validate(['email' => ['required', 'email']]);

        $this->teamService->invite($team, $request->email);

        return back();
    }

    public function removeMember(Request $request, int $userId)
    {
        $team = $request->user()->currentTeam;
        $this->authorize('manage', $team);

        $member = $team->users()->findOrFail($userId);
        $this->teamService->removeMember($team, $member);

        return back();
    }
}
```

- [ ] **Step 8: Add routes**

Add to `routes/web.php` inside the auth middleware group:

```php
use App\Http\Controllers\Settings\TeamController;

Route::get('/settings/team', [TeamController::class, 'show'])->name('settings.team');
Route::post('/settings/team/invite', [TeamController::class, 'invite'])->name('settings.team.invite');
Route::delete('/settings/team/members/{userId}', [TeamController::class, 'removeMember'])->name('settings.team.remove');
```

- [ ] **Step 9: Create Team.vue page**

```vue
<!-- resources/js/Pages/Settings/Team.vue -->
<script setup>
import { useForm } from '@inertiajs/vue3';

const props = defineProps({
    team: Object,
    members: Array,
    invitations: Array,
});

const inviteForm = useForm({ email: '' });

function invite() {
    inviteForm.post('/settings/team/invite', {
        onSuccess: () => inviteForm.reset(),
    });
}
</script>

<template>
    <div class="p-4 md:p-8 max-w-2xl">
        <h1 class="text-lg font-semibold mb-6">Team Settings</h1>

        <!-- Team Name -->
        <div class="mb-8">
            <h2 class="text-sm font-medium text-text-secondary mb-2">Team</h2>
            <p class="text-text-heading">{{ team.name }}</p>
        </div>

        <!-- Members -->
        <div class="mb-8">
            <h2 class="text-sm font-medium text-text-secondary mb-3">Members</h2>
            <div class="space-y-2">
                <div v-for="member in members" :key="member.id"
                    class="flex items-center justify-between bg-bg-card border border-border rounded px-3 py-2">
                    <div>
                        <span class="text-sm text-text-heading">{{ member.name }}</span>
                        <span class="text-xs text-text-muted ml-2">{{ member.email }}</span>
                    </div>
                    <span class="text-xs font-mono text-text-secondary">{{ member.role }}</span>
                </div>
            </div>
        </div>

        <!-- Invite -->
        <div>
            <h2 class="text-sm font-medium text-text-secondary mb-3">Invite Member</h2>
            <form @submit.prevent="invite" class="flex gap-2">
                <input v-model="inviteForm.email" type="email" placeholder="email@example.com"
                    class="flex-1 bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text placeholder-text-muted focus:outline-none focus:border-interactive/50" />
                <button type="submit" :disabled="inviteForm.processing"
                    class="bg-interactive text-bg font-medium text-sm px-4 py-2 rounded hover:opacity-90 transition-opacity disabled:opacity-50">
                    Invite
                </button>
            </form>
            <p v-if="inviteForm.errors.email" class="text-error text-xs mt-1">{{ inviteForm.errors.email }}</p>
        </div>

        <!-- Pending Invitations -->
        <div v-if="invitations.length > 0" class="mt-6">
            <h2 class="text-sm font-medium text-text-secondary mb-3">Pending Invitations</h2>
            <div v-for="inv in invitations" :key="inv.id"
                class="flex items-center justify-between bg-bg-card border border-border rounded px-3 py-2 mb-2">
                <span class="text-sm text-text-secondary">{{ inv.email }}</span>
                <span class="text-xs font-mono text-text-muted">pending</span>
            </div>
        </div>
    </div>
</template>
```

- [ ] **Step 10: Run all tests**

```bash
docker compose exec app php artisan test --filter="TeamTest|TeamServiceTest"
```
Expected: all PASS

- [ ] **Step 11: Commit**

```bash
git add app/Services/ app/Policies/ app/Http/Controllers/Settings/ resources/js/Pages/Settings/ routes/web.php tests/ app/Providers/
git commit -m "feat: add team management with Owner/Member roles and invitations"
```

---

### Task 9: Settings Page with Theme Preference

**Files:**
- Create: `app/Http/Controllers/Settings/ProfileController.php`
- Create: `resources/js/Pages/Settings/Profile.vue`

- [ ] **Step 1: Write failing test**

```php
// Add to tests/Feature/DashboardTest.php or create new test file
it('updates theme preference', function () {
    $user = User::factory()->create(['theme_preference' => 'system']);

    $this->actingAs($user)->put('/settings/profile', [
        'theme_preference' => 'dark',
    ])->assertRedirect();

    expect($user->fresh()->theme_preference)->toBe('dark');
});
```

- [ ] **Step 2: Implement ProfileController**

```php
// app/Http/Controllers/Settings/ProfileController.php
<?php

namespace App\Http\Controllers\Settings;

use App\Http\Controllers\Controller;
use Illuminate\Http\Request;
use Illuminate\Validation\Rule;
use Inertia\Inertia;

class ProfileController extends Controller
{
    public function show(Request $request)
    {
        return Inertia::render('Settings/Profile', [
            'user' => $request->user()->only('name', 'email', 'avatar_url', 'theme_preference'),
        ]);
    }

    public function update(Request $request)
    {
        $validated = $request->validate([
            'name' => ['sometimes', 'string', 'max:255'],
            'theme_preference' => ['sometimes', Rule::in(['dark', 'light', 'system'])],
        ]);

        $request->user()->update($validated);

        return back();
    }
}
```

- [ ] **Step 3: Add routes**

Add to `routes/web.php` inside the auth middleware group:

```php
use App\Http\Controllers\Settings\ProfileController;

Route::get('/settings', [ProfileController::class, 'show'])->name('settings.profile');
Route::put('/settings/profile', [ProfileController::class, 'update'])->name('settings.profile.update');
```

- [ ] **Step 4: Create Profile.vue**

```vue
<!-- resources/js/Pages/Settings/Profile.vue -->
<script setup>
import { useForm } from '@inertiajs/vue3';

const props = defineProps({ user: Object });

const form = useForm({
    name: props.user.name,
    theme_preference: props.user.theme_preference,
});

function save() {
    form.put('/settings/profile');
}

function setTheme(theme) {
    form.theme_preference = theme;
    document.documentElement.setAttribute('data-theme', theme === 'system' ? '' : theme);
    save();
}
</script>

<template>
    <div class="p-4 md:p-8 max-w-2xl">
        <h1 class="text-lg font-semibold mb-6">Settings</h1>

        <!-- Name -->
        <div class="mb-8">
            <label class="text-sm font-medium text-text-secondary block mb-2">Name</label>
            <input v-model="form.name" type="text" @blur="save"
                class="w-full max-w-xs bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text focus:outline-none focus:border-interactive/50" />
        </div>

        <!-- Theme -->
        <div class="mb-8">
            <label class="text-sm font-medium text-text-secondary block mb-3">Theme</label>
            <div class="flex gap-2">
                <button v-for="theme in ['system', 'dark', 'light']" :key="theme"
                    @click="setTheme(theme)"
                    class="px-4 py-2 rounded text-sm border transition-colors"
                    :class="form.theme_preference === theme
                        ? 'bg-bg-surface border-interactive/50 text-text-heading'
                        : 'border-border text-text-muted hover:text-text'">
                    {{ theme.charAt(0).toUpperCase() + theme.slice(1) }}
                </button>
            </div>
        </div>
    </div>
</template>
```

- [ ] **Step 5: Run tests**

```bash
docker compose exec app php artisan test --filter="updates theme preference"
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/Http/Controllers/Settings/ProfileController.php resources/js/Pages/Settings/Profile.vue routes/web.php tests/
git commit -m "feat: add settings page with theme preference (dark/light/system)"
```

---

### Task 10: Device Code Approval Page

**Files:**
- Create: `app/Http/Controllers/Auth/DeviceApprovalController.php`
- Create: `resources/js/Pages/Auth/ActivateDevice.vue`
- Create: `tests/Feature/Auth/DeviceApprovalTest.php`

This is the `/activate` page where users approve `chief login` requests.

- [ ] **Step 1: Create device_codes migration**

```php
// database/migrations/xxxx_create_device_codes_table.php
Schema::create('device_codes', function (Blueprint $table) {
    $table->id();
    $table->string('device_code', 64)->unique();
    $table->string('user_code', 10)->unique();
    $table->foreignId('user_id')->nullable()->constrained()->nullOnDelete();
    $table->foreignId('team_id')->nullable()->constrained()->nullOnDelete();
    $table->timestamp('expires_at');
    $table->timestamp('approved_at')->nullable();
    $table->timestamps();
});
```

- [ ] **Step 2: Write failing test**

```php
// tests/Feature/Auth/DeviceApprovalTest.php
<?php

use App\Models\User;
use Illuminate\Support\Facades\DB;

it('shows the device activation page', function () {
    $user = User::factory()->create();

    $this->actingAs($user)->get('/activate')->assertOk()->assertInertia(fn ($page) =>
        $page->component('Auth/ActivateDevice')
    );
});

it('approves a device code', function () {
    $user = User::factory()->create();
    $team = $user->currentTeam();

    DB::table('device_codes')->insert([
        'device_code' => 'dev_code_123',
        'user_code' => 'ABCD-1234',
        'expires_at' => now()->addMinutes(15),
        'created_at' => now(),
        'updated_at' => now(),
    ]);

    $this->actingAs($user)->post('/activate', [
        'user_code' => 'ABCD-1234',
        'team_id' => $team->id,
    ])->assertRedirect();

    $code = DB::table('device_codes')->where('user_code', 'ABCD-1234')->first();
    expect($code->approved_at)->not->toBeNull();
    expect($code->user_id)->toBe($user->id);
    expect($code->team_id)->toBe($team->id);
});

it('rejects expired device code', function () {
    $user = User::factory()->create();

    DB::table('device_codes')->insert([
        'device_code' => 'dev_code_expired',
        'user_code' => 'EXPD-0000',
        'expires_at' => now()->subMinutes(1),
        'created_at' => now(),
        'updated_at' => now(),
    ]);

    $this->actingAs($user)->post('/activate', [
        'user_code' => 'EXPD-0000',
    ])->assertSessionHasErrors('user_code');
});
```

- [ ] **Step 3: Implement DeviceApprovalController**

```php
// app/Http/Controllers/Auth/DeviceApprovalController.php
<?php

namespace App\Http\Controllers\Auth;

use App\Http\Controllers\Controller;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Inertia\Inertia;

class DeviceApprovalController extends Controller
{
    public function show(Request $request)
    {
        $teams = $request->user()->teams->map(fn ($team) => [
            'id' => $team->id,
            'name' => $team->name,
        ]);

        return Inertia::render('Auth/ActivateDevice', [
            'teams' => $teams,
        ]);
    }

    public function store(Request $request)
    {
        $request->validate([
            'user_code' => ['required', 'string'],
            'team_id' => ['nullable', 'exists:teams,id'],
        ]);

        $code = DB::table('device_codes')
            ->where('user_code', strtoupper($request->user_code))
            ->whereNull('approved_at')
            ->first();

        if (! $code) {
            return back()->withErrors(['user_code' => 'Invalid or already used code.']);
        }

        if (now()->isAfter($code->expires_at)) {
            return back()->withErrors(['user_code' => 'This code has expired. Run chief login again.']);
        }

        $team = $request->team_id
            ? $request->user()->teams()->findOrFail($request->team_id)
            : $request->user()->currentTeam;

        DB::table('device_codes')
            ->where('id', $code->id)
            ->update([
                'user_id' => $request->user()->id,
                'team_id' => $team->id,
                'approved_at' => now(),
                'updated_at' => now(),
            ]);

        return back()->with('success', 'Device approved and connected to ' . $team->name);
    }
}
```

- [ ] **Step 4: Add routes**

Add to `routes/web.php` inside auth middleware group:

```php
use App\Http\Controllers\Auth\DeviceApprovalController;

Route::get('/activate', [DeviceApprovalController::class, 'show'])->name('device.activate');
Route::post('/activate', [DeviceApprovalController::class, 'store']);
```

- [ ] **Step 5: Create ActivateDevice.vue**

```vue
<!-- resources/js/Pages/Auth/ActivateDevice.vue -->
<script setup>
import { useForm } from '@inertiajs/vue3';

const props = defineProps({
    teams: Array,
});

const form = useForm({
    user_code: '',
    team_id: props.teams.length === 1 ? props.teams[0].id : null,
});

function submit() {
    form.post('/activate');
}
</script>

<template>
    <div class="p-4 md:p-8 max-w-md mx-auto mt-12">
        <h1 class="text-xl font-semibold text-text-heading mb-2 text-center">Activate Device</h1>
        <p class="text-text-secondary text-sm mb-8 text-center">
            Enter the code shown in your terminal after running <code class="font-mono text-xs bg-bg-surface px-1.5 py-0.5 rounded">chief login</code>
        </p>

        <form @submit.prevent="submit" class="space-y-4">
            <div>
                <input v-model="form.user_code" type="text" placeholder="ABCD-1234"
                    class="w-full bg-bg-surface border border-border rounded px-3 py-3 text-center text-lg font-mono text-text-heading tracking-widest placeholder-text-muted focus:outline-none focus:border-interactive/50"
                    maxlength="9" />
                <p v-if="form.errors.user_code" class="text-error text-xs mt-1">{{ form.errors.user_code }}</p>
            </div>

            <div v-if="teams.length > 1">
                <label class="text-sm text-text-secondary block mb-1">Add to team</label>
                <select v-model="form.team_id"
                    class="w-full bg-bg-surface border border-border rounded px-3 py-2 text-sm text-text focus:outline-none focus:border-interactive/50">
                    <option v-for="team in teams" :key="team.id" :value="team.id">{{ team.name }}</option>
                </select>
            </div>

            <button type="submit" :disabled="form.processing"
                class="w-full bg-interactive text-bg font-medium text-sm py-2.5 rounded hover:opacity-90 transition-opacity disabled:opacity-50">
                Approve Device
            </button>
        </form>
    </div>
</template>
```

- [ ] **Step 6: Run tests**

```bash
docker compose exec app php artisan test --filter="DeviceApprovalTest"
```
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add app/Http/Controllers/Auth/DeviceApprovalController.php resources/js/Pages/Auth/ActivateDevice.vue database/migrations/ routes/web.php tests/
git commit -m "feat: add device code approval page with team selector"
```

---

### Task 11: Code Style & CI Setup

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `pint.json`

- [ ] **Step 1: Configure Laravel Pint**

```json
// pint.json
{
    "preset": "laravel"
}
```

- [ ] **Step 2: Run Pint to fix style**

```bash
docker compose exec app ./vendor/bin/pint
```

- [ ] **Step 3: Create GitHub Actions CI**

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  tests:
    runs-on: ubuntu-latest

    services:
      mariadb:
        image: mariadb:11
        env:
          MYSQL_DATABASE: uplink_test
          MYSQL_USER: uplink
          MYSQL_PASSWORD: secret
          MYSQL_ROOT_PASSWORD: secret
        ports:
          - 3306:3306
        options: >-
          --health-cmd="healthcheck.sh --connect --innodb_initialized"
          --health-interval=10s
          --health-timeout=5s
          --health-retries=5

    steps:
      - uses: actions/checkout@v4

      - name: Setup PHP
        uses: shivammathur/setup-php@v2
        with:
          php-version: '8.4'
          extensions: pdo_mysql, redis, pcntl, zip, bcmath
          coverage: none

      - name: Install Composer dependencies
        run: composer install --no-progress --prefer-dist

      - name: Install npm dependencies
        run: npm ci

      - name: Build assets
        run: npm run build

      - name: Copy env
        run: cp .env.example .env

      - name: Generate key
        run: php artisan key:generate

      - name: Run migrations
        run: php artisan migrate --force
        env:
          DB_HOST: 127.0.0.1
          DB_DATABASE: uplink_test
          DB_USERNAME: uplink
          DB_PASSWORD: secret

      - name: Run Pint (code style)
        run: ./vendor/bin/pint --test

      - name: Run Pest tests
        run: php artisan test
        env:
          DB_HOST: 127.0.0.1
          DB_DATABASE: uplink_test
          DB_USERNAME: uplink
          DB_PASSWORD: secret

  js-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npm ci
      - run: npm run build
```

- [ ] **Step 4: Commit**

```bash
git add .github/ pint.json
git commit -m "feat: add GitHub Actions CI and Laravel Pint config"
```

---

### Task 12: Run Full Test Suite & Final Verification

- [ ] **Step 1: Run all tests**

```bash
docker compose exec app php artisan test
```
Expected: all tests pass

- [ ] **Step 2: Run Pint**

```bash
docker compose exec app ./vendor/bin/pint --test
```
Expected: no style issues

- [ ] **Step 3: Verify Docker Compose works end-to-end**

```bash
docker compose down -v
docker compose up -d
docker compose exec app php artisan key:generate
docker compose exec app php artisan migrate
npm run dev
# Visit http://localhost:8000 — should see login page
# Register a new user — should see dashboard
# Visit /settings/team — should see team management
# Visit /activate — should see device activation page
```

- [ ] **Step 4: Commit any final fixes**

```bash
git add -A
git commit -m "chore: final verification and cleanup"
```
