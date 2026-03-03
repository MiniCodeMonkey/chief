<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useData } from 'vitepress'

const { page } = useData()
const isOpen = ref(false)
const copied = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

function getRawMarkdown(): string {
  return (window as any).__DOC_RAW || ''
}

function getPageTitle(): string {
  return page.value.title || 'this page'
}

function buildPrompt(raw: string): string {
  const title = getPageTitle()
  return `I'm reading the Melliza documentation page "${title}". Here is the full page content:\n\n${raw}\n\nPlease help me understand this and answer any questions I have about it.`
}

async function copyAsMarkdown() {
  const raw = getRawMarkdown()
  if (!raw) return
  try {
    await navigator.clipboard.writeText(raw)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    // Fallback for older browsers
    const textarea = document.createElement('textarea')
    textarea.value = raw
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  }
  isOpen.value = false
}

function openInChatGPT() {
  const raw = getRawMarkdown()
  if (!raw) return
  const prompt = buildPrompt(raw)
  const url = `https://chatgpt.com/?q=${encodeURIComponent(prompt)}`
  window.open(url, '_blank')
  isOpen.value = false
}

function openInGemini() {
  const raw = getRawMarkdown()
  if (!raw) return
  const prompt = buildPrompt(raw)
  const url = `https://gemini.google.com/app?q=${encodeURIComponent(prompt)}`
  window.open(url, '_blank')
  isOpen.value = false
}

function toggle() {
  isOpen.value = !isOpen.value
}

function handleClickOutside(e: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <div class="llm-actions" ref="dropdownRef">
    <button class="llm-actions-trigger" @click="toggle" aria-label="LLM Actions">
      <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="1"></circle>
        <circle cx="19" cy="12" r="1"></circle>
        <circle cx="5" cy="12" r="1"></circle>
      </svg>
      <span class="llm-actions-label">AI</span>
    </button>

    <Transition name="dropdown">
      <div v-if="isOpen" class="llm-actions-menu">
        <button class="llm-actions-item" @click="copyAsMarkdown">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
          </svg>
          <span>{{ copied ? 'Copied!' : 'Copy as Markdown' }}</span>
        </button>
        <button class="llm-actions-item" @click="openInChatGPT">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path>
          </svg>
          <span>Open in ChatGPT</span>
        </button>
        <button class="llm-actions-item" @click="openInGemini">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"></path>
            <path d="M2 17l10 5 10-5"></path>
            <path d="M2 12l10 5 10-5"></path>
          </svg>
          <span>Open in Gemini</span>
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.llm-actions {
  position: relative;
  display: inline-block;
}

.llm-actions-trigger {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background-color: #292e42;
  border: 1px solid #3b4261;
  border-radius: 8px;
  color: #a9b1d6;
  cursor: pointer;
  font-size: 13px;
  font-family: inherit;
  transition: all 0.2s ease;
}

.llm-actions-trigger:hover {
  background-color: #3b4261;
  border-color: #7aa2f7;
  color: #7aa2f7;
}

.llm-actions-label {
  font-weight: 500;
}

.llm-actions-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 200px;
  background-color: #1f2335;
  border: 1px solid #3b4261;
  border-radius: 10px;
  padding: 4px;
  z-index: 100;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
}

.llm-actions-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  background: none;
  border: none;
  border-radius: 6px;
  color: #a9b1d6;
  cursor: pointer;
  font-size: 13px;
  font-family: inherit;
  text-align: left;
  transition: all 0.15s ease;
}

.llm-actions-item:hover {
  background-color: rgba(122, 162, 247, 0.1);
  color: #7aa2f7;
}

.llm-actions-item svg {
  flex-shrink: 0;
}

/* Dropdown transition */
.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
