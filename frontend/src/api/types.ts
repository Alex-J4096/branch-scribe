export type ApiEnvelope<T> = {
  data: T
  error: null
}

export type ApiErrorEnvelope = {
  data: null
  error: {
    code: string
    message: string
  }
}

export type Project = {
  id: string
  name: string
  description: string | null
  default_language: string
  default_style_profile: Record<string, unknown>
  default_model_profile_id: string | null
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type Branch = {
  id: string
  project_id: string
  name: string
  description: string | null
  base_branch_id: string | null
  fork_from_block_id: string | null
  fork_from_revision_id: string | null
  status: 'active' | 'archived'
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type Block = {
  id: string
  project_id: string
  branch_id: string | null
  type: 'scene' | 'chapter' | 'note' | 'summary' | 'canon' | 'outline'
  title: string | null
  current_revision_id: string | null
  parent_block_id: string | null
  position_x: number
  position_y: number
  order_index: number
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type Revision = {
  id: string
  block_id: string
  parent_revision_id: string | null
  content: string
  content_format: string
  content_hash: string | null
  source: 'user' | 'llm' | 'import' | 'system'
  generation_run_id: string | null
  metadata: Record<string, unknown>
  created_at: string
}

export type BlockDetail = {
  block: Block
  current_revision: Revision | null
}

export type GraphEdge = {
  id: string
  project_id: string
  source_block_id: string
  target_block_id: string
  edge_type: 'next' | 'fork' | 'alternative' | 'references' | 'summarizes'
  label: string | null
  metadata: Record<string, unknown>
  created_at: string
}

export type ProjectGraph = {
  nodes: Block[]
  edges: GraphEdge[]
}

export type CreateProjectInput = {
  name: string
  description?: string
}

export type CreateBlockInput = {
  branch_id?: string | null
  type: Block['type']
  title?: string | null
  content: string
  position_x: number
  position_y: number
}

export type CreateRevisionInput = {
  content: string
  content_format?: string
  source?: Revision['source']
  set_current?: boolean
}
