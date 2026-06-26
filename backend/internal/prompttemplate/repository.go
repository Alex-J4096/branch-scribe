package prompttemplate

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

func (r *Repository) List(ctx context.Context, projectID string, taskType string) ([]PromptTemplate, error) {
	args := []any{projectID}
	query := selectPromptTemplateSQL + ` WHERE project_id = $1`
	if strings.TrimSpace(taskType) != "" {
		args = append(args, strings.TrimSpace(taskType))
		query += fmt.Sprintf(" AND task_type = $%d", len(args))
	}
	query += ` ORDER BY task_type ASC, is_default DESC, created_at DESC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]PromptTemplate, 0)
	for rows.Next() {
		template, err := scanPromptTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, rows.Err()
}

func (r *Repository) Create(ctx context.Context, projectID string, req CreatePromptTemplateRequest) (PromptTemplate, error) {
	req, err := req.normalized()
	if err != nil {
		return PromptTemplate{}, err
	}

	version := 1
	if req.Version != nil {
		version = *req.Version
	}
	isDefault := false
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return PromptTemplate{}, err
	}
	defer tx.Rollback(ctx)

	if isDefault {
		if err := clearDefault(ctx, tx, projectID, req.TaskType, nil); err != nil {
			return PromptTemplate{}, err
		}
	}

	template, err := scanPromptTemplate(tx.QueryRow(ctx, insertPromptTemplateSQL,
		projectID,
		req.Name,
		req.TaskType,
		req.TemplateText,
		version,
		isDefault,
		req.Metadata,
	))
	if err != nil {
		return PromptTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PromptTemplate{}, err
	}
	return template, nil
}

func (r *Repository) Get(ctx context.Context, templateID string) (PromptTemplate, error) {
	template, err := scanPromptTemplate(r.db.QueryRow(ctx, selectPromptTemplateSQL+` WHERE id = $1`, templateID))
	if err != nil {
		return PromptTemplate{}, normalizeNotFound(err)
	}
	return template, nil
}

func (r *Repository) Update(ctx context.Context, templateID string, req UpdatePromptTemplateRequest) (PromptTemplate, error) {
	current, err := r.Get(ctx, templateID)
	if err != nil {
		return PromptTemplate{}, err
	}

	setClauses := make([]string, 0, 6)
	args := make([]any, 0, 7)
	taskTypeForDefault := current.TaskType

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return PromptTemplate{}, ErrInvalidPromptTemplate
		}
		args = append(args, name)
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", len(args)))
	}
	if req.TaskType != nil {
		taskType := strings.TrimSpace(*req.TaskType)
		if taskType == "" {
			return PromptTemplate{}, ErrInvalidPromptTemplate
		}
		taskTypeForDefault = taskType
		args = append(args, taskType)
		setClauses = append(setClauses, fmt.Sprintf("task_type = $%d", len(args)))
	}
	if req.TemplateText != nil {
		templateText := strings.TrimSpace(*req.TemplateText)
		if templateText == "" {
			return PromptTemplate{}, ErrInvalidPromptTemplate
		}
		args = append(args, templateText)
		setClauses = append(setClauses, fmt.Sprintf("template_text = $%d", len(args)))
	}
	if req.Version != nil {
		if *req.Version <= 0 {
			return PromptTemplate{}, ErrInvalidPromptTemplate
		}
		args = append(args, *req.Version)
		setClauses = append(setClauses, fmt.Sprintf("version = $%d", len(args)))
	}
	if req.IsDefault != nil {
		args = append(args, *req.IsDefault)
		setClauses = append(setClauses, fmt.Sprintf("is_default = $%d", len(args)))
	}
	if len(req.Metadata) > 0 {
		if !json.Valid(req.Metadata) {
			return PromptTemplate{}, fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidPromptTemplate)
		}
		args = append(args, req.Metadata)
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", len(args)))
	}

	if len(setClauses) == 0 {
		return current, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return PromptTemplate{}, err
	}
	defer tx.Rollback(ctx)

	if req.IsDefault != nil && *req.IsDefault {
		projectID := ""
		if current.ProjectID != nil {
			projectID = *current.ProjectID
		}
		if err := clearDefault(ctx, tx, projectID, taskTypeForDefault, &templateID); err != nil {
			return PromptTemplate{}, err
		}
	}

	args = append(args, templateID)
	query := fmt.Sprintf(updatePromptTemplateSQL, strings.Join(setClauses, ", "), len(args))
	template, err := scanPromptTemplate(tx.QueryRow(ctx, query, args...))
	if err != nil {
		return PromptTemplate{}, normalizeNotFound(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PromptTemplate{}, err
	}
	return template, nil
}

func (r *Repository) Delete(ctx context.Context, templateID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM prompt_templates WHERE id = $1`, templateID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPromptTemplateNotFound
	}
	return nil
}

const selectPromptTemplateSQL = `
	SELECT
		id::text,
		project_id::text,
		name,
		task_type,
		template_text,
		version,
		is_default,
		metadata,
		created_at,
		updated_at
	FROM prompt_templates
`

const insertPromptTemplateSQL = `
	INSERT INTO prompt_templates (
		project_id,
		name,
		task_type,
		template_text,
		version,
		is_default,
		metadata
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING
		id::text,
		project_id::text,
		name,
		task_type,
		template_text,
		version,
		is_default,
		metadata,
		created_at,
		updated_at
`

const updatePromptTemplateSQL = `
	UPDATE prompt_templates
	SET %s
	WHERE id = $%d
	RETURNING
		id::text,
		project_id::text,
		name,
		task_type,
		template_text,
		version,
		is_default,
		metadata,
		created_at,
		updated_at
`

type scanner interface {
	Scan(dest ...any) error
}

func scanPromptTemplate(scanner scanner) (PromptTemplate, error) {
	var template PromptTemplate
	var projectID sql.NullString

	err := scanner.Scan(
		&template.ID,
		&projectID,
		&template.Name,
		&template.TaskType,
		&template.TemplateText,
		&template.Version,
		&template.IsDefault,
		&template.Metadata,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return PromptTemplate{}, err
	}
	if projectID.Valid {
		template.ProjectID = &projectID.String
	}
	return template, nil
}

func clearDefault(ctx context.Context, tx pgx.Tx, projectID string, taskType string, exceptID *string) error {
	if exceptID == nil {
		_, err := tx.Exec(ctx, `UPDATE prompt_templates SET is_default = false WHERE project_id = $1 AND task_type = $2`, projectID, taskType)
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE prompt_templates SET is_default = false WHERE project_id = $1 AND task_type = $2 AND id <> $3`, projectID, taskType, *exceptID)
	return err
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPromptTemplateNotFound
	}
	return err
}
