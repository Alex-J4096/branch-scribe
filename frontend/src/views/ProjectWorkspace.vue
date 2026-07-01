<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  ArrowLeft,
  BookOpen,
  ChevronDown,
  ChevronRight,
  Database,
  GitBranch,
  Layers3,
  Link2,
  MapPin,
  Move,
  Pencil,
  MessageSquareText,
  ExternalLink,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightOpen,
  Plus,
  RefreshCw,
  Save,
  Settings,
  Telescope,
  Clock3,
  Download,
  Trash2,
  X,
} from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'
import BlockGraph from '@/components/graph/BlockGraph.vue'
import BlockInspector from '@/components/inspector/BlockInspector.vue'
import { useWorkspaceStore } from '@/stores/workspace'
import type { CreateBlockInput, GraphEdge } from '@/api/types'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const workspace = useWorkspaceStore()

const projectId = computed(() => String(route.params.projectId))
const newBlockTitle = ref('')
const selectedBranchId = ref<string | null>(null)
const edgeSourceBlockId = ref('')
const edgeTargetBlockId = ref('')
const edgeType = ref<GraphEdge['edge_type']>('next')
const edgeLabel = ref('')
const selectedEdgeId = ref<string | null>(null)
const selectedEdgeType = ref<GraphEdge['edge_type']>('next')
const selectedEdgeLabel = ref('')
const selectedSummaryModelProfileId = ref('')
const branchSummarySourceMode = ref<'full_text' | 'prefer_block_summaries' | 'block_summaries_only'>('full_text')
const selectedBranchSummaryPromptId = ref('')
const branchSummarySourceSelections = ref<Record<string, 'full_text' | 'summary' | 'exclude'>>({})
const isSummarySettingsOpen = ref(false)
const summarySettingsMessage = ref('')
const branchSummaryError = ref('')
const isEditingBranch = ref(false)
const branchEditName = ref('')
const branchEditDescription = ref('')
const branchActionError = ref('')
const isLeftDrawerOpen = ref(true)
const isToolWindowOpen = ref(true)
const activeWorkspacePanel = ref<'sidebar' | 'editor' | 'llm'>('editor')
const canvasRef = ref<HTMLElement | null>(null)
const toolWindowRef = ref<HTMLElement | null>(null)
const toolWindowPosition = ref<{ x: number; y: number } | null>(null)
let toolDrag: { pointerId: number; offsetX: number; offsetY: number } | null = null
let branchSummaryAbortController: AbortController | null = null
const openLeftSections = ref({
  branches: true,
  createBlock: true,
  blockList: true,
  edgeManager: true,
  createEdge: false,
})

const edgeTypes: Array<{ value: GraphEdge['edge_type']; label: string }> = [
  { value: 'next', label: 'Next' },
  { value: 'alternative', label: 'Alternative' },
  { value: 'references', label: 'References' },
  { value: 'summarizes', label: 'Summarizes' },
  { value: 'fork', label: 'Fork' },
]

const projectQuery = useQuery({
  queryKey: computed(() => ['project', projectId.value]),
  queryFn: () => api.getProject(projectId.value),
})

const branchesQuery = useQuery({
  queryKey: computed(() => ['branches', projectId.value]),
  queryFn: () => api.listBranches(projectId.value),
})

const graphQuery = useQuery({
  queryKey: computed(() => ['graph', projectId.value]),
  queryFn: () => api.getGraph(projectId.value),
})

const summariesQuery = useQuery({
  queryKey: computed(() => ['summaries', projectId.value]),
  queryFn: () => api.listSummaries(projectId.value),
})

const modelProfilesQuery = useQuery({
  queryKey: ['model-profiles'],
  queryFn: api.listModelProfiles,
})
const promptTemplatesQuery = useQuery({
  queryKey: computed(() => ['prompt-templates', projectId.value]),
  queryFn: () => api.listPromptTemplates(projectId.value),
})
const branchSummaryPrompts = computed(() =>
  (promptTemplatesQuery.data.value ?? []).filter((template) => template.task_type === 'branch_summary'),
)
const defaultBranchSummaryPrompt = computed(() =>
  branchSummaryPrompts.value.find((template) => template.is_default) ?? null,
)

const branches = computed(() => branchesQuery.data.value ?? [])
const branchPalette = ['#2f7d76', '#9b6b28', '#7a4fa3', '#466987', '#b64f6b', '#607449']
const branchColors = computed(() => Object.fromEntries(
  branches.value.map((branch, index) => [branch.id, branchPalette[index % branchPalette.length]]),
))
const graph = computed(() => graphQuery.data.value ?? { nodes: [], edges: [] })
const blocks = computed(() => graph.value.nodes)
const selectedBranchSummary = computed(() =>
  (summariesQuery.data.value ?? []).find(
    (summary) => summary.target_type === 'branch' && summary.target_id === selectedBranchId.value,
  ) ?? null,
)
const selectedBranch = computed(() =>
  branches.value.find((branch) => branch.id === selectedBranchId.value) ?? null,
)
const selectedBranchBlockCount = computed(() =>
  blocks.value.filter((block) => block.branch_id === selectedBranchId.value).length,
)
const selectedBranchBlocks = computed(() =>
  blocks.value
    .filter((block) => block.branch_id === selectedBranchId.value)
    .sort((left, right) => left.order_index - right.order_index || left.created_at.localeCompare(right.created_at)),
)
const validBlockSummaryIds = computed(() => new Set(
  (summariesQuery.data.value ?? [])
    .filter((summary) => summary.status === 'valid' && (summary.target_type === 'block' || summary.target_type === 'chapter'))
    .map((summary) => summary.target_id),
))
const includedSummarySourceCount = computed(() =>
  selectedBranchBlocks.value.filter((block) => branchSummarySourceSelections.value[block.id] !== 'exclude').length,
)
const compressedSummarySourceCount = computed(() =>
  selectedBranchBlocks.value.filter((block) => branchSummarySourceSelections.value[block.id] === 'summary').length,
)

function branchSummaryStatus(branchId: string) {
  const summary = (summariesQuery.data.value ?? []).find(
    (item) => item.target_type === 'branch' && item.target_id === branchId,
  )
  if (!summary) return '无摘要'
  return summary.status === 'valid' ? '摘要有效' : summary.status === 'stale' ? '摘要过期' : '摘要失败'
}

watch(
  branches,
  (value) => {
    if (!selectedBranchId.value && value[0]) {
      selectedBranchId.value = value[0].id
    }
  },
  { immediate: true },
)

function hydrateBranchSummarySettings() {
    const branchSettings = selectedBranch.value?.metadata?.summary_settings
    const settings = branchSettings && typeof branchSettings === 'object'
      ? branchSettings as Record<string, unknown>
      : null
    const savedSelections = settings?.source_selections
      ?? selectedBranchSummary.value?.metadata?.source_selections
    const next: Record<string, 'full_text' | 'summary' | 'exclude'> = {}
    if (Array.isArray(savedSelections)) {
      for (const item of savedSelections) {
        if (
          item && typeof item === 'object'
          && typeof item.block_id === 'string'
          && (item.mode === 'full_text' || item.mode === 'summary' || item.mode === 'exclude')
        ) {
          next[item.block_id] = item.mode
        }
      }
    }
    for (const block of selectedBranchBlocks.value) {
      if (!next[block.id]) next[block.id] = 'full_text'
      if (next[block.id] === 'summary' && !validBlockSummaryIds.value.has(block.id)) {
        next[block.id] = 'full_text'
      }
    }
    branchSummarySourceSelections.value = next
    const savedPromptID = settings?.prompt_template_id
      ?? selectedBranchSummary.value?.metadata?.prompt_template_id
    selectedBranchSummaryPromptId.value = typeof savedPromptID === 'string' ? savedPromptID : ''
    const savedModelID = settings?.model_profile_id
    if (typeof savedModelID === 'string') selectedSummaryModelProfileId.value = savedModelID
    const savedSourceMode = settings?.source_mode
    if (
      savedSourceMode === 'full_text'
      || savedSourceMode === 'prefer_block_summaries'
      || savedSourceMode === 'block_summaries_only'
    ) {
      branchSummarySourceMode.value = savedSourceMode
    }
}

watch(
  [selectedBranchId, selectedBranchBlocks, selectedBranchSummary, selectedBranch],
  () => {
    if (!isSummarySettingsOpen.value) hydrateBranchSummarySettings()
  },
  { immediate: true },
)

function openSummarySettings() {
  hydrateBranchSummarySettings()
  summarySettingsMessage.value = ''
  isSummarySettingsOpen.value = true
}

function closeSummarySettings() {
  hydrateBranchSummarySettings()
  summarySettingsMessage.value = ''
  isSummarySettingsOpen.value = false
}

function applySummarySourcePreset(mode: 'full_text' | 'prefer_block_summaries' | 'block_summaries_only') {
  branchSummarySourceMode.value = mode
  branchSummarySourceSelections.value = Object.fromEntries(
    selectedBranchBlocks.value.map((block) => {
      if (mode === 'full_text') return [block.id, 'full_text']
      if (validBlockSummaryIds.value.has(block.id)) return [block.id, 'summary']
      return [block.id, mode === 'block_summaries_only' ? 'exclude' : 'full_text']
    }),
  )
}

function openSummaryPromptManager() {
  void router.push({
    name: 'prompt-settings',
    params: { projectId: projectId.value },
    query: { task: 'branch_summary', from: route.fullPath },
  })
}

watch(
  () => (modelProfilesQuery.data.value ?? []).filter((profile) => profile.profile_type === 'llm'),
  (profiles) => {
    if (!profiles.some((profile) => profile.id === selectedSummaryModelProfileId.value)) {
      selectedSummaryModelProfileId.value = profiles.find((profile) => profile.has_api_key)?.id ?? profiles[0]?.id ?? ''
    }
  },
  { immediate: true },
)

const selectedBlock = computed(() => graph.value.nodes.find((node) => node.id === workspace.selectedBlockId) ?? null)
const selectedEdge = computed(() => graph.value.edges.find((edge) => edge.id === selectedEdgeId.value) ?? null)

const createBlock = useMutation({
  mutationFn: (input: CreateBlockInput) => api.createBlock(projectId.value, input),
  onSuccess: async (detail) => {
    newBlockTitle.value = ''
    workspace.selectBlock(detail.block.id)
    await queryClient.invalidateQueries({ queryKey: ['graph', projectId.value] })
  },
})

const createEdge = useMutation({
  mutationFn: () =>
    api.createEdge(projectId.value, {
      source_block_id: edgeSourceBlockId.value,
      target_block_id: edgeTargetBlockId.value,
      edge_type: edgeType.value,
      label: edgeLabel.value.trim() || undefined,
    }),
  onSuccess: async () => {
    edgeLabel.value = ''
    await queryClient.invalidateQueries({ queryKey: ['graph', projectId.value] })
  },
})

const updateEdge = useMutation({
  mutationFn: () => {
    if (!selectedEdge.value) throw new Error('No selected edge')
    return api.updateEdge(projectId.value, selectedEdge.value.id, {
      edge_type: selectedEdgeType.value,
      label: selectedEdgeLabel.value.trim() || null,
      metadata: selectedEdge.value.metadata,
    })
  },
  onSuccess: async () => {
    await queryClient.invalidateQueries({ queryKey: ['graph', projectId.value] })
  },
})

const deleteEdge = useMutation({
  mutationFn: (edgeId: string) => api.deleteEdge(projectId.value, edgeId),
  onSuccess: async () => {
    selectedEdgeId.value = null
    await queryClient.invalidateQueries({ queryKey: ['graph', projectId.value] })
  },
})

const deleteBlock = useMutation({
  mutationFn: api.deleteBlock,
  onSuccess: async (_result, blockId) => {
    if (workspace.selectedBlockId === blockId) {
      workspace.selectBlock(null)
    }
    await queryClient.invalidateQueries({ queryKey: ['graph', projectId.value] })
  },
})

const archiveBranch = useMutation({
  mutationFn: (branchId: string) => api.updateBranch(branchId, { status: 'archived' }),
  onSuccess: async (_branch, archivedBranchId) => {
    selectedBranchId.value = branches.value.find(
      (branch) => branch.id !== archivedBranchId && branch.status === 'active',
    )?.id ?? archivedBranchId
    await queryClient.invalidateQueries({ queryKey: ['branches', projectId.value] })
  },
})

function beginEditBranch() {
  if (!selectedBranch.value) return
  branchEditName.value = selectedBranch.value.name
  branchEditDescription.value = selectedBranch.value.description ?? ''
  branchActionError.value = ''
  isEditingBranch.value = true
}

const updateSelectedBranch = useMutation({
  mutationFn: () => {
    if (!selectedBranchId.value) throw new Error('请先选择分支')
    const name = branchEditName.value.trim()
    if (!name) throw new Error('分支名称不能为空')
    return api.updateBranch(selectedBranchId.value, {
      name,
      description: branchEditDescription.value.trim(),
    })
  },
  onSuccess: async () => {
    isEditingBranch.value = false
    branchActionError.value = ''
    await queryClient.invalidateQueries({ queryKey: ['branches', projectId.value] })
  },
  onError: (error) => {
    branchActionError.value = error instanceof Error ? error.message : '保存分支失败'
  },
})

const restoreBranch = useMutation({
  mutationFn: (branchId: string) => api.updateBranch(branchId, { status: 'active' }),
  onSuccess: async () => {
    await queryClient.invalidateQueries({ queryKey: ['branches', projectId.value] })
  },
})

const deleteSelectedBranch = useMutation({
  mutationFn: async () => {
    if (!selectedBranchId.value || !selectedBranch.value) throw new Error('请先选择分支')
    if (selectedBranchBlockCount.value > 0) throw new Error('只能删除不含节点的空分支')
    if (!window.confirm(`确定永久删除空分支“${selectedBranch.value.name}”吗？`)) return null
    return api.deleteBranch(selectedBranchId.value)
  },
  onSuccess: async (result) => {
    if (!result) return
    selectedBranchId.value = null
    isEditingBranch.value = false
    branchActionError.value = ''
    await queryClient.invalidateQueries({ queryKey: ['branches', projectId.value] })
  },
  onError: (error) => {
    branchActionError.value = error instanceof Error ? error.message : '删除分支失败'
  },
})

async function forkBlockToNewBranch(blockId: string) {
  const source = graph.value.nodes.find((block) => block.id === blockId)
  if (!source?.current_revision_id) return
  const name = window.prompt('新分支名称', `${source.title || '未命名片段'} 分支`)
  if (!name?.trim()) return
  const newBranch = await api.forkBranch(projectId.value, {
    name: name.trim(),
    base_branch_id: source.branch_id,
    fork_from_block_id: source.id,
    fork_from_revision_id: source.current_revision_id,
  })
  const forked = await api.forkBlock(source.id, {
    branch_id: newBranch.id,
    revision_id: source.current_revision_id,
    title: source.title,
    position_x: source.position_x + 260,
    position_y: source.position_y + 120,
  })
  selectedBranchId.value = newBranch.id
  workspace.selectBlock(forked.block.id)
  await refreshWorkspace()
}

const generateBranchSummary = useMutation({
  mutationFn: () => {
    if (!selectedBranchId.value || !selectedSummaryModelProfileId.value) {
      throw new Error('请选择分支和可用模型')
    }
    const input = {
      project_id: projectId.value,
      model_profile_id: selectedSummaryModelProfileId.value,
      source_mode: branchSummarySourceMode.value,
      prompt_template_id: selectedBranchSummaryPromptId.value || null,
      source_selections: selectedBranchBlocks.value.map((block) => ({
        block_id: block.id,
        mode: branchSummarySourceSelections.value[block.id] ?? 'full_text',
      })),
    }
    branchSummaryAbortController = new AbortController()
    return selectedBranchSummary.value
      ? api.refreshSummary(selectedBranchSummary.value.id, input, branchSummaryAbortController.signal)
      : api.generateBranchSummary(selectedBranchId.value, input, branchSummaryAbortController.signal)
  },
  onSuccess: async () => {
    branchSummaryError.value = ''
    await queryClient.invalidateQueries({ queryKey: ['summaries', projectId.value] })
  },
  onError: (error) => {
    branchSummaryError.value = error instanceof DOMException && error.name === 'AbortError'
      ? '摘要生成已取消'
      : error instanceof Error ? error.message : '分支摘要生成失败'
    void queryClient.invalidateQueries({ queryKey: ['summaries', projectId.value] })
  },
  onSettled: () => {
    branchSummaryAbortController = null
  },
})

const saveBranchSummarySettings = useMutation({
  mutationFn: () => {
    if (!selectedBranch.value) throw new Error('请先选择分支')
    return api.updateBranch(selectedBranch.value.id, {
      metadata: {
        ...selectedBranch.value.metadata,
        summary_settings: {
          model_profile_id: selectedSummaryModelProfileId.value,
          prompt_template_id: selectedBranchSummaryPromptId.value || null,
          source_mode: branchSummarySourceMode.value,
          source_selections: selectedBranchBlocks.value.map((block) => ({
            block_id: block.id,
            mode: branchSummarySourceSelections.value[block.id] ?? 'full_text',
          })),
        },
      },
    })
  },
  onSuccess: async () => {
    summarySettingsMessage.value = ''
    await queryClient.invalidateQueries({ queryKey: ['branches', projectId.value] })
    isSummarySettingsOpen.value = false
  },
  onError: (error) => {
    summarySettingsMessage.value = error instanceof Error ? error.message : '摘要设置保存失败'
  },
})

async function saveAndGenerateBranchSummary() {
  try {
    await saveBranchSummarySettings.mutateAsync()
    generateBranchSummary.mutate()
  } catch {
    // 保存错误由 mutation 展示，配置未保存时不发起生成。
  }
}

function cancelBranchSummaryGeneration() {
  branchSummaryAbortController?.abort()
}

watch(
  () => graph.value.nodes,
  (nodes) => {
    if (!nodes.some((node) => node.id === edgeSourceBlockId.value)) {
      edgeSourceBlockId.value = nodes[0]?.id ?? ''
    }
    if (!nodes.some((node) => node.id === edgeTargetBlockId.value)) {
      edgeTargetBlockId.value = nodes.find((node) => node.id !== edgeSourceBlockId.value)?.id ?? ''
    }
  },
  { immediate: true },
)

watch(
  selectedEdge,
  (edge) => {
    if (!edge) {
      selectedEdgeType.value = 'next'
      selectedEdgeLabel.value = ''
      return
    }
    selectedEdgeType.value = edge.edge_type
    selectedEdgeLabel.value = edge.label ?? ''
  },
  { immediate: true },
)

function submitBlock() {
  const title = newBlockTitle.value.trim()
  createBlock.mutate({
    branch_id: selectedBranchId.value,
    type: 'scene',
    title: title || undefined,
    content: '',
    position_x: 80 + graph.value.nodes.length * 60,
    position_y: 80 + graph.value.nodes.length * 34,
  })
}

function submitEdge() {
  if (!edgeSourceBlockId.value || !edgeTargetBlockId.value || edgeSourceBlockId.value === edgeTargetBlockId.value) return
  createEdge.mutate()
}

function selectEdge(edgeId: string | null) {
  selectedEdgeId.value = edgeId
}

function toggleLeftSection(section: keyof typeof openLeftSections.value) {
  openLeftSections.value[section] = !openLeftSections.value[section]
}

function startToolDrag(event: PointerEvent) {
  if (event.button !== 0 || !canvasRef.value || !toolWindowRef.value) return
  const canvasRect = canvasRef.value.getBoundingClientRect()
  const windowRect = toolWindowRef.value.getBoundingClientRect()
  toolWindowPosition.value = {
    x: windowRect.left - canvasRect.left,
    y: windowRect.top - canvasRect.top,
  }
  toolDrag = {
    pointerId: event.pointerId,
    offsetX: event.clientX - windowRect.left,
    offsetY: event.clientY - windowRect.top,
  }
  window.addEventListener('pointermove', moveToolWindow)
  window.addEventListener('pointerup', stopToolDrag)
  event.preventDefault()
}

function moveToolWindow(event: PointerEvent) {
  if (!toolDrag || event.pointerId !== toolDrag.pointerId || !canvasRef.value || !toolWindowRef.value) return
  const canvasRect = canvasRef.value.getBoundingClientRect()
  const maxX = Math.max(0, canvasRect.width - toolWindowRef.value.offsetWidth)
  const maxY = Math.max(0, canvasRect.height - toolWindowRef.value.offsetHeight)
  toolWindowPosition.value = {
    x: Math.min(maxX, Math.max(0, event.clientX - canvasRect.left - toolDrag.offsetX)),
    y: Math.min(maxY, Math.max(0, event.clientY - canvasRect.top - toolDrag.offsetY)),
  }
}

function stopToolDrag(event: PointerEvent) {
  if (!toolDrag || event.pointerId !== toolDrag.pointerId) return
  toolDrag = null
  window.removeEventListener('pointermove', moveToolWindow)
  window.removeEventListener('pointerup', stopToolDrag)
}

function openBlockToolInNewTab() {
  if (!selectedBlock.value) return
  const target = router.resolve({
    name: 'block-tool',
    params: { projectId: projectId.value, blockId: selectedBlock.value.id },
  })
  window.open(target.href, '_blank', 'noopener,noreferrer')
}

onBeforeUnmount(() => {
  branchSummaryAbortController?.abort()
  window.removeEventListener('pointermove', moveToolWindow)
  window.removeEventListener('pointerup', stopToolDrag)
})

function blockLabel(blockId: string) {
  const index = graph.value.nodes.findIndex((node) => node.id === blockId)
  const block = graph.value.nodes[index]
  return block?.title || `片段 #${index + 1}`
}

function edgeCount(blockId: string, direction: 'in' | 'out') {
  return graph.value.edges.filter((edge) =>
    direction === 'out' ? edge.source_block_id === blockId : edge.target_block_id === blockId,
  ).length
}

async function refreshWorkspace() {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ['project', projectId.value] }),
    queryClient.invalidateQueries({ queryKey: ['branches', projectId.value] }),
    queryClient.invalidateQueries({ queryKey: ['graph', projectId.value] }),
  ])
}
</script>

<template>
  <main
    class="workspace"
    :class="{
      'is-left-drawer-collapsed': !isLeftDrawerOpen,
    }"
  >
    <header class="workspace__topbar">
      <button class="icon-button" type="button" title="返回项目" @click="router.push({ name: 'projects' })">
        <ArrowLeft :size="18" aria-hidden="true" />
      </button>
      <div class="workspace__title">
        <strong>{{ projectQuery.data.value?.name ?? 'BranchScribe' }}</strong>
        <span>{{ graph.nodes.length }} blocks</span>
      </div>
      <nav class="workspace__nav" aria-label="项目工具">
        <button class="button" type="button" @click="refreshWorkspace">
          <RefreshCw :size="16" aria-hidden="true" />
          刷新
        </button>
        <button class="button" type="button" @click="router.push({ name: 'model-profiles', query: { from: route.fullPath } })">
          <Settings :size="16" aria-hidden="true" />
          模型
        </button>
        <button class="button" type="button" @click="router.push({ name: 'prompt-settings', params: { projectId } })">
          <MessageSquareText :size="16" aria-hidden="true" />
          Prompt
        </button>
        <button class="button" type="button" @click="router.push({ name: 'canon-manager', params: { projectId, entityType: 'character' } })">
          <BookOpen :size="16" aria-hidden="true" />
          角色
        </button>
        <button class="button" type="button" @click="router.push({ name: 'canon-manager', params: { projectId, entityType: 'location' } })">
          <MapPin :size="16" aria-hidden="true" />
          地点
        </button>
        <button class="button" type="button" @click="router.push({ name: 'canon-manager', params: { projectId, entityType: 'rule' } })">
          <Layers3 :size="16" aria-hidden="true" />
          规则
        </button>
        <button class="button" type="button" @click="router.push({ name: 'memory-manager', params: { projectId } })">
          <Database :size="16" aria-hidden="true" />
          Memory
        </button>
        <button class="button" type="button" @click="router.push({ name: 'foreshadowing-manager', params: { projectId } })">
          <Telescope :size="16" aria-hidden="true" />
          伏笔
        </button>
        <button class="button" type="button" @click="router.push({ name: 'timeline-manager', params: { projectId } })">
          <Clock3 :size="16" aria-hidden="true" />
          时间线
        </button>
        <button class="button" type="button" @click="router.push({ name: 'transfer-manager', params: { projectId } })">
          <Download :size="16" aria-hidden="true" />
          导出
        </button>
      </nav>
    </header>

    <aside class="workspace__sidebar drawer drawer--left">
      <div class="drawer__bar">
        <button class="icon-button" type="button" :title="isLeftDrawerOpen ? '收起左侧菜单' : '展开左侧菜单'" @click="isLeftDrawerOpen = !isLeftDrawerOpen">
          <PanelLeftClose v-if="isLeftDrawerOpen" :size="18" aria-hidden="true" />
          <PanelLeftOpen v-else :size="18" aria-hidden="true" />
        </button>
        <strong v-if="isLeftDrawerOpen">工作台</strong>
      </div>

      <div v-show="isLeftDrawerOpen" class="drawer__content">
        <section class="panel-section panel-section--collapsible">
          <button class="panel-section__header panel-section__header--button" type="button" @click="toggleLeftSection('branches')">
            <span>
              <ChevronDown v-if="openLeftSections.branches" :size="16" aria-hidden="true" />
              <ChevronRight v-else :size="16" aria-hidden="true" />
              <h2>分支</h2>
            </span>
            <GitBranch :size="16" aria-hidden="true" />
          </button>
          <div v-show="openLeftSections.branches" class="branch-list">
            <button
              v-for="branch in branches"
              :key="branch.id"
              class="branch-list__item"
              :class="{ 'is-active': branch.id === selectedBranchId }"
              type="button"
              @click="selectedBranchId = branch.id"
            >
              <i class="branch-color-dot" :style="{ background: branchColors[branch.id] }"></i>
              <span>{{ branch.name }}</span>
              <small>{{ branch.status }} · {{ branchSummaryStatus(branch.id) }}</small>
            </button>
            <button
              v-if="selectedBranch?.status === 'active'"
              class="button"
              type="button"
              :disabled="archiveBranch.isPending.value"
              @click="archiveBranch.mutate(selectedBranch.id)"
            >
              归档当前分支
            </button>
            <div v-if="selectedBranch" class="branch-management">
              <div class="branch-management__actions">
                <button class="button" type="button" @click="beginEditBranch">
                  <Pencil :size="14" aria-hidden="true" />编辑分支
                </button>
                <button
                  v-if="selectedBranch.status === 'archived'"
                  class="button"
                  type="button"
                  :disabled="restoreBranch.isPending.value"
                  @click="restoreBranch.mutate(selectedBranch.id)"
                >
                  恢复分支
                </button>
                <button
                  v-if="selectedBranchBlockCount === 0"
                  class="button button--danger"
                  type="button"
                  :disabled="deleteSelectedBranch.isPending.value"
                  @click="deleteSelectedBranch.mutate()"
                >
                  <Trash2 :size="14" aria-hidden="true" />删除空分支
                </button>
              </div>
              <form v-if="isEditingBranch" class="branch-edit-form" @submit.prevent="updateSelectedBranch.mutate()">
                <label class="field-label">
                  <span>名称</span>
                  <input v-model="branchEditName" maxlength="120" />
                </label>
                <label class="field-label">
                  <span>说明</span>
                  <textarea v-model="branchEditDescription" rows="3"></textarea>
                </label>
                <div class="branch-management__actions">
                  <button class="button button--primary" type="submit" :disabled="updateSelectedBranch.isPending.value">保存</button>
                  <button class="button" type="button" @click="isEditingBranch = false">取消</button>
                </div>
              </form>
              <div v-if="branchActionError" class="llm-message llm-message--error">{{ branchActionError }}</div>
            </div>
            <div v-if="selectedBranchId" class="branch-summary-actions">
              <div v-if="selectedBranchSummary?.status === 'stale'" class="llm-message llm-message--warning">
                分支正文已变化，摘要需要刷新。
              </div>
              <div v-if="branchSummaryError" class="llm-message llm-message--error">{{ branchSummaryError }}</div>
              <div class="branch-summary-actions__composition">
                <span>{{ includedSummarySourceCount }}/{{ selectedBranchBlocks.length }} 个 Block</span>
                <span>{{ compressedSummarySourceCount }} 个使用已有摘要</span>
              </div>
              <div class="branch-management__actions">
                <button class="button" type="button" @click="openSummarySettings">
                  <Settings :size="15" aria-hidden="true" />摘要设置
                </button>
                <button
                  v-if="generateBranchSummary.isPending.value"
                  class="button button--danger"
                  type="button"
                  @click="cancelBranchSummaryGeneration"
                >
                  <X :size="15" aria-hidden="true" />取消生成
                </button>
                <button
                  v-else
                  class="button button--primary"
                  type="button"
                  :disabled="!selectedSummaryModelProfileId || includedSummarySourceCount === 0"
                  @click="generateBranchSummary.mutate()"
                >
                  <RefreshCw :size="15" aria-hidden="true" />
                  {{ generateBranchSummary.isPending.value ? '生成中' : selectedBranchSummary ? '刷新摘要' : '生成摘要' }}
                </button>
              </div>
            </div>
          </div>
        </section>

        <section class="panel-section panel-section--collapsible">
          <button class="panel-section__header panel-section__header--button" type="button" @click="toggleLeftSection('createBlock')">
            <span>
              <ChevronDown v-if="openLeftSections.createBlock" :size="16" aria-hidden="true" />
              <ChevronRight v-else :size="16" aria-hidden="true" />
              <h2>新建 Block</h2>
            </span>
            <Plus :size="16" aria-hidden="true" />
          </button>
          <form v-show="openLeftSections.createBlock" class="compact-form" @submit.prevent="submitBlock">
            <input v-model="newBlockTitle" type="text" placeholder="片段标题（可选）" />
            <button class="button button--primary" type="submit" :disabled="createBlock.isPending.value">
              <Plus :size="16" aria-hidden="true" />
              创建
            </button>
          </form>
        </section>

        <section class="panel-section panel-section--collapsible">
          <button class="panel-section__header panel-section__header--button" type="button" @click="toggleLeftSection('blockList')">
            <span>
              <ChevronDown v-if="openLeftSections.blockList" :size="16" aria-hidden="true" />
              <ChevronRight v-else :size="16" aria-hidden="true" />
              <h2>Block 列表</h2>
            </span>
            <Layers3 :size="16" aria-hidden="true" />
          </button>
          <div v-if="openLeftSections.blockList && blocks.length === 0" class="empty-state empty-state--compact">暂无 block</div>
          <ul v-if="openLeftSections.blockList && blocks.length > 0" class="block-list">
            <li v-for="block in blocks" :key="block.id" class="block-list__row" :class="{ 'is-active': block.id === workspace.selectedBlockId }">
              <button class="block-list__select" type="button" @click="workspace.selectBlock(block.id)">
                <span>{{ blockLabel(block.id) }}</span>
                <small>{{ block.type }} · 出 {{ edgeCount(block.id, 'out') }} · 入 {{ edgeCount(block.id, 'in') }}</small>
              </button>
              <button
                class="icon-button icon-button--danger"
                type="button"
                title="删除 block"
                :disabled="deleteBlock.isPending.value"
                @click="deleteBlock.mutate(block.id)"
              >
                <Trash2 :size="15" aria-hidden="true" />
              </button>
            </li>
          </ul>
        </section>

        <section class="panel-section panel-section--collapsible">
          <button
            class="panel-section__header panel-section__header--button edge-manager__header"
            type="button"
            @click="toggleLeftSection('edgeManager')"
          >
            <span>
              <ChevronDown v-if="openLeftSections.edgeManager" :size="16" aria-hidden="true" />
              <ChevronRight v-else :size="16" aria-hidden="true" />
              <Link2 :size="16" aria-hidden="true" />
              <h2>Edge 管理</h2>
            </span>
            <small>{{ graph.edges.length }} edges</small>
          </button>
          <div v-show="openLeftSections.edgeManager" class="edge-manager">
            <select v-model="selectedEdgeId">
              <option :value="null">选择一条 edge</option>
              <option v-for="edge in graph.edges" :key="edge.id" :value="edge.id">
                {{ blockLabel(edge.source_block_id) }} -> {{ blockLabel(edge.target_block_id) }} · {{ edge.edge_type }}
              </option>
            </select>
            <template v-if="selectedEdge">
              <div class="edge-manager__meta">
                <span class="edge-manager__endpoint">{{ blockLabel(selectedEdge.source_block_id) }}</span>
                <span class="edge-manager__arrow">→</span>
                <span class="edge-manager__endpoint">{{ blockLabel(selectedEdge.target_block_id) }}</span>
              </div>
              <select v-model="selectedEdgeType">
                <option v-for="item in edgeTypes" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
              <input v-model="selectedEdgeLabel" type="text" placeholder="标签（可选）" />
              <div class="edge-manager__actions">
                <button class="button button--primary" type="button" :disabled="updateEdge.isPending.value" @click="updateEdge.mutate()">
                  保存 Edge
                </button>
                <button class="button" type="button" :disabled="deleteEdge.isPending.value" @click="deleteEdge.mutate(selectedEdge.id)">
                  <Trash2 :size="15" aria-hidden="true" />
                  删除
                </button>
              </div>
            </template>
            <div v-else class="empty-state empty-state--compact">点击画布上的连线或从列表选择</div>
          </div>
        </section>

        <section class="panel-section panel-section--collapsible">
          <button class="panel-section__header panel-section__header--button" type="button" @click="toggleLeftSection('createEdge')">
            <span>
              <ChevronDown v-if="openLeftSections.createEdge" :size="16" aria-hidden="true" />
              <ChevronRight v-else :size="16" aria-hidden="true" />
              <h2>创建 Edge</h2>
            </span>
            <Link2 :size="16" aria-hidden="true" />
          </button>
          <form v-show="openLeftSections.createEdge" class="edge-form" @submit.prevent="submitEdge">
            <select v-model="edgeSourceBlockId" :disabled="graph.nodes.length < 2">
              <option value="" disabled>起点</option>
              <option v-for="node in graph.nodes" :key="node.id" :value="node.id">{{ blockLabel(node.id) }}</option>
            </select>
            <select v-model="edgeTargetBlockId" :disabled="graph.nodes.length < 2">
              <option value="" disabled>终点</option>
              <option v-for="node in graph.nodes" :key="node.id" :value="node.id" :disabled="node.id === edgeSourceBlockId">
                {{ blockLabel(node.id) }}
              </option>
            </select>
            <select v-model="edgeType">
              <option v-for="item in edgeTypes" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
            <input v-model="edgeLabel" type="text" placeholder="标签（可选）" />
            <button
              class="button"
              type="submit"
              :disabled="graph.nodes.length < 2 || edgeSourceBlockId === edgeTargetBlockId || createEdge.isPending.value"
            >
              <Link2 :size="16" aria-hidden="true" />
              连接
            </button>
          </form>
        </section>
      </div>
    </aside>

    <section ref="canvasRef" class="workspace__canvas">
      <BlockGraph
        :project-id="projectId"
        :graph="graph"
        :branch-colors="branchColors"
        :selected-block-id="workspace.selectedBlockId"
        :selected-edge-id="selectedEdgeId"
        @select-block="workspace.selectBlock"
        @select-edge="selectEdge"
        @fork-block="forkBlockToNewBranch"
      />
      <section
        v-if="isToolWindowOpen"
        ref="toolWindowRef"
        class="workspace-tool-window"
        :style="toolWindowPosition ? { left: `${toolWindowPosition.x}px`, top: `${toolWindowPosition.y}px`, right: 'auto', bottom: 'auto' } : undefined"
      >
        <div class="workspace-tool-window__bar" @pointerdown="startToolDrag">
          <strong>
            <Move :size="15" aria-hidden="true" />
            Block 工具
          </strong>
          <div class="workspace-tool-window__actions">
            <button
              class="icon-button"
              type="button"
              title="在新标签页打开"
              :disabled="!selectedBlock"
              @pointerdown.stop
              @click="openBlockToolInNewTab"
            >
              <ExternalLink :size="16" aria-hidden="true" />
            </button>
            <button class="icon-button" type="button" title="收起工具窗口" @pointerdown.stop @click="isToolWindowOpen = false">
              <X :size="17" aria-hidden="true" />
            </button>
          </div>
        </div>
        <div class="workspace-tabs">
          <button
            class="workspace-tabs__item"
            :class="{ 'is-active': activeWorkspacePanel === 'sidebar' }"
            type="button"
            @click="activeWorkspacePanel = 'sidebar'"
          >
            详情
          </button>
          <button
            class="workspace-tabs__item"
            :class="{ 'is-active': activeWorkspacePanel === 'editor' }"
            type="button"
            @click="activeWorkspacePanel = 'editor'"
          >
            正文
          </button>
          <button
            class="workspace-tabs__item"
            :class="{ 'is-active': activeWorkspacePanel === 'llm' }"
            type="button"
            @click="activeWorkspacePanel = 'llm'"
          >
            LLM 操作
          </button>
        </div>
        <div
          class="workspace-tool-window__body"
          :class="{ 'workspace-tool-window__body--single': activeWorkspacePanel === 'editor' || activeWorkspacePanel === 'llm' }"
        >
          <BlockInspector
            v-if="selectedBlock"
            :project-id="projectId"
            :block-id="selectedBlock.id"
            :mode="activeWorkspacePanel"
            @changed="refreshWorkspace"
          />
          <div v-else class="empty-state empty-state--panel">选择一个 block</div>
        </div>
      </section>
      <button
        v-else
        class="workspace-tool-window__launcher button"
        type="button"
        title="打开 Block 工具"
        @click="isToolWindowOpen = true"
      >
        <PanelRightOpen :size="17" aria-hidden="true" />
        Block 工具
      </button>
    </section>

    <div v-if="isSummarySettingsOpen" class="dialog-backdrop" @click.self="closeSummarySettings">
      <section class="dialog summary-settings-dialog" role="dialog" aria-modal="true" aria-labelledby="summary-settings-title">
        <header class="dialog__header">
          <div>
            <p class="eyebrow">Branch summary</p>
            <h2 id="summary-settings-title">摘要设置 · {{ selectedBranch?.name }}</h2>
          </div>
          <button class="icon-button" type="button" title="关闭" @click="closeSummarySettings">
            <X :size="18" aria-hidden="true" />
          </button>
        </header>

        <div class="summary-settings-dialog__body">
          <section class="summary-settings-group">
            <div class="summary-settings-group__heading">
              <div>
                <strong>生成配置</strong>
                <p>选择生成摘要使用的模型和 Prompt。</p>
              </div>
              <button class="button" type="button" @click="openSummaryPromptManager">
                <MessageSquareText :size="15" aria-hidden="true" />管理 Prompt
              </button>
            </div>
            <div class="summary-settings-fields">
              <label class="field-label">
                <span>模型</span>
                <select v-model="selectedSummaryModelProfileId">
                  <option value="" disabled>选择摘要模型</option>
                  <option v-for="profile in (modelProfilesQuery.data.value ?? []).filter((item) => item.profile_type === 'llm')" :key="profile.id" :value="profile.id">
                    {{ profile.name }} · {{ profile.model }}
                  </option>
                </select>
              </label>
              <label class="field-label">
                <span>Prompt</span>
                <select v-model="selectedBranchSummaryPromptId">
                  <option value="">
                    {{ defaultBranchSummaryPrompt ? `使用默认：${defaultBranchSummaryPrompt.name}` : '使用系统默认分支摘要 Prompt' }}
                  </option>
                  <option v-for="template in branchSummaryPrompts" :key="template.id" :value="template.id">
                    {{ template.name }}
                  </option>
                </select>
              </label>
            </div>
          </section>

          <section class="summary-settings-group">
            <div class="summary-settings-group__heading">
              <div>
                <strong>上下文构成</strong>
                <p>较新的关键片段可保留正文，较远内容可改用摘要；不需要的内容可以排除。</p>
              </div>
              <span>{{ includedSummarySourceCount }} 个已选</span>
            </div>
            <div class="summary-source-presets">
              <button class="button" type="button" @click="applySummarySourcePreset('full_text')">全部正文</button>
              <button class="button" type="button" @click="applySummarySourcePreset('prefer_block_summaries')">有摘要则压缩</button>
              <button class="button" type="button" @click="applySummarySourcePreset('block_summaries_only')">仅已有摘要</button>
            </div>
            <div v-if="selectedBranchBlocks.length === 0" class="empty-state empty-state--compact">当前分支还没有 Block。</div>
            <ol v-else class="summary-source-list">
              <li v-for="(block, index) in selectedBranchBlocks" :key="block.id" class="summary-source-list__item">
                <span class="summary-source-list__index">{{ index + 1 }}</span>
                <div>
                  <strong>{{ block.title || `无标题片段 #${index + 1}` }}</strong>
                  <small>{{ block.type }} · {{ validBlockSummaryIds.has(block.id) ? '有有效摘要' : '暂无有效摘要' }}</small>
                </div>
                <select v-model="branchSummarySourceSelections[block.id]" :aria-label="`${block.title || '无标题片段'}的摘要输入方式`">
                  <option value="full_text">使用正文</option>
                  <option value="summary" :disabled="!validBlockSummaryIds.has(block.id)">使用摘要</option>
                  <option value="exclude">不包含</option>
                </select>
              </li>
            </ol>
          </section>
        </div>

        <footer class="dialog__footer">
          <span v-if="includedSummarySourceCount === 0" class="summary-settings-dialog__warning">至少保留一个 Block。</span>
          <span v-if="summarySettingsMessage" class="summary-settings-dialog__warning">{{ summarySettingsMessage }}</span>
          <button class="button" type="button" @click="closeSummarySettings">取消</button>
          <button
            class="button"
            type="button"
            :disabled="saveBranchSummarySettings.isPending.value"
            @click="saveBranchSummarySettings.mutate()"
          >
            <Save :size="15" aria-hidden="true" />
            {{ saveBranchSummarySettings.isPending.value ? '保存中' : '完成' }}
          </button>
          <button
            class="button button--primary"
            type="button"
            :disabled="!selectedSummaryModelProfileId || includedSummarySourceCount === 0 || saveBranchSummarySettings.isPending.value"
            @click="saveAndGenerateBranchSummary"
          >
            <RefreshCw :size="15" aria-hidden="true" />
            {{ selectedBranchSummary ? '按此设置刷新' : '按此设置生成' }}
          </button>
        </footer>
      </section>
    </div>
  </main>
</template>
