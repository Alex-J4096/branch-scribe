<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, GitBranch, Layers3, Link2, Plus, RefreshCw, Trash2 } from 'lucide-vue-next'
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

const branches = computed(() => branchesQuery.data.value ?? [])
const graph = computed(() => graphQuery.data.value ?? { nodes: [], edges: [] })
const blocks = computed(() => graph.value.nodes)

watch(
  branches,
  (value) => {
    if (!selectedBranchId.value && value[0]) {
      selectedBranchId.value = value[0].id
    }
  },
  { immediate: true },
)

const selectedBlock = computed(() => graph.value.nodes.find((node) => node.id === workspace.selectedBlockId) ?? null)

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

const deleteBlock = useMutation({
  mutationFn: api.deleteBlock,
  onSuccess: async (_result, blockId) => {
    if (workspace.selectedBlockId === blockId) {
      workspace.selectBlock(null)
    }
    await queryClient.invalidateQueries({ queryKey: ['graph', projectId.value] })
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
  <main class="workspace">
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
    </header>

    <aside class="workspace__sidebar">
      <section class="panel-section">
        <div class="panel-section__header">
          <h2>分支</h2>
          <GitBranch :size="16" aria-hidden="true" />
        </div>
        <div class="branch-list">
          <button
            v-for="branch in branches"
            :key="branch.id"
            class="branch-list__item"
            :class="{ 'is-active': branch.id === selectedBranchId }"
            type="button"
            @click="selectedBranchId = branch.id"
          >
            <span>{{ branch.name }}</span>
            <small>{{ branch.status }}</small>
          </button>
        </div>
      </section>

      <section class="panel-section">
        <div class="panel-section__header">
          <h2>新建 Block</h2>
        </div>
        <form class="compact-form" @submit.prevent="submitBlock">
          <input v-model="newBlockTitle" type="text" placeholder="片段标题（可选）" />
          <button class="button button--primary" type="submit" :disabled="createBlock.isPending.value">
            <Plus :size="16" aria-hidden="true" />
            创建
          </button>
        </form>
      </section>

      <section class="panel-section">
        <div class="panel-section__header">
          <h2>Block 列表</h2>
          <Layers3 :size="16" aria-hidden="true" />
        </div>
        <div v-if="blocks.length === 0" class="empty-state empty-state--compact">暂无 block</div>
        <ul v-else class="block-list">
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

      <section class="panel-section">
        <div class="panel-section__header">
          <h2>创建 Edge</h2>
          <Link2 :size="16" aria-hidden="true" />
        </div>
        <form class="edge-form" @submit.prevent="submitEdge">
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
    </aside>

    <section class="workspace__canvas">
      <BlockGraph
        :project-id="projectId"
        :graph="graph"
        :selected-block-id="workspace.selectedBlockId"
        @select-block="workspace.selectBlock"
      />
    </section>

    <aside class="workspace__inspector">
      <BlockInspector
        v-if="selectedBlock"
        :project-id="projectId"
        :block-id="selectedBlock.id"
        @changed="refreshWorkspace"
      />
      <div v-else class="empty-state empty-state--panel">选择一个 block</div>
    </aside>
  </main>
</template>
