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

export type BranchPath = {
  branch: Branch
  blocks: Block[]
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

export type CanonEntity = {
  id: string
  project_id: string
  type: 'character' | 'location' | 'faction' | 'item' | 'rule' | 'event'
  name: string
  aliases: string[]
  description: string | null
  attributes: Record<string, unknown>
  importance: number
  status: 'canon' | 'draft' | 'deprecated'
  created_at: string
  updated_at: string
}

export type CharacterState = {
  id: string
  project_id: string
  character_id: string
  block_id: string | null
  state_key: string
  state_value: Record<string, unknown>
  notes: string | null
  occurred_at: string | null
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type CharacterCardProposal = {
  character_id: string
  source_block_id: string
  source_block_ids: string[]
  description: string
  attributes: Record<string, unknown>
  change_summary: string
  model: string
  generation_run_id: string
}

export type Foreshadowing = {
  id: string
  project_id: string
  title: string
  description: string | null
  status: 'planted' | 'developed' | 'resolved' | 'abandoned'
  planted_block_id: string | null
  resolved_block_id: string | null
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type ForeshadowingInput = {
  title: string
  description?: string | null
  status: Foreshadowing['status']
  planted_block_id?: string | null
  resolved_block_id?: string | null
  metadata?: Record<string, unknown>
}

export type ConsistencyConflict = {
  canon_entity_id: string
  canon_name: string
  severity: 'warning' | 'error'
  claim: string
  canon_fact: string
  explanation: string
  suggestion: string
}

export type ConsistencyCheckResult = {
  block_id: string
  consistent: boolean
  summary: string
  conflicts: ConsistencyConflict[]
  model: string
  generation_run_id: string
}

export type TimelineEvent = {
  id: string
  project_id: string
  title: string
  description: string | null
  event_time: string | null
  sort_order: number
  block_id: string | null
  canon_entity_id: string | null
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type TimelineEventInput = {
  title: string
  description?: string | null
  event_time?: string | null
  sort_order: number
  block_id?: string | null
  canon_entity_id?: string | null
  metadata?: Record<string, unknown>
}

export type TimelineExtractionResult = {
  block_id: string
  events: Array<{
    title: string
    description: string
    event_time: string | null
    sort_order: number
    canon_entity_id: string | null
  }>
  model: string
  generation_run_id: string
}

export type MemoryChunk = {
  id: string
  project_id: string
  source_type: string
  source_id: string | null
  chunk_text: string
  chunk_kind: string
  tags: string[]
  metadata: Record<string, unknown>
  created_at: string
  similarity?: number
}

export type ModelProfile = {
  id: string
  name: string
  provider: 'openai_compatible' | 'openai' | 'anthropic' | 'gemini' | 'openrouter' | 'deepseek' | 'moonshot' | 'siliconflow'
  model: string
  base_url: string | null
  has_api_key: boolean
  temperature: number
  top_p: number
  max_tokens: number
  context_window: number
  profile_type: 'llm' | 'embedding'
  embedding_profile_id: string | null
  embedding_dimensions: number | null
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type PromptTemplate = {
  id: string
  project_id: string | null
  name: string
  task_type: 'free_write' | 'continue' | 'rewrite_block' | 'rewrite_selection' | 'expand' | 'condense' | 'polish' | string
  template_text: string
  version: number
  is_default: boolean
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type GenerationRun = {
  id: string
  project_id: string
  block_id: string | null
  task_type: string
  provider: string
  model: string
  temperature: number | null
  top_p: number | null
  max_tokens: number | null
  context_window: number | null
  prompt_template_id: string | null
  input_context_snapshot: Record<string, unknown>
  output_revision_id: string | null
  input_tokens: number
  output_tokens: number
  latency_ms: number
  status: 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  error_message: string | null
  created_at: string
}

export type GenerateOnceInput = {
  project_id: string
  block_id: string
  task_type: string
  model_profile_id: string
  prompt_template_id?: string | null
  selected_text?: string
  user_instruction?: string
  context_node_count?: number
  conversation_id?: string | null
  temperature?: number
  top_p?: number
  max_tokens?: number
  excluded_context_item_ids?: string[]
  regenerate_message_id?: string
}

export type LLMConversation = {
  id: string
  project_id: string
  block_id: string
  title: string
  created_at: string
  updated_at: string
}

export type LLMConversationMessage = {
  id: string
  conversation_id: string
  role: 'user' | 'assistant'
  content: string
  generation_run_id: string | null
  model: string | null
  created_at: string
  updated_at: string
}

export type ContextItem = {
  id: string
  type: string
  title: string
  content: string
  source_id?: string
  estimated_tokens: number
  included: boolean
  required: boolean
  status?: 'valid' | 'stale'
}

export type SummarySnapshot = {
  id: string
  project_id: string
  target_type: 'block' | 'chapter' | 'branch'
  target_id: string
  summary_text: string
  covered_revision_ids: string[]
  token_count: number
  model: string | null
  status: 'valid' | 'stale' | 'failed'
  metadata: Record<string, unknown>
  created_at: string
}

export type GenerateSummaryInput = {
  project_id: string
  model_profile_id: string
}

export type ContextPreview = {
  system_prompt: string
  user_prompt: string
  final_prompt: string
  estimated_tokens: number
  token_budget: number
  items: ContextItem[]
  excluded_item_ids: string[]
  prompt_template_id: string | null
}

export type GenerateOnceResult = {
  output_text: string
  reasoning_text: string
  generation_run: GenerationRun
  prompt: string
  system_prompt: string
  user_prompt: string
  context_preview: ContextPreview
  model_profile_id: string
  prompt_template_id: string | null
  conversation_id: string | null
}

export type GenerateCandidatesResult = {
  candidates: GenerateOnceResult[]
}

export type GenerateStreamEvent = {
  type: 'delta' | 'reasoning' | 'done' | 'error'
  content?: string
  reasoning?: string
  generation_run?: GenerationRun
  prompt?: string
  system_prompt?: string
  user_prompt?: string
  context_preview?: ContextPreview
  model_profile_id?: string
  prompt_template_id?: string | null
  conversation_id?: string | null
  error?: string
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
  parent_revision_id?: string | null
  content: string
  content_format?: string
  source?: Revision['source']
  generation_run_id?: string | null
  metadata?: Record<string, unknown>
  set_current?: boolean
}

export type BlockAssociationsInput = {
  character_ids?: string[]
  location_id?: string | null
  tags?: string[]
}

export type CanonEntityInput = {
  type: CanonEntity['type']
  name: string
  aliases?: string[]
  description?: string | null
  attributes?: Record<string, unknown>
  importance?: number
  status?: CanonEntity['status']
}

export type MemoryChunkInput = {
  source_type: string
  source_id?: string | null
  chunk_text: string
  chunk_kind: string
  tags?: string[]
  metadata?: Record<string, unknown>
}

export type MemoryChunkFromBlockInput = {
  chunk_kind?: string
  tags?: string[]
  metadata?: Record<string, unknown>
}

export type MemorySearchInput = {
  q?: string
  source_type?: string
  chunk_kind?: string
  tag?: string
  mode?: 'keyword' | 'semantic'
  model_profile_id?: string
  limit?: number
}

export type MemoryReindexResult = {
  memory_indexed: number
  canon_indexed: number
  model: string
  dimensions: number
}

export type ModelProfileInput = {
  name: string
  provider: ModelProfile['provider']
  model: string
  base_url?: string | null
  api_key?: string | null
  clear_api_key?: boolean
  temperature?: number
  top_p?: number
  max_tokens?: number
  context_window?: number
  profile_type?: ModelProfile['profile_type']
  embedding_profile_id?: string | null
  clear_embedding_profile?: boolean
  embedding_dimensions?: number | null
  metadata?: Record<string, unknown>
}

export type PromptTemplateInput = {
  name: string
  task_type: PromptTemplate['task_type']
  template_text: string
  version?: number
  is_default?: boolean
  metadata?: Record<string, unknown>
}
