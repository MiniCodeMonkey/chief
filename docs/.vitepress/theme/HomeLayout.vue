<script setup lang="ts">
import { useData } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import Hero from './components/Hero.vue'
import HowItWorks from './components/HowItWorks.vue'

const { Layout } = DefaultTheme
const { frontmatter } = useData()
</script>

<template>
  <Layout>
    <template #home-hero-before>
      <Hero />
      <HowItWorks />
    </template>
    <!-- Hide default hero -->
    <template #home-hero-info></template>
    <template #home-hero-image></template>
    <template #home-hero-actions></template>
  </Layout>
</template>

<style>
/* Hide the default VitePress hero container on home page */
.VPHome .VPHero {
  display: none !important;
}
</style>
