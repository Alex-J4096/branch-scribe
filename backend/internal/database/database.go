package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.HealthCheckPeriod = 30 * time.Second

	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, cfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := ensureCompatibility(connectCtx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func ensureCompatibility(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		ALTER TABLE model_profiles
		DROP CONSTRAINT IF EXISTS model_profiles_api_key_ref_check;

		ALTER TABLE model_profiles
			ADD COLUMN IF NOT EXISTS profile_type TEXT NOT NULL DEFAULT 'llm',
			ADD COLUMN IF NOT EXISTS embedding_profile_id UUID REFERENCES model_profiles(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS embedding_dimensions INTEGER;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'model_profiles_profile_type_check'
			) THEN
				ALTER TABLE model_profiles
					ADD CONSTRAINT model_profiles_profile_type_check
					CHECK (profile_type IN ('llm', 'embedding'));
			END IF;
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'model_profiles_embedding_dimensions_check'
			) THEN
				ALTER TABLE model_profiles
					ADD CONSTRAINT model_profiles_embedding_dimensions_check
					CHECK (embedding_dimensions IS NULL OR embedding_dimensions > 0);
			END IF;
		END $$;

		INSERT INTO model_profiles (
			project_id,
			name,
			provider,
			model,
			base_url,
			api_key_ref,
			temperature,
			top_p,
			max_tokens,
			context_window,
			profile_type,
			embedding_dimensions,
			metadata
		)
		SELECT
			source.project_id,
			source.name || ' Embedding',
			source.provider,
			source.metadata->>'embedding_model',
			source.base_url,
			source.api_key_ref,
			0,
			1,
			1,
			32768,
			'embedding',
			NULLIF(source.metadata->>'embedding_dimensions', '')::integer,
			jsonb_build_object('migrated_from_profile_id', source.id::text)
		FROM model_profiles source
		WHERE source.profile_type = 'llm'
			AND NULLIF(source.metadata->>'embedding_model', '') IS NOT NULL
			AND NOT EXISTS (
				SELECT 1
				FROM model_profiles existing
				WHERE existing.profile_type = 'embedding'
					AND existing.metadata->>'migrated_from_profile_id' = source.id::text
			);

		UPDATE model_profiles source
		SET embedding_profile_id = embedding.id
		FROM model_profiles embedding
		WHERE source.profile_type = 'llm'
			AND source.embedding_profile_id IS NULL
			AND embedding.profile_type = 'embedding'
			AND embedding.metadata->>'migrated_from_profile_id' = source.id::text;

		UPDATE model_profiles
		SET metadata = metadata - 'embedding_model' - 'embedding_dimensions'
		WHERE profile_type = 'llm'
			AND embedding_profile_id IS NOT NULL
		;

		CREATE TABLE IF NOT EXISTS llm_conversations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			block_id UUID NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
			title TEXT NOT NULL DEFAULT '新对话',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS llm_messages (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			conversation_id UUID NOT NULL REFERENCES llm_conversations(id) ON DELETE CASCADE,
			role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
			content TEXT NOT NULL,
			generation_run_id UUID REFERENCES generation_runs(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_llm_conversations_block_updated
			ON llm_conversations(block_id, updated_at DESC);
		CREATE INDEX IF NOT EXISTS idx_llm_messages_conversation_created
			ON llm_messages(conversation_id, created_at, id);

		CREATE TABLE IF NOT EXISTS character_states (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			character_id UUID NOT NULL REFERENCES canon_entities(id) ON DELETE CASCADE,
			block_id UUID REFERENCES blocks(id) ON DELETE SET NULL,
			state_key TEXT NOT NULL,
			state_value JSONB NOT NULL DEFAULT '{}'::jsonb,
			notes TEXT,
			occurred_at TEXT,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS foreshadowings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'planted'
				CHECK (status IN ('planted', 'developed', 'resolved', 'abandoned')),
			planted_block_id UUID REFERENCES blocks(id) ON DELETE SET NULL,
			resolved_block_id UUID REFERENCES blocks(id) ON DELETE SET NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS timeline_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			description TEXT,
			event_time TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			block_id UUID REFERENCES blocks(id) ON DELETE SET NULL,
			canon_entity_id UUID REFERENCES canon_entities(id) ON DELETE SET NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_character_states_project_character
			ON character_states(project_id, character_id);
		CREATE INDEX IF NOT EXISTS idx_character_states_block ON character_states(block_id);
		CREATE INDEX IF NOT EXISTS idx_foreshadowings_project_status
			ON foreshadowings(project_id, status);
		CREATE INDEX IF NOT EXISTS idx_timeline_events_project_order
			ON timeline_events(project_id, sort_order, created_at);
		DROP TRIGGER IF EXISTS set_character_states_updated_at ON character_states;
		CREATE TRIGGER set_character_states_updated_at
			BEFORE UPDATE ON character_states
			FOR EACH ROW EXECUTE FUNCTION set_updated_at();
		DROP TRIGGER IF EXISTS set_foreshadowings_updated_at ON foreshadowings;
		CREATE TRIGGER set_foreshadowings_updated_at
			BEFORE UPDATE ON foreshadowings
			FOR EACH ROW EXECUTE FUNCTION set_updated_at();
		DROP TRIGGER IF EXISTS set_timeline_events_updated_at ON timeline_events;
		CREATE TRIGGER set_timeline_events_updated_at
			BEFORE UPDATE ON timeline_events
			FOR EACH ROW EXECUTE FUNCTION set_updated_at();

		CREATE OR REPLACE FUNCTION seed_default_prompt_operations(target_project_id UUID)
		RETURNS void
		LANGUAGE sql
		AS $seed$
			INSERT INTO prompt_templates (
				project_id, name, task_type, template_text, version, is_default, metadata
			)
			SELECT target_project_id, operation.name, operation.task_type,
				operation.template_text, 1, true, jsonb_build_object('built_in', true)
			FROM (
				VALUES
					('自由生成', 'free_write', E'请完全根据用户指令生成正文，不要依赖当前 block 正文。必须遵守硬设定，并参考相关记忆。只输出生成后的正文。\n\n项目简介：\n{{project_description}}\n\n硬设定：\n{{canon_facts}}\n\n相关记忆：\n{{memory_chunks}}\n\n用户指令：\n{{user_instruction}}'),
					('续写', 'continue', E'请基于当前片段继续写作，保持人物、语气和叙事连贯，必须遵守硬设定。\n\n硬设定：\n{{canon_facts}}\n\n分支摘要：\n{{branch_summary}}\n\n章节摘要：\n{{chapter_summary}}\n\n最近正文：\n{{recent_blocks}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'),
					('改写', 'rewrite_block', E'请根据用户指令改写当前片段，必须遵守硬设定，只输出改写后的正文。\n\n硬设定：\n{{canon_facts}}\n\n章节摘要：\n{{chapter_summary}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'),
					('局部改写', 'rewrite_selection', E'请在理解当前片段、前后文和硬设定的基础上改写选中文本，只输出改写后的选中文本。\n\n硬设定：\n{{canon_facts}}\n\n章节摘要：\n{{chapter_summary}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n选中文本：\n{{selected_text}}\n\n用户指令：\n{{user_instruction}}'),
					('扩写', 'expand', E'请扩写当前片段，补充细节、动作和感官描写，必须遵守硬设定，只输出扩写后的正文。\n\n硬设定：\n{{canon_facts}}\n\n最近正文：\n{{recent_blocks}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'),
					('缩写', 'condense', E'请压缩当前片段，保留关键情节、风格和硬设定，只输出压缩后的正文。\n\n硬设定：\n{{canon_facts}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'),
					('润色', 'polish', E'请润色当前片段，提升表达和节奏，必须遵守硬设定，只输出润色后的正文。\n\n硬设定：\n{{canon_facts}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}')
			) AS operation(name, task_type, template_text)
			WHERE NOT EXISTS (
				SELECT 1 FROM prompt_templates existing
				WHERE existing.project_id = target_project_id
					AND existing.task_type = operation.task_type
			);
		$seed$;

		CREATE OR REPLACE FUNCTION seed_default_prompt_operations_for_project()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $seed$
		BEGIN
			PERFORM seed_default_prompt_operations(NEW.id);
			RETURN NEW;
		END;
		$seed$;

		DROP TRIGGER IF EXISTS seed_project_prompt_operations ON projects;
		CREATE TRIGGER seed_project_prompt_operations
			AFTER INSERT ON projects
			FOR EACH ROW
			EXECUTE FUNCTION seed_default_prompt_operations_for_project();

		CREATE TABLE IF NOT EXISTS app_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		DO $seed$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM app_migrations WHERE name = 'default_prompt_operations_v1'
			) THEN
				PERFORM seed_default_prompt_operations(id) FROM projects;
				INSERT INTO app_migrations (name) VALUES ('default_prompt_operations_v1');
			END IF;
		END;
		$seed$;
	`)
	return err
}
