<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, Database, Edit3, Save, Search, Trash2 } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'
import type { MemoryChunk, MemoryChunkInput } from '@/api/types'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()

const projectId = computed(() => String(route.params.projectId))
const searchQuery = ref('')
const chunkKindFilter = ref('')
const tagFilter = ref('')
const blockIdForMemory = ref('')
const blockTagsDraft = ref('')
const editingId = ref<string | null>(null)
const form = ref({
  source_type: 'manual',
  source_id: '',
  chunk_text: '',
  chunk_kind: 'note',
  tags: '',
  metadata: '{}',
})

const memoryQuery = useQuery({
  queryKey: computed(() => ['memory-manager', projectId.value, searchQuery.value, chunkKindFilter.value, tagFilter.value]),
  queryFn: () =>
    api.listMemoryChunks(projectId.value, {
      q: searchQuery.value.trim() || undefined,
      chunk_kind: chunkKindFilter.value.trim() || undefined,
      tag: tagFilter.value.trim() || undefined,
    }),
})

const graphQuery = useQuery({
  queryKey: computed(() => ['graph', projectId.value]),
  queryFn: () => api.getGraph(projectId.value),
})

const chunks = computed(() => memoryQuery.data.value ?? [])
const blocks = computed(() => graphQuery.data.value?.nodes ?? [])
const editingChunk = computed(() => chunks.value.find((chunk) => chunk.id === editingId.value) ?? null)

const saveChunk = useMutation({
  mutationFn: () => {
    const input = buildInput()
    if (editingId.value) {
      return api.updateMemoryChunk(editingId.value, input)
    }
    return api.createMemoryChunk(projectId.value, input)
  },
  onSuccess: async () => {
    resetForm()
    await queryClient.invalidateQueries({ queryKey: ['memory-manager', projectId.value] })
  },
})

const createFromBlock = useMutation({
  mutationFn: () =>
    api.createMemoryChunkFromBlock(blockIdForMemory.value, {
      chunk_kind: 'block_revision',
      tags: parseList(blockTagsDraft.value),
      metadata: { source: 'block_inspector' },
    }),
  onSuccess: async () => {
    blockTagsDraft.value = ''
    await queryClient.invalidateQueries({ queryKey: ['memory-manager', projectId.value] })
  },
})

const deleteChunk = useMutation({
  mutationFn: (chunkId: string) => api.deleteMemoryChunk(chunkId),
  onSuccess: async () => {
    resetForm()
    await queryClient.invalidateQueries({ queryKey: ['memory-manager', projectId.value] })
  },
})

function buildInput(): MemoryChunkInput {
  return {
    source_type: form.value.source_type.trim() || 'manual',
    source_id: form.value.source_id.trim() || null,
    chunk_text: form.value.chunk_text.trim(),
    chunk_kind: form.value.chunk_kind.trim() || 'note',
    tags: parseList(form.value.tags),
    metadata: parseMetadata(form.value.metadata),
  }
}

function parseList(value: string) {
  const seen = new Set<string>()
  const items: string[] = []
  for (const item of value.split(/[,，\n]/)) {
    const trimmed = item.trim()
    if (!trimmed || seen.has(trimmed)) continue
    seen.add(trimmed)
    items.push(trimmed)
  }
  return items
}

function parseMetadata(value: string) {
  try {
    const parsed = JSON.parse(value || '{}')
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function startEdit(chunk: MemoryChunk) {
  editingId.value = chunk.id
  form.value = {
    source_type: chunk.source_type,
    source_id: chunk.source_id ?? '',
    chunk_text: chunk.chunk_text,
    chunk_kind: chunk.chunk_kind,
    tags: chunk.tags.join(', '),
    metadata: JSON.stringify(chunk.metadata ?? {}, null, 2),
  }
}

function resetForm() {
  editingId.value = null
  form.value = {
    source_type: 'manual',
    source_id: '',
    chunk_text: '',
    chunk_kind: 'note',
    tags: '',
    metadata: '{}',
  }
}

function blockLabel(blockId: string) {
  const index = blocks.value.findIndex((block) => block.id === blockId)
  const block = blocks.value[index]
  return block?.title || `Block #${index + 1}`
}
</script>

<template>
  <main class="manager-page">
    <header class="manager-page__topbar">
      <button class="icon-button" type="button" title="返回工作台" @click="router.push({ name: 'workspace', params: { projectId } })">
        <ArrowLeft :size="18" aria-hidden="true" />
      </button>
      <div>
        <h1>Memory</h1>
        <p>{{ chunks.length }} chunks</p>
      </div>
    </header>

    <section class="manager-layout">
      <div class="manager-form-stack">
        <form class="manager-form" @submit.prevent="saveChunk.mutate()">
          <h2>{{ editingChunk ? '编辑 Memory' : '手动创建 Memory' }}</h2>
          <div class="manager-form__row">
            <label class="field-label">
              <span>Source Type</span>
              <input v-model="form.source_type" type="text" required />
            </label>
            <label class="field-label">
              <span>Chunk Kind</span>
              <input v-model="form.chunk_kind" type="text" required />
            </label>
          </div>
          <label class="field-label">
            <span>Source ID</span>
            <input v-model="form.source_id" type="text" />
          </label>
          <label class="field-label">
            <span>Chunk Text</span>
            <textarea v-model="form.chunk_text" rows="8" required />
          </label>
          <label class="field-label">
            <span>Tags</span>
            <input v-model="form.tags" type="text" placeholder="用逗号分隔" />
          </label>
          <label class="field-label">
            <span>Metadata JSON</span>
            <textarea v-model="form.metadata" rows="4" spellcheck="false" />
          </label>
          <div class="manager-form__actions">
            <button class="button button--primary" type="submit" :disabled="saveChunk.isPending.value || !form.chunk_text.trim()">
              <Save :size="16" aria-hidden="true" />
              保存
            </button>
            <button v-if="editingChunk" class="button" type="button" @click="resetForm">取消</button>
          </div>
        </form>

        <form class="manager-form" @submit.prevent="createFromBlock.mutate()">
          <h2>从 Block 生成</h2>
          <label class="field-label">
            <span>Block</span>
            <select v-model="blockIdForMemory" required>
              <option value="" disabled>选择 Block</option>
              <option v-for="block in blocks" :key="block.id" :value="block.id">
                {{ blockLabel(block.id) }}
              </option>
            </select>
          </label>
          <label class="field-label">
            <span>Tags</span>
            <input v-model="blockTagsDraft" type="text" placeholder="用逗号分隔" />
          </label>
          <button class="button" type="submit" :disabled="createFromBlock.isPending.value || !blockIdForMemory">
            <Database :size="16" aria-hidden="true" />
            生成 Memory
          </button>
        </form>
      </div>

      <section class="manager-list">
        <div class="manager-list__filters">
          <label class="field-label">
            <span>搜索</span>
            <div class="input-with-icon">
              <Search :size="15" aria-hidden="true" />
              <input v-model="searchQuery" type="text" placeholder="正文关键词" />
            </div>
          </label>
          <label class="field-label">
            <span>Kind</span>
            <input v-model="chunkKindFilter" type="text" />
          </label>
          <label class="field-label">
            <span>Tag</span>
            <input v-model="tagFilter" type="text" />
          </label>
        </div>

        <div v-if="memoryQuery.isLoading.value" class="empty-state">正在加载</div>
        <div v-else-if="!chunks.length" class="empty-state">还没有 memory chunks</div>
        <article v-for="chunk in chunks" v-else :key="chunk.id" class="manager-item">
          <div>
            <h2>{{ chunk.chunk_kind }}</h2>
            <p>{{ chunk.chunk_text }}</p>
            <div class="manager-item__meta">
              <span>{{ chunk.source_type }}</span>
              <span v-for="tag in chunk.tags" :key="tag">{{ tag }}</span>
            </div>
          </div>
          <div class="manager-item__actions">
            <button class="icon-button" type="button" title="编辑" @click="startEdit(chunk)">
              <Edit3 :size="16" aria-hidden="true" />
            </button>
            <button class="icon-button icon-button--danger" type="button" title="删除" @click="deleteChunk.mutate(chunk.id)">
              <Trash2 :size="16" aria-hidden="true" />
            </button>
          </div>
        </article>
      </section>
    </section>
  </main>
</template>
