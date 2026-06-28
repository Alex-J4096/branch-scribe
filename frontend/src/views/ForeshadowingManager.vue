<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, Edit3, Save, Trash2 } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'
import type { Foreshadowing, ForeshadowingInput } from '@/api/types'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const projectId = computed(() => String(route.params.projectId))
const statusFilter = ref<Foreshadowing['status'] | ''>('')
const editingId = ref<string | null>(null)
const form = ref(emptyForm())

const foreshadowingsQuery = useQuery({
  queryKey: computed(() => ['foreshadowings', projectId.value, statusFilter.value]),
  queryFn: () => api.listForeshadowings(projectId.value, statusFilter.value || undefined),
})
const blocksQuery = useQuery({
  queryKey: computed(() => ['blocks', projectId.value]),
  queryFn: () => api.listBlocks(projectId.value),
})
const items = computed(() => foreshadowingsQuery.data.value ?? [])
const blocks = computed(() => blocksQuery.data.value ?? [])

const saveItem = useMutation({
  mutationFn: () => {
    const input = buildInput()
    return editingId.value
      ? api.updateForeshadowing(editingId.value, input)
      : api.createForeshadowing(projectId.value, input)
  },
  onSuccess: async () => {
    resetForm()
    await queryClient.invalidateQueries({ queryKey: ['foreshadowings', projectId.value] })
  },
})

const deleteItem = useMutation({
  mutationFn: (id: string) => api.deleteForeshadowing(id),
  onSuccess: async (_, id) => {
    if (editingId.value === id) resetForm()
    await queryClient.invalidateQueries({ queryKey: ['foreshadowings', projectId.value] })
  },
})

function emptyForm() {
  return {
    title: '',
    description: '',
    status: 'planted' as Foreshadowing['status'],
    plantedBlockId: '',
    resolvedBlockId: '',
  }
}

function buildInput(): ForeshadowingInput {
  return {
    title: form.value.title.trim(),
    description: form.value.description.trim() || null,
    status: form.value.status,
    planted_block_id: form.value.plantedBlockId || null,
    resolved_block_id: form.value.status === 'resolved' ? form.value.resolvedBlockId || null : null,
    metadata: {},
  }
}

function startEdit(item: Foreshadowing) {
  editingId.value = item.id
  form.value = {
    title: item.title,
    description: item.description ?? '',
    status: item.status,
    plantedBlockId: item.planted_block_id ?? '',
    resolvedBlockId: item.resolved_block_id ?? '',
  }
}

function resetForm() {
  editingId.value = null
  form.value = emptyForm()
}

function blockName(id: string | null) {
  if (!id) return '未关联'
  return blocks.value.find((block) => block.id === id)?.title || '未命名片段'
}

function statusLabel(status: Foreshadowing['status']) {
  return { planted: '已埋设', developed: '发展中', resolved: '已回收', abandoned: '已废弃' }[status]
}
</script>

<template>
  <main class="manager-page">
    <header class="manager-page__topbar">
      <button class="icon-button" type="button" title="返回工作台" @click="router.push({ name: 'workspace', params: { projectId } })">
        <ArrowLeft :size="18" aria-hidden="true" />
      </button>
      <div>
        <h1>伏笔管理</h1>
        <p>{{ items.length }} items</p>
      </div>
    </header>

    <section class="manager-layout">
      <form class="manager-form" @submit.prevent="saveItem.mutate()">
        <h2>{{ editingId ? '编辑伏笔' : '新建伏笔' }}</h2>
        <label class="field-label">
          <span>标题</span>
          <input v-model="form.title" required type="text" placeholder="一句话概括伏笔" />
        </label>
        <label class="field-label">
          <span>描述</span>
          <textarea v-model="form.description" rows="6" placeholder="线索内容、预期走向或回收条件" />
        </label>
        <label class="field-label">
          <span>生命周期</span>
          <select v-model="form.status">
            <option value="planted">已埋设</option>
            <option value="developed">发展中</option>
            <option value="resolved">已回收</option>
            <option value="abandoned">已废弃</option>
          </select>
        </label>
        <label class="field-label">
          <span>埋设 Block</span>
          <select v-model="form.plantedBlockId">
            <option value="">未关联</option>
            <option v-for="block in blocks" :key="block.id" :value="block.id">{{ block.title || '未命名片段' }}</option>
          </select>
        </label>
        <label v-if="form.status === 'resolved'" class="field-label">
          <span>回收 Block</span>
          <select v-model="form.resolvedBlockId">
            <option value="">未关联</option>
            <option v-for="block in blocks" :key="block.id" :value="block.id">{{ block.title || '未命名片段' }}</option>
          </select>
        </label>
        <div class="manager-form__actions">
          <button class="button button--primary" type="submit" :disabled="saveItem.isPending.value || !form.title.trim()">
            <Save :size="16" aria-hidden="true" />保存
          </button>
          <button v-if="editingId" class="button" type="button" @click="resetForm">取消</button>
        </div>
      </form>

      <section class="manager-list">
        <div class="manager-list__filters">
          <label class="field-label">
            <span>状态筛选</span>
            <select v-model="statusFilter">
              <option value="">全部</option>
              <option value="planted">已埋设</option>
              <option value="developed">发展中</option>
              <option value="resolved">已回收</option>
              <option value="abandoned">已废弃</option>
            </select>
          </label>
        </div>
        <div v-if="foreshadowingsQuery.isLoading.value" class="empty-state">正在加载…</div>
        <div v-else-if="!items.length" class="empty-state">还没有伏笔记录</div>
        <article v-for="item in items" v-else :key="item.id" class="foreshadowing-card">
          <div class="foreshadowing-card__header">
            <div>
              <span class="status-pill" :data-status="item.status">{{ statusLabel(item.status) }}</span>
              <h2>{{ item.title }}</h2>
            </div>
            <div class="manager-form__actions">
              <button class="icon-button" type="button" title="编辑" @click="startEdit(item)"><Edit3 :size="15" /></button>
              <button class="icon-button icon-button--danger" type="button" title="删除" @click="deleteItem.mutate(item.id)"><Trash2 :size="15" /></button>
            </div>
          </div>
          <p v-if="item.description">{{ item.description }}</p>
          <dl>
            <div><dt>埋设</dt><dd>{{ blockName(item.planted_block_id) }}</dd></div>
            <div v-if="item.status === 'resolved'"><dt>回收</dt><dd>{{ blockName(item.resolved_block_id) }}</dd></div>
          </dl>
        </article>
      </section>
    </section>
  </main>
</template>
