package branch

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidBranch  = errors.New("branch name is required")
	ErrBranchNotFound = errors.New("branch not found")
)

type Branch struct {
	ID                 string          `json:"id"`
	ProjectID          string          `json:"project_id"`
	Name               string          `json:"name"`
	Description        *string         `json:"description"`
	BaseBranchID       *string         `json:"base_branch_id"`
	ForkFromBlockID    *string         `json:"fork_from_block_id"`
	ForkFromRevisionID *string         `json:"fork_from_revision_id"`
	Status             string          `json:"status"`
	Metadata           json.RawMessage `json:"metadata"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type CreateBranchRequest struct {
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Metadata    json.RawMessage `json:"metadata"`
}

type ForkBranchRequest struct {
	Name               string          `json:"name"`
	Description        *string         `json:"description"`
	BaseBranchID       *string         `json:"base_branch_id"`
	ForkFromBlockID    *string         `json:"fork_from_block_id"`
	ForkFromRevisionID *string         `json:"fork_from_revision_id"`
	Metadata           json.RawMessage `json:"metadata"`
}

type UpdateBranchRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Status      *string         `json:"status"`
	Metadata    json.RawMessage `json:"metadata"`
}

type BranchPath struct {
	Branch Branch      `json:"branch"`
	Blocks []PathBlock `json:"blocks"`
}

type PathBlock struct {
	ID                string          `json:"id"`
	BranchID          *string         `json:"branch_id"`
	Title             *string         `json:"title"`
	Type              string          `json:"type"`
	CurrentRevisionID *string         `json:"current_revision_id"`
	OrderIndex        int             `json:"order_index"`
	Metadata          json.RawMessage `json:"metadata"`
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidBranch
	}
	return name, nil
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
