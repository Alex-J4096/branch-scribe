package project

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

func (r *Repository) List(ctx context.Context) ([]Project, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id::text,
			name,
			description,
			default_language,
			default_style_profile,
			default_model_profile_id::text,
			metadata,
			created_at,
			updated_at
		FROM projects
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

func (r *Repository) Create(ctx context.Context, req CreateProjectRequest) (Project, error) {
	req, err := req.normalized()
	if err != nil {
		return Project{}, err
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Project{}, err
	}
	defer rollback(ctx, tx)

	project, err := queryProjectRow(ctx, tx.QueryRow(ctx, `
		INSERT INTO projects (name, description, default_language, default_style_profile, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id::text,
			name,
			description,
			default_language,
			default_style_profile,
			default_model_profile_id::text,
			metadata,
			created_at,
			updated_at
	`, req.Name, nullableString(req.Description), req.DefaultLanguage, req.DefaultStyleProfile, req.Metadata))
	if err != nil {
		return Project{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO branches (project_id, name, description)
		VALUES ($1, $2, $3)
	`, project.ID, "主线", "默认故事主线"); err != nil {
		return Project{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Project{}, err
	}

	return project, nil
}

func (r *Repository) Get(ctx context.Context, id string) (Project, error) {
	project, err := queryProjectRow(ctx, r.db.QueryRow(ctx, selectProjectByIDSQL, id))
	if err != nil {
		return Project{}, normalizeNotFound(err)
	}
	return project, nil
}

func (r *Repository) Update(ctx context.Context, id string, req UpdateProjectRequest) (Project, error) {
	setClauses := make([]string, 0, 5)
	args := make([]any, 0, 6)

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return Project{}, ErrInvalidProject
		}
		args = append(args, name)
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", len(args)))
	}

	if req.Description != nil {
		args = append(args, *req.Description)
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", len(args)))
	}

	if req.DefaultLanguage != nil {
		defaultLanguage := strings.TrimSpace(*req.DefaultLanguage)
		if defaultLanguage == "" {
			defaultLanguage = "zh"
		}
		args = append(args, defaultLanguage)
		setClauses = append(setClauses, fmt.Sprintf("default_language = $%d", len(args)))
	}

	if len(req.DefaultStyleProfile) > 0 {
		if !json.Valid(req.DefaultStyleProfile) {
			return Project{}, fmt.Errorf("%w: default_style_profile must be valid JSON", ErrInvalidProject)
		}
		args = append(args, req.DefaultStyleProfile)
		setClauses = append(setClauses, fmt.Sprintf("default_style_profile = $%d", len(args)))
	}

	if len(req.Metadata) > 0 {
		if !json.Valid(req.Metadata) {
			return Project{}, fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidProject)
		}
		args = append(args, req.Metadata)
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", len(args)))
	}

	if len(setClauses) == 0 {
		return r.Get(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE projects
		SET %s
		WHERE id = $%d
		RETURNING
			id::text,
			name,
			description,
			default_language,
			default_style_profile,
			default_model_profile_id::text,
			metadata,
			created_at,
			updated_at
	`, strings.Join(setClauses, ", "), len(args))

	project, err := queryProjectRow(ctx, r.db.QueryRow(ctx, query, args...))
	if err != nil {
		return Project{}, normalizeNotFound(err)
	}

	return project, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProjectNotFound
	}
	return nil
}

const selectProjectByIDSQL = `
	SELECT
		id::text,
		name,
		description,
		default_language,
		default_style_profile,
		default_model_profile_id::text,
		metadata,
		created_at,
		updated_at
	FROM projects
	WHERE id = $1
`

type projectScanner interface {
	Scan(dest ...any) error
}

func queryProjectRow(_ context.Context, row projectScanner) (Project, error) {
	return scanProject(row)
}

func scanProject(scanner projectScanner) (Project, error) {
	var project Project
	var description sql.NullString
	var defaultModelProfileID sql.NullString

	err := scanner.Scan(
		&project.ID,
		&project.Name,
		&description,
		&project.DefaultLanguage,
		&project.DefaultStyleProfile,
		&defaultModelProfileID,
		&project.Metadata,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return Project{}, err
	}

	if description.Valid {
		project.Description = &description.String
	}
	if defaultModelProfileID.Valid {
		project.DefaultModelProfileID = &defaultModelProfileID.String
	}

	return project, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProjectNotFound
	}
	return err
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
