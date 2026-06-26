package generation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

func (r *Repository) GetModelProfile(ctx context.Context, projectID string, profileID string) (ModelProfile, error) {
	var profile ModelProfile
	var baseURL sql.NullString
	var apiKey sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT id::text, provider, model, base_url, api_key_ref, temperature, top_p, max_tokens, context_window
		FROM model_profiles
		WHERE id = $1 AND project_id = $2
	`, profileID, projectID).Scan(
		&profile.ID,
		&profile.Provider,
		&profile.Model,
		&baseURL,
		&apiKey,
		&profile.Temperature,
		&profile.TopP,
		&profile.MaxTokens,
		&profile.ContextWindow,
	)
	if err != nil {
		return ModelProfile{}, normalizeNotFound(err)
	}
	if baseURL.Valid {
		profile.BaseURL = &baseURL.String
	}
	if apiKey.Valid {
		resolved, err := resolveAPIKeyRef(apiKey.String)
		if err != nil {
			return ModelProfile{}, err
		}
		profile.APIKey = &resolved
	}
	return profile, nil
}

func resolveAPIKeyRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	envName, ok := strings.CutPrefix(ref, "env:")
	if !ok || strings.TrimSpace(envName) == "" {
		return "", fmt.Errorf("%w: api key must be stored as env reference", ErrInvalidGenerationRequest)
	}
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return "", fmt.Errorf("%w: api key environment variable %s is not configured", ErrInvalidGenerationRequest, envName)
	}
	return value, nil
}

func (r *Repository) GetBlockContext(ctx context.Context, projectID string, blockID string) (BlockContext, error) {
	var blockContext BlockContext
	var projectDescription sql.NullString
	var title sql.NullString
	var content sql.NullString
	var contentFormat sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT p.description, b.title, br.content, br.content_format
		FROM blocks b
		JOIN projects p ON p.id = b.project_id
		LEFT JOIN block_revisions br ON br.id = b.current_revision_id
		WHERE b.id = $1 AND b.project_id = $2
	`, blockID, projectID).Scan(&projectDescription, &title, &content, &contentFormat)
	if err != nil {
		return BlockContext{}, normalizeNotFound(err)
	}
	if projectDescription.Valid {
		blockContext.ProjectDescription = &projectDescription.String
	}
	if title.Valid {
		blockContext.BlockTitle = &title.String
	}
	if content.Valid {
		blockContext.Content = content.String
	}
	if contentFormat.Valid {
		blockContext.ContentFormat = contentFormat.String
	}
	return blockContext, nil
}

func (r *Repository) GetBlockMetadataContext(ctx context.Context, projectID string, blockID string) (BlockContext, error) {
	var blockContext BlockContext
	var projectDescription sql.NullString
	var title sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT p.description, b.title
		FROM blocks b
		JOIN projects p ON p.id = b.project_id
		WHERE b.id = $1 AND b.project_id = $2
	`, blockID, projectID).Scan(&projectDescription, &title)
	if err != nil {
		return BlockContext{}, normalizeNotFound(err)
	}
	if projectDescription.Valid {
		blockContext.ProjectDescription = &projectDescription.String
	}
	if title.Valid {
		blockContext.BlockTitle = &title.String
	}
	return blockContext, nil
}

func (r *Repository) GetPromptTemplate(ctx context.Context, projectID string, templateID string) (PromptTemplate, error) {
	var template PromptTemplate
	err := r.db.QueryRow(ctx, `
		SELECT id::text, task_type, template_text
		FROM prompt_templates
		WHERE id = $1 AND project_id = $2
	`, templateID, projectID).Scan(&template.ID, &template.TaskType, &template.TemplateText)
	if err != nil {
		return PromptTemplate{}, normalizeNotFound(err)
	}
	return template, nil
}

func (r *Repository) GetDefaultPromptTemplate(ctx context.Context, projectID string, taskType string) (PromptTemplate, error) {
	var template PromptTemplate
	err := r.db.QueryRow(ctx, `
		SELECT id::text, task_type, template_text
		FROM prompt_templates
		WHERE project_id = $1 AND task_type = $2 AND is_default = true
		ORDER BY updated_at DESC
		LIMIT 1
	`, projectID, taskType).Scan(&template.ID, &template.TaskType, &template.TemplateText)
	if err != nil {
		return PromptTemplate{}, normalizeNotFound(err)
	}
	return template, nil
}

func (r *Repository) CreateRun(ctx context.Context, input GenerationRunInput) (GenerationRun, error) {
	run, err := scanGenerationRun(r.db.QueryRow(ctx, `
		INSERT INTO generation_runs (
			project_id,
			block_id,
			task_type,
			provider,
			model,
			temperature,
			top_p,
			max_tokens,
			context_window,
			prompt_template_id,
			input_context_snapshot,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'running')
		RETURNING
			id::text,
			project_id::text,
			block_id::text,
			task_type,
			provider,
			model,
			temperature,
			top_p,
			max_tokens,
			context_window,
			prompt_template_id::text,
			input_context_snapshot,
			output_revision_id::text,
			input_tokens,
			output_tokens,
			latency_ms,
			status,
			error_message,
			created_at
	`, input.ProjectID, nullableString(input.BlockID), input.TaskType, input.Provider, input.Model, input.Temperature, input.TopP, input.MaxTokens, input.ContextWindow, nullableString(input.PromptTemplateID), input.InputContextSnapshot))
	if err != nil {
		return GenerationRun{}, err
	}
	return run, nil
}

func (r *Repository) MarkRunSucceeded(ctx context.Context, runID string, result CompletionResult, latencyMS int) (GenerationRun, error) {
	run, err := scanGenerationRun(r.db.QueryRow(ctx, updateGenerationRunSQL, "succeeded", nil, result.InputTokens, result.OutputTokens, latencyMS, runID))
	if err != nil {
		return GenerationRun{}, normalizeNotFound(err)
	}
	return run, nil
}

func (r *Repository) MarkRunFailed(ctx context.Context, runID string, message string, latencyMS int) (GenerationRun, error) {
	run, err := scanGenerationRun(r.db.QueryRow(ctx, updateGenerationRunSQL, "failed", message, 0, 0, latencyMS, runID))
	if err != nil {
		return GenerationRun{}, normalizeNotFound(err)
	}
	return run, nil
}

const updateGenerationRunSQL = `
	UPDATE generation_runs
	SET
		status = $1,
		error_message = $2,
		input_tokens = $3,
		output_tokens = $4,
		latency_ms = $5
	WHERE id = $6
	RETURNING
		id::text,
		project_id::text,
		block_id::text,
		task_type,
		provider,
		model,
		temperature,
		top_p,
		max_tokens,
		context_window,
		prompt_template_id::text,
		input_context_snapshot,
		output_revision_id::text,
		input_tokens,
		output_tokens,
		latency_ms,
		status,
		error_message,
		created_at
`

type scanner interface {
	Scan(dest ...any) error
}

func scanGenerationRun(scanner scanner) (GenerationRun, error) {
	var run GenerationRun
	var blockID sql.NullString
	var temperature sql.NullFloat64
	var topP sql.NullFloat64
	var maxTokens sql.NullInt64
	var contextWindow sql.NullInt64
	var promptTemplateID sql.NullString
	var outputRevisionID sql.NullString
	var errorMessage sql.NullString

	err := scanner.Scan(
		&run.ID,
		&run.ProjectID,
		&blockID,
		&run.TaskType,
		&run.Provider,
		&run.Model,
		&temperature,
		&topP,
		&maxTokens,
		&contextWindow,
		&promptTemplateID,
		&run.InputContextSnapshot,
		&outputRevisionID,
		&run.InputTokens,
		&run.OutputTokens,
		&run.LatencyMS,
		&run.Status,
		&errorMessage,
		&run.CreatedAt,
	)
	if err != nil {
		return GenerationRun{}, err
	}
	if blockID.Valid {
		run.BlockID = &blockID.String
	}
	if temperature.Valid {
		run.Temperature = &temperature.Float64
	}
	if topP.Valid {
		run.TopP = &topP.Float64
	}
	if maxTokens.Valid {
		value := int(maxTokens.Int64)
		run.MaxTokens = &value
	}
	if contextWindow.Valid {
		value := int(contextWindow.Int64)
		run.ContextWindow = &value
	}
	if promptTemplateID.Valid {
		run.PromptTemplateID = &promptTemplateID.String
	}
	if outputRevisionID.Valid {
		run.OutputRevisionID = &outputRevisionID.String
	}
	if errorMessage.Valid {
		run.ErrorMessage = &errorMessage.String
	}
	if len(run.InputContextSnapshot) == 0 {
		run.InputContextSnapshot = json.RawMessage(`{}`)
	}
	return run, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrGenerationResourceNotFound
	}
	return err
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func providerError(message string) error {
	return fmt.Errorf("%w: %s", ErrProviderRequestFailed, message)
}
