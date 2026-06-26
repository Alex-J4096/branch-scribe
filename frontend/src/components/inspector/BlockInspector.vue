<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { AlertCircle, Bot, Check, ChevronDown, ChevronRight, CopyPlus, FileText, GitFork, Save, Sparkles, SquarePen } from 'lucide-vue-next'

import { api } from '@/api/client'
import RichTextEditor from '@/components/editor/RichTextEditor.vue'
import type { GenerateOnceResult, ModelProfile, Revision } from '@/api/types'

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
const selectedModelProfileId = ref('')
const generationTaskType = ref('continue')
const generationInstruction = ref('')
const generationSelectedText = ref('')
const editorSelectedText = ref('')
const generationResult = ref<GenerateOnceResult | null>(null)
const streamingOutput = ref('')
const generationError = ref('')
const isGenerationStreaming = ref(false)
const draftSavedAt = ref<string | null>(null)
const restoredLocalDraft = ref(false)
const openSections = ref({
  title: true,
  editor: true,
  llm: false,
  fork: false,
  revisions: true,
})
let autosaveTimer: ReturnType<typeof window.setTimeout> | null = null
let generationAbortController: AbortController | null = null
const richTextEditorRef = ref<InstanceType<typeof RichTextEditor> | null>(null)

const generationTasks = [
  { value: 'free_write', label: '自由生成' },
  { value: 'continue', label: '续写' },
  { value: 'rewrite_block', label: '改写' },
  { value: 'rewrite_selection', label: '局部改写' },
  { value: 'expand', label: '扩写' },
  { value: 'condense', label: '缩写' },
  { value: 'polish', label: '润色' },
]

const blockQuery = useQuery({
  queryKey: computed(() => ['block', props.blockId]),
  queryFn: () => api.getBlock(props.blockId),
})

const revisionsQuery = useQuery({
  queryKey: computed(() => ['revisions', props.blockId]),
  queryFn: () => api.listRevisions(props.blockId),
})

const modelProfilesQuery = useQuery({
  queryKey: computed(() => ['model-profiles', props.projectId]),
  queryFn: () => api.listModelProfiles(props.projectId),
})

const blockDetail = computed(() => blockQuery.data.value)
const revisions = computed(() => revisionsQuery.data.value ?? [])
const modelProfiles = computed(() => modelProfilesQuery.data.value ?? [])
const currentRevision = computed(() => blockDetail.value?.current_revision ?? null)
const displayTitle = computed(() => blockDetail.value?.block.title || '无标题片段')
const isContentDirty = computed(() => draftContent.value !== (currentRevision.value?.content ?? ''))
const wordCount = computed(() => countWords(stripHTML(draftContent.value)))
const currentRevisionHash = computed(() => currentRevision.value?.content_hash?.slice(0, 10) ?? 'no hash')
const selectedModelProfile = computed(
  () => modelProfiles.value.find((profile) => profile.id === selectedModelProfileId.value) ?? null,
)
const selectedTextForGeneration = computed(() => {
  if (generationTaskType.value !== 'rewrite_selection') {
    return generationSelectedText.value.trim()
  }
  return (editorSelectedText.value || generationSelectedText.value).trim()
})
const canGenerate = computed(() => {
  if (!selectedModelProfileId.value || !blockDetail.value) return false
  if (generationTaskType.value === 'rewrite_selection') {
    return Boolean(selectedTextForGeneration.value)
  }
  return true
})
const generatedRevisionContent = computed(() => {
  const source = generationResult.value?.output_text || streamingOutput.value
  if (!source) return ''
  const output = textToHtml(source)
  const taskType = generationResult.value?.generation_run.task_type ?? generationTaskType.value
  if (taskType === 'continue' && draftContent.value.trim()) {
    return `${draftContent.value.trim()}\n${output}`
  }
  return output
})
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

watch(
  modelProfiles,
  (value) => {
    if (selectedModelProfileId.value && value.some((profile) => profile.id === selectedModelProfileId.value)) {
      return
    }
    selectedModelProfileId.value = value.find((profile) => profile.has_api_key)?.id ?? value[0]?.id ?? ''
  },
  { immediate: true },
)

watch(
  () => props.blockId,
  () => {
    cancelGeneration()
    generationResult.value = null
    streamingOutput.value = ''
    generationError.value = ''
    generationSelectedText.value = ''
    editorSelectedText.value = ''
  },
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

const saveGeneratedRevision = useMutation({
  mutationFn: () => {
    if (!generationResult.value) {
      throw new Error('没有可保存的生成结果')
    }
    const content = generationResult.value.generation_run.task_type === 'rewrite_selection'
      ? replaceEditorSelectionWithGeneratedContent()
      : generatedRevisionContent.value

    return api.createRevision(props.blockId, {
      parent_revision_id: currentRevision.value?.id ?? null,
      content,
      content_format: 'html',
      source: 'llm',
      generation_run_id: generationResult.value.generation_run.id,
      metadata: {
        task_type: generationResult.value.generation_run.task_type,
        model_profile_id: generationResult.value.model_profile_id,
        prompt_template_id: generationResult.value.prompt_template_id,
      },
      set_current: true,
    })
  },
  onSuccess: async () => {
    generationResult.value = null
    streamingOutput.value = ''
    generationError.value = ''
    generationSelectedText.value = ''
    clearLocalDraft()
    await refreshInspector()
  },
  onError: (error) => {
    generationError.value = error instanceof Error ? error.message : '保存生成结果失败'
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
  cancelGeneration()
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

function modelProfileLabel(profile: ModelProfile) {
  return `${profile.name} · ${profile.provider} · ${profile.model}`
}

function textToHtml(value: string) {
  const paragraphs = value
    .trim()
    .split(/\n{2,}/)
    .map((paragraph) => paragraph.trim())
    .filter(Boolean)

  if (paragraphs.length === 0) {
    return '<p></p>'
  }

  return paragraphs.map((paragraph) => `<p>${escapeHTML(paragraph).replace(/\n/g, '<br>')}</p>`).join('\n')
}

function escapeHTML(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
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

function toggleSection(section: keyof typeof openSections.value) {
  openSections.value[section] = !openSections.value[section]
}

async function startGenerationStream() {
  if (!canGenerate.value || isGenerationStreaming.value) return

  const selectedText = selectedTextForGeneration.value
  if (generationTaskType.value === 'rewrite_selection' && !selectedText) {
    generationError.value = '请先在正文编辑器中选中需要局部改写的文本'
    return
  }

  generationAbortController = new AbortController()
  isGenerationStreaming.value = true
  generationError.value = ''
  generationResult.value = null
  streamingOutput.value = ''

  try {
    await api.generateStream(
      {
        project_id: props.projectId,
        block_id: props.blockId,
        task_type: generationTaskType.value,
        model_profile_id: selectedModelProfileId.value,
        selected_text: selectedText,
        user_instruction: generationInstruction.value.trim(),
      },
      (event) => {
        if (event.type === 'delta') {
          streamingOutput.value += event.content ?? ''
          return
        }
        if (event.type === 'done' && event.generation_run) {
          generationResult.value = {
            output_text: streamingOutput.value,
            generation_run: event.generation_run,
            prompt: event.prompt ?? '',
            model_profile_id: event.model_profile_id ?? selectedModelProfileId.value,
            prompt_template_id: event.prompt_template_id ?? null,
          }
          return
        }
        if (event.type === 'error') {
          generationError.value = event.error ?? '生成失败'
        }
      },
      generationAbortController.signal,
    )
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      generationError.value = '生成已取消'
    } else {
      generationError.value = error instanceof Error ? error.message : '生成失败'
    }
  } finally {
    isGenerationStreaming.value = false
    generationAbortController = null
  }
}

function cancelGeneration() {
  if (generationAbortController) {
    generationAbortController.abort()
    generationAbortController = null
  }
}

function handleEditorSelectionChange(value: string) {
  editorSelectedText.value = value
  if (value) {
    generationSelectedText.value = value
  }
}

function replaceEditorSelectionWithGeneratedContent() {
  const output = generationResult.value?.output_text ?? ''
  const replacementHTML = textToHtml(output)
  const replaced = richTextEditorRef.value?.replaceSelectionWithHTML(replacementHTML)
  if (!replaced) {
    throw new Error('选区已失效，请重新选中文本后保存生成结果')
  }
  draftContent.value = replaced
  return replaced
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

      <section class="inspector-section">
        <button class="panel-section__header panel-section__header--button" type="button" @click="toggleSection('title')">
          <span>
            <ChevronDown v-if="openSections.title" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>标题</h2>
          </span>
          <SquarePen :size="16" aria-hidden="true" />
        </button>
        <form v-show="openSections.title" class="title-form" @submit.prevent="updateTitle.mutate()">
          <input v-model="titleDraft" type="text" placeholder="Block 标题（可选）" />
          <button class="button" type="submit" :disabled="updateTitle.isPending.value">
            <Save :size="16" aria-hidden="true" />
            标题
          </button>
        </form>
      </section>

      <section class="inspector-section">
        <button class="panel-section__header panel-section__header--button" type="button" @click="toggleSection('editor')">
          <span>
            <ChevronDown v-if="openSections.editor" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>正文</h2>
          </span>
          <FileText :size="16" aria-hidden="true" />
        </button>
        <div v-show="openSections.editor" class="inspector-section__body">
          <div class="editor-field">
            <span>正文 · {{ wordCount }} 字 · {{ isContentDirty ? '未保存' : '当前 revision' }}</span>
            <RichTextEditor
              ref="richTextEditorRef"
              v-model="draftContent"
              :content-format="currentRevision?.content_format"
              @selection-change="handleEditorSelectionChange"
            />
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
        </div>
      </section>

      <section class="inspector-section">
        <button class="panel-section__header panel-section__header--button" type="button" @click="toggleSection('llm')">
          <span>
            <ChevronDown v-if="openSections.llm" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>LLM 操作</h2>
          </span>
          <Bot :size="16" aria-hidden="true" />
        </button>
        <div v-show="openSections.llm" class="inspector-section__body llm-panel">
          <label class="field-label">
            <span>模型</span>
            <select v-model="selectedModelProfileId">
              <option value="" disabled>选择模型配置</option>
              <option v-for="profile in modelProfiles" :key="profile.id" :value="profile.id">
                {{ modelProfileLabel(profile) }}
              </option>
            </select>
          </label>

          <div class="llm-task-grid" role="group" aria-label="LLM task type">
            <button
              v-for="task in generationTasks"
              :key="task.value"
              class="button"
              :class="{ 'button--primary': generationTaskType === task.value }"
              type="button"
              @click="generationTaskType = task.value"
            >
              {{ task.label }}
            </button>
          </div>

          <label v-if="generationTaskType === 'rewrite_selection'" class="field-label">
            <span>选中文本</span>
            <textarea v-model="generationSelectedText" rows="4" placeholder="在正文编辑器中选中文本后会自动带入" />
          </label>

          <label class="field-label">
            <span>用户指令</span>
            <textarea v-model="generationInstruction" rows="3" placeholder="补充风格、方向、限制或人物语气" />
          </label>

          <div v-if="!modelProfiles.length" class="llm-message llm-message--warning">
            <AlertCircle :size="16" aria-hidden="true" />
            <span>还没有模型配置，请先在模型设置中创建一个可用 profile。</span>
          </div>
          <div v-else-if="selectedModelProfile && !selectedModelProfile.has_api_key" class="llm-message llm-message--warning">
            <AlertCircle :size="16" aria-hidden="true" />
            <span>当前模型没有 API key 环境变量引用，生成请求会失败。</span>
          </div>
          <div v-if="generationError" class="llm-message llm-message--error">
            <AlertCircle :size="16" aria-hidden="true" />
            <span>{{ generationError }}</span>
          </div>

          <div class="inspector__actions">
            <button class="button button--primary" type="button" :disabled="!canGenerate || isGenerationStreaming" @click="startGenerationStream">
              <Sparkles :size="16" aria-hidden="true" />
              {{ isGenerationStreaming ? '生成中' : '流式生成' }}
            </button>
            <button v-if="isGenerationStreaming" class="button" type="button" @click="cancelGeneration">
              取消
            </button>
          </div>

          <div v-if="generationResult || streamingOutput" class="llm-output">
            <div class="llm-output__meta">
              <span>{{ generationResult?.generation_run.model ?? selectedModelProfile?.model ?? 'streaming' }}</span>
              <span v-if="generationResult">{{ generationResult.generation_run.output_tokens }} output tokens</span>
              <span v-else>streaming</span>
            </div>
            <div class="llm-output__text">{{ generationResult?.output_text ?? streamingOutput }}</div>
            <div class="inspector__actions">
              <button class="button button--primary" type="button" :disabled="!generationResult || saveGeneratedRevision.isPending.value" @click="saveGeneratedRevision.mutate()">
                <Check :size="16" aria-hidden="true" />
                {{ generationResult?.generation_run.task_type === 'rewrite_selection' ? '替换选区并保存' : '保存为 Revision' }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <section class="inspector-section">
        <button class="panel-section__header panel-section__header--button" type="button" @click="toggleSection('fork')">
          <span>
            <ChevronDown v-if="openSections.fork" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>Fork</h2>
          </span>
          <GitFork :size="16" aria-hidden="true" />
        </button>
        <div v-show="openSections.fork" class="fork-form">
          <input v-model="forkTitle" type="text" placeholder="Fork 标题（可选）" />
          <button class="button" type="button" :disabled="forkBlock.isPending.value" @click="forkBlock.mutate()">
            <GitFork :size="16" aria-hidden="true" />
            Fork
          </button>
        </div>
      </section>

      <section class="inspector-section revision-list">
        <button class="panel-section__header panel-section__header--button" type="button" @click="toggleSection('revisions')">
          <span>
            <ChevronDown v-if="openSections.revisions" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>历史版本</h2>
          </span>
          <CopyPlus :size="16" aria-hidden="true" />
        </button>
        <div v-show="openSections.revisions" class="inspector-section__body">
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
        </div>
      </section>
    </template>
  </section>
</template>
