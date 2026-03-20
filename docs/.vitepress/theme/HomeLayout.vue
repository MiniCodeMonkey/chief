<script setup lang="ts">
import { useData } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import Hero from './components/Hero.vue'
import HowItWorks from './components/HowItWorks.vue'
import Features from './components/Features.vue'
import Footer from './components/Footer.vue'
import LlmActions from './components/LlmActions.vue'

const { Layout } = DefaultTheme
const { frontmatter } = useData()
</script>

<template>
  <Layout>
    <template #home-hero-before>
      <Hero />
      <HowItWorks />
      <Features />
      <Footer />
    </template>
    <!-- Hide default hero -->
    <template #home-hero-info></template>
    <template #home-hero-image></template>
    <template #home-hero-actions></template>
    <!-- LLM Actions on doc pages only -->
    <template #doc-before>
      <div class="llm-actions-bar">
        <LlmActions />
      </div>
    </template>
  </Layout>
</template>

<style>
/* Hide the default VitePress hero container on home page */
.VPHome .VPHero {
  display: none !important;
}

/* LLM Actions bar on doc pages */
.llm-actions-bar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}
</style>
