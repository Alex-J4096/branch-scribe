package project

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidProject  = errors.New("project name is required")
	ErrProjectNotFound = errors.New("project not found")
)

type Project struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Description           *string         `json:"description"`
	DefaultLanguage       string          `json:"default_language"`
	DefaultStyleProfile   json.RawMessage `json:"default_style_profile"`
	DefaultModelProfileID *string         `json:"default_model_profile_id"`
	Metadata              json.RawMessage `json:"metadata"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

type CreateProjectRequest struct {
	Name                string          `json:"name"`
	Description         *string         `json:"description"`
	DefaultLanguage     string          `json:"default_language"`
	DefaultStyleProfile json.RawMessage `json:"default_style_profile"`
	Metadata            json.RawMessage `json:"metadata"`
}

type UpdateProjectRequest struct {
	Name                *string         `json:"name"`
	Description         *string         `json:"description"`
	DefaultLanguage     *string         `json:"default_language"`
	DefaultStyleProfile json.RawMessage `json:"default_style_profile"`
	Metadata            json.RawMessage `json:"metadata"`
}

func (req CreateProjectRequest) normalized() (CreateProjectRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return req, ErrInvalidProject
	}

	req.DefaultLanguage = strings.TrimSpace(req.DefaultLanguage)
	if req.DefaultLanguage == "" {
		req.DefaultLanguage = "zh"
	}

	req.DefaultStyleProfile = normalizeJSON(req.DefaultStyleProfile)
	req.Metadata = normalizeJSON(req.Metadata)
	return req, nil
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
