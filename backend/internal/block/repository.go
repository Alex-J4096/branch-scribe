package block

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

func (r *Repository) List(ctx context.Context, projectID string) ([]Block, error) {
	rows, err := r.db.Query(ctx, selectBlockSQL+` WHERE project_id = $1 ORDER BY order_index ASC, created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := make([]Block, 0)
	for rows.Next() {
		block, err := scanBlock(rows)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, rows.Err()
}

func (r *Repository) Create(ctx context.Context, projectID string, req CreateBlockRequest) (BlockDetail, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BlockDetail{}, err
	}
	defer rollback(ctx, tx)

	newBlock, err := scanBlock(tx.QueryRow(ctx, insertBlockSQL,
		projectID,
		nullableString(req.BranchID),
		normalizeBlockType(req.Type),
		nullableString(normalizeOptionalTitle(req.Title)),
		nullableString(req.ParentBlockID),
		req.PositionX,
		req.PositionY,
		req.OrderIndex,
		normalizeJSON(req.Metadata),
	))
	if err != nil {
		return BlockDetail{}, err
	}

	revision, err := insertRevision(ctx, tx, newBlock.ID, nil, req.Content, normalizeContentFormat(req.ContentFormat), "user", nil, json.RawMessage(`{}`))
	if err != nil {
		return BlockDetail{}, err
	}

	newBlock, err = scanBlock(tx.QueryRow(ctx, updateBlockCurrentRevisionSQL, revision.ID, newBlock.ID))
	if err != nil {
		return BlockDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return BlockDetail{}, err
	}

	return BlockDetail{Block: newBlock, CurrentRevision: &revision}, nil
}

func (r *Repository) Get(ctx context.Context, blockID string) (BlockDetail, error) {
	newBlock, err := scanBlock(r.db.QueryRow(ctx, selectBlockSQL+` WHERE id = $1`, blockID))
	if err != nil {
		return BlockDetail{}, normalizeNotFound(err)
	}

	var currentRevision *Revision
	if newBlock.CurrentRevisionID != nil {
		revision, err := r.GetRevision(ctx, *newBlock.CurrentRevisionID)
		if err != nil {
			return BlockDetail{}, err
		}
		currentRevision = &revision
	}

	return BlockDetail{Block: newBlock, CurrentRevision: currentRevision}, nil
}

func (r *Repository) Update(ctx context.Context, blockID string, req UpdateBlockRequest) (Block, error) {
	setClauses := make([]string, 0, 8)
	args := make([]any, 0, 9)

	if req.BranchID != nil {
		args = append(args, *req.BranchID)
		setClauses = append(setClauses, fmt.Sprintf("branch_id = $%d", len(args)))
	}
	if req.Type != nil {
		args = append(args, normalizeBlockType(*req.Type))
		setClauses = append(setClauses, fmt.Sprintf("type = $%d", len(args)))
	}
	if req.Title != nil {
		args = append(args, nullableString(normalizeOptionalTitle(req.Title)))
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", len(args)))
	}
	if req.ParentBlockID != nil {
		args = append(args, *req.ParentBlockID)
		setClauses = append(setClauses, fmt.Sprintf("parent_block_id = $%d", len(args)))
	}
	if req.PositionX != nil {
		args = append(args, *req.PositionX)
		setClauses = append(setClauses, fmt.Sprintf("position_x = $%d", len(args)))
	}
	if req.PositionY != nil {
		args = append(args, *req.PositionY)
		setClauses = append(setClauses, fmt.Sprintf("position_y = $%d", len(args)))
	}
	if req.OrderIndex != nil {
		args = append(args, *req.OrderIndex)
		setClauses = append(setClauses, fmt.Sprintf("order_index = $%d", len(args)))
	}
	if len(req.Metadata) > 0 {
		if !json.Valid(req.Metadata) {
			return Block{}, fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidBlock)
		}
		args = append(args, req.Metadata)
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", len(args)))
	}

	if len(setClauses) == 0 {
		detail, err := r.Get(ctx, blockID)
		return detail.Block, err
	}

	args = append(args, blockID)
	query := fmt.Sprintf(updateBlockSQL, strings.Join(setClauses, ", "), len(args))
	newBlock, err := scanBlock(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		return Block{}, normalizeNotFound(err)
	}
	return newBlock, nil
}

func (r *Repository) UpdateAssociations(ctx context.Context, blockID string, req UpdateBlockAssociationsRequest) (Block, error) {
	characterIDs := normalizeStringList(req.CharacterIDs)
	tags := normalizeStringList(req.Tags)
	locationID := normalizeOptionalTitle(req.LocationID)

	metadataPatch := map[string]any{
		"character_ids": characterIDs,
		"tags":          tags,
	}
	if locationID == nil {
		metadataPatch["location_id"] = nil
	} else {
		metadataPatch["location_id"] = *locationID
	}

	patchJSON, err := json.Marshal(metadataPatch)
	if err != nil {
		return Block{}, err
	}

	newBlock, err := scanBlock(r.db.QueryRow(ctx, `
		UPDATE blocks
		SET metadata = COALESCE(metadata, '{}'::jsonb) || $1::jsonb
		WHERE id = $2
		RETURNING
			id::text,
			project_id::text,
			branch_id::text,
			type,
			title,
			current_revision_id::text,
			parent_block_id::text,
			position_x,
			position_y,
			order_index,
			metadata,
			created_at,
			updated_at
	`, patchJSON, blockID))
	if err != nil {
		return Block{}, normalizeNotFound(err)
	}
	return newBlock, nil
}

func (r *Repository) Delete(ctx context.Context, blockID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM blocks WHERE id = $1`, blockID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBlockNotFound
	}
	return nil
}

func (r *Repository) Fork(ctx context.Context, blockID string, req ForkBlockRequest) (BlockDetail, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BlockDetail{}, err
	}
	defer rollback(ctx, tx)

	source, err := scanBlock(tx.QueryRow(ctx, selectBlockSQL+` WHERE id = $1`, blockID))
	if err != nil {
		return BlockDetail{}, normalizeNotFound(err)
	}

	revisionID := source.CurrentRevisionID
	if req.RevisionID != nil {
		revisionID = req.RevisionID
	}
	if revisionID == nil {
		return BlockDetail{}, fmt.Errorf("%w: source block has no revision", ErrInvalidBlock)
	}

	sourceRevision, err := scanRevision(tx.QueryRow(ctx, selectRevisionSQL+` WHERE id = $1`, *revisionID))
	if err != nil {
		return BlockDetail{}, normalizeNotFound(err)
	}

	title := source.Title
	if req.Title != nil {
		title = normalizeOptionalTitle(req.Title)
	} else if source.Title != nil {
		defaultTitle := *source.Title + " Fork"
		title = &defaultTitle
	}

	branchID := source.BranchID
	if req.BranchID != nil {
		branchID = req.BranchID
	}

	newBlock, err := scanBlock(tx.QueryRow(ctx, insertBlockSQL,
		source.ProjectID,
		nullableString(branchID),
		source.Type,
		nullableString(title),
		source.ID,
		req.PositionX,
		req.PositionY,
		source.OrderIndex+1,
		normalizeJSON(req.Metadata),
	))
	if err != nil {
		return BlockDetail{}, err
	}

	revision, err := insertRevision(ctx, tx, newBlock.ID, &sourceRevision.ID, sourceRevision.Content, sourceRevision.ContentFormat, "user", nil, sourceRevision.Metadata)
	if err != nil {
		return BlockDetail{}, err
	}

	newBlock, err = scanBlock(tx.QueryRow(ctx, updateBlockCurrentRevisionSQL, revision.ID, newBlock.ID))
	if err != nil {
		return BlockDetail{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO graph_edges (project_id, source_block_id, target_block_id, edge_type, label)
		VALUES ($1, $2, $3, 'fork', $4)
		ON CONFLICT (project_id, source_block_id, target_block_id, edge_type) DO NOTHING
	`, source.ProjectID, source.ID, newBlock.ID, nullableString(req.EdgeLabel)); err != nil {
		return BlockDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return BlockDetail{}, err
	}

	return BlockDetail{Block: newBlock, CurrentRevision: &revision}, nil
}

func (r *Repository) ListRevisions(ctx context.Context, blockID string) ([]Revision, error) {
	rows, err := r.db.Query(ctx, selectRevisionSQL+` WHERE block_id = $1 ORDER BY created_at DESC`, blockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	revisions := make([]Revision, 0)
	for rows.Next() {
		revision, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}

	return revisions, rows.Err()
}

func (r *Repository) CreateRevision(ctx context.Context, blockID string, req CreateRevisionRequest) (Revision, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Revision{}, err
	}
	defer rollback(ctx, tx)

	parentRevisionID := req.ParentRevisionID
	if parentRevisionID == nil {
		source, err := scanBlock(tx.QueryRow(ctx, selectBlockSQL+` WHERE id = $1`, blockID))
		if err != nil {
			return Revision{}, normalizeNotFound(err)
		}
		parentRevisionID = source.CurrentRevisionID
	}

	revision, err := insertRevision(ctx, tx, blockID, parentRevisionID, req.Content, normalizeContentFormat(req.ContentFormat), normalizeSource(req.Source), nullableString(req.GenerationRunID), normalizeJSON(req.Metadata))
	if err != nil {
		return Revision{}, err
	}

	if req.GenerationRunID != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE generation_runs
			SET output_revision_id = $1
			WHERE id = $2 AND block_id = $3
		`, revision.ID, *req.GenerationRunID, blockID); err != nil {
			return Revision{}, err
		}
	}

	if req.SetCurrent == nil || *req.SetCurrent {
		if _, err := tx.Exec(ctx, `UPDATE blocks SET current_revision_id = $1 WHERE id = $2`, revision.ID, blockID); err != nil {
			return Revision{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Revision{}, err
	}

	return revision, nil
}

func (r *Repository) GetRevision(ctx context.Context, revisionID string) (Revision, error) {
	revision, err := scanRevision(r.db.QueryRow(ctx, selectRevisionSQL+` WHERE id = $1`, revisionID))
	if err != nil {
		return Revision{}, normalizeNotFound(err)
	}
	return revision, nil
}

func (r *Repository) SelectRevision(ctx context.Context, blockID string, revisionID string) (Block, error) {
	newBlock, err := scanBlock(r.db.QueryRow(ctx, `
		UPDATE blocks
		SET current_revision_id = $1
		WHERE id = $2
			AND EXISTS (
				SELECT 1 FROM block_revisions
				WHERE id = $1 AND block_id = $2
			)
		RETURNING
			id::text,
			project_id::text,
			branch_id::text,
			type,
			title,
			current_revision_id::text,
			parent_block_id::text,
			position_x,
			position_y,
			order_index,
			metadata,
			created_at,
			updated_at
	`, revisionID, blockID))
	if err != nil {
		return Block{}, normalizeNotFound(err)
	}
	return newBlock, nil
}

const selectBlockSQL = `
	SELECT
		id::text,
		project_id::text,
		branch_id::text,
		type,
		title,
		current_revision_id::text,
		parent_block_id::text,
		position_x,
		position_y,
		order_index,
		metadata,
		created_at,
		updated_at
	FROM blocks
`

const insertBlockSQL = `
	INSERT INTO blocks (
		project_id,
		branch_id,
		type,
		title,
		parent_block_id,
		position_x,
		position_y,
		order_index,
		metadata
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING
		id::text,
		project_id::text,
		branch_id::text,
		type,
		title,
		current_revision_id::text,
		parent_block_id::text,
		position_x,
		position_y,
		order_index,
		metadata,
		created_at,
		updated_at
`

const updateBlockSQL = `
	UPDATE blocks
	SET %s
	WHERE id = $%d
	RETURNING
		id::text,
		project_id::text,
		branch_id::text,
		type,
		title,
		current_revision_id::text,
		parent_block_id::text,
		position_x,
		position_y,
		order_index,
		metadata,
		created_at,
		updated_at
`

const updateBlockCurrentRevisionSQL = `
	UPDATE blocks
	SET current_revision_id = $1
	WHERE id = $2
	RETURNING
		id::text,
		project_id::text,
		branch_id::text,
		type,
		title,
		current_revision_id::text,
		parent_block_id::text,
		position_x,
		position_y,
		order_index,
		metadata,
		created_at,
		updated_at
`

const selectRevisionSQL = `
	SELECT
		id::text,
		block_id::text,
		parent_revision_id::text,
		content,
		content_format,
		content_hash,
		source,
		generation_run_id::text,
		metadata,
		created_at
	FROM block_revisions
`

type blockScanner interface {
	Scan(dest ...any) error
}

func scanBlock(scanner blockScanner) (Block, error) {
	var newBlock Block
	var branchID sql.NullString
	var title sql.NullString
	var currentRevisionID sql.NullString
	var parentBlockID sql.NullString

	err := scanner.Scan(
		&newBlock.ID,
		&newBlock.ProjectID,
		&branchID,
		&newBlock.Type,
		&title,
		&currentRevisionID,
		&parentBlockID,
		&newBlock.PositionX,
		&newBlock.PositionY,
		&newBlock.OrderIndex,
		&newBlock.Metadata,
		&newBlock.CreatedAt,
		&newBlock.UpdatedAt,
	)
	if err != nil {
		return Block{}, err
	}

	if branchID.Valid {
		newBlock.BranchID = &branchID.String
	}
	if title.Valid {
		newBlock.Title = &title.String
	}
	if currentRevisionID.Valid {
		newBlock.CurrentRevisionID = &currentRevisionID.String
	}
	if parentBlockID.Valid {
		newBlock.ParentBlockID = &parentBlockID.String
	}

	return newBlock, nil
}

func scanRevision(scanner blockScanner) (Revision, error) {
	var revision Revision
	var parentRevisionID sql.NullString
	var contentHash sql.NullString
	var generationRunID sql.NullString

	err := scanner.Scan(
		&revision.ID,
		&revision.BlockID,
		&parentRevisionID,
		&revision.Content,
		&revision.ContentFormat,
		&contentHash,
		&revision.Source,
		&generationRunID,
		&revision.Metadata,
		&revision.CreatedAt,
	)
	if err != nil {
		return Revision{}, err
	}

	if parentRevisionID.Valid {
		revision.ParentRevisionID = &parentRevisionID.String
	}
	if contentHash.Valid {
		revision.ContentHash = &contentHash.String
	}
	if generationRunID.Valid {
		revision.GenerationRunID = &generationRunID.String
	}

	return revision, nil
}

func insertRevision(ctx context.Context, tx pgx.Tx, blockID string, parentRevisionID *string, content string, contentFormat string, source string, generationRunID any, metadata json.RawMessage) (Revision, error) {
	hash := contentHash(content)
	return scanRevision(tx.QueryRow(ctx, `
		INSERT INTO block_revisions (
			block_id,
			parent_revision_id,
			content,
			content_format,
			content_hash,
			source,
			generation_run_id,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING
			id::text,
			block_id::text,
			parent_revision_id::text,
			content,
			content_format,
			content_hash,
			source,
			generation_run_id::text,
			metadata,
			created_at
	`, blockID, nullableString(parentRevisionID), content, contentFormat, hash, source, generationRunID, normalizeJSON(metadata)))
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBlockNotFound
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
