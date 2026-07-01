<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Minus, Monitor, Moon, SlidersHorizontal, Sun } from 'lucide-vue-next'

const rootRef = ref<HTMLElement | null>(null)
const isOpen = ref(false)
const fontSize = ref(16)
const theme = ref<'light' | 'dark' | 'system'>('system')
const usesSerif = ref(true)
const systemPrefersDark = ref(false)
const resolvedTheme = computed(() => theme.value === 'system' ? (systemPrefersDark.value ? 'dark' : 'light') : theme.value)
let colorSchemeMedia: MediaQueryList | null = null

function applySettings() {
  const root = document.documentElement
  root.dataset.displayTheme = resolvedTheme.value
  root.dataset.displayFont = usesSerif.value ? 'serif' : 'sans'
  root.style.setProperty('--app-content-font-size', `${fontSize.value}px`)
}

function adjustFontSize(delta: number) {
  fontSize.value = Math.min(22, Math.max(13, fontSize.value + delta))
  localStorage.setItem('branchscribe:display-font-size', String(fontSize.value))
  applySettings()
}

function setTheme(value: 'light' | 'dark' | 'system') {
  theme.value = value
  localStorage.setItem('branchscribe:display-theme', value)
  applySettings()
}

function toggleSerif() {
  usesSerif.value = !usesSerif.value
  localStorage.setItem('branchscribe:display-serif', String(usesSerif.value))
  applySettings()
}

function syncSystemTheme(event?: MediaQueryListEvent) {
  systemPrefersDark.value = event?.matches ?? colorSchemeMedia?.matches ?? false
  applySettings()
}

function closeOnOutsidePointer(event: PointerEvent) {
  if (isOpen.value && !rootRef.value?.contains(event.target as Node)) isOpen.value = false
}

function closeOnEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') isOpen.value = false
}

onMounted(() => {
  const savedFontSize = Number(localStorage.getItem('branchscribe:display-font-size') ?? 16)
  const savedTheme = localStorage.getItem('branchscribe:display-theme')
  const savedSerif = localStorage.getItem('branchscribe:display-serif')
  if (Number.isFinite(savedFontSize)) fontSize.value = Math.min(22, Math.max(13, savedFontSize))
  if (savedTheme === 'light' || savedTheme === 'dark' || savedTheme === 'system') theme.value = savedTheme
  if (savedSerif === 'true' || savedSerif === 'false') usesSerif.value = savedSerif === 'true'
  colorSchemeMedia = window.matchMedia('(prefers-color-scheme: dark)')
  syncSystemTheme()
  colorSchemeMedia.addEventListener('change', syncSystemTheme)
  window.addEventListener('pointerdown', closeOnOutsidePointer)
  window.addEventListener('keydown', closeOnEscape)
})

onBeforeUnmount(() => {
  colorSchemeMedia?.removeEventListener('change', syncSystemTheme)
  window.removeEventListener('pointerdown', closeOnOutsidePointer)
  window.removeEventListener('keydown', closeOnEscape)
})
</script>

<template>
  <div ref="rootRef" class="display-settings">
    <button
      class="button"
      :class="{ 'is-active': isOpen }"
      type="button"
      aria-haspopup="dialog"
      :aria-expanded="isOpen"
      @click="isOpen = !isOpen"
    >
      <SlidersHorizontal :size="16" aria-hidden="true" />
      显示设置
    </button>
    <section v-if="isOpen" class="display-settings-menu" aria-label="显示设置">
      <header>
        <strong>显示设置</strong>
        <span>应用于整个应用</span>
      </header>
      <div class="display-settings-menu__row">
        <div>
          <span>内容字号</span>
          <small>正文、对话与摘要</small>
        </div>
        <div class="display-settings-menu__stepper">
          <button type="button" aria-label="减小字号" :disabled="fontSize <= 13" @click="adjustFontSize(-1)">
            <Minus :size="14" aria-hidden="true" />
          </button>
          <output aria-live="polite">{{ fontSize }} px</output>
          <button type="button" aria-label="增大字号" :disabled="fontSize >= 22" @click="adjustFontSize(1)">A+</button>
        </div>
      </div>
      <div class="display-settings-menu__group">
        <span>主题模式</span>
        <div class="display-settings-menu__themes">
          <button type="button" :class="{ 'is-active': theme === 'light' }" @click="setTheme('light')">
            <Sun :size="14" aria-hidden="true" />浅色
          </button>
          <button type="button" :class="{ 'is-active': theme === 'dark' }" @click="setTheme('dark')">
            <Moon :size="14" aria-hidden="true" />深色
          </button>
          <button type="button" :class="{ 'is-active': theme === 'system' }" @click="setTheme('system')">
            <Monitor :size="14" aria-hidden="true" />系统
          </button>
        </div>
      </div>
      <button
        class="display-settings-menu__toggle"
        :class="{ 'is-active': usesSerif }"
        type="button"
        role="switch"
        :aria-checked="usesSerif"
        @click="toggleSerif"
      >
        <span>
          <strong>衬线字体</strong>
          <small>更适合长篇正文阅读</small>
        </span>
        <i aria-hidden="true"><span /></i>
      </button>
    </section>
  </div>
</template>
