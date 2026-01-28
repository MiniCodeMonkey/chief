<script setup lang="ts">
// Key features grid for landing page
</script>

<template>
  <section class="features">
    <div class="features-container">
      <h2 class="section-title">Key Features</h2>
      <p class="section-subtitle">Everything you need, nothing you don't</p>

      <div class="features-grid">
        <!-- Single Binary -->
        <div class="feature-card">
          <div class="feature-icon">
            <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
              <rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
              <line x1="6" y1="6" x2="6.01" y2="6"></line>
              <line x1="6" y1="18" x2="6.01" y2="18"></line>
            </svg>
          </div>
          <h3 class="feature-title">Single Binary</h3>
          <p class="feature-description">
            No runtime dependencies. Download the binary and run it. Works on macOS, Linux, and Windows.
          </p>
        </div>

        <!-- Self-Contained State -->
        <div class="feature-card">
          <div class="feature-icon feature-icon-purple">
            <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
            </svg>
          </div>
          <h3 class="feature-title">Self-Contained State</h3>
          <p class="feature-description">
            Everything lives in <code>.chief/</code>. Move your project anywhere and state moves with it.
          </p>
        </div>

        <!-- Works Anywhere -->
        <div class="feature-card">
          <div class="feature-icon feature-icon-green">
            <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="2" y1="12" x2="22" y2="12"></line>
              <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path>
            </svg>
          </div>
          <h3 class="feature-title">Works Anywhere</h3>
          <p class="feature-description">
            Run locally or SSH into a remote server. Start a loop before bed and wake up to code.
          </p>
        </div>

        <!-- Beautiful TUI -->
        <div class="feature-card">
          <div class="feature-icon feature-icon-cyan">
            <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
              <line x1="8" y1="21" x2="16" y2="21"></line>
              <line x1="12" y1="17" x2="12" y2="21"></line>
            </svg>
          </div>
          <h3 class="feature-title">Beautiful TUI</h3>
          <p class="feature-description">
            Real-time progress, live logs, and keyboard controls. Watch Claude work in style.
          </p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.features {
  padding: 6rem 1.5rem;
  background-color: #1a1b26;
}

.features-container {
  max-width: 1280px;
  margin: 0 auto;
}

.section-title {
  font-size: 2.5rem;
  font-weight: 700;
  text-align: center;
  color: #a9b1d6;
  margin: 0 0 0.5rem 0;
}

.section-subtitle {
  font-size: 1.125rem;
  text-align: center;
  color: #565f89;
  margin: 0 0 4rem 0;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 2rem;
}

.feature-card {
  background-color: #16161e;
  border: 1px solid #3b4261;
  border-radius: 12px;
  padding: 2rem;
  transition: transform 0.2s, border-color 0.2s;
}

.feature-card:hover {
  transform: translateY(-4px);
  border-color: #7aa2f7;
}

.feature-card:nth-child(2):hover {
  border-color: #bb9af7;
}

.feature-card:nth-child(3):hover {
  border-color: #9ece6a;
}

.feature-card:nth-child(4):hover {
  border-color: #7dcfff;
}

.feature-icon {
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: rgba(122, 162, 247, 0.1);
  border-radius: 12px;
  color: #7aa2f7;
  margin-bottom: 1.25rem;
}

.feature-icon-purple {
  background-color: rgba(187, 154, 247, 0.1);
  color: #bb9af7;
}

.feature-icon-green {
  background-color: rgba(158, 206, 106, 0.1);
  color: #9ece6a;
}

.feature-icon-cyan {
  background-color: rgba(125, 207, 255, 0.1);
  color: #7dcfff;
}

.feature-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #a9b1d6;
  margin: 0 0 0.75rem 0;
}

.feature-description {
  font-size: 0.95rem;
  line-height: 1.6;
  color: #565f89;
  margin: 0;
}

.feature-description code {
  background-color: #292e42;
  padding: 0.125rem 0.375rem;
  border-radius: 4px;
  font-size: 0.875rem;
  color: #7aa2f7;
}

/* Responsive design */
@media (max-width: 768px) {
  .features-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .features {
    padding: 4rem 1rem;
  }

  .section-title {
    font-size: 2rem;
  }

  .section-subtitle {
    margin-bottom: 3rem;
  }

  .feature-card {
    padding: 1.5rem;
  }

  .feature-icon {
    width: 48px;
    height: 48px;
  }

  .feature-icon svg {
    width: 24px;
    height: 24px;
  }
}
</style>
