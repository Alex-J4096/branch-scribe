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
	`)
	return err
}
