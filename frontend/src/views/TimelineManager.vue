<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, Edit3, Save, Sparkles, Trash2 } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'
import type { TimelineEvent, TimelineEventInput } from '@/api/types'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const projectId = computed(() => String(route.params.projectId))
const editingId = ref('')
const extractBlockId = ref('')
const modelProfileId = ref('')
const form = ref(emptyForm())

const eventsQuery = useQuery({ queryKey: computed(() => ['timeline-events', projectId.value]), queryFn: () => api.listTimelineEvents(projectId.value) })
const blocksQuery = useQuery({ queryKey: computed(() => ['blocks', projectId.value]), queryFn: () => api.listBlocks(projectId.value) })
const canonQuery = useQuery({ queryKey: computed(() => ['canon', projectId.value]), queryFn: () => api.listCanonEntities(projectId.value) })
const profilesQuery = useQuery({ queryKey: computed(() => ['model-profiles', projectId.value]), queryFn: () => api.listModelProfiles(projectId.value) })
const events = computed(() => eventsQuery.data.value ?? [])
const blocks = computed(() => blocksQuery.data.value ?? [])
const canon = computed(() => canonQuery.data.value ?? [])
const profiles = computed(() => (profilesQuery.data.value ?? []).filter((item) => item.profile_type === 'llm'))

const saveEvent = useMutation({
  mutationFn: () => editingId.value ? api.updateTimelineEvent(editingId.value, buildInput()) : api.createTimelineEvent(projectId.value, buildInput()),
  onSuccess: async () => { resetForm(); await queryClient.invalidateQueries({ queryKey: ['timeline-events', projectId.value] }) },
})
const deleteEvent = useMutation({
  mutationFn: (id: string) => api.deleteTimelineEvent(id),
  onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['timeline-events', projectId.value] }) },
})
const extractEvents = useMutation({
  mutationFn: () => api.extractTimelineEvents(projectId.value, extractBlockId.value, modelProfileId.value),
  onSuccess: async (result) => {
    for (const event of result.events) {
      await api.createTimelineEvent(projectId.value, {
        ...event,
        block_id: result.block_id,
        metadata: { source: 'llm_extract', model: result.model, generation_run_id: result.generation_run_id },
      })
    }
    await queryClient.invalidateQueries({ queryKey: ['timeline-events', projectId.value] })
  },
})

function emptyForm() {
  return { title: '', description: '', eventTime: '', sortOrder: 0, blockId: '', canonEntityId: '' }
}
function buildInput(): TimelineEventInput {
  return {
    title: form.value.title.trim(), description: form.value.description.trim() || null,
    event_time: form.value.eventTime.trim() || null, sort_order: form.value.sortOrder,
    block_id: form.value.blockId || null, canon_entity_id: form.value.canonEntityId || null, metadata: {},
  }
}
function editEvent(event: TimelineEvent) {
  editingId.value = event.id
  form.value = {
    title: event.title, description: event.description ?? '', eventTime: event.event_time ?? '',
    sortOrder: event.sort_order, blockId: event.block_id ?? '', canonEntityId: event.canon_entity_id ?? '',
  }
}
function resetForm() { editingId.value = ''; form.value = emptyForm() }
function blockName(id: string | null) { return blocks.value.find((item) => item.id === id)?.title || '未关联 Block' }
function canonName(id: string | null) { return canon.value.find((item) => item.id === id)?.name || '' }
</script>

<template>
  <main class="manager-page">
    <header class="manager-page__topbar">
      <button class="icon-button" type="button" title="返回工作台" @click="router.push({ name: 'workspace', params: { projectId } })"><ArrowLeft :size="18" /></button>
      <div><h1>故事时间线</h1><p>{{ events.length }} events</p></div>
    </header>
    <section class="manager-layout">
      <div class="manager-form-stack">
        <form class="manager-form" @submit.prevent="saveEvent.mutate()">
          <h2>{{ editingId ? '编辑事件' : '新建事件' }}</h2>
          <label class="field-label"><span>标题</span><input v-model="form.title" required /></label>
          <label class="field-label"><span>描述</span><textarea v-model="form.description" rows="4" /></label>
          <div class="manager-form__row">
            <label class="field-label"><span>故事内时间</span><input v-model="form.eventTime" placeholder="第三日午夜" /></label>
            <label class="field-label"><span>排序</span><input v-model.number="form.sortOrder" type="number" /></label>
          </div>
          <label class="field-label"><span>来源 Block</span><select v-model="form.blockId"><option value="">未关联</option><option v-for="block in blocks" :key="block.id" :value="block.id">{{ block.title || '未命名片段' }}</option></select></label>
          <label class="field-label"><span>相关 Canon</span><select v-model="form.canonEntityId"><option value="">未关联</option><option v-for="entity in canon" :key="entity.id" :value="entity.id">{{ entity.name }}</option></select></label>
          <div class="manager-form__actions"><button class="button button--primary" :disabled="!form.title.trim()"><Save :size="15" />保存</button><button v-if="editingId" class="button" type="button" @click="resetForm">取消</button></div>
        </form>
        <section class="manager-form">
          <h2>从 Block 提取事件</h2>
          <label class="field-label"><span>Block</span><select v-model="extractBlockId"><option value="" disabled>选择正文</option><option v-for="block in blocks" :key="block.id" :value="block.id">{{ block.title || '未命名片段' }}</option></select></label>
          <label class="field-label"><span>模型</span><select v-model="modelProfileId"><option value="" disabled>选择模型</option><option v-for="profile in profiles" :key="profile.id" :value="profile.id">{{ profile.name }} · {{ profile.model }}</option></select></label>
          <button class="button" type="button" :disabled="!extractBlockId || !modelProfileId || extractEvents.isPending.value" @click="extractEvents.mutate()"><Sparkles :size="15" />{{ extractEvents.isPending.value ? '提取中' : '提取并加入时间线' }}</button>
        </section>
      </div>
      <section class="manager-list timeline-list">
        <div v-if="!events.length" class="empty-state">还没有时间线事件</div>
        <article v-for="event in events" :key="event.id" class="timeline-event">
          <div class="timeline-event__marker" />
          <div class="timeline-event__content">
            <header><div><small>{{ event.event_time || `顺序 ${event.sort_order}` }}</small><h2>{{ event.title }}</h2></div><div class="manager-form__actions"><button class="icon-button" title="编辑" @click="editEvent(event)"><Edit3 :size="15" /></button><button class="icon-button icon-button--danger" title="删除" @click="deleteEvent.mutate(event.id)"><Trash2 :size="15" /></button></div></header>
            <p v-if="event.description">{{ event.description }}</p>
            <footer><span>{{ blockName(event.block_id) }}</span><span v-if="canonName(event.canon_entity_id)">{{ canonName(event.canon_entity_id) }}</span></footer>
          </div>
        </article>
      </section>
    </section>
  </main>
</template>
