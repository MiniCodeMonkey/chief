<script setup lang="ts">
import { ref } from 'vue'

const copied = ref(false)
const installCommand = 'brew install minicodemonkey/chief/chief'

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
</script>

<template>
  <section class="hero-section">
    <div class="hero-container">
      <!-- Left side: Text content -->
      <div class="hero-content">
        <h1 class="hero-headline">
          <span class="hero-title-gradient">Build Big Projects with Claude</span>
        </h1>
        <p class="hero-subheadline">
          Chief breaks your work into tasks. Claude builds them one by one.
        </p>

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
          <a href="/guide/" class="btn-primary">Get Started</a>
          <a href="https://github.com/minicodemonkey/chief" target="_blank" rel="noopener" class="btn-secondary">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
            </svg>
            View on GitHub
          </a>
        </div>
      </div>

      <!-- Right side: Animated terminal -->
      <div class="hero-terminal">
        <div class="terminal-window">
          <div class="terminal-header">
            <div class="terminal-buttons">
              <span class="terminal-btn terminal-btn-red"></span>
              <span class="terminal-btn terminal-btn-yellow"></span>
              <span class="terminal-btn terminal-btn-green"></span>
            </div>
            <span class="terminal-title">Terminal</span>
          </div>
          <div class="terminal-body">
            <div class="terminal-line line-1">
              <span class="terminal-prompt">$</span>
              <span class="terminal-command typing-1">brew install minicodemonkey/chief/chief</span>
            </div>
            <div class="terminal-line line-2">
              <span class="terminal-output output-1">==> Fetching minicodemonkey/chief/chief</span>
            </div>
            <div class="terminal-line line-3">
              <span class="terminal-output output-2">==> Downloading chief-darwin-arm64.tar.gz</span>
            </div>
            <div class="terminal-line line-4">
              <span class="terminal-output output-3">==> Installing chief</span>
            </div>
            <div class="terminal-line line-5">
              <span class="terminal-output output-4 text-tokyo-green">==> Installation successful!</span>
            </div>
            <div class="terminal-line line-6">
              <span class="terminal-prompt">$</span>
              <span class="terminal-command typing-2">chief new</span>
            </div>
            <div class="terminal-line line-7">
              <span class="terminal-output output-5">Creating project in .chief/prds/main...</span>
            </div>
            <div class="terminal-line line-8">
              <span class="terminal-output output-6 text-tokyo-green">Launching Claude to help you get started...</span>
            </div>
          </div>
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
  padding: 4rem 1.5rem;
  background: linear-gradient(180deg, #1a1b26 0%, #16161e 100%);
}

.hero-container {
  max-width: 1280px;
  width: 100%;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4rem;
  align-items: center;
}

.hero-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
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

.hero-subheadline code {
  background-color: #292e42;
  color: #bb9af7;
  padding: 0.125rem 0.375rem;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Monaco, Consolas, monospace;
}

.install-command {
  display: flex;
  align-items: center;
  gap: 0;
  background-color: #16161e;
  border: 1px solid #292e42;
  border-radius: 8px;
  padding: 0.75rem 1rem;
  max-width: fit-content;
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
  margin-top: 0.5rem;
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

/* Terminal styles */
.hero-terminal {
  display: flex;
  justify-content: center;
}

.terminal-window {
  width: 100%;
  max-width: 520px;
  background-color: #16161e;
  border: 1px solid #292e42;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
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
  margin-right: 44px; /* offset for buttons */
}

.terminal-body {
  padding: 1.25rem;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Monaco, Consolas, monospace;
  font-size: 0.85rem;
  line-height: 1.8;
  min-height: 280px;
}

.terminal-line {
  display: flex;
  gap: 0.5rem;
  opacity: 0;
  animation: fadeIn 0.3s forwards;
}

.terminal-prompt {
  color: #9ece6a;
}

.terminal-command {
  color: #a9b1d6;
}

.terminal-output {
  color: #565f89;
}

.text-tokyo-green {
  color: #9ece6a;
}

.text-tokyo-accent {
  color: #7aa2f7;
}

.text-tokyo-purple {
  color: #bb9af7;
}

/* Animation delays for each line */
.line-1 { animation-delay: 0.5s; }
.line-2 { animation-delay: 1.6s; }
.line-3 { animation-delay: 2.2s; }
.line-4 { animation-delay: 2.8s; }
.line-5 { animation-delay: 3.4s; }
.line-6 { animation-delay: 4.0s; }
.line-7 { animation-delay: 4.8s; }
.line-8 { animation-delay: 5.4s; }

/* Animation loop: restart after all lines complete */
.terminal-body {
  animation: resetTerminal 8s infinite;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes resetTerminal {
  0%, 95% {
    opacity: 1;
  }
  98% {
    opacity: 0;
  }
  100% {
    opacity: 1;
  }
}

/* Typing effect for commands */
.typing-1, .typing-2 {
  overflow: hidden;
  white-space: nowrap;
  width: 0;
  animation: typing 0.8s steps(35) forwards;
}

.typing-1 {
  animation-delay: 0.6s;
}

.typing-2 {
  animation-delay: 4.2s;
  animation-duration: 0.3s;
  animation-timing-function: steps(10);
}

@keyframes typing {
  from { width: 0; }
  to { width: 100%; }
}

/* Blinking cursor */
.terminal-cursor {
  color: #7aa2f7;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

/* Responsive design */
@media (max-width: 1024px) {
  .hero-container {
    grid-template-columns: 1fr;
    gap: 3rem;
  }

  .hero-content {
    text-align: center;
    align-items: center;
  }

  .hero-headline {
    font-size: 2.75rem;
  }

  .hero-actions {
    justify-content: center;
  }

  .hero-terminal {
    order: -1;
  }

  .terminal-window {
    max-width: 100%;
  }
}

@media (max-width: 640px) {
  .hero-section {
    padding: 2rem 1rem;
    min-height: auto;
  }

  .hero-headline {
    font-size: 2rem;
  }

  .hero-subheadline {
    font-size: 1.1rem;
  }

  .install-command {
    flex-wrap: wrap;
    justify-content: center;
    width: 100%;
  }

  .install-code {
    font-size: 0.8rem;
    word-break: break-all;
  }

  .hero-actions {
    flex-direction: column;
    width: 100%;
  }

  .btn-primary,
  .btn-secondary {
    width: 100%;
    justify-content: center;
    /* Touch target: minimum 44px height */
    min-height: 44px;
  }

  .terminal-body {
    font-size: 0.75rem;
    padding: 1rem;
    min-height: 240px;
  }

  /* Copy button touch target */
  .copy-button {
    min-width: 44px;
    min-height: 44px;
  }
}

/* Very small screens (375px) */
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

  .terminal-body {
    font-size: 0.65rem;
    padding: 0.75rem;
    min-height: 200px;
  }

  .terminal-window {
    border-radius: 8px;
  }

  .terminal-header {
    padding: 0.5rem 0.75rem;
  }
}
</style>
