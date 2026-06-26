CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT,
    default_language TEXT NOT NULL DEFAULT 'zh',
    default_style_profile JSONB NOT NULL DEFAULT '{}'::jsonb,
    default_model_profile_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS branches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    base_branch_id UUID REFERENCES branches(id),
    fork_from_block_id UUID,
    fork_from_revision_id UUID,
    status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT branches_status_check CHECK (status IN ('active', 'archived'))
);

CREATE TABLE IF NOT EXISTS blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_id UUID REFERENCES branches(id) ON DELETE SET NULL,
    type TEXT NOT NULL,
    title TEXT,
    current_revision_id UUID,
    parent_block_id UUID REFERENCES blocks(id) ON DELETE SET NULL,
    position_x DOUBLE PRECISION NOT NULL DEFAULT 0,
    position_y DOUBLE PRECISION NOT NULL DEFAULT 0,
    order_index INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT blocks_type_check CHECK (type IN ('scene', 'chapter', 'note', 'summary', 'canon', 'outline'))
);

CREATE TABLE IF NOT EXISTS block_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    block_id UUID NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
    parent_revision_id UUID REFERENCES block_revisions(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    content_format TEXT NOT NULL DEFAULT 'markdown',
    content_hash TEXT,
    source TEXT NOT NULL,
    generation_run_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT block_revisions_content_format_check CHECK (content_format IN ('markdown', 'html', 'text')),
    CONSTRAINT block_revisions_source_check CHECK (source IN ('user', 'llm', 'import', 'system'))
);

CREATE TABLE IF NOT EXISTS graph_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_block_id UUID NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
    target_block_id UUID NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
    edge_type TEXT NOT NULL,
    label TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT graph_edges_edge_type_check CHECK (edge_type IN ('next', 'fork', 'alternative', 'references', 'summarizes')),
    CONSTRAINT graph_edges_no_self_edge_check CHECK (source_block_id <> target_block_id)
);

CREATE TABLE IF NOT EXISTS canon_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    aliases TEXT[] NOT NULL DEFAULT '{}',
    description TEXT,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    importance INTEGER NOT NULL DEFAULT 5,
    status TEXT NOT NULL DEFAULT 'canon',
    embedding VECTOR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT canon_entities_type_check CHECK (type IN ('character', 'location', 'faction', 'item', 'rule', 'event')),
    CONSTRAINT canon_entities_importance_check CHECK (importance BETWEEN 1 AND 10),
    CONSTRAINT canon_entities_status_check CHECK (status IN ('canon', 'draft', 'deprecated'))
);

CREATE TABLE IF NOT EXISTS memory_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    source_id UUID,
    chunk_text TEXT NOT NULL,
    chunk_kind TEXT NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    embedding VECTOR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS summary_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    summary_text TEXT NOT NULL,
    covered_revision_ids UUID[] NOT NULL DEFAULT '{}',
    token_count INTEGER NOT NULL DEFAULT 0,
    model TEXT,
    status TEXT NOT NULL DEFAULT 'valid',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT summary_snapshots_status_check CHECK (status IN ('valid', 'stale', 'failed'))
);

CREATE TABLE IF NOT EXISTS model_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    base_url TEXT,
    api_key_ref TEXT,
    temperature DOUBLE PRECISION NOT NULL DEFAULT 0.8,
    top_p DOUBLE PRECISION NOT NULL DEFAULT 0.9,
    max_tokens INTEGER NOT NULL DEFAULT 2048,
    context_window INTEGER NOT NULL DEFAULT 32768,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT model_profiles_provider_check CHECK (provider IN ('openai_compatible', 'openai', 'anthropic', 'gemini', 'openrouter', 'deepseek', 'moonshot')),
    CONSTRAINT model_profiles_temperature_check CHECK (temperature >= 0 AND temperature <= 2),
    CONSTRAINT model_profiles_top_p_check CHECK (top_p >= 0 AND top_p <= 1),
    CONSTRAINT model_profiles_max_tokens_check CHECK (max_tokens > 0),
    CONSTRAINT model_profiles_context_window_check CHECK (context_window > 0)
);

CREATE TABLE IF NOT EXISTS prompt_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    task_type TEXT NOT NULL,
    template_text TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    is_default BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT prompt_templates_version_check CHECK (version > 0)
);

CREATE TABLE IF NOT EXISTS generation_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    block_id UUID REFERENCES blocks(id) ON DELETE SET NULL,
    task_type TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    temperature DOUBLE PRECISION,
    top_p DOUBLE PRECISION,
    max_tokens INTEGER,
    context_window INTEGER,
    prompt_template_id UUID REFERENCES prompt_templates(id) ON DELETE SET NULL,
    input_context_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_revision_id UUID,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT generation_runs_status_check CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT generation_runs_input_tokens_check CHECK (input_tokens >= 0),
    CONSTRAINT generation_runs_output_tokens_check CHECK (output_tokens >= 0),
    CONSTRAINT generation_runs_latency_ms_check CHECK (latency_ms >= 0)
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'projects_default_model_profile_id_fkey') THEN
        ALTER TABLE projects
            ADD CONSTRAINT projects_default_model_profile_id_fkey
            FOREIGN KEY (default_model_profile_id) REFERENCES model_profiles(id) ON DELETE SET NULL;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'branches_fork_from_block_id_fkey') THEN
        ALTER TABLE branches
            ADD CONSTRAINT branches_fork_from_block_id_fkey
            FOREIGN KEY (fork_from_block_id) REFERENCES blocks(id) ON DELETE SET NULL;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'branches_fork_from_revision_id_fkey') THEN
        ALTER TABLE branches
            ADD CONSTRAINT branches_fork_from_revision_id_fkey
            FOREIGN KEY (fork_from_revision_id) REFERENCES block_revisions(id) ON DELETE SET NULL;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'blocks_current_revision_id_fkey') THEN
        ALTER TABLE blocks
            ADD CONSTRAINT blocks_current_revision_id_fkey
            FOREIGN KEY (current_revision_id) REFERENCES block_revisions(id) ON DELETE SET NULL;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'block_revisions_generation_run_id_fkey') THEN
        ALTER TABLE block_revisions
            ADD CONSTRAINT block_revisions_generation_run_id_fkey
            FOREIGN KEY (generation_run_id) REFERENCES generation_runs(id) ON DELETE SET NULL;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'generation_runs_output_revision_id_fkey') THEN
        ALTER TABLE generation_runs
            ADD CONSTRAINT generation_runs_output_revision_id_fkey
            FOREIGN KEY (output_revision_id) REFERENCES block_revisions(id) ON DELETE SET NULL;
    END IF;
END;
$$;

CREATE INDEX IF NOT EXISTS idx_projects_created_at ON projects(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_branches_project_id ON branches(project_id);
CREATE INDEX IF NOT EXISTS idx_branches_base_branch_id ON branches(base_branch_id);
CREATE INDEX IF NOT EXISTS idx_branches_status ON branches(status);
CREATE INDEX IF NOT EXISTS idx_blocks_project_id ON blocks(project_id);
CREATE INDEX IF NOT EXISTS idx_blocks_branch_id ON blocks(branch_id);
CREATE INDEX IF NOT EXISTS idx_blocks_parent_block_id ON blocks(parent_block_id);
CREATE INDEX IF NOT EXISTS idx_blocks_type ON blocks(type);
CREATE INDEX IF NOT EXISTS idx_block_revisions_block_id_created_at ON block_revisions(block_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_graph_edges_project_id ON graph_edges(project_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_source_block_id ON graph_edges(source_block_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_target_block_id ON graph_edges(target_block_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_edges_unique_connection
    ON graph_edges(project_id, source_block_id, target_block_id, edge_type);
CREATE INDEX IF NOT EXISTS idx_canon_entities_project_type ON canon_entities(project_id, type);
CREATE INDEX IF NOT EXISTS idx_canon_entities_project_status ON canon_entities(project_id, status);
CREATE INDEX IF NOT EXISTS idx_canon_entities_name ON canon_entities USING gin (to_tsvector('simple', name));
CREATE INDEX IF NOT EXISTS idx_memory_chunks_project_kind ON memory_chunks(project_id, chunk_kind);
CREATE INDEX IF NOT EXISTS idx_memory_chunks_tags ON memory_chunks USING gin (tags);
CREATE INDEX IF NOT EXISTS idx_summary_snapshots_project_target ON summary_snapshots(project_id, target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_summary_snapshots_status ON summary_snapshots(status);
CREATE INDEX IF NOT EXISTS idx_model_profiles_project_id ON model_profiles(project_id);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_project_task ON prompt_templates(project_id, task_type);
CREATE INDEX IF NOT EXISTS idx_generation_runs_project_created_at ON generation_runs(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_generation_runs_block_id ON generation_runs(block_id);

CREATE OR REPLACE TRIGGER set_projects_updated_at
    BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE TRIGGER set_branches_updated_at
    BEFORE UPDATE ON branches
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE TRIGGER set_blocks_updated_at
    BEFORE UPDATE ON blocks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE TRIGGER set_canon_entities_updated_at
    BEFORE UPDATE ON canon_entities
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE TRIGGER set_model_profiles_updated_at
    BEFORE UPDATE ON model_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE TRIGGER set_prompt_templates_updated_at
    BEFORE UPDATE ON prompt_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
