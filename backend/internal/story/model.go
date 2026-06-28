package story

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidRecord = errors.New("invalid story record")
	ErrNotFound      = errors.New("story record not found")
)

type CharacterState struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	CharacterID string          `json:"character_id"`
	BlockID     *string         `json:"block_id"`
	StateKey    string          `json:"state_key"`
	StateValue  json.RawMessage `json:"state_value"`
	Notes       *string         `json:"notes"`
	OccurredAt  *string         `json:"occurred_at"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type CharacterStateInput struct {
	CharacterID string          `json:"character_id"`
	BlockID     *string         `json:"block_id"`
	StateKey    string          `json:"state_key"`
	StateValue  json.RawMessage `json:"state_value"`
	Notes       *string         `json:"notes"`
	OccurredAt  *string         `json:"occurred_at"`
	Metadata    json.RawMessage `json:"metadata"`
}

type Foreshadowing struct {
	ID              string          `json:"id"`
	ProjectID       string          `json:"project_id"`
	Title           string          `json:"title"`
	Description     *string         `json:"description"`
	Status          string          `json:"status"`
	PlantedBlockID  *string         `json:"planted_block_id"`
	ResolvedBlockID *string         `json:"resolved_block_id"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type ForeshadowingInput struct {
	Title           string          `json:"title"`
	Description     *string         `json:"description"`
	Status          string          `json:"status"`
	PlantedBlockID  *string         `json:"planted_block_id"`
	ResolvedBlockID *string         `json:"resolved_block_id"`
	Metadata        json.RawMessage `json:"metadata"`
}

type TimelineEvent struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"project_id"`
	Title         string          `json:"title"`
	Description   *string         `json:"description"`
	EventTime     *string         `json:"event_time"`
	SortOrder     int             `json:"sort_order"`
	BlockID       *string         `json:"block_id"`
	CanonEntityID *string         `json:"canon_entity_id"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type TimelineEventInput struct {
	Title         string          `json:"title"`
	Description   *string         `json:"description"`
	EventTime     *string         `json:"event_time"`
	SortOrder     int             `json:"sort_order"`
	BlockID       *string         `json:"block_id"`
	CanonEntityID *string         `json:"canon_entity_id"`
	Metadata      json.RawMessage `json:"metadata"`
}

func normalizeText(value string) string { return strings.TrimSpace(value) }

func normalizeJSON(value json.RawMessage) (json.RawMessage, error) {
	if len(value) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(value) {
		return nil, ErrInvalidRecord
	}
	return value, nil
}

func validForeshadowingStatus(value string) bool {
	switch value {
	case "planted", "developed", "resolved", "abandoned":
		return true
	default:
		return false
	}
}
