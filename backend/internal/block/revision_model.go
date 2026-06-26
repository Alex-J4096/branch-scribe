package block

import (
	"encoding/json"
	"errors"
	"strings"
)

var ErrInvalidRevision = errors.New("revision content is required")

type CreateRevisionRequest struct {
	ParentRevisionID *string         `json:"parent_revision_id"`
	Content          string          `json:"content"`
	ContentFormat    string          `json:"content_format"`
	Source           string          `json:"source"`
	GenerationRunID  *string         `json:"generation_run_id"`
	Metadata         json.RawMessage `json:"metadata"`
	SetCurrent       *bool           `json:"set_current"`
}

func normalizeSource(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "user"
	}
	return value
}
