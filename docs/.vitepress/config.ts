import { defineConfig } from 'vitepress'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  title: 'Chief',
  description: 'Autonomous PRD Agent',
  base: '/chief/',

  vite: {
    plugins: [tailwindcss()]
  },

  themeConfig: {
    siteTitle: 'Chief',

    nav: [
      { text: 'Home', link: '/' },
      { text: 'Docs', link: '/guide/' }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/minicodemonkey/chief' }
    ]
  }
})
