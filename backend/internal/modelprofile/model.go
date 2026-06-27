package modelprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidModelProfile  = errors.New("invalid model profile")
	ErrModelProfileNotFound = errors.New("model profile not found")
)

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type ModelProfile struct {
	ID            string          `json:"id"`
	ProjectID     *string         `json:"project_id"`
	Name          string          `json:"name"`
	Provider      string          `json:"provider"`
	Model         string          `json:"model"`
	BaseURL       *string         `json:"base_url"`
	HasAPIKey     bool            `json:"has_api_key"`
	Temperature   float64         `json:"temperature"`
	TopP          float64         `json:"top_p"`
	MaxTokens     int             `json:"max_tokens"`
	ContextWindow int             `json:"context_window"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type CreateModelProfileRequest struct {
	Name          string          `json:"name"`
	Provider      string          `json:"provider"`
	Model         string          `json:"model"`
	BaseURL       *string         `json:"base_url"`
	APIKey        *string         `json:"api_key"`
	Temperature   *float64        `json:"temperature"`
	TopP          *float64        `json:"top_p"`
	MaxTokens     *int            `json:"max_tokens"`
	ContextWindow *int            `json:"context_window"`
	Metadata      json.RawMessage `json:"metadata"`
}

type UpdateModelProfileRequest struct {
	Name          *string         `json:"name"`
	Provider      *string         `json:"provider"`
	Model         *string         `json:"model"`
	BaseURL       *string         `json:"base_url"`
	APIKey        *string         `json:"api_key"`
	ClearAPIKey   *bool           `json:"clear_api_key"`
	Temperature   *float64        `json:"temperature"`
	TopP          *float64        `json:"top_p"`
	MaxTokens     *int            `json:"max_tokens"`
	ContextWindow *int            `json:"context_window"`
	Metadata      json.RawMessage `json:"metadata"`
}

func (req CreateModelProfileRequest) normalized() (CreateModelProfileRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = normalizeProvider(req.Provider)
	req.Model = strings.TrimSpace(req.Model)
	if req.Name == "" || req.Model == "" {
		return req, ErrInvalidModelProfile
	}
	req.BaseURL = normalizeOptionalString(req.BaseURL)
	req.APIKey = normalizeOptionalString(req.APIKey)
	req.Metadata = normalizeJSON(req.Metadata)
	return req, nil
}

func normalizeProvider(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "openai_compatible"
	}
	return value
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

func normalizeAPIKeyStorage(value *string) (*string, error) {
	value = normalizeOptionalString(value)
	if value == nil {
		return nil, nil
	}
	if !strings.HasPrefix(*value, "env:") {
		return value, nil
	}
	envName := strings.TrimSpace(strings.TrimPrefix(*value, "env:"))
	if !envVarNamePattern.MatchString(envName) {
		return nil, fmt.Errorf("%w: api key must be an environment variable name", ErrInvalidModelProfile)
	}
	ref := "env:" + envName
	return &ref, nil
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
