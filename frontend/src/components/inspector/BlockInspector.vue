<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  AlertCircle,
  Bot,
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
  CopyPlus,
  Eye,
  FileText,
  GitFork,
  MapPin,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  Send,
  Settings2,
  SquarePen,
  Tags,
  Trash2,
  Wrench,
} from 'lucide-vue-next'

import { api } from '@/api/client'
import RichTextEditor from '@/components/editor/RichTextEditor.vue'
import type {
  CanonEntity,
  ContextPreview,
  GenerateOnceInput,
  GenerateOnceResult,
  LLMConversationMessage,
  PromptTemplate,
  Revision,
} from '@/api/types'

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
const selectedPromptTemplateId = ref('')
const generationInstruction = ref('')
const generateTwoVersions = ref(false)
const contextNodeCount = ref(1)
const selectedConversationId = ref('')
const pendingUserMessage = ref('')
const editingMessageId = ref('')
const editingMessageContent = ref('')
const savingMessageRevisionId = ref('')
const regeneratingMessageId = ref('')
const temperatureOverride = ref(1)
const topPOverride = ref(1)
const maxTokensOverride = ref(4096)
const generationSelectedText = ref('')
const operationEditorOpen = ref(false)
const operationEditorId = ref('')
const operationNameDraft = ref('')
const operationPromptDraft = ref('')
const operationError = ref('')
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

const promptTemplatesQuery = useQuery({
  queryKey: computed(() => ['prompt-templates', props.projectId]),
  queryFn: () => api.listPromptTemplates(props.projectId),
})

const defaultOperationOrder = ['free_write', 'continue', 'rewrite_block', 'rewrite_selection', 'expand', 'condense', 'polish']

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

const conversationsQuery = useQuery({
  queryKey: computed(() => ['llm-conversations', props.blockId]),
  queryFn: () => api.listLLMConversations(props.blockId),
})

const conversationMessagesQuery = useQuery({
  queryKey: computed(() => ['llm-messages', selectedConversationId.value]),
  queryFn: () => api.listLLMConversationMessages(selectedConversationId.value),
  enabled: computed(() => Boolean(selectedConversationId.value)),
})

const blockDetail = computed(() => blockQuery.data.value)
const revisions = computed(() => revisionsQuery.data.value ?? [])
const modelProfiles = computed(() => (modelProfilesQuery.data.value ?? []).filter((profile) => profile.profile_type === 'llm'))
const promptOperations = computed(() => [...(promptTemplatesQuery.data.value ?? [])].sort((left, right) => {
  const leftIndex = defaultOperationOrder.indexOf(left.task_type)
  const rightIndex = defaultOperationOrder.indexOf(right.task_type)
  if (leftIndex < 0 && rightIndex < 0) return left.created_at.localeCompare(right.created_at)
  if (leftIndex < 0) return 1
  if (rightIndex < 0) return -1
  return leftIndex - rightIndex
}))
const selectedPromptOperation = computed(
  () => promptOperations.value.find((operation) => operation.id === selectedPromptTemplateId.value) ?? null,
)
const characters = computed(() => charactersQuery.data.value ?? [])
const locations = computed(() => locationsQuery.data.value ?? [])
const conversations = computed(() => conversationsQuery.data.value ?? [])
const conversationMessages = computed(() => conversationMessagesQuery.data.value ?? [])
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
  promptOperations,
  (operations) => {
    const selected = operations.find((operation) => operation.id === selectedPromptTemplateId.value)
    if (selected) {
      generationTaskType.value = selected.task_type
      return
    }
    const next = operations.find((operation) => operation.task_type === generationTaskType.value)
      ?? operations.find((operation) => operation.task_type === 'continue')
      ?? operations[0]
    selectedPromptTemplateId.value = next?.id ?? ''
    if (next) generationTaskType.value = next.task_type
  },
  { immediate: true },
)

watch(
  selectedModelProfile,
  (profile) => {
    if (!profile) return
    temperatureOverride.value = profile.temperature
    topPOverride.value = profile.top_p
    maxTokensOverride.value = profile.max_tokens
  },
  { immediate: true },
)

watch(
  conversations,
  (items) => {
    if (selectedConversationId.value && items.some((item) => item.id === selectedConversationId.value)) return
    selectedConversationId.value = items[0]?.id ?? ''
  },
  { immediate: true },
)

watch(selectedConversationId, () => {
  generationResult.value = null
  streamingOutput.value = ''
  reasoningOutput.value = ''
  pendingUserMessage.value = ''
  editingMessageId.value = ''
})

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
    selectedConversationId.value = ''
    pendingUserMessage.value = ''
    editingMessageId.value = ''
  },
)

watch(
  () => props.mode,
  (mode) => {
    if (mode === 'llm') {
      openSections.value.llm = true
    }
  },
)

const createConversation = useMutation({
  mutationFn: () => api.createLLMConversation(props.blockId, { project_id: props.projectId }),
  onSuccess: async (conversation) => {
    selectedConversationId.value = conversation.id
    await queryClient.invalidateQueries({ queryKey: ['llm-conversations', props.blockId] })
  },
})

const deleteConversation = useMutation({
  mutationFn: (conversationId: string) => api.deleteLLMConversation(conversationId),
  onSuccess: async () => {
    selectedConversationId.value = ''
    await queryClient.invalidateQueries({ queryKey: ['llm-conversations', props.blockId] })
  },
})

const updateConversationMessage = useMutation({
  mutationFn: () => api.updateLLMConversationMessage(editingMessageId.value, editingMessageContent.value),
  onSuccess: async () => {
    editingMessageId.value = ''
    editingMessageContent.value = ''
    generationResult.value = null
    streamingOutput.value = ''
    pendingUserMessage.value = ''
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['llm-messages', selectedConversationId.value] }),
      queryClient.invalidateQueries({ queryKey: ['llm-conversations', props.blockId] }),
    ])
  },
})

const savePromptOperation = useMutation({
  mutationFn: () => {
    const name = operationNameDraft.value.trim()
    const templateText = operationPromptDraft.value.trim()
    if (!name || !templateText) throw new Error('操作名称和 Prompt 都不能为空')
    if (operationEditorId.value) {
      return api.updatePromptTemplate(operationEditorId.value, {
        name,
        template_text: templateText,
      })
    }
    return api.createPromptTemplate(props.projectId, {
      name,
      task_type: `custom_${Date.now()}`,
      template_text: templateText,
      is_default: true,
      metadata: { custom: true },
    })
  },
  onSuccess: async (operation) => {
    operationError.value = ''
    operationEditorOpen.value = false
    selectedPromptTemplateId.value = operation.id
    generationTaskType.value = operation.task_type
    await queryClient.invalidateQueries({ queryKey: ['prompt-templates', props.projectId] })
  },
  onError: (error) => {
    operationError.value = error instanceof Error ? error.message : '保存操作失败'
  },
})

const deletePromptOperation = useMutation({
  mutationFn: (operation: PromptTemplate) => api.deletePromptTemplate(operation.id),
  onSuccess: async () => {
    selectedPromptTemplateId.value = ''
    operationEditorOpen.value = false
    await queryClient.invalidateQueries({ queryKey: ['prompt-templates', props.projectId] })
  },
  onError: (error) => {
    operationError.value = error instanceof Error ? error.message : '删除操作失败'
  },
})

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
    pendingUserMessage.value = ''
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
  mutationFn: async () => {
    if (!selectedModelProfileId.value) throw new Error('请先选择可用的模型配置')
    if (!generationInstruction.value.trim()) throw new Error('请输入想和 Agent 讨论或执行的内容')
    if (!selectedConversationId.value) {
      const conversation = await api.createLLMConversation(props.blockId, { project_id: props.projectId })
      selectedConversationId.value = conversation.id
      await queryClient.invalidateQueries({ queryKey: ['llm-conversations', props.blockId] })
    } else {
      await queryClient.invalidateQueries({ queryKey: ['llm-messages', selectedConversationId.value] })
    }
    const input = buildGenerateInput()
    return api.generateCandidates({
      ...input,
      count: 2,
    })
  },
  onSuccess: async (result) => {
    candidateResults.value = result.candidates
    candidateRevisionIds.value = []
    generationInstruction.value = ''
    generationError.value = ''
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['llm-messages', selectedConversationId.value] }),
      queryClient.invalidateQueries({ queryKey: ['llm-conversations', props.blockId] }),
    ])
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

async function startGenerationStream(regeneration?: { instruction: string; messageId: string }) {
  if (!canGenerate.value || isGenerationStreaming.value) return
  const instruction = regeneration?.instruction.trim() ?? generationInstruction.value.trim()
  if (!instruction) {
    generationError.value = '请输入想和 Agent 讨论或执行的内容'
    return
  }

  const selectedText = selectedTextForGeneration.value
  if (generationTaskType.value === 'rewrite_selection' && !selectedText) {
    generationError.value = '请先在正文编辑器中选中需要局部改写的文本'
    return
  }

  if (!selectedConversationId.value) {
    const conversation = await api.createLLMConversation(props.blockId, { project_id: props.projectId })
    selectedConversationId.value = conversation.id
    await queryClient.invalidateQueries({ queryKey: ['llm-conversations', props.blockId] })
  } else {
    await queryClient.invalidateQueries({ queryKey: ['llm-messages', selectedConversationId.value] })
  }

  generationAbortController = new AbortController()
  isGenerationStreaming.value = true
  generationError.value = ''
  generationResult.value = null
  streamingOutput.value = ''
  reasoningOutput.value = ''
  regeneratingMessageId.value = regeneration?.messageId ?? ''
  pendingUserMessage.value = regeneration ? '' : instruction

  try {
    await api.generateStream(
      buildGenerateInput(instruction, regeneration?.messageId),
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
            conversation_id: event.conversation_id ?? selectedConversationId.value ?? null,
          }
          if (event.context_preview) {
            contextPreview.value = event.context_preview
            excludedContextItemIds.value = event.context_preview.excluded_item_ids
          }
          if (!regeneration) generationInstruction.value = ''
          pendingUserMessage.value = ''
          void Promise.all([
            queryClient.invalidateQueries({ queryKey: ['llm-messages', selectedConversationId.value] }),
            queryClient.invalidateQueries({ queryKey: ['llm-conversations', props.blockId] }),
          ])
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
    if (generationResult.value && selectedConversationId.value) {
      await queryClient.invalidateQueries({ queryKey: ['llm-messages', selectedConversationId.value] })
      streamingOutput.value = ''
      reasoningOutput.value = ''
    }
    if (regeneration) {
      regeneratingMessageId.value = ''
    }
    generationAbortController = null
  }
}

function buildGenerateInput(instruction = generationInstruction.value.trim(), regenerateMessageId?: string): GenerateOnceInput {
  return {
    project_id: props.projectId,
    block_id: props.blockId,
    task_type: generationTaskType.value,
    model_profile_id: selectedModelProfileId.value,
    prompt_template_id: selectedPromptTemplateId.value || null,
    selected_text: selectedTextForGeneration.value,
    user_instruction: instruction,
    context_node_count: contextNodeCount.value,
    conversation_id: selectedConversationId.value || null,
    temperature: temperatureOverride.value,
    top_p: topPOverride.value,
    max_tokens: maxTokensOverride.value,
    excluded_context_item_ids: excludedContextItemIds.value,
    regenerate_message_id: regenerateMessageId,
  }
}

function selectPromptOperation(operation: PromptTemplate) {
  selectedPromptTemplateId.value = operation.id
  generationTaskType.value = operation.task_type
  operationEditorOpen.value = false
}

function beginCreatePromptOperation() {
  operationEditorId.value = ''
  operationNameDraft.value = ''
  operationPromptDraft.value = '请根据用户指令完成写作任务。\n\n硬设定：\n{{canon_facts}}\n\n最近正文：\n{{recent_blocks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'
  operationError.value = ''
  operationEditorOpen.value = true
}

function beginEditPromptOperation(operation: PromptTemplate) {
  operationEditorId.value = operation.id
  operationNameDraft.value = operation.name
  operationPromptDraft.value = operation.template_text
  operationError.value = ''
  operationEditorOpen.value = true
}

function removeEditedPromptOperation() {
  const operation = promptOperations.value.find((item) => item.id === operationEditorId.value)
  if (operation) deletePromptOperation.mutate(operation)
}

async function copyMessage(content: string) {
  await navigator.clipboard.writeText(content)
}

async function saveAssistantMessageAsRevision(message: LLMConversationMessage) {
  if (message.role !== 'assistant') return
  savingMessageRevisionId.value = message.id
  generationError.value = ''
  try {
    await api.createRevision(props.blockId, {
      parent_revision_id: currentRevision.value?.id ?? null,
      content: textToHtml(message.content),
      content_format: 'html',
      source: 'llm',
      generation_run_id: message.generation_run_id,
      metadata: {
        source_message_id: message.id,
        source_conversation_id: message.conversation_id,
      },
      set_current: true,
    })
    clearLocalDraft()
    await refreshInspector()
  } catch (error) {
    generationError.value = error instanceof Error ? error.message : '保存回复为 Revision 失败'
  } finally {
    savingMessageRevisionId.value = ''
  }
}

function beginEditMessage(message: LLMConversationMessage) {
  editingMessageId.value = message.id
  editingMessageContent.value = message.content
}

function cancelEditMessage() {
  editingMessageId.value = ''
  editingMessageContent.value = ''
}

function regenerateMessage(message: LLMConversationMessage) {
  if (isGenerationStreaming.value || generateCandidates.isPending.value) return
  const messageIndex = conversationMessages.value.findIndex((item) => item.id === message.id)
  const target = message.role === 'assistant'
    ? message
    : conversationMessages.value.slice(messageIndex + 1).find((item) => item.role === 'assistant')
  const source = message.role === 'user'
    ? message
    : conversationMessages.value.slice(0, messageIndex).reverse().find((item) => item.role === 'user')
  if (!source || !target) {
    generationError.value = '未找到这条消息对应的用户输入或 Agent 回复'
    return
  }
  void startGenerationStream({ instruction: source.content, messageId: target.id })
}

function submitComposer() {
  if (generateTwoVersions.value) {
    if (!generateCandidates.isPending.value) generateCandidates.mutate()
  } else if (!isGenerationStreaming.value) {
    void startGenerationStream()
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

function updateContextNodeCount(event: Event) {
  contextNodeCount.value = Math.max(0, Number((event.target as HTMLInputElement).value) || 0)
  contextPreview.value = null
}

function toggleAllContextNodes(event: Event) {
  contextNodeCount.value = (event.target as HTMLInputElement).checked ? -1 : 1
  contextPreview.value = null
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
  <section class="inspector" :class="{ 'inspector--single-tool': props.mode === 'editor' || props.mode === 'llm' }">
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

      <section
        v-if="visibleSections.has('editor')"
        class="inspector-section"
        :class="{ 'inspector-section--single': props.mode === 'editor' }"
      >
        <button
          v-if="props.mode !== 'editor'"
          class="panel-section__header panel-section__header--button"
          type="button"
          @click="toggleSection('editor')"
        >
          <span>
            <ChevronDown v-if="openSections.editor" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>正文</h2>
          </span>
          <FileText :size="16" aria-hidden="true" />
        </button>
        <div v-show="props.mode === 'editor' || openSections.editor" class="inspector-section__body inspector-editor">
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

      <section
        v-if="visibleSections.has('llm')"
        class="inspector-section"
        :class="{ 'inspector-section--single': props.mode === 'llm' }"
      >
        <button
          v-if="props.mode !== 'llm'"
          class="panel-section__header panel-section__header--button"
          type="button"
          @click="toggleSection('llm')"
        >
          <span>
            <ChevronDown v-if="openSections.llm" :size="16" aria-hidden="true" />
            <ChevronRight v-else :size="16" aria-hidden="true" />
            <h2>LLM 操作</h2>
          </span>
          <Bot :size="16" aria-hidden="true" />
        </button>
        <div v-show="props.mode === 'llm' || openSections.llm" class="inspector-section__body llm-panel llm-chat">
          <header class="llm-chat__header">
            <select v-model="selectedConversationId" aria-label="选择对话">
              <option value="">新对话</option>
              <option v-for="conversation in conversations" :key="conversation.id" :value="conversation.id">
                {{ conversation.title }}
              </option>
            </select>
            <button class="icon-button" type="button" title="新建对话" @click="createConversation.mutate()">
              <Plus :size="16" aria-hidden="true" />
            </button>
            <button
              class="icon-button"
              type="button"
              title="删除当前对话"
              :disabled="!selectedConversationId"
              @click="selectedConversationId && deleteConversation.mutate(selectedConversationId)"
            >
              <Trash2 :size="16" aria-hidden="true" />
            </button>
          </header>

          <div class="llm-chat__messages">
            <div v-if="!conversationMessages.length && !pendingUserMessage && !streamingOutput" class="llm-chat__empty">
              <Bot :size="24" aria-hidden="true" />
              <strong>和写作 Agent 继续推演</strong>
              <span>可以讨论情节、要求续写，或在多轮对话后把满意的回复保存为 Revision。</span>
            </div>

            <article
              v-for="message in conversationMessages"
              :key="message.id"
              class="chat-message"
              :class="`chat-message--${message.role}`"
            >
              <div class="chat-message__role">
                {{ message.role === 'user' ? '你' : `Agent${message.model ? ` · ${message.model}` : ''}` }}
              </div>
              <template v-if="editingMessageId === message.id">
                <textarea v-model="editingMessageContent" class="chat-message__editor" rows="5" />
                <div class="chat-message__edit-actions">
                  <small>仅保存当前消息，不影响后续对话</small>
                  <button class="button" type="button" @click="cancelEditMessage">取消</button>
                  <button class="button button--primary" type="button" @click="updateConversationMessage.mutate()">保存</button>
                </div>
              </template>
              <template v-else>
                <div class="chat-message__content">
                  {{ regeneratingMessageId === message.id ? (streamingOutput || '正在重新生成…') : message.content }}
                </div>
                <div class="chat-message__actions">
                  <button class="chat-message__action" type="button" title="复制消息" @click="copyMessage(message.content)">
                    <Copy :size="14" aria-hidden="true" />
                  </button>
                  <button
                    class="chat-message__action"
                    type="button"
                    title="编辑消息"
                    @click="beginEditMessage(message)"
                  >
                    <Pencil :size="14" aria-hidden="true" />
                  </button>
                  <button
                    class="chat-message__action"
                    type="button"
                    title="重新生成"
                    :disabled="isGenerationStreaming || generateCandidates.isPending.value"
                    @click="regenerateMessage(message)"
                  >
                    <RefreshCw :size="14" aria-hidden="true" />
                  </button>
                  <button
                    v-if="message.role === 'assistant'"
                    class="chat-message__action"
                    type="button"
                    title="保存为 Revision"
                    :disabled="savingMessageRevisionId === message.id"
                    @click="saveAssistantMessageAsRevision(message)"
                  >
                    <Save :size="14" aria-hidden="true" />
                  </button>
                </div>
              </template>
            </article>

            <article v-if="pendingUserMessage" class="chat-message chat-message--user">
              <div class="chat-message__role">你</div>
              <div class="chat-message__content">{{ pendingUserMessage }}</div>
            </article>
            <article
              v-if="!regeneratingMessageId && (streamingOutput || isGenerationStreaming)"
              class="chat-message chat-message--assistant"
            >
              <div class="chat-message__role">Agent{{ selectedModelProfile?.model ? ` · ${selectedModelProfile.model}` : '' }}</div>
              <div class="chat-message__content">{{ streamingOutput || '正在思考…' }}</div>
              <div v-if="streamingOutput" class="chat-message__actions">
                <button class="chat-message__action" type="button" title="复制消息" @click="copyMessage(streamingOutput)">
                  <Copy :size="14" aria-hidden="true" />
                </button>
                <button
                  v-if="generationResult"
                  class="chat-message__action"
                  type="button"
                  title="保存为 Revision"
                  :disabled="saveGeneratedRevision.isPending.value"
                  @click="saveGeneratedRevision.mutate()"
                >
                  <Save :size="14" aria-hidden="true" />
                </button>
              </div>
            </article>
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

          <div class="llm-composer">
            <textarea
              v-model="generationInstruction"
              rows="3"
              placeholder="给 Agent 发消息，Enter 发送，Shift + Enter 换行"
              @keydown.enter.exact.prevent="submitComposer"
            />
            <label v-if="generationTaskType === 'rewrite_selection'" class="llm-composer__selection">
              <span>待改写选区</span>
              <textarea v-model="generationSelectedText" rows="2" placeholder="请先在正文中选择文本" />
            </label>
            <div class="llm-composer__toolbar">
              <select v-model="selectedModelProfileId" class="llm-composer__model" aria-label="快捷模型切换">
                <option value="" disabled>选择模型</option>
                <option v-for="profile in modelProfiles" :key="profile.id" :value="profile.id">
                  {{ profile.name }} · {{ profile.model }}
                </option>
              </select>

              <details class="llm-tool-menu">
                <summary><Wrench :size="15" aria-hidden="true" />{{ selectedPromptOperation?.name ?? '写作操作' }}</summary>
                <div class="llm-tool-menu__panel llm-tool-menu__operations">
                  <template v-if="!operationEditorOpen">
                    <header class="llm-menu-header">
                      <div><strong>写作操作</strong><small>选择操作，或编辑它背后的 Prompt</small></div>
                      <button class="llm-menu-icon" type="button" title="新建操作" @click="beginCreatePromptOperation">
                        <Plus :size="16" aria-hidden="true" />
                      </button>
                    </header>
                    <div class="llm-operation-list">
                      <button
                        v-for="operation in promptOperations"
                        :key="operation.id"
                        class="llm-operation"
                        :class="{ 'llm-operation--active': selectedPromptTemplateId === operation.id }"
                        type="button"
                        @click="selectPromptOperation(operation)"
                      >
                        <span class="llm-operation__mark"><Check v-if="selectedPromptTemplateId === operation.id" :size="13" /></span>
                        <span><strong>{{ operation.name }}</strong><small>{{ operation.metadata.built_in ? '默认操作' : '自定义操作' }}</small></span>
                        <span class="llm-operation__edit" title="编辑 Prompt" @click.stop="beginEditPromptOperation(operation)">
                          <Pencil :size="14" aria-hidden="true" />
                        </span>
                      </button>
                    </div>
                    <div v-if="!promptOperations.length" class="llm-menu-empty">还没有写作操作，点击右上角 ＋ 创建。</div>
                  </template>
                  <form v-else class="llm-operation-editor" @submit.prevent="savePromptOperation.mutate()">
                    <header class="llm-menu-header">
                      <div><strong>{{ operationEditorId ? '编辑写作操作' : '新建写作操作' }}</strong><small>模板变量会在发送前自动替换</small></div>
                    </header>
                    <label><span>操作名称</span><input v-model="operationNameDraft" placeholder="例如：转换为第一人称" /></label>
                    <label><span>Prompt</span><textarea v-model="operationPromptDraft" rows="12" /></label>
                    <p class="llm-operation-editor__hint">
                      可用变量：<code v-pre>{{current_block}}</code>、<code v-pre>{{recent_blocks}}</code>、<code v-pre>{{canon_facts}}</code>、<code v-pre>{{selected_text}}</code>、<code v-pre>{{user_instruction}}</code>
                    </p>
                    <p v-if="operationError" class="llm-operation-editor__error">{{ operationError }}</p>
                    <footer class="llm-operation-editor__actions">
                      <button
                        v-if="operationEditorId"
                        class="llm-menu-danger"
                        type="button"
                        :disabled="deletePromptOperation.isPending.value"
                        @click="removeEditedPromptOperation"
                      >
                        <Trash2 :size="14" />删除
                      </button>
                      <span />
                      <button class="button" type="button" @click="operationEditorOpen = false">取消</button>
                      <button class="button button--primary" type="submit" :disabled="savePromptOperation.isPending.value">保存</button>
                    </footer>
                  </form>
                </div>
              </details>

              <details class="llm-tool-menu">
                <summary><Eye :size="15" aria-hidden="true" />上下文</summary>
                <div class="llm-tool-menu__panel llm-tool-menu__context">
                  <header class="llm-menu-header">
                    <div><strong>本轮上下文</strong><small>控制 Agent 能看到的故事范围</small></div>
                  </header>
                  <div class="llm-context-section">
                    <span class="llm-context-section__label">故事线前文</span>
                  <div class="context-node-control">
                    <input
                      class="context-node-control__count"
                      :value="contextNodeCount < 0 ? 1 : contextNodeCount"
                      type="number"
                      min="0"
                      aria-label="前文节点数量"
                      :disabled="contextNodeCount < 0"
                      @input="updateContextNodeCount"
                    />
                    <label class="context-node-control__all">
                      <input type="checkbox" :checked="contextNodeCount < 0" @change="toggleAllContextNodes" />
                      <span>全部故事线前文</span>
                    </label>
                  </div>
                  </div>
                  <div class="llm-context-preview-head">
                    <span>上下文项目</span>
                    <button type="button" @click="previewGenerationContext.mutate()"><RefreshCw :size="13" />刷新预览</button>
                  </div>
                  <small v-if="contextPreview" class="llm-context-budget">{{ contextPreview.estimated_tokens }} / {{ contextPreview.token_budget }} tokens · 已选择 {{ includedContextItems.length }} 项</small>
                  <div class="llm-context-items">
                  <label v-for="item in contextPreview?.items ?? []" :key="item.id" class="context-item llm-context-item">
                    <input
                      type="checkbox"
                      :checked="!excludedContextItemIds.includes(item.id)"
                      :disabled="item.required"
                      @change="toggleContextItem(item.id)"
                    />
                    <span><strong>{{ item.title }}</strong><small>{{ contextItemTypeLabel(item.type) }} · {{ item.estimated_tokens }} tokens</small></span>
                  </label>
                  </div>
                </div>
              </details>

              <details class="llm-tool-menu">
                <summary><Settings2 :size="15" aria-hidden="true" />参数</summary>
                <div class="llm-tool-menu__panel llm-tool-menu__params">
                  <label><span>Temperature</span><input v-model.number="temperatureOverride" type="number" min="0" max="2" step="0.1" /></label>
                  <label><span>Top P</span><input v-model.number="topPOverride" type="number" min="0" max="1" step="0.05" /></label>
                  <label><span>Max tokens</span><input v-model.number="maxTokensOverride" type="number" min="1" step="128" /></label>
                </div>
              </details>

              <button
                class="llm-version-toggle"
                :class="{ 'llm-version-toggle--active': generateTwoVersions }"
                type="button"
                :aria-pressed="generateTwoVersions"
                title="开启后每次发送生成两个候选版本"
                @click="generateTwoVersions = !generateTwoVersions"
              >
                <CopyPlus :size="14" aria-hidden="true" />
                双版本
              </button>

              <button v-if="isGenerationStreaming" class="llm-composer__send" type="button" title="停止" @click="cancelGeneration">■</button>
              <button
                v-else
                class="llm-composer__send"
                type="button"
                :title="generateTwoVersions ? '生成两个版本' : '发送'"
                :disabled="!canGenerate || !generationInstruction.trim() || generateCandidates.isPending.value"
                @click="submitComposer"
              >
                <RefreshCw v-if="generateCandidates.isPending.value" class="spin" :size="16" aria-hidden="true" />
                <CopyPlus v-else-if="generateTwoVersions" :size="16" aria-hidden="true" />
                <Send v-else :size="17" aria-hidden="true" />
              </button>
            </div>
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
