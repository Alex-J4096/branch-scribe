package memory

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

func (r *Repository) List(ctx context.Context, projectID string, filter ListFilter) ([]Chunk, error) {
	clauses := []string{"project_id = $1"}
	args := []any{projectID}
	if filter.SourceType != "" {
		clauses = append(clauses, fmt.Sprintf("source_type = $%d", len(args)+1))
		args = append(args, filter.SourceType)
	}
	if filter.ChunkKind != "" {
		clauses = append(clauses, fmt.Sprintf("chunk_kind = $%d", len(args)+1))
		args = append(args, filter.ChunkKind)
	}
	if filter.Tag != "" {
		clauses = append(clauses, fmt.Sprintf("$%d = ANY(tags)", len(args)+1))
		args = append(args, filter.Tag)
	}
	if filter.Query != "" {
		clauses = append(clauses, fmt.Sprintf("chunk_text ILIKE $%d", len(args)+1))
		args = append(args, "%"+filter.Query+"%")
	}

	rows, err := r.db.Query(ctx, selectChunkSQL+` WHERE `+strings.Join(clauses, " AND ")+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]Chunk, 0)
	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (r *Repository) Create(ctx context.Context, projectID string, req CreateChunkRequest) (Chunk, error) {
	req, err := req.normalized()
	if err != nil {
		return Chunk{}, err
	}
	if !json.Valid(req.Metadata) {
		return Chunk{}, fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidMemoryChunk)
	}

	chunk, err := scanChunk(r.db.QueryRow(ctx, `
		INSERT INTO memory_chunks (
			project_id,
			source_type,
			source_id,
			chunk_text,
			chunk_kind,
			tags,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id::text,
			project_id::text,
			source_type,
			source_id::text,
			chunk_text,
			chunk_kind,
			tags,
			metadata,
			created_at
	`, projectID, req.SourceType, nullableString(req.SourceID), req.ChunkText, req.ChunkKind, req.Tags, req.Metadata))
	if err != nil {
		return Chunk{}, err
	}
	return chunk, nil
}

func (r *Repository) CreateFromBlock(ctx context.Context, blockID string, req CreateFromBlockRequest) (Chunk, error) {
	chunkKind := normalizeRequiredText(req.ChunkKind)
	if chunkKind == "" {
		chunkKind = "block_revision"
	}
	tags := normalizeTags(req.Tags)
	metadata := normalizeJSON(req.Metadata)
	if !json.Valid(metadata) {
		return Chunk{}, fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidMemoryChunk)
	}

	var projectID string
	var revisionID string
	var content string
	var contentFormat string
	err := r.db.QueryRow(ctx, `
		SELECT
			b.project_id::text,
			br.id::text,
			br.content,
			br.content_format
		FROM blocks b
		JOIN block_revisions br ON br.id = b.current_revision_id
		WHERE b.id = $1
	`, blockID).Scan(&projectID, &revisionID, &content, &contentFormat)
	if err != nil {
		return Chunk{}, normalizeNotFound(err)
	}

	chunkText := normalizeContent(content, contentFormat)
	if chunkText == "" {
		return Chunk{}, fmt.Errorf("%w: block current revision is empty", ErrInvalidMemoryChunk)
	}

	return r.Create(ctx, projectID, CreateChunkRequest{
		SourceType: "block_revision",
		SourceID:   &revisionID,
		ChunkText:  chunkText,
		ChunkKind:  chunkKind,
		Tags:       tags,
		Metadata:   metadata,
	})
}

func (r *Repository) Get(ctx context.Context, chunkID string) (Chunk, error) {
	chunk, err := scanChunk(r.db.QueryRow(ctx, selectChunkSQL+` WHERE id = $1`, chunkID))
	if err != nil {
		return Chunk{}, normalizeNotFound(err)
	}
	return chunk, nil
}

func (r *Repository) Update(ctx context.Context, chunkID string, req UpdateChunkRequest) (Chunk, error) {
	setClauses := make([]string, 0, 6)
	args := make([]any, 0, 7)

	if req.SourceType != nil {
		sourceType := normalizeRequiredText(*req.SourceType)
		if sourceType == "" {
			return Chunk{}, ErrInvalidMemoryChunk
		}
		args = append(args, sourceType)
		setClauses = append(setClauses, fmt.Sprintf("source_type = $%d", len(args)))
	}
	if req.SourceID != nil {
		args = append(args, nullableString(normalizeOptionalString(req.SourceID)))
		setClauses = append(setClauses, fmt.Sprintf("source_id = $%d", len(args)))
	}
	if req.ChunkText != nil {
		chunkText := strings.TrimSpace(*req.ChunkText)
		if chunkText == "" {
			return Chunk{}, ErrInvalidMemoryChunk
		}
		args = append(args, chunkText)
		setClauses = append(setClauses, fmt.Sprintf("chunk_text = $%d", len(args)))
	}
	if req.ChunkKind != nil {
		chunkKind := normalizeRequiredText(*req.ChunkKind)
		if chunkKind == "" {
			return Chunk{}, ErrInvalidMemoryChunk
		}
		args = append(args, chunkKind)
		setClauses = append(setClauses, fmt.Sprintf("chunk_kind = $%d", len(args)))
	}
	if req.Tags != nil {
		args = append(args, normalizeTags(req.Tags))
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", len(args)))
	}
	if len(req.Metadata) > 0 {
		if !json.Valid(req.Metadata) {
			return Chunk{}, fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidMemoryChunk)
		}
		args = append(args, req.Metadata)
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", len(args)))
	}
	if len(setClauses) == 0 {
		return r.Get(ctx, chunkID)
	}

	args = append(args, chunkID)
	query := fmt.Sprintf(updateChunkSQL, strings.Join(setClauses, ", "), len(args))
	chunk, err := scanChunk(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		return Chunk{}, normalizeNotFound(err)
	}
	return chunk, nil
}

func (r *Repository) Delete(ctx context.Context, chunkID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM memory_chunks WHERE id = $1`, chunkID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMemoryChunkNotFound
	}
	return nil
}

const selectChunkSQL = `
	SELECT
		id::text,
		project_id::text,
		source_type,
		source_id::text,
		chunk_text,
		chunk_kind,
		tags,
		metadata,
		created_at
	FROM memory_chunks
`

const updateChunkSQL = `
	UPDATE memory_chunks
	SET %s
	WHERE id = $%d
	RETURNING
		id::text,
		project_id::text,
		source_type,
		source_id::text,
		chunk_text,
		chunk_kind,
		tags,
		metadata,
		created_at
`

type chunkScanner interface {
	Scan(dest ...any) error
}

func scanChunk(scanner chunkScanner) (Chunk, error) {
	var chunk Chunk
	var sourceID sql.NullString
	err := scanner.Scan(
		&chunk.ID,
		&chunk.ProjectID,
		&chunk.SourceType,
		&sourceID,
		&chunk.ChunkText,
		&chunk.ChunkKind,
		&chunk.Tags,
		&chunk.Metadata,
		&chunk.CreatedAt,
	)
	if err != nil {
		return Chunk{}, err
	}
	if sourceID.Valid {
		chunk.SourceID = &sourceID.String
	}
	if chunk.Tags == nil {
		chunk.Tags = []string{}
	}
	return chunk, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMemoryChunkNotFound
	}
	return err
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
