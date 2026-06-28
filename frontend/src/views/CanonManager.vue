<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, Edit3, History, Save, Search, Sparkles, Trash2 } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'
import type { CanonEntity, CanonEntityInput, CharacterCardProposal } from '@/api/types'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()

const projectId = computed(() => String(route.params.projectId))
const entityType = computed(() => normalizeEntityType(String(route.params.entityType)))
const searchQuery = ref('')
const statusFilter = ref<CanonEntity['status'] | ''>('')
const editingId = ref<string | null>(null)
const extractionCharacterId = ref('')
const extractionBlockId = ref('')
const extractionSelectedBlockIds = ref<string[]>([])
const extractionModelId = ref('')
const extractionDescription = ref('')
const extractionAttributes = ref('{}')
const extractionSummary = ref('')
const extractionModel = ref('')
const extractionGenerationRunId = ref('')
const extractionProposalBlockIds = ref<string[]>([])
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
const extractionCharacter = computed(() => entities.value.find((entity) => entity.id === extractionCharacterId.value) ?? null)

const blocksQuery = useQuery({
  queryKey: computed(() => ['blocks', projectId.value]),
  queryFn: () => api.listBlocks(projectId.value),
  enabled: computed(() => entityType.value === 'character'),
})
const modelProfilesQuery = useQuery({
  queryKey: computed(() => ['model-profiles', projectId.value]),
  queryFn: () => api.listModelProfiles(projectId.value),
  enabled: computed(() => entityType.value === 'character'),
})
const characterStatesQuery = useQuery({
  queryKey: computed(() => ['character-states', projectId.value]),
  queryFn: () => api.listCharacterStates(projectId.value),
  enabled: computed(() => entityType.value === 'character'),
})
const blocks = computed(() => blocksQuery.data.value ?? [])
const extractionStartBlock = computed(() => blocks.value.find((block) => block.id === extractionBlockId.value) ?? null)
const extractionBranchBlocks = computed(() => {
  const start = extractionStartBlock.value
  if (!start) return []
  return blocks.value.filter((block) =>
    block.branch_id === start.branch_id && block.order_index >= start.order_index,
  )
})
const llmProfiles = computed(() => (modelProfilesQuery.data.value ?? []).filter((profile) => profile.profile_type === 'llm'))
const characterCardVersions = computed(() =>
  (characterStatesQuery.data.value ?? []).filter((state) => state.state_key === 'character_card'),
)

watch(entityType, () => {
  resetForm()
})
watch(extractionBlockId, () => {
  extractionSelectedBlockIds.value = extractionBranchBlocks.value.map((block) => block.id)
})
watch([entities, () => route.query.focus], ([items, focus]) => {
  if (typeof focus !== 'string' || editingId.value === focus) return
  const entity = items.find((item) => item.id === focus)
  if (entity) startEdit(entity)
}, { immediate: true })

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

const extractCharacterCard = useMutation({
  mutationFn: () => api.extractCharacterCard(projectId.value, extractionCharacterId.value, {
    block_id: extractionBlockId.value,
    block_ids: extractionSelectedBlockIds.value,
    model_profile_id: extractionModelId.value,
  }),
  onSuccess: (proposal: CharacterCardProposal) => {
    extractionDescription.value = proposal.description
    extractionAttributes.value = JSON.stringify(proposal.attributes, null, 2)
    extractionSummary.value = proposal.change_summary
    extractionModel.value = proposal.model
    extractionGenerationRunId.value = proposal.generation_run_id
    extractionProposalBlockIds.value = proposal.source_block_ids
  },
})

const saveCharacterCardVersion = useMutation({
  mutationFn: async () => {
    const character = extractionCharacter.value
    if (!character) throw new Error('未选择角色')
    const attributes = parseAttributes(extractionAttributes.value)
    await api.createCharacterState(projectId.value, {
      character_id: character.id,
      block_id: extractionBlockId.value,
      state_key: 'character_card',
      state_value: {
        name: character.name,
        aliases: character.aliases,
        description: extractionDescription.value.trim(),
        attributes,
      },
      notes: extractionSummary.value.trim() || null,
      occurred_at: new Date().toISOString(),
      metadata: {
        source: 'llm_extract',
        model: extractionModel.value,
        generation_run_id: extractionGenerationRunId.value,
        source_block_ids: extractionProposalBlockIds.value,
      },
    })
    return api.updateCanonEntity(character.id, {
      description: extractionDescription.value.trim(),
      attributes,
    })
  },
  onSuccess: async () => {
    closeExtraction()
    await queryClient.invalidateQueries({ queryKey: ['character-states', projectId.value] })
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

function startExtraction(entity: CanonEntity) {
  extractionCharacterId.value = entity.id
  const startBlock = blocks.value.at(-1)
  extractionBlockId.value = startBlock?.id ?? ''
  extractionModelId.value = llmProfiles.value[0]?.id ?? ''
  extractionDescription.value = ''
  extractionSelectedBlockIds.value = startBlock
    ? blocks.value
      .filter((block) => block.branch_id === startBlock.branch_id && block.order_index >= startBlock.order_index)
      .map((block) => block.id)
    : []
  extractionAttributes.value = '{}'
  extractionSummary.value = ''
  extractionModel.value = ''
  extractionGenerationRunId.value = ''
  extractionProposalBlockIds.value = []
}

function closeExtraction() {
  extractionCharacterId.value = ''
  extractionDescription.value = ''
  extractionSelectedBlockIds.value = []
  extractionAttributes.value = '{}'
  extractionSummary.value = ''
  extractionModel.value = ''
  extractionGenerationRunId.value = ''
  extractionProposalBlockIds.value = []
  extractCharacterCard.reset()
}

function versionsFor(characterId: string) {
  return characterCardVersions.value
    .filter((version) => version.character_id === characterId)
    .sort((a, b) => b.created_at.localeCompare(a.created_at))
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
      <div class="manager-form-stack">
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
      <section v-if="entityType === 'character' && extractionCharacter" class="manager-form">
        <h2>从后续剧情提取 · {{ extractionCharacter.name }}</h2>
        <label class="field-label">
          <span>起始剧情节点</span>
          <select v-model="extractionBlockId">
            <option value="" disabled>选择汇总剧情的起点</option>
            <option v-for="block in blocks" :key="block.id" :value="block.id">
              {{ block.title || '未命名片段' }}
            </option>
          </select>
        </label>
        <fieldset v-if="extractionBranchBlocks.length" class="character-block-picker">
          <legend>最终加入摘要的同分支 Block</legend>
          <label v-for="block in extractionBranchBlocks" :key="block.id">
            <input v-model="extractionSelectedBlockIds" type="checkbox" :value="block.id" :disabled="block.id === extractionBlockId" />
            <span>{{ block.title || '未命名片段' }}</span>
            <small>#{{ block.order_index }}</small>
          </label>
        </fieldset>
        <label class="field-label">
          <span>模型</span>
          <select v-model="extractionModelId">
            <option value="" disabled>选择模型</option>
            <option v-for="profile in llmProfiles" :key="profile.id" :value="profile.id">
              {{ profile.name }} · {{ profile.model }}
            </option>
          </select>
        </label>
        <button
          class="button"
          type="button"
          :disabled="!extractionBlockId || !extractionModelId || !extractionSelectedBlockIds.length || extractCharacterCard.isPending.value"
          @click="extractCharacterCard.mutate()"
        >
          <Sparkles :size="15" aria-hidden="true" />
          {{ extractCharacterCard.isPending.value ? '正在提取' : '生成候选角色卡' }}
        </button>
        <template v-if="extractionDescription">
          <label class="field-label"><span>完整角色描述</span><textarea v-model="extractionDescription" rows="7" /></label>
          <label class="field-label"><span>Attributes JSON</span><textarea v-model="extractionAttributes" rows="7" spellcheck="false" /></label>
          <label class="field-label"><span>版本变化摘要</span><textarea v-model="extractionSummary" rows="3" /></label>
          <div class="manager-form__actions">
            <button class="button button--primary" type="button" :disabled="saveCharacterCardVersion.isPending.value" @click="saveCharacterCardVersion.mutate()">
              <Save :size="15" aria-hidden="true" />保存为新版本
            </button>
            <button class="button" type="button" @click="closeExtraction">取消</button>
          </div>
        </template>
        <button v-else class="button" type="button" @click="closeExtraction">关闭</button>
        <p v-if="extractCharacterCard.error.value" class="llm-message llm-message--error">
          {{ extractCharacterCard.error.value instanceof Error ? extractCharacterCard.error.value.message : '提取失败' }}
        </p>
      </section>
      </div>

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
            <button v-if="entityType === 'character'" class="icon-button" type="button" title="从后续剧情提取新版角色卡" @click="startExtraction(entity)">
              <Sparkles :size="16" aria-hidden="true" />
            </button>
            <button class="icon-button" type="button" title="编辑" @click="startEdit(entity)">
              <Edit3 :size="16" aria-hidden="true" />
            </button>
            <button class="icon-button icon-button--danger" type="button" title="删除" @click="deleteEntity.mutate(entity.id)">
              <Trash2 :size="16" aria-hidden="true" />
            </button>
          </div>
          <details v-if="entityType === 'character' && versionsFor(entity.id).length" class="manager-item__versions">
            <summary><History :size="14" aria-hidden="true" />历史版本（{{ versionsFor(entity.id).length }}）</summary>
            <article v-for="(version, index) in versionsFor(entity.id)" :key="version.id">
              <strong>版本 {{ versionsFor(entity.id).length - index }}</strong>
              <time>{{ new Date(version.created_at).toLocaleString() }}</time>
              <p>{{ version.notes || '无变化摘要' }}</p>
              <pre>{{ JSON.stringify(version.state_value, null, 2) }}</pre>
            </article>
          </details>
        </article>
      </section>
    </section>
  </main>
</template>
