package canon

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

func (r *Repository) List(ctx context.Context, projectID string, filter ListFilter) ([]Entity, error) {
	clauses := []string{"project_id = $1"}
	args := []any{projectID}
	if filter.Type != "" {
		clauses = append(clauses, fmt.Sprintf("type = $%d", len(args)+1))
		args = append(args, filter.Type)
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)+1))
		args = append(args, filter.Status)
	}
	if filter.Query != "" {
		clauses = append(clauses, fmt.Sprintf("(name ILIKE $%d OR description ILIKE $%d OR $%d = ANY(aliases))", len(args)+1, len(args)+1, len(args)+2))
		args = append(args, "%"+filter.Query+"%", filter.Query)
	}

	rows, err := r.db.Query(ctx, selectEntitySQL+` WHERE `+strings.Join(clauses, " AND ")+` ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities := make([]Entity, 0)
	for rows.Next() {
		entity, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	return entities, rows.Err()
}

func (r *Repository) Create(ctx context.Context, projectID string, req CreateEntityRequest) (Entity, error) {
	req, err := req.normalized()
	if err != nil {
		return Entity{}, err
	}
	if !json.Valid(req.Attributes) {
		return Entity{}, fmt.Errorf("%w: attributes must be valid JSON", ErrInvalidCanonEntity)
	}

	entity, err := scanEntity(r.db.QueryRow(ctx, `
		INSERT INTO canon_entities (
			project_id,
			type,
			name,
			aliases,
			description,
			attributes,
			importance,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING
			id::text,
			project_id::text,
			type,
			name,
			aliases,
			description,
			attributes,
			importance,
			status,
			created_at,
			updated_at
	`, projectID, req.Type, req.Name, req.Aliases, nullableString(req.Description), req.Attributes, *req.Importance, req.Status))
	if err != nil {
		return Entity{}, err
	}
	return entity, nil
}

func (r *Repository) Get(ctx context.Context, entityID string) (Entity, error) {
	entity, err := scanEntity(r.db.QueryRow(ctx, selectEntitySQL+` WHERE id = $1`, entityID))
	if err != nil {
		return Entity{}, normalizeNotFound(err)
	}
	return entity, nil
}

func (r *Repository) Update(ctx context.Context, entityID string, req UpdateEntityRequest) (Entity, error) {
	setClauses := make([]string, 0, 8)
	args := make([]any, 0, 9)

	if req.Type != nil {
		entityType := normalizeType(*req.Type)
		if !isValidType(entityType) {
			return Entity{}, ErrInvalidCanonEntity
		}
		args = append(args, entityType)
		setClauses = append(setClauses, fmt.Sprintf("type = $%d", len(args)))
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return Entity{}, ErrInvalidCanonEntity
		}
		args = append(args, name)
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", len(args)))
	}
	if req.Aliases != nil {
		args = append(args, normalizeAliases(req.Aliases))
		setClauses = append(setClauses, fmt.Sprintf("aliases = $%d", len(args)))
	}
	if req.Description != nil {
		args = append(args, nullableString(normalizeOptionalString(req.Description)))
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", len(args)))
	}
	if len(req.Attributes) > 0 {
		if !json.Valid(req.Attributes) {
			return Entity{}, fmt.Errorf("%w: attributes must be valid JSON", ErrInvalidCanonEntity)
		}
		args = append(args, req.Attributes)
		setClauses = append(setClauses, fmt.Sprintf("attributes = $%d", len(args)))
	}
	if req.Importance != nil {
		if *req.Importance < 1 || *req.Importance > 10 {
			return Entity{}, ErrInvalidCanonEntity
		}
		args = append(args, *req.Importance)
		setClauses = append(setClauses, fmt.Sprintf("importance = $%d", len(args)))
	}
	if req.Status != nil {
		status := normalizeStatus(*req.Status)
		if !isValidStatus(status) {
			return Entity{}, ErrInvalidCanonEntity
		}
		args = append(args, status)
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if len(setClauses) == 0 {
		return r.Get(ctx, entityID)
	}

	args = append(args, entityID)
	query := fmt.Sprintf(updateEntitySQL, strings.Join(setClauses, ", "), len(args))
	entity, err := scanEntity(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		return Entity{}, normalizeNotFound(err)
	}
	return entity, nil
}

func (r *Repository) Delete(ctx context.Context, entityID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM canon_entities WHERE id = $1`, entityID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCanonEntityNotFound
	}
	return nil
}

type ListFilter struct {
	Type   string
	Status string
	Query  string
}

const selectEntitySQL = `
	SELECT
		id::text,
		project_id::text,
		type,
		name,
		aliases,
		description,
		attributes,
		importance,
		status,
		created_at,
		updated_at
	FROM canon_entities
`

const updateEntitySQL = `
	UPDATE canon_entities
	SET %s
	WHERE id = $%d
	RETURNING
		id::text,
		project_id::text,
		type,
		name,
		aliases,
		description,
		attributes,
		importance,
		status,
		created_at,
		updated_at
`

type entityScanner interface {
	Scan(dest ...any) error
}

func scanEntity(scanner entityScanner) (Entity, error) {
	var entity Entity
	var description sql.NullString
	err := scanner.Scan(
		&entity.ID,
		&entity.ProjectID,
		&entity.Type,
		&entity.Name,
		&entity.Aliases,
		&description,
		&entity.Attributes,
		&entity.Importance,
		&entity.Status,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		return Entity{}, err
	}
	if description.Valid {
		entity.Description = &description.String
	}
	if entity.Aliases == nil {
		entity.Aliases = []string{}
	}
	return entity, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCanonEntityNotFound
	}
	return err
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
