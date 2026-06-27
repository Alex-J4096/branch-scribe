<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, Edit3, Save, Search, Trash2 } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'
import type { CanonEntity, CanonEntityInput } from '@/api/types'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()

const projectId = computed(() => String(route.params.projectId))
const entityType = computed(() => normalizeEntityType(String(route.params.entityType)))
const searchQuery = ref('')
const statusFilter = ref<CanonEntity['status'] | ''>('')
const editingId = ref<string | null>(null)
const form = ref({
  name: '',
  aliases: '',
  description: '',
  status: 'canon' as CanonEntity['status'],
  importance: 5,
  attributes: '{}',
})

const pageMeta = computed(() => {
  switch (entityType.value) {
    case 'character':
      return { title: '角色设定', empty: '还没有角色卡' }
    case 'location':
      return { title: '地点设定', empty: '还没有地点卡' }
    case 'rule':
      return { title: '世界规则', empty: '还没有世界观规则' }
    default:
      return { title: 'Canon 设定', empty: '还没有设定' }
  }
})

const entitiesQuery = useQuery({
  queryKey: computed(() => ['canon-manager', projectId.value, entityType.value, statusFilter.value, searchQuery.value]),
  queryFn: () =>
    api.listCanonEntities(projectId.value, {
      type: entityType.value,
      status: statusFilter.value || undefined,
      q: searchQuery.value.trim() || undefined,
    }),
})

const entities = computed(() => entitiesQuery.data.value ?? [])
const editingEntity = computed(() => entities.value.find((entity) => entity.id === editingId.value) ?? null)

watch(entityType, () => {
  resetForm()
})

const saveEntity = useMutation({
  mutationFn: () => {
    const input = buildInput()
    if (editingId.value) {
      return api.updateCanonEntity(editingId.value, input)
    }
    return api.createCanonEntity(projectId.value, input)
  },
  onSuccess: async () => {
    resetForm()
    await queryClient.invalidateQueries({ queryKey: ['canon-manager', projectId.value] })
    await queryClient.invalidateQueries({ queryKey: ['canon', projectId.value] })
  },
})

const deleteEntity = useMutation({
  mutationFn: (entityId: string) => api.deleteCanonEntity(entityId),
  onSuccess: async () => {
    resetForm()
    await queryClient.invalidateQueries({ queryKey: ['canon-manager', projectId.value] })
    await queryClient.invalidateQueries({ queryKey: ['canon', projectId.value] })
  },
})

function normalizeEntityType(value: string): CanonEntity['type'] {
  if (value === 'character' || value === 'location' || value === 'rule') return value
  return 'character'
}

function buildInput(): CanonEntityInput {
  return {
    type: entityType.value,
    name: form.value.name.trim(),
    aliases: parseList(form.value.aliases),
    description: form.value.description.trim() || null,
    status: form.value.status,
    importance: form.value.importance,
    attributes: parseAttributes(form.value.attributes),
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

function parseAttributes(value: string) {
  try {
    const parsed = JSON.parse(value || '{}')
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function startEdit(entity: CanonEntity) {
  editingId.value = entity.id
  form.value = {
    name: entity.name,
    aliases: entity.aliases.join(', '),
    description: entity.description ?? '',
    status: entity.status,
    importance: entity.importance,
    attributes: JSON.stringify(entity.attributes ?? {}, null, 2),
  }
}

function resetForm() {
  editingId.value = null
  form.value = {
    name: '',
    aliases: '',
    description: '',
    status: 'canon',
    importance: 5,
    attributes: '{}',
  }
}
</script>

<template>
  <main class="manager-page">
    <header class="manager-page__topbar">
      <button class="icon-button" type="button" title="返回工作台" @click="router.push({ name: 'workspace', params: { projectId } })">
        <ArrowLeft :size="18" aria-hidden="true" />
      </button>
      <div>
        <h1>{{ pageMeta.title }}</h1>
        <p>{{ entities.length }} items</p>
      </div>
    </header>

    <section class="manager-layout">
      <form class="manager-form" @submit.prevent="saveEntity.mutate()">
        <h2>{{ editingEntity ? '编辑' : '新建' }}</h2>
        <label class="field-label">
          <span>名称</span>
          <input v-model="form.name" type="text" required />
        </label>
        <label class="field-label">
          <span>别名</span>
          <input v-model="form.aliases" type="text" placeholder="用逗号分隔" />
        </label>
        <label class="field-label">
          <span>描述</span>
          <textarea v-model="form.description" rows="7" />
        </label>
        <div class="manager-form__row">
          <label class="field-label">
            <span>状态</span>
            <select v-model="form.status">
              <option value="canon">canon</option>
              <option value="draft">draft</option>
              <option value="deprecated">deprecated</option>
            </select>
          </label>
          <label class="field-label">
            <span>重要度（1–10）</span>
            <input v-model.number="form.importance" type="number" min="1" max="10" />
          </label>
        </div>
        <label class="field-label">
          <span>Attributes JSON</span>
          <textarea v-model="form.attributes" rows="5" spellcheck="false" />
        </label>
        <div class="manager-form__actions">
          <button class="button button--primary" type="submit" :disabled="saveEntity.isPending.value || !form.name.trim()">
            <Save :size="16" aria-hidden="true" />
            保存
          </button>
          <button v-if="editingEntity" class="button" type="button" @click="resetForm">取消</button>
        </div>
      </form>

      <section class="manager-list">
        <div class="manager-list__filters">
          <label class="field-label">
            <span>搜索</span>
            <div class="input-with-icon">
              <Search :size="15" aria-hidden="true" />
              <input v-model="searchQuery" type="text" placeholder="名称、描述、别名" />
            </div>
          </label>
          <label class="field-label">
            <span>状态</span>
            <select v-model="statusFilter">
              <option value="">全部</option>
              <option value="canon">canon</option>
              <option value="draft">draft</option>
              <option value="deprecated">deprecated</option>
            </select>
          </label>
        </div>

        <div v-if="entitiesQuery.isLoading.value" class="empty-state">正在加载</div>
        <div v-else-if="!entities.length" class="empty-state">{{ pageMeta.empty }}</div>
        <article v-for="entity in entities" v-else :key="entity.id" class="manager-item">
          <div>
            <h2>{{ entity.name }}</h2>
            <p>{{ entity.description || '无描述' }}</p>
            <div class="manager-item__meta">
              <span>{{ entity.status }}</span>
              <span>重要度 {{ entity.importance }}</span>
              <span v-for="alias in entity.aliases" :key="alias">{{ alias }}</span>
            </div>
          </div>
          <div class="manager-item__actions">
            <button class="icon-button" type="button" title="编辑" @click="startEdit(entity)">
              <Edit3 :size="16" aria-hidden="true" />
            </button>
            <button class="icon-button icon-button--danger" type="button" title="删除" @click="deleteEntity.mutate(entity.id)">
              <Trash2 :size="16" aria-hidden="true" />
            </button>
          </div>
        </article>
      </section>
    </section>
  </main>
</template>
