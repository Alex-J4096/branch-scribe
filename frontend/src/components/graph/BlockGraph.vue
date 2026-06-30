<script setup lang="ts">
import { ref, watch } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { ConnectionLineType, Handle, isEdge, MarkerType, Position, VueFlow } from '@vue-flow/core'
import type { Connection, Edge, Node, ValidConnectionFunc } from '@vue-flow/core'

import { api } from '@/api/client'
import type { ProjectGraph } from '@/api/types'

const props = defineProps<{
  projectId: string
  graph: ProjectGraph
  branchColors?: Record<string, string>
  selectedBlockId: string | null
  selectedEdgeId: string | null
}>()

const emit = defineEmits<{
  selectBlock: [blockId: string | null]
  selectEdge: [edgeId: string | null]
  forkBlock: [blockId: string]
}>()

const queryClient = useQueryClient()
const flowNodes = ref<Node[]>([])
const flowEdges = ref<Edge[]>([])
const manualConnection = ref<{
  sourceId: string
  startX: number
  startY: number
  currentX: number
  currentY: number
} | null>(null)

watch(
  () => [props.graph.nodes, props.selectedBlockId] as const,
  () => {
    flowNodes.value = buildFlowNodes()
  },
  { immediate: true },
)

watch(
  () => [props.graph.edges, props.selectedEdgeId] as const,
  () => {
    flowEdges.value = buildFlowEdges()
  },
  { immediate: true },
)

function buildFlowNodes(): Node[] {
  return props.graph.nodes.map((block, index) => ({
    id: block.id,
    type: 'default',
    position: {
      x: block.position_x,
      y: block.position_y,
    },
    data: {
      label: `${block.title || `片段 #${index + 1}`} · ${block.type}`,
    },
    style: block.branch_id && props.branchColors?.[block.branch_id]
      ? { borderColor: props.branchColors[block.branch_id], borderWidth: '3px' }
      : undefined,
    class: block.id === props.selectedBlockId ? 'story-node is-selected' : 'story-node',
  }))
}

function buildFlowEdges(): Edge[] {
  return props.graph.edges.map((edge) => ({
    id: edge.id,
    type: 'smoothstep',
    source: edge.source_block_id,
    target: edge.target_block_id,
    sourceHandle: 'source',
    targetHandle: 'target',
    label: edge.label ?? edge.edge_type,
    animated: edge.edge_type === 'fork',
    class: `story-edge story-edge--${edge.edge_type}${edge.id === props.selectedEdgeId ? ' is-selected' : ''}`,
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: edgeColor(edge.edge_type),
      width: 20,
      height: 20,
      markerUnits: 'userSpaceOnUse',
    },
    style: {
      stroke: edgeColor(edge.edge_type),
      strokeWidth: 2.5,
    },
    labelBgStyle: {
      fill: '#ffffff',
      fillOpacity: 0.92,
    },
    labelStyle: {
      fill: '#253241',
      fontSize: 12,
      fontWeight: 700,
    },
  }))
}

function canCreateNextConnection(connection: Connection) {
  if (!connection.source || !connection.target || connection.source === connection.target) return false
  return !props.graph.edges.some(
    (edge) =>
      edge.source_block_id === connection.source &&
      edge.target_block_id === connection.target &&
      edge.edge_type === 'next',
  )
}

// Vue Flow 在两端调用这个校验：
//   1. 用户从 handle 拖拽新建连线时，传入的是没有 id 的 Connection；
//   2. setEdges 解析从后端加载的 edge 时，传入的是带 id 的既有 Edge。
// 如果对已加载的 `next` edge 也跑重复校验，Vue Flow 会以
// "An edge needs a source and a target" 丢弃它，导致画布上看不到箭头。
// 因此：带 id 的既有 edge 一律放行；只对没有 id 的新建拖拽走重复校验。
const isValidConnection: ValidConnectionFunc = (connection) => {
  if (!connection.source || !connection.target || connection.source === connection.target) return false
  if (isEdge(connection)) return true
  return canCreateNextConnection(connection)
}

async function createDraggedEdge(connection: Connection) {
  if (!canCreateNextConnection(connection)) return

  await api.createEdge(props.projectId, {
    source_block_id: connection.source,
    target_block_id: connection.target,
    edge_type: 'next',
  })
  await queryClient.invalidateQueries({ queryKey: ['graph', props.projectId] })
}

function edgeColor(edgeType: string) {
  switch (edgeType) {
    case 'fork':
      return '#9b6b28'
    case 'alternative':
      return '#7a4fa3'
    case 'references':
      return '#466987'
    case 'summarizes':
      return '#607449'
    case 'next':
    default:
      return '#2f7d76'
  }
}

function beginManualConnection(event: PointerEvent, sourceId: string) {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  target.setPointerCapture?.(event.pointerId)

  manualConnection.value = {
    sourceId,
    startX: rect.right,
    startY: rect.top + rect.height / 2,
    currentX: event.clientX,
    currentY: event.clientY,
  }

  window.addEventListener('pointermove', updateManualConnection)
  window.addEventListener('pointerup', finishManualConnection, { once: true })
  window.addEventListener('pointercancel', cancelManualConnection, { once: true })
}

function updateManualConnection(event: PointerEvent) {
  if (!manualConnection.value) return
  manualConnection.value.currentX = event.clientX
  manualConnection.value.currentY = event.clientY
}

async function finishManualConnection(event: PointerEvent) {
  window.removeEventListener('pointermove', updateManualConnection)
  window.removeEventListener('pointercancel', cancelManualConnection)

  const sourceId = manualConnection.value?.sourceId
  manualConnection.value = null
  if (!sourceId) return

  const targetId = findBlockAtPoint(event.clientX, event.clientY, sourceId)
  if (!targetId) return

  await createDraggedEdge({ source: sourceId, target: targetId })
}

function cancelManualConnection() {
  window.removeEventListener('pointermove', updateManualConnection)
  window.removeEventListener('pointerup', finishManualConnection)
  manualConnection.value = null
}

function findBlockAtPoint(x: number, y: number, sourceId: string) {
  const elements = document.elementsFromPoint(x, y)
  for (const element of elements) {
    const node = element.closest<HTMLElement>('[data-block-id]')
    const blockId = node?.dataset.blockId
    if (blockId && blockId !== sourceId) {
      return blockId
    }
  }

  let nearestBlockId: string | null = null
  let nearestDistance = Number.POSITIVE_INFINITY
  document.querySelectorAll<HTMLElement>('[data-block-id]').forEach((node) => {
    const blockId = node.dataset.blockId
    if (!blockId || blockId === sourceId) return

    const rect = node.getBoundingClientRect()
    const targetX = rect.left
    const targetY = rect.top + rect.height / 2
    const distance = Math.hypot(targetX - x, targetY - y)
    if (distance < nearestDistance) {
      nearestDistance = distance
      nearestBlockId = blockId
    }
  })

  if (nearestDistance <= 120) {
    return nearestBlockId
  }
  return null
}

async function updatePosition(event: { node: Node }) {
  await api.updateBlockPosition(props.projectId, String(event.node.id), {
    position_x: event.node.position.x,
    position_y: event.node.position.y,
  })
  await queryClient.invalidateQueries({ queryKey: ['graph', props.projectId] })
}
</script>

<template>
  <VueFlow
    class="story-flow"
    v-model:nodes="flowNodes"
    v-model:edges="flowEdges"
    :fit-view-on-init="true"
    :min-zoom="0.35"
    :max-zoom="1.6"
    :connection-radius="42"
    :connect-on-click="false"
    :is-valid-connection="isValidConnection"
    :connection-line-type="ConnectionLineType.SmoothStep"
    @node-click="({ node }) => emit('selectBlock', String(node.id))"
    @edge-click="({ edge }) => emit('selectEdge', String(edge.id))"
    @pane-click="emit('selectEdge', null)"
    @node-drag-stop="updatePosition"
    @connect="createDraggedEdge"
  >
    <svg v-if="manualConnection" class="connection-preview" aria-hidden="true">
      <line
        :x1="manualConnection.startX"
        :y1="manualConnection.startY"
        :x2="manualConnection.currentX"
        :y2="manualConnection.currentY"
      />
    </svg>

    <template #node-default="{ id, data, selected }">
      <div
        class="story-node__body"
        :class="{ 'is-selected': selected || String(id) === selectedBlockId }"
        :data-block-id="id"
        :aria-current="String(id) === selectedBlockId ? 'true' : undefined"
        @contextmenu.prevent="emit('forkBlock', String(id))"
      >
        <Handle
          id="target"
          class="story-node__handle story-node__handle--target"
          type="target"
          :position="Position.Left"
        />
        <span class="story-node__drop-zone" aria-hidden="true"></span>
        <span>{{ data.label }}</span>
        <button
          class="story-node__connect-zone"
          type="button"
          title="拖动连接"
          @pointerdown.stop.prevent="beginManualConnection($event, String(id))"
        >
          <span aria-hidden="true"></span>
        </button>
        <Handle
          id="source"
          class="story-node__handle story-node__handle--source"
          type="source"
          :position="Position.Right"
        />
      </div>
    </template>
  </VueFlow>
</template>
