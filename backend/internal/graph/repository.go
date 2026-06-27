package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Get(ctx context.Context, projectID string) (ProjectGraph, error) {
	nodes, err := r.listNodes(ctx, projectID)
	if err != nil {
		return ProjectGraph{}, err
	}

	edges, err := r.listEdges(ctx, projectID)
	if err != nil {
		return ProjectGraph{}, err
	}

	return ProjectGraph{Nodes: nodes, Edges: edges}, nil
}

func (r *Repository) CreateEdge(ctx context.Context, projectID string, req CreateEdgeRequest) (Edge, error) {
	if req.SourceBlockID == "" || req.TargetBlockID == "" || req.SourceBlockID == req.TargetBlockID {
		return Edge{}, ErrInvalidGraph
	}
	if len(req.Metadata) > 0 && !json.Valid(req.Metadata) {
		return Edge{}, ErrInvalidGraph
	}

	edge, err := scanEdge(r.db.QueryRow(ctx, `
		INSERT INTO graph_edges (
			project_id,
			source_block_id,
			target_block_id,
			edge_type,
			label,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, source_block_id, target_block_id, edge_type)
		DO UPDATE SET label = EXCLUDED.label, metadata = EXCLUDED.metadata
		RETURNING
			id::text,
			project_id::text,
			source_block_id::text,
			target_block_id::text,
			edge_type,
			label,
			metadata,
			created_at
	`, projectID, req.SourceBlockID, req.TargetBlockID, normalizeEdgeType(req.EdgeType), nullableString(req.Label), normalizeJSON(req.Metadata)))
	if err != nil {
		return Edge{}, err
	}

	return edge, nil
}

func (r *Repository) UpdatePosition(ctx context.Context, projectID string, blockID string, req UpdatePositionRequest) (BlockNode, error) {
	node, err := scanNode(r.db.QueryRow(ctx, `
		UPDATE blocks
		SET position_x = $1, position_y = $2
		WHERE id = $3 AND project_id = $4
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
	`, req.PositionX, req.PositionY, blockID, projectID))
	if err != nil {
		return BlockNode{}, normalizeNotFound(err)
	}
	return node, nil
}

func (r *Repository) UpdateEdge(ctx context.Context, projectID string, edgeID string, req UpdateEdgeRequest) (Edge, error) {
	if len(req.Metadata) > 0 && !json.Valid(req.Metadata) {
		return Edge{}, ErrInvalidGraph
	}
	edge, err := scanEdge(r.db.QueryRow(ctx, `
		UPDATE graph_edges
		SET
			edge_type = $1,
			label = $2,
			metadata = $3
		WHERE id = $4 AND project_id = $5
		RETURNING
			id::text,
			project_id::text,
			source_block_id::text,
			target_block_id::text,
			edge_type,
			label,
			metadata,
			created_at
	`, normalizeEdgeType(req.EdgeType), nullableString(req.Label), normalizeJSON(req.Metadata), edgeID, projectID))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Edge{}, ErrInvalidGraph
		}
		return Edge{}, normalizeNotFound(err)
	}
	return edge, nil
}

func (r *Repository) DeleteEdge(ctx context.Context, projectID string, edgeID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM graph_edges WHERE id = $1 AND project_id = $2`, edgeID, projectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrGraphNotFound
	}
	return nil
}

func (r *Repository) listNodes(ctx context.Context, projectID string) ([]BlockNode, error) {
	rows, err := r.db.Query(ctx, `
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
		WHERE project_id = $1
		ORDER BY order_index ASC, created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]BlockNode, 0)
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

func (r *Repository) listEdges(ctx context.Context, projectID string) ([]Edge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id::text,
			project_id::text,
			source_block_id::text,
			target_block_id::text,
			edge_type,
			label,
			metadata,
			created_at
		FROM graph_edges
		WHERE project_id = $1
		ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	edges := make([]Edge, 0)
	for rows.Next() {
		edge, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanNode(scanner scanner) (BlockNode, error) {
	var node BlockNode
	var branchID sql.NullString
	var title sql.NullString
	var currentRevisionID sql.NullString
	var parentBlockID sql.NullString

	err := scanner.Scan(
		&node.ID,
		&node.ProjectID,
		&branchID,
		&node.Type,
		&title,
		&currentRevisionID,
		&parentBlockID,
		&node.PositionX,
		&node.PositionY,
		&node.OrderIndex,
		&node.Metadata,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		return BlockNode{}, err
	}

	if branchID.Valid {
		node.BranchID = &branchID.String
	}
	if title.Valid {
		node.Title = &title.String
	}
	if currentRevisionID.Valid {
		node.CurrentRevisionID = &currentRevisionID.String
	}
	if parentBlockID.Valid {
		node.ParentBlockID = &parentBlockID.String
	}
	return node, nil
}

func scanEdge(scanner scanner) (Edge, error) {
	var edge Edge
	var label sql.NullString

	err := scanner.Scan(
		&edge.ID,
		&edge.ProjectID,
		&edge.SourceBlockID,
		&edge.TargetBlockID,
		&edge.EdgeType,
		&label,
		&edge.Metadata,
		&edge.CreatedAt,
	)
	if err != nil {
		return Edge{}, err
	}

	if label.Valid {
		edge.Label = &label.String
	}
	return edge, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrGraphNotFound
	}
	return err
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
