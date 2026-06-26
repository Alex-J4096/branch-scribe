package graph

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidGraph  = errors.New("invalid graph request")
	ErrGraphNotFound = errors.New("graph item not found")
)

type BlockNode struct {
	ID                string          `json:"id"`
	ProjectID         string          `json:"project_id"`
	BranchID          *string         `json:"branch_id"`
	Type              string          `json:"type"`
	Title             *string         `json:"title"`
	CurrentRevisionID *string         `json:"current_revision_id"`
	ParentBlockID     *string         `json:"parent_block_id"`
	PositionX         float64         `json:"position_x"`
	PositionY         float64         `json:"position_y"`
	OrderIndex        int             `json:"order_index"`
	Metadata          json.RawMessage `json:"metadata"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type Edge struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"project_id"`
	SourceBlockID string          `json:"source_block_id"`
	TargetBlockID string          `json:"target_block_id"`
	EdgeType      string          `json:"edge_type"`
	Label         *string         `json:"label"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
}

type ProjectGraph struct {
	Nodes []BlockNode `json:"nodes"`
	Edges []Edge      `json:"edges"`
}

type CreateEdgeRequest struct {
	SourceBlockID string          `json:"source_block_id"`
	TargetBlockID string          `json:"target_block_id"`
	EdgeType      string          `json:"edge_type"`
	Label         *string         `json:"label"`
	Metadata      json.RawMessage `json:"metadata"`
}

type UpdatePositionRequest struct {
	PositionX float64 `json:"position_x"`
	PositionY float64 `json:"position_y"`
}

func normalizeEdgeType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "next"
	}
	return value
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
