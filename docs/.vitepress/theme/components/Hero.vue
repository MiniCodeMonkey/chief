<script setup lang="ts">
import { ref, onMounted } from 'vue'

const copied = ref(false)
const installCommand = 'brew install minicodemonkey/chief/chief'
const playerRef = ref<HTMLDivElement | null>(null)

async function copyInstallCommand() {
  try {
    await navigator.clipboard.writeText(installCommand)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy:', err)
  }
}

onMounted(async () => {
  if (!playerRef.value) return
  const AsciinemaPlayer = await import('asciinema-player')
  AsciinemaPlayer.create('/demo.cast', playerRef.value, {
    autoPlay: true,
    loop: true,
    speed: 1.2,
    idleTimeLimit: 2,
    fit: 'width',
    terminalFontFamily: "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Monaco, Consolas, monospace",
    controls: false,
  })
})
</script>

<template>
  <section class="hero-section">
    <div class="hero-container">
      <!-- Top: Text content centered -->
      <div class="hero-content">
        <h1 class="hero-headline">
          <span class="hero-title-gradient">Build Big Projects, Autonomously</span>
        </h1>
        <p class="hero-subheadline">
          Chief breaks your work into tasks and builds them one by one.
        </p>

        <div class="hero-row">
          <!-- Install command with copy button -->
          <div class="install-command">
            <code class="install-code">{{ installCommand }}</code>
            <button
              class="copy-button"
              @click="copyInstallCommand"
              :title="copied ? 'Copied!' : 'Copy to clipboard'"
            >
              <svg v-if="!copied" xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12"></polyline>
              </svg>
            </button>
          </div>

          <!-- CTA buttons -->
          <div class="hero-actions">
            <a href="/guide/quick-start" class="btn-primary">Get Started</a>
            <a href="https://github.com/minicodemonkey/chief" target="_blank" rel="noopener" class="btn-secondary">
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
              </svg>
              View on GitHub
            </a>
          </div>
        </div>
      </div>

      <!-- Bottom: Full-width terminal -->
      <div class="hero-terminal">
        <div class="terminal-window">
          <div class="terminal-header">
            <div class="terminal-buttons">
              <span class="terminal-btn terminal-btn-red"></span>
              <span class="terminal-btn terminal-btn-yellow"></span>
              <span class="terminal-btn terminal-btn-green"></span>
            </div>
            <span class="terminal-title">chief</span>
          </div>
          <div class="terminal-player" ref="playerRef"></div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero-section {
  min-height: calc(100vh - 64px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 3rem 1.5rem;
  background: linear-gradient(180deg, #1a1b26 0%, #16161e 100%);
}

.hero-container {
  max-width: 1100px;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 2.5rem;
  align-items: center;
}

.hero-content {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  text-align: center;
  align-items: center;
}

.hero-headline {
  font-size: 3.5rem;
  font-weight: 800;
  line-height: 1.1;
  margin: 0;
}

.hero-title-gradient {
  background: linear-gradient(135deg, #7aa2f7 0%, #bb9af7 50%, #7dcfff 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.hero-subheadline {
  font-size: 1.25rem;
  color: #9aa5ce;
  line-height: 1.6;
  margin: 0;
}

.hero-row {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  flex-wrap: wrap;
  justify-content: center;
  margin-top: 0.5rem;
}

.install-command {
  display: flex;
  align-items: center;
  gap: 0;
  background-color: #16161e;
  border: 1px solid #292e42;
  border-radius: 8px;
  padding: 0.75rem 1rem;
}

.install-code {
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Monaco, Consolas, monospace;
  font-size: 0.9rem;
  color: #a9b1d6;
  background: none;
  padding: 0;
}

.copy-button {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.375rem;
  margin-left: 0.75rem;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  color: #565f89;
  transition: color 0.2s, background-color 0.2s;
}

.copy-button:hover {
  color: #7aa2f7;
  background-color: rgba(122, 162, 247, 0.1);
}

.copy-button svg {
  width: 18px;
  height: 18px;
}

.hero-actions {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}

.btn-primary {
  display: inline-flex;
  align-items: center;
  padding: 0.75rem 1.5rem;
  font-size: 1rem;
  font-weight: 600;
  color: #1a1b26;
  background-color: #7aa2f7;
  border-radius: 8px;
  text-decoration: none;
  transition: background-color 0.2s;
}

.btn-primary:hover {
  background-color: #89b4fa;
}

.btn-secondary {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.5rem;
  font-size: 1rem;
  font-weight: 600;
  color: #a9b1d6;
  background-color: #292e42;
  border: 1px solid #3b4261;
  border-radius: 8px;
  text-decoration: none;
  transition: background-color 0.2s, border-color 0.2s;
}

.btn-secondary:hover {
  background-color: #3b4261;
  border-color: #565f89;
}

.btn-secondary svg {
  width: 20px;
  height: 20px;
}

/* Terminal chrome */
.hero-terminal {
  width: 100%;
}

.terminal-window {
  width: 100%;
  background-color: #16161e;
  border: 1px solid #292e42;
  border-radius: 12px;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

/* Clip only the header for rounded corners, let player content flow naturally */
.terminal-header {
  border-radius: 12px 12px 0 0;
}

.terminal-header {
  display: flex;
  align-items: center;
  padding: 0.75rem 1rem;
  background-color: #1f2335;
  border-bottom: 1px solid #292e42;
}

.terminal-buttons {
  display: flex;
  gap: 0.5rem;
}

.terminal-btn {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.terminal-btn-red {
  background-color: #f7768e;
}

.terminal-btn-yellow {
  background-color: #e0af68;
}

.terminal-btn-green {
  background-color: #9ece6a;
}

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 0.8rem;
  color: #565f89;
  margin-right: 44px;
}

/* Asciinema player container */
.terminal-player {
  background-color: #16161e;
}

/* Override asciinema player styles */
.terminal-player :deep(.ap-wrapper) {
  background-color: #16161e !important;
}

.terminal-player :deep(.ap-player) {
  background-color: #16161e !important;
}

.terminal-player :deep(.ap-control-bar) {
  display: none !important;
}

.terminal-player :deep(.ap-start-button) {
  display: none !important;
}

.terminal-player :deep(.ap-terminal) {
  background-color: #16161e !important;
}

/* Responsive design */
@media (max-width: 768px) {
  .hero-section {
    padding: 2rem 1rem;
    min-height: auto;
  }

  .hero-headline {
    font-size: 2.25rem;
  }

  .hero-subheadline {
    font-size: 1.1rem;
  }

  .hero-row {
    flex-direction: column;
  }

  .install-command {
    width: 100%;
    justify-content: center;
  }

  .install-code {
    font-size: 0.8rem;
  }

  .hero-actions {
    flex-direction: column;
    width: 100%;
  }

  .btn-primary,
  .btn-secondary {
    width: 100%;
    justify-content: center;
    min-height: 44px;
  }

  .copy-button {
    min-width: 44px;
    min-height: 44px;
  }
}

@media (max-width: 420px) {
  .hero-section {
    padding: 1.5rem 0.75rem;
  }

  .hero-headline {
    font-size: 1.75rem;
  }

  .hero-subheadline {
    font-size: 1rem;
  }

  .install-code {
    font-size: 0.7rem;
  }

  .terminal-window {
    border-radius: 8px;
  }

  .terminal-header {
    padding: 0.5rem 0.75rem;
  }
}
</style>
