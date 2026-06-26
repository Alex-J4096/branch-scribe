import type {
  ApiEnvelope,
  ApiErrorEnvelope,
  Block,
  BlockDetail,
  Branch,
  CreateBlockInput,
  ModelProfileInput,
  CreateProjectInput,
  CreateRevisionInput,
  GraphEdge,
  ModelProfile,
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
}
