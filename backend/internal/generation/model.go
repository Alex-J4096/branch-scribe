package generation

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidGenerationRequest   = errors.New("invalid generation request")
	ErrGenerationResourceNotFound = errors.New("generation resource not found")
	ErrUnsupportedProvider        = errors.New("unsupported provider")
	ErrProviderRequestFailed      = errors.New("provider request failed")
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Provider    string        `json:"provider"`
	Model       string        `json:"model"`
	BaseURL     string        `json:"base_url"`
	APIKey      string        `json:"-"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	TopP        float64       `json:"top_p"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
}

type GenerateOnceRequest struct {
	ProjectID        string  `json:"project_id"`
	BlockID          string  `json:"block_id"`
	TaskType         string  `json:"task_type"`
	ModelProfileID   string  `json:"model_profile_id"`
	PromptTemplateID *string `json:"prompt_template_id"`
	SelectedText     string  `json:"selected_text"`
	UserInstruction  string  `json:"user_instruction"`
}

type GenerateOnceResponse struct {
	OutputText       string        `json:"output_text"`
	GenerationRun    GenerationRun `json:"generation_run"`
	Prompt           string        `json:"prompt"`
	ModelProfileID   string        `json:"model_profile_id"`
	PromptTemplateID *string       `json:"prompt_template_id"`
}

type GenerateStreamEvent struct {
	Type             string         `json:"type"`
	Content          string         `json:"content,omitempty"`
	GenerationRun    *GenerationRun `json:"generation_run,omitempty"`
	Prompt           string         `json:"prompt,omitempty"`
	ModelProfileID   string         `json:"model_profile_id,omitempty"`
	PromptTemplateID *string        `json:"prompt_template_id,omitempty"`
	Error            string         `json:"error,omitempty"`
}

type GenerationRun struct {
	ID                   string          `json:"id"`
	ProjectID            string          `json:"project_id"`
	BlockID              *string         `json:"block_id"`
	TaskType             string          `json:"task_type"`
	Provider             string          `json:"provider"`
	Model                string          `json:"model"`
	Temperature          *float64        `json:"temperature"`
	TopP                 *float64        `json:"top_p"`
	MaxTokens            *int            `json:"max_tokens"`
	ContextWindow        *int            `json:"context_window"`
	PromptTemplateID     *string         `json:"prompt_template_id"`
	InputContextSnapshot json.RawMessage `json:"input_context_snapshot"`
	OutputRevisionID     *string         `json:"output_revision_id"`
	InputTokens          int             `json:"input_tokens"`
	OutputTokens         int             `json:"output_tokens"`
	LatencyMS            int             `json:"latency_ms"`
	Status               string          `json:"status"`
	ErrorMessage         *string         `json:"error_message"`
	CreatedAt            time.Time       `json:"created_at"`
}

type ModelProfile struct {
	ID            string
	Provider      string
	Model         string
	BaseURL       *string
	APIKey        *string
	Temperature   float64
	TopP          float64
	MaxTokens     int
	ContextWindow int
}

type PromptTemplate struct {
	ID           string
	TaskType     string
	TemplateText string
}

type BlockContext struct {
	ProjectDescription *string
	BlockTitle         *string
	Content            string
	ContentFormat      string
}

type GenerationRunInput struct {
	ProjectID            string
	BlockID              *string
	TaskType             string
	Provider             string
	Model                string
	Temperature          float64
	TopP                 float64
	MaxTokens            int
	ContextWindow        int
	PromptTemplateID     *string
	InputContextSnapshot json.RawMessage
}

type CompletionResult struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

type TokenEvent struct {
	Type         string
	Content      string
	Error        string
	InputTokens  int
	OutputTokens int
}

func (req GenerateOnceRequest) normalized() (GenerateOnceRequest, error) {
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.BlockID = strings.TrimSpace(req.BlockID)
	req.TaskType = strings.TrimSpace(req.TaskType)
	req.ModelProfileID = strings.TrimSpace(req.ModelProfileID)
	req.SelectedText = strings.TrimSpace(req.SelectedText)
	req.UserInstruction = strings.TrimSpace(req.UserInstruction)
	if req.PromptTemplateID != nil {
		trimmed := strings.TrimSpace(*req.PromptTemplateID)
		if trimmed == "" {
			req.PromptTemplateID = nil
		} else {
			req.PromptTemplateID = &trimmed
		}
	}
	if req.ProjectID == "" || req.BlockID == "" || req.TaskType == "" || req.ModelProfileID == "" {
		return req, ErrInvalidGenerationRequest
	}
	return req, nil
}
