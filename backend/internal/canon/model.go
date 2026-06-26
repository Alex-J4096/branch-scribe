package canon

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidCanonEntity  = errors.New("invalid canon entity")
	ErrCanonEntityNotFound = errors.New("canon entity not found")
)

type Entity struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Aliases     []string        `json:"aliases"`
	Description *string         `json:"description"`
	Attributes  json.RawMessage `json:"attributes"`
	Importance  int             `json:"importance"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type CreateEntityRequest struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Aliases     []string        `json:"aliases"`
	Description *string         `json:"description"`
	Attributes  json.RawMessage `json:"attributes"`
	Importance  *int            `json:"importance"`
	Status      string          `json:"status"`
}

type UpdateEntityRequest struct {
	Type        *string         `json:"type"`
	Name        *string         `json:"name"`
	Aliases     []string        `json:"aliases"`
	Description *string         `json:"description"`
	Attributes  json.RawMessage `json:"attributes"`
	Importance  *int            `json:"importance"`
	Status      *string         `json:"status"`
}

func (req CreateEntityRequest) normalized() (CreateEntityRequest, error) {
	req.Type = normalizeType(req.Type)
	req.Name = strings.TrimSpace(req.Name)
	req.Status = normalizeStatus(req.Status)
	req.Aliases = normalizeAliases(req.Aliases)
	req.Description = normalizeOptionalString(req.Description)
	req.Attributes = normalizeJSON(req.Attributes)
	if req.Name == "" || !isValidType(req.Type) || !isValidStatus(req.Status) {
		return req, ErrInvalidCanonEntity
	}
	if req.Importance == nil {
		value := 5
		req.Importance = &value
	}
	if *req.Importance < 1 || *req.Importance > 10 {
		return req, ErrInvalidCanonEntity
	}
	return req, nil
}

func normalizeType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "character"
	}
	return value
}

func normalizeStatus(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "canon"
	}
	return value
}

func isValidType(value string) bool {
	switch value {
	case "character", "location", "faction", "item", "rule", "event":
		return true
	default:
		return false
	}
}

func isValidStatus(value string) bool {
	switch value {
	case "canon", "draft", "deprecated":
		return true
	default:
		return false
	}
}

func normalizeAliases(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	aliases := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		aliases = append(aliases, value)
	}
	return aliases
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
