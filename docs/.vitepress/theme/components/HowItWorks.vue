<script setup lang="ts">
// Three-step workflow component for landing page
</script>

<template>
  <section class="how-it-works">
    <div class="how-it-works-container">
      <h2 class="section-title">How It Works</h2>
      <p class="section-subtitle">You watch, Claude works</p>

      <div class="steps-grid">
        <!-- Step 1: Write PRD -->
        <div class="step">
          <div class="step-icon">
            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
              <polyline points="14 2 14 8 20 8"></polyline>
              <line x1="16" y1="13" x2="8" y2="13"></line>
              <line x1="16" y1="17" x2="8" y2="17"></line>
              <polyline points="10 9 9 9 8 9"></polyline>
            </svg>
          </div>
          <div class="step-number">1</div>
          <h3 class="step-title">Write PRD</h3>
          <p class="step-description">
            Define your feature as user stories with acceptance criteria. Chief converts it to a structured format.
          </p>
        </div>

        <!-- Arrow connector -->
        <div class="step-connector">
          <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="5" y1="12" x2="19" y2="12"></line>
            <polyline points="12 5 19 12 12 19"></polyline>
          </svg>
        </div>

        <!-- Step 2: Chief Runs Loop -->
        <div class="step">
          <div class="step-icon step-icon-accent">
            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"></path>
              <path d="M21 3v5h-5"></path>
            </svg>
          </div>
          <div class="step-number">2</div>
          <h3 class="step-title">Chief Runs Loop</h3>
          <p class="step-description">
            Chief invokes Claude Code autonomously, iterating through each story until all acceptance criteria pass.
          </p>
        </div>

        <!-- Arrow connector -->
        <div class="step-connector">
          <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="5" y1="12" x2="19" y2="12"></line>
            <polyline points="12 5 19 12 12 19"></polyline>
          </svg>
        </div>

        <!-- Step 3: Code Gets Built -->
        <div class="step">
          <div class="step-icon step-icon-green">
            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="16 18 22 12 16 6"></polyline>
              <polyline points="8 6 2 12 8 18"></polyline>
            </svg>
          </div>
          <div class="step-number">3</div>
          <h3 class="step-title">Code Gets Built</h3>
          <p class="step-description">
            Each completed story is automatically committed. You return to find working, tested code.
          </p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.how-it-works {
  padding: 6rem 1.5rem;
  background-color: #16161e;
}

.how-it-works-container {
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

.steps-grid {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  gap: 1.5rem;
}

.step {
  flex: 1;
  max-width: 280px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  position: relative;
}

.step-icon {
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #1a1b26;
  border: 2px solid #3b4261;
  border-radius: 16px;
  color: #7aa2f7;
  margin-bottom: 1rem;
  transition: transform 0.2s, border-color 0.2s;
}

.step:hover .step-icon {
  transform: translateY(-4px);
  border-color: #7aa2f7;
}

.step-icon-accent {
  color: #bb9af7;
}

.step:hover .step-icon-accent {
  border-color: #bb9af7;
}

.step-icon-green {
  color: #9ece6a;
}

.step:hover .step-icon-green {
  border-color: #9ece6a;
}

.step-number {
  position: absolute;
  top: -8px;
  right: calc(50% - 52px);
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #292e42;
  border: 1px solid #3b4261;
  border-radius: 50%;
  font-size: 0.75rem;
  font-weight: 600;
  color: #9aa5ce;
}

.step-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #a9b1d6;
  margin: 0.75rem 0;
}

.step-description {
  font-size: 0.95rem;
  line-height: 1.6;
  color: #565f89;
  margin: 0;
}

.step-connector {
  display: flex;
  align-items: center;
  padding-top: 28px;
  color: #3b4261;
}

/* Responsive design */
@media (max-width: 900px) {
  .steps-grid {
    flex-direction: column;
    align-items: center;
  }

  .step {
    max-width: 320px;
  }

  .step-connector {
    transform: rotate(90deg);
    padding-top: 0;
    padding: 0.5rem 0;
  }
}

@media (max-width: 640px) {
  .how-it-works {
    padding: 4rem 1rem;
  }

  .section-title {
    font-size: 2rem;
  }

  .section-subtitle {
    margin-bottom: 3rem;
  }

  .step-icon {
    width: 64px;
    height: 64px;
  }

  .step-icon svg {
    width: 28px;
    height: 28px;
  }

  .step-number {
    right: calc(50% - 44px);
  }
}
</style>
