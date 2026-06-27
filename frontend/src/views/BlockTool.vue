<script setup lang="ts">
import { computed, ref } from 'vue'
import { ArrowLeft, Bot, FileText, SlidersHorizontal } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import BlockInspector from '@/components/inspector/BlockInspector.vue'

const route = useRoute()
const router = useRouter()
const projectId = computed(() => String(route.params.projectId))
const blockId = computed(() => String(route.params.blockId))
const activePanel = ref<'sidebar' | 'editor' | 'llm'>('editor')
</script>

<template>
  <main class="block-tool-page">
    <header class="block-tool-page__header">
      <div>
        <button class="button" type="button" @click="router.push({ name: 'workspace', params: { projectId } })">
          <ArrowLeft :size="16" aria-hidden="true" />
          返回工作台
        </button>
        <div>
          <h1>Block 工具</h1>
          <p>独立标签页 · {{ blockId }}</p>
        </div>
      </div>
      <nav class="block-tool-page__tabs" aria-label="Block 工具面板">
        <button class="button" :class="{ 'button--primary': activePanel === 'sidebar' }" type="button" @click="activePanel = 'sidebar'">
          <SlidersHorizontal :size="16" aria-hidden="true" />
          详情
        </button>
        <button class="button" :class="{ 'button--primary': activePanel === 'editor' }" type="button" @click="activePanel = 'editor'">
          <FileText :size="16" aria-hidden="true" />
          正文
        </button>
        <button class="button" :class="{ 'button--primary': activePanel === 'llm' }" type="button" @click="activePanel = 'llm'">
          <Bot :size="16" aria-hidden="true" />
          LLM 操作
        </button>
      </nav>
    </header>
    <section class="block-tool-page__content">
      <BlockInspector :project-id="projectId" :block-id="blockId" :mode="activePanel" />
    </section>
  </main>
</template>
