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
		DROP CONSTRAINT IF EXISTS model_profiles_api_key_ref_check
	`)
	return err
}
