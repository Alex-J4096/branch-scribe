import type {
  ApiEnvelope,
  ApiErrorEnvelope,
  Block,
  BlockDetail,
  Branch,
  CanonEntity,
  CanonEntityInput,
  CreateBlockInput,
  ModelProfileInput,
  PromptTemplateInput,
  CreateProjectInput,
  CreateRevisionInput,
  GenerateOnceInput,
  GenerateOnceResult,
  GenerateStreamEvent,
  GraphEdge,
  ModelProfile,
  PromptTemplate,
  Project,
  ProjectGraph,
  Revision,
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

  const envelope = (await response.json()) as ApiEnvelope<T> | ApiErrorEnvelope
  if (!response.ok || envelope.error) {
    const error = envelope.error ?? {
      code: 'HTTP_ERROR',
      message: `Request failed with status ${response.status}`,
    }
    throw new ApiClientError(response.status, error.code, error.message)
  }

  return envelope.data
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
    const envelope = (await response.json()) as ApiErrorEnvelope
    const error = envelope.error ?? {
      code: 'HTTP_ERROR',
      message: `Request failed with status ${response.status}`,
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

  listBranches: (projectId: string) => request<Branch[]>(`/projects/${projectId}/branches`),

  getGraph: (projectId: string) => request<ProjectGraph>(`/projects/${projectId}/graph`),
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
  forkBlock: (blockId: string, input: { title?: string | null; position_x: number; position_y: number }) =>
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

  getBlock: (blockId: string) => request<BlockDetail>(`/blocks/${blockId}`),
  updateBlock: (blockId: string, input: Partial<Pick<Block, 'title' | 'type' | 'order_index'>>) =>
    request<Block>(`/blocks/${blockId}`, {
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

  listModelProfiles: (projectId: string) => request<ModelProfile[]>(`/projects/${projectId}/model-profiles`),
  createModelProfile: (projectId: string, input: ModelProfileInput) =>
    request<ModelProfile>(`/projects/${projectId}/model-profiles`, {
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
  generateStream,
}
