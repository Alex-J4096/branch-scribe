import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { VueQueryPlugin } from '@tanstack/vue-query'

import App from './App.vue'
import { router } from './router'
import './styles/main.css'

function applySavedDisplaySettings() {
  const root = document.documentElement
  const savedTheme = localStorage.getItem('branchscribe:display-theme')
    ?? localStorage.getItem('branchscribe:block-tool-theme')
    ?? 'system'
  const savedFont = localStorage.getItem('branchscribe:display-serif')
    ?? localStorage.getItem('branchscribe:block-tool-serif')
    ?? 'true'
  const savedFontSize = Number(
    localStorage.getItem('branchscribe:display-font-size')
      ?? localStorage.getItem('branchscribe:block-tool-font-size')
      ?? 16,
  )
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)')
  root.dataset.displayTheme = savedTheme === 'system'
    ? (prefersDark.matches ? 'dark' : 'light')
    : savedTheme
  root.dataset.displayFont = savedFont === 'false' ? 'sans' : 'serif'
  root.style.setProperty('--app-content-font-size', `${Math.min(22, Math.max(13, savedFontSize || 16))}px`)

  prefersDark.addEventListener('change', (event) => {
    if ((localStorage.getItem('branchscribe:display-theme') ?? 'system') === 'system') {
      root.dataset.displayTheme = event.matches ? 'dark' : 'light'
    }
  })
}

applySavedDisplaySettings()

createApp(App).use(createPinia()).use(VueQueryPlugin).use(router).mount('#app')
