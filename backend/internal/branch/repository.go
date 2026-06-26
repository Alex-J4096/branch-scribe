package branch

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

func (r *Repository) List(ctx context.Context, projectID string) ([]Branch, error) {
	rows, err := r.db.Query(ctx, selectBranchSQL+` WHERE project_id = $1 ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	branches := make([]Branch, 0)
	for rows.Next() {
		branch, err := scanBranch(rows)
		if err != nil {
			return nil, err
		}
		branches = append(branches, branch)
	}

	return branches, rows.Err()
}

func (r *Repository) Create(ctx context.Context, projectID string, req CreateBranchRequest) (Branch, error) {
	name, err := normalizeName(req.Name)
	if err != nil {
		return Branch{}, err
	}

	return scanBranch(r.db.QueryRow(ctx, insertBranchSQL, projectID, name, nullableString(req.Description), nil, nil, nil, normalizeJSON(req.Metadata)))
}

func (r *Repository) Fork(ctx context.Context, projectID string, req ForkBranchRequest) (Branch, error) {
	name, err := normalizeName(req.Name)
	if err != nil {
		return Branch{}, err
	}

	return scanBranch(r.db.QueryRow(
		ctx,
		insertBranchSQL,
		projectID,
		name,
		nullableString(req.Description),
		nullableString(req.BaseBranchID),
		nullableString(req.ForkFromBlockID),
		nullableString(req.ForkFromRevisionID),
		normalizeJSON(req.Metadata),
	))
}

func (r *Repository) Update(ctx context.Context, branchID string, req UpdateBranchRequest) (Branch, error) {
	setClauses := make([]string, 0, 4)
	args := make([]any, 0, 5)

	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return Branch{}, err
		}
		args = append(args, name)
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", len(args)))
	}

	if req.Description != nil {
		args = append(args, *req.Description)
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", len(args)))
	}

	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if status != "active" && status != "archived" {
			return Branch{}, fmt.Errorf("%w: status must be active or archived", ErrInvalidBranch)
		}
		args = append(args, status)
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", len(args)))
	}

	if len(req.Metadata) > 0 {
		if !json.Valid(req.Metadata) {
			return Branch{}, fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidBranch)
		}
		args = append(args, req.Metadata)
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", len(args)))
	}

	if len(setClauses) == 0 {
		return r.Get(ctx, branchID)
	}

	args = append(args, branchID)
	query := fmt.Sprintf(updateBranchSQL, strings.Join(setClauses, ", "), len(args))

	branch, err := scanBranch(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		return Branch{}, normalizeNotFound(err)
	}
	return branch, nil
}

func (r *Repository) Get(ctx context.Context, branchID string) (Branch, error) {
	branch, err := scanBranch(r.db.QueryRow(ctx, selectBranchSQL+` WHERE id = $1`, branchID))
	if err != nil {
		return Branch{}, normalizeNotFound(err)
	}
	return branch, nil
}

func (r *Repository) Delete(ctx context.Context, branchID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM branches WHERE id = $1`, branchID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBranchNotFound
	}
	return nil
}

const selectBranchSQL = `
	SELECT
		id::text,
		project_id::text,
		name,
		description,
		base_branch_id::text,
		fork_from_block_id::text,
		fork_from_revision_id::text,
		status,
		metadata,
		created_at,
		updated_at
	FROM branches
`

const insertBranchSQL = `
	INSERT INTO branches (
		project_id,
		name,
		description,
		base_branch_id,
		fork_from_block_id,
		fork_from_revision_id,
		metadata
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING
		id::text,
		project_id::text,
		name,
		description,
		base_branch_id::text,
		fork_from_block_id::text,
		fork_from_revision_id::text,
		status,
		metadata,
		created_at,
		updated_at
`

const updateBranchSQL = `
	UPDATE branches
	SET %s
	WHERE id = $%d
	RETURNING
		id::text,
		project_id::text,
		name,
		description,
		base_branch_id::text,
		fork_from_block_id::text,
		fork_from_revision_id::text,
		status,
		metadata,
		created_at,
		updated_at
`

type branchScanner interface {
	Scan(dest ...any) error
}

func scanBranch(scanner branchScanner) (Branch, error) {
	var branch Branch
	var description sql.NullString
	var baseBranchID sql.NullString
	var forkFromBlockID sql.NullString
	var forkFromRevisionID sql.NullString

	err := scanner.Scan(
		&branch.ID,
		&branch.ProjectID,
		&branch.Name,
		&description,
		&baseBranchID,
		&forkFromBlockID,
		&forkFromRevisionID,
		&branch.Status,
		&branch.Metadata,
		&branch.CreatedAt,
		&branch.UpdatedAt,
	)
	if err != nil {
		return Branch{}, err
	}

	if description.Valid {
		branch.Description = &description.String
	}
	if baseBranchID.Valid {
		branch.BaseBranchID = &baseBranchID.String
	}
	if forkFromBlockID.Valid {
		branch.ForkFromBlockID = &forkFromBlockID.String
	}
	if forkFromRevisionID.Valid {
		branch.ForkFromRevisionID = &forkFromRevisionID.String
	}

	return branch, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBranchNotFound
	}
	return err
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
