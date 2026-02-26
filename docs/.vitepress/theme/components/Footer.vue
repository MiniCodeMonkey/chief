<script setup lang="ts">
// Landing page footer with CTA
</script>

<template>
  <footer class="footer">
    <div class="footer-container">
      <!-- CTA Section -->
      <div class="cta-section">
        <h2 class="cta-title">Ready to build something big?</h2>
        <p class="cta-subtitle">Get started in minutes with Chief</p>
        <div class="cta-buttons">
          <a href="/guide/" class="cta-button primary">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polygon points="5 3 19 12 5 21 5 3"></polygon>
            </svg>
            Quick Start Guide
          </a>
          <a href="https://github.com/minicodemonkey/chief" target="_blank" rel="noopener noreferrer" class="cta-button secondary">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"></path>
            </svg>
            View on GitHub
          </a>
        </div>
      </div>

      <!-- Copyright -->
      <div class="copyright">
        <p>&copy; {{ new Date().getFullYear() }} Chief. Released under the MIT License.</p>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.footer {
  padding: 5rem 1.5rem 2rem;
  background-color: #16161e;
  border-top: 1px solid #3b4261;
}

.footer-container {
  max-width: 1280px;
  margin: 0 auto;
}

.cta-section {
  text-align: center;
  padding-bottom: 3rem;
}

.cta-title {
  font-size: 2.25rem;
  font-weight: 700;
  color: #a9b1d6;
  margin: 0 0 0.75rem 0;
}

.cta-subtitle {
  font-size: 1.125rem;
  color: #565f89;
  margin: 0 0 2rem 0;
}

.cta-buttons {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
}

.cta-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.875rem 1.75rem;
  border-radius: 8px;
  font-size: 1rem;
  font-weight: 500;
  text-decoration: none;
  transition: all 0.2s;
}

.cta-button.primary {
  background-color: #7aa2f7;
  color: #1a1b26;
}

.cta-button.primary:hover {
  background-color: #89b4fa;
  transform: translateY(-2px);
}

.cta-button.secondary {
  background-color: transparent;
  color: #a9b1d6;
  border: 1px solid #3b4261;
}

.cta-button.secondary:hover {
  border-color: #7aa2f7;
  color: #7aa2f7;
  transform: translateY(-2px);
}

.copyright {
  text-align: center;
  padding-top: 2rem;
  border-top: 1px solid #3b4261;
}

.copyright p {
  font-size: 0.875rem;
  color: #565f89;
  margin: 0;
}

/* Responsive design */
@media (max-width: 640px) {
  .footer {
    padding: 3rem 1rem 1.5rem;
  }

  .cta-title {
    font-size: 1.75rem;
  }

  .cta-subtitle {
    font-size: 1rem;
  }

  .cta-buttons {
    flex-direction: column;
    align-items: center;
  }

  .cta-button {
    width: 100%;
    max-width: 280px;
    justify-content: center;
  }
}
</style>
