<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { CopyPlus, GitFork, Save, SquarePen } from 'lucide-vue-next'

import { api } from '@/api/client'
import RichTextEditor from '@/components/editor/RichTextEditor.vue'
import type { Revision } from '@/api/types'

const props = defineProps<{
  projectId: string
  blockId: string
}>()

const emit = defineEmits<{
  changed: []
}>()

const queryClient = useQueryClient()
const draftContent = ref('')
const titleDraft = ref('')
const forkTitle = ref('')
const diffBaseRevisionId = ref('')
const diffTargetRevisionId = ref('')
const draftSavedAt = ref<string | null>(null)
const restoredLocalDraft = ref(false)
let autosaveTimer: ReturnType<typeof window.setTimeout> | null = null

const blockQuery = useQuery({
  queryKey: computed(() => ['block', props.blockId]),
  queryFn: () => api.getBlock(props.blockId),
})

const revisionsQuery = useQuery({
  queryKey: computed(() => ['revisions', props.blockId]),
  queryFn: () => api.listRevisions(props.blockId),
})

const blockDetail = computed(() => blockQuery.data.value)
const revisions = computed(() => revisionsQuery.data.value ?? [])
const currentRevision = computed(() => blockDetail.value?.current_revision ?? null)
const displayTitle = computed(() => blockDetail.value?.block.title || '无标题片段')
const isContentDirty = computed(() => draftContent.value !== (currentRevision.value?.content ?? ''))
const wordCount = computed(() => countWords(stripHTML(draftContent.value)))
const currentRevisionHash = computed(() => currentRevision.value?.content_hash?.slice(0, 10) ?? 'no hash')
const draftStorageKey = computed(() => `branchscribe:draft:${props.projectId}:${props.blockId}`)
const diffBaseRevision = computed(() => revisions.value.find((revision) => revision.id === diffBaseRevisionId.value) ?? null)
const diffTargetRevision = computed(() => revisions.value.find((revision) => revision.id === diffTargetRevisionId.value) ?? null)
const diffSegments = computed(() =>
  buildDiff(
    stripHTML(diffBaseRevision.value?.content ?? ''),
    stripHTML(diffTargetRevision.value?.content ?? ''),
  ),
)

watch(
  [currentRevision, draftStorageKey],
  (revision) => {
    const current = revision[0]
    const savedDraft = readLocalDraft()
    if (savedDraft && savedDraft.baseRevisionId === current?.id && savedDraft.content !== (current?.content ?? '')) {
      draftContent.value = savedDraft.content
      draftSavedAt.value = savedDraft.savedAt
      restoredLocalDraft.value = true
      return
    }

    draftContent.value = current?.content ?? ''
    draftSavedAt.value = null
    restoredLocalDraft.value = false
    if (savedDraft && savedDraft.baseRevisionId !== current?.id) {
      clearLocalDraft()
    }
  },
  { immediate: true },
)

watch(draftContent, () => {
  scheduleDraftAutosave()
})

watch(
  () => blockDetail.value?.block.title,
  (title) => {
    titleDraft.value = title ?? ''
  },
  { immediate: true },
)

watch(
  revisions,
  (value) => {
    if (!value.some((revision) => revision.id === diffBaseRevisionId.value)) {
      diffBaseRevisionId.value = value[1]?.id ?? value[0]?.id ?? ''
    }
    if (!value.some((revision) => revision.id === diffTargetRevisionId.value)) {
      diffTargetRevisionId.value = value[0]?.id ?? ''
    }
  },
  { immediate: true },
)

const createRevision = useMutation({
  mutationFn: () =>
    api.createRevision(props.blockId, {
      content: draftContent.value,
      content_format: 'html',
      source: 'user',
      set_current: true,
    }),
  onSuccess: async () => {
    clearLocalDraft()
    await refreshInspector()
  },
})

const updateTitle = useMutation({
  mutationFn: () => api.updateBlock(props.blockId, { title: titleDraft.value.trim() || null }),
  onSuccess: refreshInspector,
})

const selectRevision = useMutation({
  mutationFn: (revisionId: string) => api.selectRevision(props.blockId, revisionId),
  onSuccess: async () => {
    clearLocalDraft()
    await refreshInspector()
  },
})

const forkBlock = useMutation({
  mutationFn: () =>
    api.forkBlock(props.blockId, {
      title: forkTitle.value.trim() || undefined,
      position_x: (blockDetail.value?.block.position_x ?? 0) + 260,
      position_y: (blockDetail.value?.block.position_y ?? 0) + 90,
    }),
  onSuccess: async () => {
    forkTitle.value = ''
    await refreshInspector()
  },
})

async function refreshInspector() {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ['block', props.blockId] }),
    queryClient.invalidateQueries({ queryKey: ['revisions', props.blockId] }),
    queryClient.invalidateQueries({ queryKey: ['graph', props.projectId] }),
  ])
  emit('changed')
}

onBeforeUnmount(() => {
  if (autosaveTimer) {
    window.clearTimeout(autosaveTimer)
  }
})

function revisionLabel(revision: Revision) {
  const created = new Date(revision.created_at)
  return `${revision.source} · ${created.toLocaleString()}`
}

function stripHTML(value: string) {
  return value
    .replace(/<[^>]*>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/\s+/g, ' ')
    .trim()
}

function countWords(value: string) {
  const cjkMatches = value.match(/[\u4e00-\u9fff]/g) ?? []
  const wordMatches = value.replace(/[\u4e00-\u9fff]/g, ' ').match(/[A-Za-z0-9]+(?:[-'][A-Za-z0-9]+)*/g) ?? []
  return cjkMatches.length + wordMatches.length
}

function tokenize(value: string) {
  return value.match(/[\u4e00-\u9fff]|[A-Za-z0-9]+(?:[-'][A-Za-z0-9]+)*|\s+|[^\sA-Za-z0-9\u4e00-\u9fff]/g) ?? []
}

function buildDiff(base: string, target: string) {
  const baseTokens = tokenize(base)
  const targetTokens = tokenize(target)
  const table = Array.from({ length: baseTokens.length + 1 }, () => Array<number>(targetTokens.length + 1).fill(0))

  for (let i = baseTokens.length - 1; i >= 0; i -= 1) {
    for (let j = targetTokens.length - 1; j >= 0; j -= 1) {
      table[i][j] =
        baseTokens[i] === targetTokens[j]
          ? table[i + 1][j + 1] + 1
          : Math.max(table[i + 1][j], table[i][j + 1])
    }
  }

  const segments: Array<{ type: 'equal' | 'insert' | 'delete'; text: string }> = []
  let i = 0
  let j = 0
  while (i < baseTokens.length && j < targetTokens.length) {
    if (baseTokens[i] === targetTokens[j]) {
      pushSegment(segments, 'equal', baseTokens[i])
      i += 1
      j += 1
    } else if (table[i + 1][j] >= table[i][j + 1]) {
      pushSegment(segments, 'delete', baseTokens[i])
      i += 1
    } else {
      pushSegment(segments, 'insert', targetTokens[j])
      j += 1
    }
  }
  while (i < baseTokens.length) {
    pushSegment(segments, 'delete', baseTokens[i])
    i += 1
  }
  while (j < targetTokens.length) {
    pushSegment(segments, 'insert', targetTokens[j])
    j += 1
  }

  return segments
}

function pushSegment(
  segments: Array<{ type: 'equal' | 'insert' | 'delete'; text: string }>,
  type: 'equal' | 'insert' | 'delete',
  text: string,
) {
  const previous = segments[segments.length - 1]
  if (previous?.type === type) {
    previous.text += text
    return
  }
  segments.push({ type, text })
}

function scheduleDraftAutosave() {
  if (autosaveTimer) {
    window.clearTimeout(autosaveTimer)
  }

  autosaveTimer = window.setTimeout(() => {
    if (!isContentDirty.value) {
      clearLocalDraft()
      return
    }
    writeLocalDraft()
  }, 600)
}

function readLocalDraft() {
  try {
    const raw = window.localStorage.getItem(draftStorageKey.value)
    if (!raw) return null
    return JSON.parse(raw) as { content: string; baseRevisionId: string | null; savedAt: string }
  } catch {
    return null
  }
}

function writeLocalDraft() {
  const savedAt = new Date().toISOString()
  window.localStorage.setItem(
    draftStorageKey.value,
    JSON.stringify({
      content: draftContent.value,
      baseRevisionId: currentRevision.value?.id ?? null,
      savedAt,
    }),
  )
  draftSavedAt.value = savedAt
  restoredLocalDraft.value = false
}

function clearLocalDraft() {
  window.localStorage.removeItem(draftStorageKey.value)
  draftSavedAt.value = null
  restoredLocalDraft.value = false
}

function discardDraft() {
  clearLocalDraft()
  draftContent.value = currentRevision.value?.content ?? ''
}

function formatDraftTime(value: string) {
  return new Date(value).toLocaleTimeString()
}
</script>

<template>
  <section class="inspector">
    <div v-if="blockQuery.isLoading.value" class="empty-state empty-state--panel">正在加载 block</div>

    <template v-else-if="blockDetail">
      <div class="inspector__header">
        <div>
          <h2>{{ displayTitle }}</h2>
          <p>{{ blockDetail.block.type }} · {{ revisions.length }} revisions</p>
        </div>
        <SquarePen :size="18" aria-hidden="true" />
      </div>

      <form class="title-form" @submit.prevent="updateTitle.mutate()">
        <input v-model="titleDraft" type="text" placeholder="Block 标题（可选）" />
        <button class="button" type="submit" :disabled="updateTitle.isPending.value">
          <Save :size="16" aria-hidden="true" />
          标题
        </button>
      </form>

      <div class="editor-field">
        <span>正文 · {{ wordCount }} 字 · {{ isContentDirty ? '未保存' : '当前 revision' }}</span>
        <RichTextEditor v-model="draftContent" :content-format="currentRevision?.content_format" />
      </div>

      <div class="revision-status">
        <span>{{ currentRevision?.content_format ?? 'empty' }}</span>
        <span>{{ currentRevisionHash }}</span>
        <span>{{ currentRevision ? revisionLabel(currentRevision) : '无 revision' }}</span>
        <span v-if="draftSavedAt">{{ restoredLocalDraft ? '已恢复本地草稿' : '草稿已自动保存' }} · {{ formatDraftTime(draftSavedAt) }}</span>
      </div>

      <div class="inspector__actions">
        <button class="button button--primary" type="button" :disabled="createRevision.isPending.value" @click="createRevision.mutate()">
          <Save :size="16" aria-hidden="true" />
          保存 Revision
        </button>
        <button v-if="draftSavedAt" class="button" type="button" @click="discardDraft">丢弃草稿</button>
      </div>

      <div class="fork-form">
        <input v-model="forkTitle" type="text" placeholder="Fork 标题（可选）" />
        <button class="button" type="button" :disabled="forkBlock.isPending.value" @click="forkBlock.mutate()">
          <GitFork :size="16" aria-hidden="true" />
          Fork
        </button>
      </div>

      <section class="revision-list">
        <div class="panel-section__header">
          <h2>历史版本</h2>
          <CopyPlus :size="16" aria-hidden="true" />
        </div>
        <div class="diff-controls">
          <select v-model="diffBaseRevisionId" :disabled="revisions.length < 2">
            <option value="" disabled>旧版本</option>
            <option v-for="revision in revisions" :key="revision.id" :value="revision.id">
              {{ revisionLabel(revision) }}
            </option>
          </select>
          <select v-model="diffTargetRevisionId" :disabled="revisions.length < 2">
            <option value="" disabled>新版本</option>
            <option v-for="revision in revisions" :key="revision.id" :value="revision.id">
              {{ revisionLabel(revision) }}
            </option>
          </select>
        </div>
        <div v-if="diffBaseRevision && diffTargetRevision && diffBaseRevision.id !== diffTargetRevision.id" class="diff-viewer">
          <span v-for="(segment, index) in diffSegments" :key="index" :class="`diff-viewer__${segment.type}`">
            {{ segment.text }}
          </span>
        </div>
        <button
          v-for="revision in revisions"
          :key="revision.id"
          class="revision-list__item"
          :class="{ 'is-current': revision.id === blockDetail.block.current_revision_id }"
          type="button"
          @click="selectRevision.mutate(revision.id)"
        >
          <span>{{ revisionLabel(revision) }}</span>
          <small>{{ revision.content_hash?.slice(0, 10) }}</small>
        </button>
      </section>
    </template>
  </section>
</template>
