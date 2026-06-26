<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { ConnectionLineType, Handle, Position, VueFlow } from '@vue-flow/core'
import type { Connection, Edge, Node, ValidConnectionFunc } from '@vue-flow/core'

import { api } from '@/api/client'
import type { ProjectGraph } from '@/api/types'

const props = defineProps<{
  projectId: string
  graph: ProjectGraph
  selectedBlockId: string | null
}>()

const emit = defineEmits<{
  selectBlock: [blockId: string | null]
}>()

const queryClient = useQueryClient()
const manualConnection = ref<{
  sourceId: string
  startX: number
  startY: number
  currentX: number
  currentY: number
} | null>(null)

const nodes = computed<Node[]>(() =>
  props.graph.nodes.map((block, index) => ({
    id: block.id,
    type: 'default',
    position: {
      x: block.position_x,
      y: block.position_y,
    },
    data: {
      label: `${block.title || `片段 #${index + 1}`} · ${block.type}`,
    },
    class: block.id === props.selectedBlockId ? 'story-node is-selected' : 'story-node',
  })),
)

const edges = computed<Edge[]>(() =>
  props.graph.edges.map((edge) => ({
    id: edge.id,
    source: edge.source_block_id,
    target: edge.target_block_id,
    label: edge.label ?? edge.edge_type,
    animated: edge.edge_type === 'fork',
    class: `story-edge story-edge--${edge.edge_type}`,
  })),
)

function canCreateNextConnection(connection: Connection) {
  if (!connection.source || !connection.target || connection.source === connection.target) return false
  return !props.graph.edges.some(
    (edge) =>
      edge.source_block_id === connection.source &&
      edge.target_block_id === connection.target &&
      edge.edge_type === 'next',
  )
}

const isValidConnection: ValidConnectionFunc = (connection) => canCreateNextConnection(connection)

async function createDraggedEdge(connection: Connection) {
  if (!canCreateNextConnection(connection)) return

  await api.createEdge(props.projectId, {
    source_block_id: connection.source,
    target_block_id: connection.target,
    edge_type: 'next',
  })
  await queryClient.invalidateQueries({ queryKey: ['graph', props.projectId] })
}

function beginManualConnection(event: PointerEvent, sourceId: string) {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()

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

  const targetId = findBlockAtPoint(event.clientX, event.clientY)
  if (!targetId) return

  await createDraggedEdge({ source: sourceId, target: targetId })
}

function cancelManualConnection() {
  window.removeEventListener('pointermove', updateManualConnection)
  window.removeEventListener('pointerup', finishManualConnection)
  manualConnection.value = null
}

function findBlockAtPoint(x: number, y: number) {
  const elements = document.elementsFromPoint(x, y)
  for (const element of elements) {
    const node = element.closest<HTMLElement>('[data-block-id]')
    const blockId = node?.dataset.blockId
    if (blockId && blockId !== manualConnection.value?.sourceId) {
      return blockId
    }
  }

  let nearestBlockId: string | null = null
  let nearestDistance = Number.POSITIVE_INFINITY
  document.querySelectorAll<HTMLElement>('[data-block-id]').forEach((node) => {
    const blockId = node.dataset.blockId
    if (!blockId || blockId === manualConnection.value?.sourceId) return

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
    :nodes="nodes"
    :edges="edges"
    :fit-view-on-init="true"
    :min-zoom="0.35"
    :max-zoom="1.6"
    :connection-radius="42"
    :connect-on-click="false"
    :is-valid-connection="isValidConnection"
    :connection-line-type="ConnectionLineType.SmoothStep"
    @node-click="({ node }) => emit('selectBlock', String(node.id))"
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
      <div class="story-node__body" :class="{ 'is-selected': selected }" :data-block-id="id">
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
