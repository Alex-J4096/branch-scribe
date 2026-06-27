package modelprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, projectID string) ([]ModelProfile, error) {
	rows, err := r.db.Query(ctx, selectModelProfileSQL+` WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := make([]ModelProfile, 0)
	for rows.Next() {
		profile, err := scanModelProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (r *Repository) Create(ctx context.Context, projectID string, req CreateModelProfileRequest) (ModelProfile, error) {
	req, err := req.normalized()
	if err != nil {
		return ModelProfile{}, err
	}

	temperature := 0.8
	if req.Temperature != nil {
		temperature = *req.Temperature
	}
	topP := 0.9
	if req.TopP != nil {
		topP = *req.TopP
	}
	maxTokens := 2048
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	contextWindow := 32768
	if req.ContextWindow != nil {
		contextWindow = *req.ContextWindow
	}
	apiKeyRef, err := normalizeAPIKeyStorage(req.APIKey)
	if err != nil {
		return ModelProfile{}, err
	}

	profile, err := scanModelProfile(r.db.QueryRow(ctx, insertModelProfileSQL,
		projectID,
		req.Name,
		req.Provider,
		req.Model,
		nullableString(req.BaseURL),
		nullableString(apiKeyRef),
		temperature,
		topP,
		maxTokens,
		contextWindow,
		req.Metadata,
	))
	if err != nil {
		return ModelProfile{}, err
	}
	return profile, nil
}

func (r *Repository) Get(ctx context.Context, profileID string) (ModelProfile, error) {
	profile, err := scanModelProfile(r.db.QueryRow(ctx, selectModelProfileSQL+` WHERE id = $1`, profileID))
	if err != nil {
		return ModelProfile{}, normalizeNotFound(err)
	}
	return profile, nil
}

func (r *Repository) Update(ctx context.Context, profileID string, req UpdateModelProfileRequest) (ModelProfile, error) {
	setClauses := make([]string, 0, 10)
	args := make([]any, 0, 11)

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return ModelProfile{}, ErrInvalidModelProfile
		}
		args = append(args, name)
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", len(args)))
	}
	if req.Provider != nil {
		args = append(args, normalizeProvider(*req.Provider))
		setClauses = append(setClauses, fmt.Sprintf("provider = $%d", len(args)))
	}
	if req.Model != nil {
		model := strings.TrimSpace(*req.Model)
		if model == "" {
			return ModelProfile{}, ErrInvalidModelProfile
		}
		args = append(args, model)
		setClauses = append(setClauses, fmt.Sprintf("model = $%d", len(args)))
	}
	if req.BaseURL != nil {
		args = append(args, nullableString(normalizeOptionalString(req.BaseURL)))
		setClauses = append(setClauses, fmt.Sprintf("base_url = $%d", len(args)))
	}
	if req.APIKey != nil {
		apiKeyRef, err := normalizeAPIKeyStorage(req.APIKey)
		if err != nil {
			return ModelProfile{}, err
		}
		args = append(args, nullableString(apiKeyRef))
		setClauses = append(setClauses, fmt.Sprintf("api_key_ref = $%d", len(args)))
	} else if req.ClearAPIKey != nil && *req.ClearAPIKey {
		args = append(args, nil)
		setClauses = append(setClauses, fmt.Sprintf("api_key_ref = $%d", len(args)))
	}
	if req.Temperature != nil {
		args = append(args, *req.Temperature)
		setClauses = append(setClauses, fmt.Sprintf("temperature = $%d", len(args)))
	}
	if req.TopP != nil {
		args = append(args, *req.TopP)
		setClauses = append(setClauses, fmt.Sprintf("top_p = $%d", len(args)))
	}
	if req.MaxTokens != nil {
		args = append(args, *req.MaxTokens)
		setClauses = append(setClauses, fmt.Sprintf("max_tokens = $%d", len(args)))
	}
	if req.ContextWindow != nil {
		args = append(args, *req.ContextWindow)
		setClauses = append(setClauses, fmt.Sprintf("context_window = $%d", len(args)))
	}
	if len(req.Metadata) > 0 {
		if !json.Valid(req.Metadata) {
			return ModelProfile{}, fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidModelProfile)
		}
		args = append(args, req.Metadata)
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", len(args)))
	}

	if len(setClauses) == 0 {
		return r.Get(ctx, profileID)
	}

	args = append(args, profileID)
	query := fmt.Sprintf(updateModelProfileSQL, strings.Join(setClauses, ", "), len(args))
	profile, err := scanModelProfile(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		return ModelProfile{}, normalizeNotFound(err)
	}
	return profile, nil
}

func (r *Repository) Delete(ctx context.Context, profileID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM model_profiles WHERE id = $1`, profileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrModelProfileNotFound
	}
	return nil
}

const selectModelProfileSQL = `
	SELECT
		id::text,
		project_id::text,
		name,
		provider,
		model,
		base_url,
		(api_key_ref IS NOT NULL),
		temperature,
		top_p,
		max_tokens,
		context_window,
		metadata,
		created_at,
		updated_at
	FROM model_profiles
`

const insertModelProfileSQL = `
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
		metadata
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	RETURNING
		id::text,
		project_id::text,
		name,
		provider,
		model,
		base_url,
		(api_key_ref IS NOT NULL),
		temperature,
		top_p,
		max_tokens,
		context_window,
		metadata,
		created_at,
		updated_at
`

const updateModelProfileSQL = `
	UPDATE model_profiles
	SET %s
	WHERE id = $%d
	RETURNING
		id::text,
		project_id::text,
		name,
		provider,
		model,
		base_url,
		(api_key_ref IS NOT NULL),
		temperature,
		top_p,
		max_tokens,
		context_window,
		metadata,
		created_at,
		updated_at
`

type scanner interface {
	Scan(dest ...any) error
}

func scanModelProfile(scanner scanner) (ModelProfile, error) {
	var profile ModelProfile
	var projectID sql.NullString
	var baseURL sql.NullString

	err := scanner.Scan(
		&profile.ID,
		&projectID,
		&profile.Name,
		&profile.Provider,
		&profile.Model,
		&baseURL,
		&profile.HasAPIKey,
		&profile.Temperature,
		&profile.TopP,
		&profile.MaxTokens,
		&profile.ContextWindow,
		&profile.Metadata,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		return ModelProfile{}, err
	}
	if projectID.Valid {
		profile.ProjectID = &projectID.String
	}
	if baseURL.Valid {
		profile.BaseURL = &baseURL.String
	}
	return profile, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrModelProfileNotFound
	}
	return err
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
