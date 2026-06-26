<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { CopyPlus, GitFork, Save, SquarePen } from 'lucide-vue-next'

import { api } from '@/api/client'
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
const forkTitle = ref('')

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

watch(
  currentRevision,
  (revision) => {
    draftContent.value = revision?.content ?? ''
  },
  { immediate: true },
)

const createRevision = useMutation({
  mutationFn: () =>
    api.createRevision(props.blockId, {
      content: draftContent.value,
      content_format: 'markdown',
      source: 'user',
      set_current: true,
    }),
  onSuccess: refreshInspector,
})

const selectRevision = useMutation({
  mutationFn: (revisionId: string) => api.selectRevision(props.blockId, revisionId),
  onSuccess: refreshInspector,
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

function revisionLabel(revision: Revision) {
  const created = new Date(revision.created_at)
  return `${revision.source} · ${created.toLocaleString()}`
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

      <label class="editor-field">
        <span>正文</span>
        <textarea v-model="draftContent" rows="16" spellcheck="false" />
      </label>

      <div class="inspector__actions">
        <button class="button button--primary" type="button" :disabled="createRevision.isPending.value" @click="createRevision.mutate()">
          <Save :size="16" aria-hidden="true" />
          保存 Revision
        </button>
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
