package prompttemplate

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidPromptTemplate  = errors.New("invalid prompt template")
	ErrPromptTemplateNotFound = errors.New("prompt template not found")
)

type PromptTemplate struct {
	ID           string          `json:"id"`
	ProjectID    *string         `json:"project_id"`
	Name         string          `json:"name"`
	TaskType     string          `json:"task_type"`
	TemplateText string          `json:"template_text"`
	Version      int             `json:"version"`
	IsDefault    bool            `json:"is_default"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type CreatePromptTemplateRequest struct {
	Name         string          `json:"name"`
	TaskType     string          `json:"task_type"`
	TemplateText string          `json:"template_text"`
	Version      *int            `json:"version"`
	IsDefault    *bool           `json:"is_default"`
	Metadata     json.RawMessage `json:"metadata"`
}

type UpdatePromptTemplateRequest struct {
	Name         *string         `json:"name"`
	TaskType     *string         `json:"task_type"`
	TemplateText *string         `json:"template_text"`
	Version      *int            `json:"version"`
	IsDefault    *bool           `json:"is_default"`
	Metadata     json.RawMessage `json:"metadata"`
}

func (req CreatePromptTemplateRequest) normalized() (CreatePromptTemplateRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.TaskType = strings.TrimSpace(req.TaskType)
	req.TemplateText = strings.TrimSpace(req.TemplateText)
	if req.Name == "" || req.TaskType == "" || req.TemplateText == "" {
		return req, ErrInvalidPromptTemplate
	}
	if req.Version != nil && *req.Version <= 0 {
		return req, ErrInvalidPromptTemplate
	}
	req.Metadata = normalizeJSON(req.Metadata)
	return req, nil
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
