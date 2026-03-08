<script setup lang="ts">
const agents = [
  { name: 'Claude Code', description: 'By Anthropic' },
  { name: 'Codex CLI', description: 'By OpenAI' },
  { name: 'OpenCode', description: 'Open source' },
]
</script>

<template>
  <section class="agent-support">
    <div class="agent-support-container">
      <p class="agent-support-heading">Works with your favorite coding agent</p>
      <div class="agent-list">
        <template v-for="(agent, i) in agents" :key="agent.name">
          <div class="agent-item">
            <div class="agent-glow"></div>
            <div class="agent-icon-wrap">
              <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="4 17 10 11 4 5" />
                <line x1="12" y1="19" x2="20" y2="19" />
              </svg>
            </div>
            <span class="agent-name">{{ agent.name }}</span>
            <span class="agent-description">{{ agent.description }}</span>
          </div>
          <span v-if="i < agents.length - 1" class="agent-divider">/</span>
        </template>
      </div>
    </div>
  </section>
</template>

<style scoped>
.agent-support {
  padding: 5rem 1.5rem;
  background-color: #1a1b26;
}

.agent-support-container {
  max-width: 1280px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2.5rem;
}

.agent-support-heading {
  font-size: 0.85rem;
  font-weight: 500;
  color: #565f89;
  text-transform: uppercase;
  letter-spacing: 0.15em;
  margin: 0;
}

.agent-list {
  display: flex;
  align-items: center;
  gap: 1.25rem;
}

.agent-divider {
  color: #292e42;
  font-size: 1.25rem;
  font-weight: 300;
  user-select: none;
}

.agent-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  padding: 1.75rem 2.25rem;
  background: linear-gradient(135deg, rgba(22, 22, 30, 0.8), rgba(26, 27, 38, 0.6));
  border: 1px solid #292e42;
  border-radius: 14px;
  transition: border-color 0.3s, transform 0.3s, box-shadow 0.3s;
  backdrop-filter: blur(8px);
  overflow: hidden;
}

.agent-glow {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 50% 30%, rgba(122, 162, 247, 0.06), transparent 70%);
  opacity: 0;
  transition: opacity 0.3s;
  pointer-events: none;
}

.agent-item:hover {
  border-color: rgba(122, 162, 247, 0.4);
  transform: translateY(-3px);
  box-shadow: 0 8px 30px rgba(122, 162, 247, 0.1);
}

.agent-item:hover .agent-glow {
  opacity: 1;
}

.agent-icon-wrap {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: rgba(41, 46, 66, 0.4);
  border-radius: 12px;
  color: #565f89;
  transition: color 0.3s, background-color 0.3s;
}

.agent-item:hover .agent-icon-wrap {
  color: #7aa2f7;
  background-color: rgba(122, 162, 247, 0.1);
}

.agent-name {
  font-size: 1.05rem;
  font-weight: 600;
  color: #a9b1d6;
  white-space: nowrap;
}

.agent-description {
  font-size: 0.8rem;
  color: #565f89;
  white-space: nowrap;
}

/* Responsive */
@media (max-width: 700px) {
  .agent-support {
    padding: 3.5rem 1rem;
  }

  .agent-list {
    flex-direction: column;
    gap: 0.75rem;
    width: 100%;
  }

  .agent-divider {
    display: none;
  }

  .agent-item {
    width: 100%;
    max-width: 280px;
    padding: 1.5rem 2rem;
  }
}
</style>
