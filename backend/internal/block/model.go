package block

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidBlock  = errors.New("invalid block")
	ErrBlockNotFound = errors.New("block not found")
)

type Block struct {
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

type BlockDetail struct {
	Block           Block     `json:"block"`
	CurrentRevision *Revision `json:"current_revision"`
}

type Revision struct {
	ID               string          `json:"id"`
	BlockID          string          `json:"block_id"`
	ParentRevisionID *string         `json:"parent_revision_id"`
	Content          string          `json:"content"`
	ContentFormat    string          `json:"content_format"`
	ContentHash      *string         `json:"content_hash"`
	Source           string          `json:"source"`
	GenerationRunID  *string         `json:"generation_run_id"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
}

type CreateBlockRequest struct {
	BranchID      *string         `json:"branch_id"`
	Type          string          `json:"type"`
	Title         *string         `json:"title"`
	Content       string          `json:"content"`
	ContentFormat string          `json:"content_format"`
	ParentBlockID *string         `json:"parent_block_id"`
	PositionX     float64         `json:"position_x"`
	PositionY     float64         `json:"position_y"`
	OrderIndex    int             `json:"order_index"`
	Metadata      json.RawMessage `json:"metadata"`
}

type UpdateBlockRequest struct {
	BranchID      *string         `json:"branch_id"`
	Type          *string         `json:"type"`
	Title         *string         `json:"title"`
	ParentBlockID *string         `json:"parent_block_id"`
	PositionX     *float64        `json:"position_x"`
	PositionY     *float64        `json:"position_y"`
	OrderIndex    *int            `json:"order_index"`
	Metadata      json.RawMessage `json:"metadata"`
}

type ForkBlockRequest struct {
	BranchID   *string         `json:"branch_id"`
	Title      *string         `json:"title"`
	PositionX  float64         `json:"position_x"`
	PositionY  float64         `json:"position_y"`
	Metadata   json.RawMessage `json:"metadata"`
	EdgeLabel  *string         `json:"edge_label"`
	RevisionID *string         `json:"revision_id"`
}

func normalizeOptionalTitle(title *string) *string {
	if title == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*title)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeBlockType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "scene"
	}
	return value
}

func normalizeContentFormat(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "markdown"
	}
	return value
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
