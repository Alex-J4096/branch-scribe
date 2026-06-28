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
  ExternalLink,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightOpen,
  Plus,
  RefreshCw,
  Settings,
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
const branchSummaryError = ref('')
const isLeftDrawerOpen = ref(true)
const isToolWindowOpen = ref(true)
const activeWorkspacePanel = ref<'sidebar' | 'editor' | 'llm'>('editor')
const canvasRef = ref<HTMLElement | null>(null)
const toolWindowRef = ref<HTMLElement | null>(null)
const toolWindowPosition = ref<{ x: number; y: number } | null>(null)
let toolDrag: { pointerId: number; offsetX: number; offsetY: number } | null = null
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
  queryKey: computed(() => ['model-profiles', projectId.value]),
  queryFn: () => api.listModelProfiles(projectId.value),
})

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
  onSuccess: async () => {
    selectedBranchId.value = branches.value.find((branch) => branch.status === 'active')?.id ?? null
    await queryClient.invalidateQueries({ queryKey: ['branches', projectId.value] })
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
    }
    return selectedBranchSummary.value
      ? api.refreshSummary(selectedBranchSummary.value.id, input)
      : api.generateBranchSummary(selectedBranchId.value, input)
  },
  onSuccess: async () => {
    branchSummaryError.value = ''
    await queryClient.invalidateQueries({ queryKey: ['summaries', projectId.value] })
  },
  onError: (error) => {
    branchSummaryError.value = error instanceof Error ? error.message : '分支摘要生成失败'
    void queryClient.invalidateQueries({ queryKey: ['summaries', projectId.value] })
  },
})

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
      <button class="button" type="button" @click="refreshWorkspace">
        <RefreshCw :size="16" aria-hidden="true" />
        刷新
      </button>
      <button class="button" type="button" @click="router.push({ name: 'model-profiles', params: { projectId } })">
        <Settings :size="16" aria-hidden="true" />
        模型
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
              v-if="selectedBranchId && branches.find((branch) => branch.id === selectedBranchId)?.status === 'active'"
              class="button"
              type="button"
              :disabled="archiveBranch.isPending.value"
              @click="archiveBranch.mutate(selectedBranchId)"
            >
              归档当前分支
            </button>
            <div v-if="selectedBranchId" class="branch-summary-actions">
              <div v-if="selectedBranchSummary?.status === 'stale'" class="llm-message llm-message--warning">
                分支正文已变化，摘要需要刷新。
              </div>
              <div v-if="branchSummaryError" class="llm-message llm-message--error">{{ branchSummaryError }}</div>
              <select v-model="selectedSummaryModelProfileId">
                <option value="" disabled>选择摘要模型</option>
                <option v-for="profile in (modelProfilesQuery.data.value ?? []).filter((item) => item.profile_type === 'llm')" :key="profile.id" :value="profile.id">
                  {{ profile.name }} · {{ profile.model }}
                </option>
              </select>
              <button
                class="button button--primary"
                type="button"
                :disabled="!selectedSummaryModelProfileId || generateBranchSummary.isPending.value"
                @click="generateBranchSummary.mutate()"
              >
                <RefreshCw :size="15" aria-hidden="true" />
                {{ generateBranchSummary.isPending.value ? '生成中' : selectedBranchSummary ? '刷新分支摘要' : '生成分支摘要' }}
              </button>
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
  </main>
</template>
