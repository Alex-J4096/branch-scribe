<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  AlertCircle,
  Bot,
  Check,
  ChevronDown,
  ChevronRight,
  CopyPlus,
  Eye,
  FileText,
  GitFork,
  MapPin,
  RefreshCw,
  Save,
  Sparkles,
  SquarePen,
  Tags,
} from 'lucide-vue-next'

import { api } from '@/api/client'
import RichTextEditor from '@/components/editor/RichTextEditor.vue'
import type { CanonEntity, ContextPreview, GenerateOnceInput, GenerateOnceResult, ModelProfile, Revision } from '@/api/types'

type InspectorMode = 'all' | 'sidebar' | 'editor' | 'llm'
type InspectorSection = 'title' | 'associations' | 'summary' | 'editor' | 'llm' | 'fork' | 'revisions'

const props = withDefaults(defineProps<{
  projectId: string
  blockId: string
  mode?: InspectorMode
}>(), {
  mode: 'all',
})

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
const selectedCharacterIds = ref<string[]>([])
const selectedLocationId = ref('')
const tagsDraft = ref('')
const generationResult = ref<GenerateOnceResult | null>(null)
const candidateResults = ref<GenerateOnceResult[]>([])
const candidateRevisionIds = ref<string[]>([])
const contextPreview = ref<ContextPreview | null>(null)
const contextPreviewError = ref('')
const excludedContextItemIds = ref<string[]>([])
const showFinalPrompt = ref(false)
const streamingOutput = ref('')
const reasoningOutput = ref('')
const generationError = ref('')
const summaryError = ref('')
const isGenerationStreaming = ref(false)
const draftSavedAt = ref<string | null>(null)
const restoredLocalDraft = ref(false)
const openSections = ref({
  title: true,
  associations: false,
  summary: true,
  editor: true,
  llm: props.mode === 'llm',
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

const charactersQuery = useQuery({
  queryKey: computed(() => ['canon', props.projectId, 'character']),
  queryFn: () => api.listCanonEntities(props.projectId, { type: 'character' }),
})

const locationsQuery = useQuery({
  queryKey: computed(() => ['canon', props.projectId, 'location']),
  queryFn: () => api.listCanonEntities(props.projectId, { type: 'location' }),
})

const summariesQuery = useQuery({
  queryKey: computed(() => ['summaries', props.projectId]),
  queryFn: () => api.listSummaries(props.projectId),
})

const blockDetail = computed(() => blockQuery.data.value)
const revisions = computed(() => revisionsQuery.data.value ?? [])
const modelProfiles = computed(() => (modelProfilesQuery.data.value ?? []).filter((profile) => profile.profile_type === 'llm'))
const characters = computed(() => charactersQuery.data.value ?? [])
const locations = computed(() => locationsQuery.data.value ?? [])
const currentRevision = computed(() => blockDetail.value?.current_revision ?? null)
const currentSummary = computed(() => {
  const block = blockDetail.value?.block
  if (!block) return null
  const targetType = block.type === 'chapter' ? 'chapter' : 'block'
  return (summariesQuery.data.value ?? []).find(
    (summary) => summary.target_type === targetType && summary.target_id === block.id,
  ) ?? null
})
const displayTitle = computed(() => blockDetail.value?.block.title || '无标题片段')
const isContentDirty = computed(() => draftContent.value !== (currentRevision.value?.content ?? ''))
const wordCount = computed(() => countWords(stripHTML(draftContent.value)))
const currentRevisionHash = computed(() => currentRevision.value?.content_hash?.slice(0, 10) ?? 'no hash')
const selectedModelProfile = computed(
  () => modelProfiles.value.find((profile) => profile.id === selectedModelProfileId.value) ?? null,
)
const showInspectorHeader = computed(() => props.mode === 'all' || props.mode === 'sidebar')
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
const savedCharacterIds = computed(() => readStringArray(blockDetail.value?.block.metadata, 'character_ids'))
const savedLocationId = computed(() => readString(blockDetail.value?.block.metadata, 'location_id'))
const savedTags = computed(() => readStringArray(blockDetail.value?.block.metadata, 'tags'))
const associatedCharacters = computed(() =>
  savedCharacterIds.value
    .map((id) => characters.value.find((character) => character.id === id))
    .filter((character): character is CanonEntity => Boolean(character)),
)
const associatedLocation = computed(
  () => locations.value.find((location) => location.id === savedLocationId.value) ?? null,
)
const hasAssociations = computed(
  () => associatedCharacters.value.length > 0 || Boolean(associatedLocation.value) || savedTags.value.length > 0,
)
const includedContextItems = computed(() => contextPreview.value?.items.filter((item) => item.included) ?? [])
const skippedContextItems = computed(() => contextPreview.value?.items.filter((item) => !item.included) ?? [])
const visibleSections = computed<Set<InspectorSection>>(() => {
  switch (props.mode) {
    case 'sidebar':
      return new Set(['title', 'associations', 'summary', 'fork', 'revisions'])
    case 'editor':
      return new Set(['editor'])
    case 'llm':
      return new Set(['llm'])
    case 'all':
    default:
      return new Set(['title', 'associations', 'summary', 'editor', 'llm', 'fork', 'revisions'])
  }
})

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
    candidateResults.value = []
    candidateRevisionIds.value = []
    contextPreview.value = null
    contextPreviewError.value = ''
    excludedContextItemIds.value = []
    showFinalPrompt.value = false
    streamingOutput.value = ''
    reasoningOutput.value = ''
    generationError.value = ''
    generationSelectedText.value = ''
    editorSelectedText.value = ''
  },
)

watch(
  () => blockDetail.value?.block.metadata,
  (metadata) => {
    selectedCharacterIds.value = readStringArray(metadata, 'character_ids')
    selectedLocationId.value = readString(metadata, 'location_id')
    tagsDraft.value = readStringArray(metadata, 'tags').join(', ')
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
    reasoningOutput.value = ''
    generationError.value = ''
    generationSelectedText.value = ''
    clearLocalDraft()
    await refreshInspector()
  },
  onError: (error) => {
    generationError.value = error instanceof Error ? error.message : '保存生成结果失败'
  },
})

const generateCandidates = useMutation({
  mutationFn: () => {
    if (!selectedModelProfileId.value) throw new Error('请先选择可用的模型配置')
    return api.generateCandidates({
      project_id: props.projectId,
      block_id: props.blockId,
      task_type: 'compare_revisions',
      model_profile_id: selectedModelProfileId.value,
      user_instruction: generationInstruction.value.trim(),
      excluded_context_item_ids: excludedContextItemIds.value,
      count: 2,
    })
  },
  onSuccess: (result) => {
    candidateResults.value = result.candidates
    candidateRevisionIds.value = []
    generationError.value = ''
  },
  onError: (error) => {
    generationError.value = error instanceof Error ? error.message : '候选版本生成失败'
  },
})

const saveCandidateRevisions = useMutation({
  mutationFn: async () => Promise.all(candidateResults.value.map((candidate, index) =>
    api.createRevision(props.blockId, {
      parent_revision_id: currentRevision.value?.id ?? null,
      content: textToHtml(candidate.output_text),
      content_format: 'html',
      source: 'llm',
      generation_run_id: candidate.generation_run.id,
      metadata: { task_type: 'compare_revisions', candidate_index: index + 1 },
      set_current: false,
    }),
  )),
  onSuccess: async (saved) => {
    candidateRevisionIds.value = saved.map((revision) => revision.id)
    await refreshInspector()
  },
})

async function chooseCandidate(index: number) {
  const revisionId = candidateRevisionIds.value[index]
  if (!revisionId) return
  await api.selectRevision(props.blockId, revisionId)
  candidateResults.value = []
  candidateRevisionIds.value = []
  await refreshInspector()
}

async function expandCandidate(index: number) {
  const revisionId = candidateRevisionIds.value[index]
  if (!revisionId) return
  await api.forkBlock(props.blockId, {
    title: `${displayTitle.value} · 候选 ${index + 1}`,
    revision_id: revisionId,
    position_x: (blockDetail.value?.block.position_x ?? 0) + 260,
    position_y: (blockDetail.value?.block.position_y ?? 0) + index * 150,
  })
  await refreshInspector()
}

async function expandCandidateToBranch(index: number) {
  const revisionId = candidateRevisionIds.value[index]
  const source = blockDetail.value?.block
  if (!revisionId || !source) return
  const branch = await api.forkBranch(props.projectId, {
    name: `${displayTitle.value} · 候选 ${index + 1}`,
    base_branch_id: source.branch_id,
    fork_from_block_id: source.id,
    fork_from_revision_id: revisionId,
  })
  await api.forkBlock(props.blockId, {
    branch_id: branch.id,
    revision_id: revisionId,
    title: `${displayTitle.value} · 候选 ${index + 1}`,
    position_x: source.position_x + 260,
    position_y: source.position_y + index * 150,
  })
  await refreshInspector()
}

const updateTitle = useMutation({
  mutationFn: () => api.updateBlock(props.blockId, { title: titleDraft.value.trim() || null }),
  onSuccess: refreshInspector,
})

const updateAssociations = useMutation({
  mutationFn: () =>
    api.updateBlockAssociations(props.blockId, {
      character_ids: selectedCharacterIds.value,
      location_id: selectedLocationId.value || null,
      tags: parseTags(tagsDraft.value),
    }),
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

const generateSummary = useMutation({
  mutationFn: () => {
    if (!selectedModelProfileId.value) {
      throw new Error('请先选择可用的模型配置')
    }
    const input = {
      project_id: props.projectId,
      model_profile_id: selectedModelProfileId.value,
    }
    return currentSummary.value
      ? api.refreshSummary(currentSummary.value.id, input)
      : api.generateBlockSummary(props.blockId, input)
  },
  onSuccess: async () => {
    summaryError.value = ''
    await queryClient.invalidateQueries({ queryKey: ['summaries', props.projectId] })
  },
  onError: (error) => {
    summaryError.value = error instanceof Error ? error.message : '摘要生成失败'
    void queryClient.invalidateQueries({ queryKey: ['summaries', props.projectId] })
  },
})

const previewGenerationContext = useMutation({
  mutationFn: () => api.previewGenerationContext(buildGenerateInput()),
  onSuccess: (preview) => {
    contextPreview.value = preview
    contextPreviewError.value = ''
    excludedContextItemIds.value = preview.excluded_item_ids
  },
  onError: (error) => {
    contextPreviewError.value = error instanceof Error ? error.message : '上下文预览失败'
  },
})

async function refreshInspector() {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ['block', props.blockId] }),
    queryClient.invalidateQueries({ queryKey: ['revisions', props.blockId] }),
    queryClient.invalidateQueries({ queryKey: ['graph', props.projectId] }),
    queryClient.invalidateQueries({ queryKey: ['summaries', props.projectId] }),
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

function canonOptionLabel(entity: CanonEntity) {
  return entity.status === 'canon' ? entity.name : `${entity.name} · ${entity.status}`
}

function readString(metadata: Record<string, unknown> | undefined, key: string) {
  const value = metadata?.[key]
  return typeof value === 'string' ? value : ''
}

function readStringArray(metadata: Record<string, unknown> | undefined, key: string) {
  const value = metadata?.[key]
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string' && item.trim() !== '')
}

function parseTags(value: string) {
  const seen = new Set<string>()
  const tags: string[] = []
  for (const tag of value.split(/[,，\n]/)) {
    const trimmed = tag.trim()
    if (!trimmed || seen.has(trimmed)) continue
    seen.add(trimmed)
    tags.push(trimmed)
  }
  return tags
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
  reasoningOutput.value = ''

  try {
    await api.generateStream(
      buildGenerateInput(),
      (event) => {
        if (event.type === 'delta') {
          streamingOutput.value += event.content ?? ''
          return
        }
        if (event.type === 'reasoning') {
          reasoningOutput.value += event.reasoning ?? ''
          return
        }
        if (event.type === 'done' && event.generation_run) {
          generationResult.value = {
            output_text: streamingOutput.value,
            reasoning_text: event.reasoning ?? reasoningOutput.value,
            generation_run: event.generation_run,
            prompt: event.prompt ?? '',
            system_prompt: event.system_prompt ?? '',
            user_prompt: event.user_prompt ?? '',
            context_preview: event.context_preview ?? contextPreview.value ?? emptyContextPreview(),
            model_profile_id: event.model_profile_id ?? selectedModelProfileId.value,
            prompt_template_id: event.prompt_template_id ?? null,
          }
          if (event.context_preview) {
            contextPreview.value = event.context_preview
            excludedContextItemIds.value = event.context_preview.excluded_item_ids
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

function buildGenerateInput(): GenerateOnceInput {
  return {
    project_id: props.projectId,
    block_id: props.blockId,
    task_type: generationTaskType.value,
    model_profile_id: selectedModelProfileId.value,
    selected_text: selectedTextForGeneration.value,
    user_instruction: generationInstruction.value.trim(),
    excluded_context_item_ids: excludedContextItemIds.value,
  }
}

function toggleContextItem(itemId: string) {
  const excluded = new Set(excludedContextItemIds.value)
  if (excluded.has(itemId)) {
    excluded.delete(itemId)
  } else {
    excluded.add(itemId)
  }
  excludedContextItemIds.value = Array.from(excluded)
  if (contextPreview.value) {
    contextPreview.value = {
      ...contextPreview.value,
      excluded_item_ids: excludedContextItemIds.value,
      items: contextPreview.value.items.map((item) =>
        item.id === itemId && !item.required ? { ...item, included: !excluded.has(item.id) } : item,
      ),
    }
  }
  if (canGenerate.value && !previewGenerationContext.isPending.value) {
    window.setTimeout(() => previewGenerationContext.mutate(), 0)
  }
}

function contextItemTypeLabel(type: string) {
  const labels: Record<string, string> = {
    current_block: '当前正文',
    canon: '设定',
    selected_text: '选区',
    recent_block: '最近正文',
    branch_summary: '分支摘要',
    chapter_summary: '章节摘要',
    memory_chunk: '记忆',
  }
  return labels[type] ?? type
}

function emptyContextPreview(): ContextPreview {
  return {
    system_prompt: '',
    user_prompt: '',
    final_prompt: '',
    estimated_tokens: 0,
    token_budget: 0,
    items: [],
    excluded_item_ids: [],
    prompt_template_id: null,
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
      <div v-if="showInspectorHeader" class="inspector__header">
        <div>
          <h2>{{ displayTitle }}</h2>
          <p>{{ blockDetail.block.type }} · {{ revisions.length }} revisions</p>
        </div>
        <SquarePen :size="18" aria-hidden="true" />
      </div>

      <div v-if="showInspectorHeader" class="association-overview" :class="{ 'is-empty': !hasAssociations }">
        <template v-if="hasAssociations">
          <span v-for="character in associatedCharacters" :key="character.id">
            {{ character.name }}
          </span>
          <span v-if="associatedLocation">
            <MapPin :size="13" aria-hidden="true" />
            {{ associatedLocation.name }}
          </span>
          <span v-for="tag in savedTags" :key="tag">
            {{ tag }}
          </span>
        </template>
        <span v-else>未关联 canon</span>
      </div>

      <section v-if="visibleSections.has('title')" class="inspector-section">
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

      <section v-if="visibleSections.has('associations')" class="inspector-section">
        <button class="panel-section__header panel-section__header--button" type="button" @click="toggleSection('associations')">
          <span>
            <ChevronDown v-if="openSections.associations" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>关联</h2>
          </span>
          <Tags :size="16" aria-hidden="true" />
        </button>
        <form v-show="openSections.associations" class="inspector-section__body association-form" @submit.prevent="updateAssociations.mutate()">
          <label class="field-label">
            <span>出现角色</span>
            <select v-model="selectedCharacterIds" multiple class="association-form__multi-select">
              <option v-for="character in characters" :key="character.id" :value="character.id">
                {{ canonOptionLabel(character) }}
              </option>
            </select>
          </label>

          <label class="field-label">
            <span>地点</span>
            <select v-model="selectedLocationId">
              <option value="">未选择地点</option>
              <option v-for="location in locations" :key="location.id" :value="location.id">
                {{ canonOptionLabel(location) }}
              </option>
            </select>
          </label>

          <label class="field-label">
            <span>标签</span>
            <input v-model="tagsDraft" type="text" placeholder="用逗号分隔标签" />
          </label>

          <div class="association-form__summary">
            <span>{{ selectedCharacterIds.length }} 角色</span>
            <span>
              <MapPin :size="13" aria-hidden="true" />
              {{ selectedLocationId ? '已选地点' : '无地点' }}
            </span>
            <span>{{ parseTags(tagsDraft).length }} 标签</span>
          </div>

          <button class="button button--primary" type="submit" :disabled="updateAssociations.isPending.value">
            <Save :size="16" aria-hidden="true" />
            保存关联
          </button>
        </form>
      </section>

      <section v-if="visibleSections.has('summary')" class="inspector-section">
        <button class="panel-section__header panel-section__header--button" type="button" @click="toggleSection('summary')">
          <span>
            <ChevronDown v-if="openSections.summary" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>{{ blockDetail.block.type === 'chapter' ? '章节摘要' : 'Block 摘要' }}</h2>
          </span>
          <FileText :size="16" aria-hidden="true" />
        </button>
        <div v-show="openSections.summary" class="inspector-section__body summary-panel">
          <div v-if="currentSummary" class="summary-panel__status" :class="`is-${currentSummary.status}`">
            <strong>{{ currentSummary.status === 'valid' ? '有效' : currentSummary.status === 'stale' ? '已过期' : '失败' }}</strong>
            <span>{{ currentSummary.token_count }} tokens · {{ currentSummary.covered_revision_ids.length }} revisions</span>
          </div>
          <p v-if="currentSummary" class="summary-panel__text">{{ currentSummary.summary_text }}</p>
          <div v-else class="empty-state empty-state--compact">尚未生成摘要</div>
          <div v-if="currentSummary?.status === 'stale'" class="llm-message llm-message--warning">
            <AlertCircle :size="16" aria-hidden="true" />
            <span>正文 revision 已变化。该摘要仍可用于前文参考，也可以刷新后再使用。</span>
          </div>
          <div v-if="summaryError" class="llm-message llm-message--error">
            <AlertCircle :size="16" aria-hidden="true" />
            <span>{{ summaryError }}</span>
          </div>
          <button
            class="button button--primary"
            type="button"
            :disabled="!selectedModelProfileId || generateSummary.isPending.value"
            @click="generateSummary.mutate()"
          >
            <RefreshCw :size="16" aria-hidden="true" />
            {{ generateSummary.isPending.value ? '生成中' : currentSummary ? '刷新摘要' : '生成摘要' }}
          </button>
        </div>
      </section>

      <section v-if="visibleSections.has('editor')" class="inspector-section">
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

      <section v-if="visibleSections.has('llm')" class="inspector-section">
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

          <div class="context-preview">
            <div class="context-preview__header">
              <div>
                <h3>上下文预览</h3>
                <p v-if="contextPreview">
                  {{ contextPreview.estimated_tokens }} / {{ contextPreview.token_budget }} tokens · {{ includedContextItems.length }} 项已包含
                </p>
                <p v-else>生成前可查看将发送给模型的上下文</p>
              </div>
              <button class="button" type="button" :disabled="!canGenerate || previewGenerationContext.isPending.value" @click="previewGenerationContext.mutate()">
                <Eye :size="16" aria-hidden="true" />
                预览
              </button>
            </div>

            <div v-if="contextPreviewError" class="llm-message llm-message--error">
              <AlertCircle :size="16" aria-hidden="true" />
              <span>{{ contextPreviewError }}</span>
            </div>

            <div v-if="contextPreview" class="context-preview__items">
              <label
                v-for="item in contextPreview.items"
                :key="item.id"
                class="context-item"
                :class="{ 'is-skipped': !item.included }"
              >
                <input
                  type="checkbox"
                  :checked="!excludedContextItemIds.includes(item.id)"
                  :disabled="item.required"
                  @change="toggleContextItem(item.id)"
                />
                <span>
                  <strong>{{ item.title }}</strong>
                  <small>
                    {{ contextItemTypeLabel(item.type) }} · {{ item.estimated_tokens }} tokens ·
                    {{ item.status === 'stale' ? '摘要已过期 · ' : '' }}{{ item.included ? '包含' : '跳过' }}
                  </small>
                  <em>{{ item.content }}</em>
                </span>
              </label>
              <div v-if="skippedContextItems.length" class="context-preview__note">
                {{ skippedContextItems.length }} 项因预算或手动取消未进入最终 prompt。
              </div>
            </div>

            <button v-if="contextPreview" class="button" type="button" @click="showFinalPrompt = !showFinalPrompt">
              {{ showFinalPrompt ? '隐藏最终 Prompt' : '查看最终 Prompt' }}
            </button>
            <div v-if="contextPreview && showFinalPrompt" class="context-preview__prompts">
              <div>
                <strong>System</strong>
                <pre>{{ contextPreview.system_prompt }}</pre>
              </div>
              <div>
                <strong>User</strong>
                <pre>{{ contextPreview.user_prompt }}</pre>
              </div>
              <div>
                <strong>Final</strong>
                <pre>{{ contextPreview.final_prompt }}</pre>
              </div>
            </div>
          </div>

          <div v-if="!modelProfiles.length" class="llm-message llm-message--warning">
            <AlertCircle :size="16" aria-hidden="true" />
            <span>还没有模型配置，请先在模型设置中创建一个可用 profile。</span>
          </div>
          <div v-else-if="selectedModelProfile && !selectedModelProfile.has_api_key" class="llm-message llm-message--warning">
            <AlertCircle :size="16" aria-hidden="true" />
            <span>当前模型没有 API key，生成请求会失败。</span>
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
            <button class="button" type="button" :disabled="!canGenerate || generateCandidates.isPending.value" @click="generateCandidates.mutate()">
              <CopyPlus :size="16" aria-hidden="true" />
              {{ generateCandidates.isPending.value ? '生成两个候选中' : '生成两个候选' }}
            </button>
          </div>

          <div v-if="candidateResults.length === 2" class="candidate-compare">
            <article v-for="(candidate, index) in candidateResults" :key="candidate.generation_run.id" class="candidate-card">
              <strong>候选 {{ index + 1 }}</strong>
              <div class="llm-output__text">{{ candidate.output_text }}</div>
              <div v-if="candidateRevisionIds[index]" class="inspector__actions">
                <button class="button button--primary" type="button" @click="chooseCandidate(index)">选择并继续</button>
                <button class="button" type="button" @click="expandCandidate(index)">展开为 Block</button>
                <button class="button" type="button" @click="expandCandidateToBranch(index)">展开为分支</button>
              </div>
            </article>
            <button
              v-if="candidateRevisionIds.length !== 2"
              class="button button--primary"
              type="button"
              :disabled="saveCandidateRevisions.isPending.value"
              @click="saveCandidateRevisions.mutate()"
            >
              保存为两个 Revision
            </button>
          </div>

          <div v-if="generationResult || streamingOutput || reasoningOutput" class="llm-output">
            <div class="llm-output__meta">
              <span>{{ generationResult?.generation_run.model ?? selectedModelProfile?.model ?? 'streaming' }}</span>
              <span v-if="generationResult">{{ generationResult.generation_run.output_tokens }} output tokens</span>
              <span v-else>streaming</span>
            </div>
            <details v-if="generationResult?.reasoning_text || reasoningOutput" class="llm-reasoning">
              <summary>模型推理内容</summary>
              <div>{{ generationResult?.reasoning_text || reasoningOutput }}</div>
            </details>
            <div v-if="generationResult?.output_text || streamingOutput" class="llm-output__text">
              {{ generationResult?.output_text ?? streamingOutput }}
            </div>
            <div class="inspector__actions">
              <button class="button button--primary" type="button" :disabled="!generationResult || saveGeneratedRevision.isPending.value" @click="saveGeneratedRevision.mutate()">
                <Check :size="16" aria-hidden="true" />
                {{ generationResult?.generation_run.task_type === 'rewrite_selection' ? '替换选区并保存' : '保存为 Revision' }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <section v-if="visibleSections.has('fork')" class="inspector-section">
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

      <section v-if="visibleSections.has('revisions')" class="inspector-section revision-list">
        <button class="panel-section__header panel-section__header--button" type="button" @click="toggleSection('revisions')">
          <span>
            <ChevronDown v-if="openSections.revisions" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>历史版本</h2>
          </span>
          <CopyPlus :size="16" aria-hidden="true" />
        </button>
        <div v-show="openSections.revisions" class="inspector-section__body">
          <div v-if="revisions.length >= 2" class="diff-controls">
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
