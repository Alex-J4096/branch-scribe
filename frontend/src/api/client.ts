import type {
  ApiEnvelope,
  ApiErrorEnvelope,
  Block,
  BlockAssociationsInput,
  BlockDetail,
  Branch,
  BranchPath,
  CanonEntity,
  CanonEntityInput,
  Foreshadowing,
  ForeshadowingInput,
  ConsistencyCheckResult,
  TimelineEvent,
  TimelineEventInput,
  TimelineExtractionResult,
  CreateBlockInput,
  ModelProfileInput,
  PromptTemplateInput,
  CreateProjectInput,
  CreateRevisionInput,
  ContextPreview,
  GenerateOnceInput,
  GenerateOnceResult,
  GenerateCandidatesResult,
  GenerateStreamEvent,
  GraphEdge,
  GenerateSummaryInput,
  MemoryChunk,
  MemoryChunkFromBlockInput,
  MemoryChunkInput,
  MemorySearchInput,
  MemoryReindexResult,
  ModelProfile,
  PromptTemplate,
  Project,
  ProjectGraph,
  Revision,
  SummarySnapshot,
} from './types'

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api'

class ApiClientError extends Error {
  code: string
  status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiClientError'
    this.code = code
    this.status = status
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init.headers ?? {}),
    },
  })

  const responseText = await response.text()
  const envelope = parseEnvelope<T>(responseText)
  if (!response.ok) {
    const error = envelope?.error ?? {
      code: 'HTTP_ERROR',
      message: responseText.trim() || `Request failed with status ${response.status}`,
    }
    throw new ApiClientError(response.status, error.code, error.message)
  }
  if (!envelope || envelope.error) {
    const error = envelope?.error ?? {
      code: 'INVALID_API_RESPONSE',
      message: 'Server returned an invalid JSON response',
    }
    throw new ApiClientError(response.status, error.code, error.message)
  }

  return envelope.data
}

function parseEnvelope<T>(text: string): ApiEnvelope<T> | ApiErrorEnvelope | null {
  if (!text.trim()) return null
  try {
    return JSON.parse(text) as ApiEnvelope<T> | ApiErrorEnvelope
  } catch {
    return null
  }
}

async function download(path: string): Promise<Blob> {
  const response = await fetch(`${apiBaseUrl}${path}`)
  if (!response.ok) {
    const responseText = await response.text()
    const envelope = parseEnvelope<never>(responseText)
    const error = envelope?.error ?? {
      code: 'HTTP_ERROR',
      message: responseText.trim() || `Request failed with status ${response.status}`,
    }
    throw new ApiClientError(response.status, error.code, error.message)
  }
  return response.blob()
}

async function generateStream(
  input: GenerateOnceInput,
  onEvent: (event: GenerateStreamEvent) => void,
  signal?: AbortSignal,
) {
  const response = await fetch(`${apiBaseUrl}/generate/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    },
    body: JSON.stringify(input),
    signal,
  })

  if (!response.ok) {
    const responseText = await response.text()
    const envelope = parseEnvelope<never>(responseText)
    const error = envelope?.error ?? {
      code: 'HTTP_ERROR',
      message: responseText.trim() || `Request failed with status ${response.status}`,
    }
    throw new ApiClientError(response.status, error.code, error.message)
  }
  if (!response.body) {
    throw new ApiClientError(response.status, 'STREAM_UNAVAILABLE', 'Response stream is unavailable')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let lineBuffer = ''
  let eventDataLines: string[] = []

  const dispatchEvent = () => {
    if (!eventDataLines.length) return
    const event = parseSSEData(eventDataLines.join('\n'))
    eventDataLines = []
    if (event) {
      onEvent(event)
    }
  }

  const consumeLine = (rawLine: string) => {
    const line = rawLine.trimEnd()
    if (line === '') {
      dispatchEvent()
      return
    }
    const trimmedStart = line.trimStart()
    if (trimmedStart.startsWith('data:')) {
      eventDataLines.push(trimmedStart.slice(5).trimStart())
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    lineBuffer += decoder.decode(value, { stream: true })
    const lines = lineBuffer.split(/\r\n|\n|\r/)
    lineBuffer = lines.pop() ?? ''

    for (const line of lines) {
      consumeLine(line)
    }
  }

  lineBuffer += decoder.decode()
  if (lineBuffer) {
    consumeLine(lineBuffer)
  }
  dispatchEvent()
}

function parseSSEData(data: string): GenerateStreamEvent | null {
  data = data.trim()
  if (!data) return null
  try {
    return JSON.parse(data) as GenerateStreamEvent
  } catch {
    throw new ApiClientError(200, 'INVALID_STREAM_EVENT', `Invalid SSE event payload: ${data.slice(0, 120)}`)
  }
}

export const api = {
  listProjects: () => request<Project[]>('/projects'),
  createProject: (input: CreateProjectInput) =>
    request<Project>('/projects', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  getProject: (projectId: string) => request<Project>(`/projects/${projectId}`),
  deleteProject: (projectId: string) =>
    request<{ deleted: boolean }>(`/projects/${projectId}`, {
      method: 'DELETE',
    }),
  downloadMarkdownExport: (projectId: string, scope: { branchId?: string; chapterId?: string }) => {
    const query = new URLSearchParams()
    if (scope.branchId) query.set('branch_id', scope.branchId)
    if (scope.chapterId) query.set('chapter_id', scope.chapterId)
    return download(`/projects/${projectId}/export/markdown?${query.toString()}`)
  },
  downloadProjectBackup: (projectId: string) => download(`/projects/${projectId}/backup`),
  importProjectBackup: (backup: unknown) =>
    request<{ project_id: string }>('/projects/import', {
      method: 'POST',
      body: JSON.stringify(backup),
    }),

  listBranches: (projectId: string) => request<Branch[]>(`/projects/${projectId}/branches`),
  getBranchPath: (branchId: string) => request<BranchPath>(`/branches/${branchId}/path`),
  forkBranch: (
    projectId: string,
    input: {
      name: string
      base_branch_id?: string | null
      fork_from_block_id: string
      fork_from_revision_id?: string | null
    },
  ) =>
    request<Branch>(`/projects/${projectId}/branches/fork`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  updateBranch: (branchId: string, input: { name?: string; description?: string; status?: Branch['status'] }) =>
    request<Branch>(`/branches/${branchId}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  deleteBranch: (branchId: string) =>
    request<{ deleted: boolean }>(`/branches/${branchId}`, { method: 'DELETE' }),

  getGraph: (projectId: string) => request<ProjectGraph>(`/projects/${projectId}/graph`),
  listBlocks: (projectId: string) => request<Block[]>(`/projects/${projectId}/blocks`),
  createBlock: (projectId: string, input: CreateBlockInput) =>
    request<BlockDetail>(`/projects/${projectId}/blocks`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  deleteBlock: (blockId: string) =>
    request<{ deleted: boolean }>(`/blocks/${blockId}`, {
      method: 'DELETE',
    }),
  updateBlockPosition: (projectId: string, blockId: string, position: { position_x: number; position_y: number }) =>
    request<Block>(`/projects/${projectId}/graph/nodes/${blockId}/position`, {
      method: 'PATCH',
      body: JSON.stringify(position),
    }),
  forkBlock: (
    blockId: string,
    input: {
      branch_id?: string | null
      title?: string | null
      position_x: number
      position_y: number
      revision_id?: string | null
    },
  ) =>
    request<BlockDetail>(`/blocks/${blockId}/fork`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  createEdge: (
    projectId: string,
    input: {
      source_block_id: string
      target_block_id: string
      edge_type: GraphEdge['edge_type']
      label?: string
    },
  ) =>
    request<GraphEdge>(`/projects/${projectId}/graph/edges`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  updateEdge: (
    projectId: string,
    edgeId: string,
    input: {
      edge_type: GraphEdge['edge_type']
      label?: string | null
      metadata?: Record<string, unknown>
    },
  ) =>
    request<GraphEdge>(`/projects/${projectId}/graph/edges/${edgeId}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  deleteEdge: (projectId: string, edgeId: string) =>
    request<{ deleted: boolean }>(`/projects/${projectId}/graph/edges/${edgeId}`, {
      method: 'DELETE',
    }),

  listCanonEntities: (projectId: string, filter: { type?: CanonEntity['type']; status?: CanonEntity['status']; q?: string } = {}) => {
    const query = new URLSearchParams()
    if (filter.type) query.set('type', filter.type)
    if (filter.status) query.set('status', filter.status)
    if (filter.q) query.set('q', filter.q)
    const suffix = query.toString() ? `?${query.toString()}` : ''
    return request<CanonEntity[]>(`/projects/${projectId}/canon${suffix}`)
  },
  createCanonEntity: (projectId: string, input: CanonEntityInput) =>
    request<CanonEntity>(`/projects/${projectId}/canon`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  getCanonEntity: (entityId: string) => request<CanonEntity>(`/canon/${entityId}`),
  updateCanonEntity: (entityId: string, input: Partial<CanonEntityInput>) =>
    request<CanonEntity>(`/canon/${entityId}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  deleteCanonEntity: (entityId: string) =>
    request<{ deleted: boolean }>(`/canon/${entityId}`, {
      method: 'DELETE',
    }),
  extractCharacterCard: (
    projectId: string,
    characterId: string,
    input: { block_id: string; block_ids: string[]; model_profile_id: string },
  ) =>
    request<import('./types').CharacterCardProposal>(`/projects/${projectId}/characters/${characterId}/extract-card`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listCharacterStates: (projectId: string, characterId?: string) => {
    const suffix = characterId ? `?character_id=${encodeURIComponent(characterId)}` : ''
    return request<import('./types').CharacterState[]>(`/projects/${projectId}/character-states${suffix}`)
  },
  createCharacterState: (
    projectId: string,
    input: {
      character_id: string
      block_id?: string | null
      state_key: string
      state_value: Record<string, unknown>
      notes?: string | null
      occurred_at?: string | null
      metadata?: Record<string, unknown>
    },
  ) =>
    request<import('./types').CharacterState>(`/projects/${projectId}/character-states`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listForeshadowings: (projectId: string, status?: Foreshadowing['status']) => {
    const suffix = status ? `?status=${encodeURIComponent(status)}` : ''
    return request<Foreshadowing[]>(`/projects/${projectId}/foreshadowings${suffix}`)
  },
  createForeshadowing: (projectId: string, input: ForeshadowingInput) =>
    request<Foreshadowing>(`/projects/${projectId}/foreshadowings`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  updateForeshadowing: (id: string, input: ForeshadowingInput) =>
    request<Foreshadowing>(`/foreshadowings/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    }),
  deleteForeshadowing: (id: string) =>
    request<{ deleted: boolean }>(`/foreshadowings/${id}`, { method: 'DELETE' }),
  checkBlockConsistency: (projectId: string, blockId: string, modelProfileId: string) =>
    request<ConsistencyCheckResult>(`/projects/${projectId}/blocks/${blockId}/check-consistency`, {
      method: 'POST',
      body: JSON.stringify({ model_profile_id: modelProfileId }),
    }),
  extractTimelineEvents: (projectId: string, blockId: string, modelProfileId: string) =>
    request<TimelineExtractionResult>(`/projects/${projectId}/blocks/${blockId}/extract-events`, {
      method: 'POST',
      body: JSON.stringify({ model_profile_id: modelProfileId }),
    }),
  listTimelineEvents: (projectId: string) =>
    request<TimelineEvent[]>(`/projects/${projectId}/timeline-events`),
  createTimelineEvent: (projectId: string, input: TimelineEventInput) =>
    request<TimelineEvent>(`/projects/${projectId}/timeline-events`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  updateTimelineEvent: (id: string, input: TimelineEventInput) =>
    request<TimelineEvent>(`/timeline-events/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    }),
  deleteTimelineEvent: (id: string) =>
    request<{ deleted: boolean }>(`/timeline-events/${id}`, { method: 'DELETE' }),

  listMemoryChunks: (
    projectId: string,
    filter: { source_type?: string; chunk_kind?: string; tag?: string; q?: string } = {},
  ) => {
    const query = new URLSearchParams()
    if (filter.source_type) query.set('source_type', filter.source_type)
    if (filter.chunk_kind) query.set('chunk_kind', filter.chunk_kind)
    if (filter.tag) query.set('tag', filter.tag)
    if (filter.q) query.set('q', filter.q)
    const suffix = query.toString() ? `?${query.toString()}` : ''
    return request<MemoryChunk[]>(`/projects/${projectId}/memory${suffix}`)
  },
  createMemoryChunk: (projectId: string, input: MemoryChunkInput) =>
    request<MemoryChunk>(`/projects/${projectId}/memory`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  searchMemoryChunks: (projectId: string, input: MemorySearchInput) =>
    request<MemoryChunk[]>(`/projects/${projectId}/memory/search`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  reindexMemory: (projectId: string, modelProfileId: string) =>
    request<MemoryReindexResult>(`/projects/${projectId}/memory/reindex`, {
      method: 'POST',
      body: JSON.stringify({ model_profile_id: modelProfileId }),
    }),
  createMemoryChunkFromBlock: (blockId: string, input: MemoryChunkFromBlockInput) =>
    request<MemoryChunk>(`/blocks/${blockId}/memory`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  getMemoryChunk: (memoryId: string) => request<MemoryChunk>(`/memory/${memoryId}`),
  updateMemoryChunk: (memoryId: string, input: Partial<MemoryChunkInput>) =>
    request<MemoryChunk>(`/memory/${memoryId}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  deleteMemoryChunk: (memoryId: string) =>
    request<{ deleted: boolean }>(`/memory/${memoryId}`, {
      method: 'DELETE',
    }),

  getBlock: (blockId: string) => request<BlockDetail>(`/blocks/${blockId}`),
  updateBlock: (blockId: string, input: Partial<Pick<Block, 'title' | 'type' | 'order_index'>>) =>
    request<Block>(`/blocks/${blockId}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  updateBlockAssociations: (blockId: string, input: BlockAssociationsInput) =>
    request<Block>(`/blocks/${blockId}/associations`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  listRevisions: (blockId: string) => request<Revision[]>(`/blocks/${blockId}/revisions`),
  createRevision: (blockId: string, input: CreateRevisionInput) =>
    request<Revision>(`/blocks/${blockId}/revisions`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  selectRevision: (blockId: string, revisionId: string) =>
    request<Block>(`/blocks/${blockId}/revisions/${revisionId}/select`, {
      method: 'POST',
    }),
  listSummaries: (projectId: string) => request<SummarySnapshot[]>(`/projects/${projectId}/summaries`),
  generateBlockSummary: (blockId: string, input: GenerateSummaryInput) =>
    request<SummarySnapshot>(`/blocks/${blockId}/summarize`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  generateBranchSummary: (branchId: string, input: GenerateSummaryInput) =>
    request<SummarySnapshot>(`/branches/${branchId}/summarize`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  refreshSummary: (summaryId: string, input: GenerateSummaryInput) =>
    request<SummarySnapshot>(`/summaries/${summaryId}/refresh`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),

  listModelProfiles: () => request<ModelProfile[]>('/model-profiles'),
  createModelProfile: (input: ModelProfileInput) =>
    request<ModelProfile>('/model-profiles', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  updateModelProfile: (profileId: string, input: Partial<ModelProfileInput>) =>
    request<ModelProfile>(`/model-profiles/${profileId}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  deleteModelProfile: (profileId: string) =>
    request<{ deleted: boolean }>(`/model-profiles/${profileId}`, {
      method: 'DELETE',
    }),

  listPromptTemplates: (projectId: string, taskType?: string) => {
    const query = taskType ? `?task_type=${encodeURIComponent(taskType)}` : ''
    return request<PromptTemplate[]>(`/projects/${projectId}/prompt-templates${query}`)
  },
  createPromptTemplate: (projectId: string, input: PromptTemplateInput) =>
    request<PromptTemplate>(`/projects/${projectId}/prompt-templates`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  getPromptTemplate: (templateId: string) => request<PromptTemplate>(`/prompt-templates/${templateId}`),
  updatePromptTemplate: (templateId: string, input: Partial<PromptTemplateInput>) =>
    request<PromptTemplate>(`/prompt-templates/${templateId}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  deletePromptTemplate: (templateId: string) =>
    request<{ deleted: boolean }>(`/prompt-templates/${templateId}`, {
      method: 'DELETE',
    }),

  generateOnce: (input: GenerateOnceInput) =>
    request<GenerateOnceResult>('/generate/once', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  generateCandidates: (input: GenerateOnceInput & { count?: number }) =>
    request<GenerateCandidatesResult>('/generate/candidates', {
      method: 'POST',
      body: JSON.stringify({ ...input, count: input.count ?? 2 }),
    }),
  previewGenerationContext: (input: GenerateOnceInput) =>
    request<ContextPreview>('/generate/context-preview', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listLLMConversations: (blockId: string) =>
    request<import('./types').LLMConversation[]>(`/blocks/${blockId}/llm-conversations`),
  createLLMConversation: (blockId: string, input: { project_id: string; title?: string }) =>
    request<import('./types').LLMConversation>(`/blocks/${blockId}/llm-conversations`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  listLLMConversationMessages: (conversationId: string) =>
    request<import('./types').LLMConversationMessage[]>(`/llm-conversations/${conversationId}/messages`),
  updateLLMConversation: (conversationId: string, title: string) =>
    request<import('./types').LLMConversation>(`/llm-conversations/${conversationId}`, {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    }),
  deleteLLMConversation: (conversationId: string) =>
    request<{ deleted: boolean }>(`/llm-conversations/${conversationId}`, { method: 'DELETE' }),
  updateLLMConversationMessage: (messageId: string, content: string) =>
    request<import('./types').LLMConversationMessage>(`/llm-messages/${messageId}`, {
      method: 'PATCH',
      body: JSON.stringify({ content }),
    }),
  deleteLLMConversationMessages: (conversationId: string, messageIds: string[]) =>
    request<{ deleted: number }>(`/llm-conversations/${conversationId}/messages`, {
      method: 'DELETE',
      body: JSON.stringify({ message_ids: messageIds }),
    }),
  generateStream,
}
