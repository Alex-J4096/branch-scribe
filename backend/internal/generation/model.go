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
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Reasoning        string `json:"reasoning,omitempty"`
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
	ProjectID              string   `json:"project_id"`
	BlockID                string   `json:"block_id"`
	TaskType               string   `json:"task_type"`
	ModelProfileID         string   `json:"model_profile_id"`
	PromptTemplateID       *string  `json:"prompt_template_id"`
	SelectedText           string   `json:"selected_text"`
	UserInstruction        string   `json:"user_instruction"`
	ContextNodeCount       *int     `json:"context_node_count"`
	ConversationID         *string  `json:"conversation_id"`
	Temperature            *float64 `json:"temperature"`
	TopP                   *float64 `json:"top_p"`
	MaxTokens              *int     `json:"max_tokens"`
	ExcludedContextItemIDs []string `json:"excluded_context_item_ids"`
	RegenerateMessageID    *string  `json:"regenerate_message_id"`
	SkipConversationSave   bool     `json:"-"`
}

type GenerateOnceResponse struct {
	OutputText       string         `json:"output_text"`
	ReasoningText    string         `json:"reasoning_text"`
	GenerationRun    GenerationRun  `json:"generation_run"`
	Prompt           string         `json:"prompt"`
	SystemPrompt     string         `json:"system_prompt"`
	UserPrompt       string         `json:"user_prompt"`
	ContextPreview   ContextPreview `json:"context_preview"`
	ModelProfileID   string         `json:"model_profile_id"`
	PromptTemplateID *string        `json:"prompt_template_id"`
	ConversationID   *string        `json:"conversation_id"`
}

type GenerateCandidatesRequest struct {
	GenerateOnceRequest
	Count int `json:"count"`
}

type GenerateCandidatesResponse struct {
	Candidates []GenerateOnceResponse `json:"candidates"`
}

type ExtractCharacterCardRequest struct {
	BlockID        string   `json:"block_id"`
	BlockIDs       []string `json:"block_ids"`
	ModelProfileID string   `json:"model_profile_id"`
}

type CharacterCardProposal struct {
	CharacterID     string          `json:"character_id"`
	SourceBlockID   string          `json:"source_block_id"`
	SourceBlockIDs  []string        `json:"source_block_ids"`
	Description     string          `json:"description"`
	Attributes      json.RawMessage `json:"attributes"`
	ChangeSummary   string          `json:"change_summary"`
	Model           string          `json:"model"`
	GenerationRunID string          `json:"generation_run_id"`
}

type BlockAnalysisRequest struct {
	ModelProfileID string `json:"model_profile_id"`
}

type ConsistencyConflict struct {
	CanonEntityID string `json:"canon_entity_id"`
	CanonName     string `json:"canon_name"`
	Severity      string `json:"severity"`
	Claim         string `json:"claim"`
	CanonFact     string `json:"canon_fact"`
	Explanation   string `json:"explanation"`
	Suggestion    string `json:"suggestion"`
}

type ConsistencyCheckResult struct {
	BlockID         string                `json:"block_id"`
	Consistent      bool                  `json:"consistent"`
	Summary         string                `json:"summary"`
	Conflicts       []ConsistencyConflict `json:"conflicts"`
	Model           string                `json:"model"`
	GenerationRunID string                `json:"generation_run_id"`
}

type TimelineEventProposal struct {
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	EventTime     *string `json:"event_time"`
	SortOrder     int     `json:"sort_order"`
	CanonEntityID *string `json:"canon_entity_id"`
}

type TimelineExtractionResult struct {
	BlockID         string                  `json:"block_id"`
	Events          []TimelineEventProposal `json:"events"`
	Model           string                  `json:"model"`
	GenerationRunID string                  `json:"generation_run_id"`
}

type GenerateStreamEvent struct {
	Type             string          `json:"type"`
	Content          string          `json:"content,omitempty"`
	Reasoning        string          `json:"reasoning,omitempty"`
	GenerationRun    *GenerationRun  `json:"generation_run,omitempty"`
	Prompt           string          `json:"prompt,omitempty"`
	SystemPrompt     string          `json:"system_prompt,omitempty"`
	UserPrompt       string          `json:"user_prompt,omitempty"`
	ContextPreview   *ContextPreview `json:"context_preview,omitempty"`
	ModelProfileID   string          `json:"model_profile_id,omitempty"`
	PromptTemplateID *string         `json:"prompt_template_id,omitempty"`
	ConversationID   *string         `json:"conversation_id,omitempty"`
	Error            string          `json:"error,omitempty"`
}

type Conversation struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	BlockID   string    `json:"block_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationMessage struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	Role            string    `json:"role"`
	Content         string    `json:"content"`
	GenerationRunID *string   `json:"generation_run_id"`
	Model           *string   `json:"model"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateConversationRequest struct {
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
}

type UpdateConversationRequest struct {
	Title string `json:"title"`
}

type UpdateConversationMessageRequest struct {
	Content string `json:"content"`
}

type DeleteConversationMessagesRequest struct {
	MessageIDs []string `json:"message_ids"`
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
	BranchID           *string
	OrderIndex         int
	CanonFacts         []CanonFact
}

type CanonFact struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Aliases     []string        `json:"aliases"`
	Description *string         `json:"description"`
	Attributes  json.RawMessage `json:"attributes"`
	Importance  int             `json:"importance"`
	Status      string          `json:"status"`
}

type RecentBlockContext struct {
	ID            string
	Title         *string
	Content       string
	ContentFormat string
	OrderIndex    int
}

type MemoryContext struct {
	ID        string
	ChunkText string
	ChunkKind string
	Tags      []string
}

type SummaryContext struct {
	ID          string
	TargetType  string
	SummaryText string
	TokenCount  int
	Status      string
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
	Reasoning    string
	InputTokens  int
	OutputTokens int
}

type GenerateBlockSummaryRequest struct {
	ProjectID      string `json:"project_id"`
	ModelProfileID string `json:"model_profile_id"`
}

type GenerateBranchSummaryRequest = GenerateBlockSummaryRequest

type SummarySnapshot struct {
	ID                 string          `json:"id"`
	ProjectID          string          `json:"project_id"`
	TargetType         string          `json:"target_type"`
	TargetID           string          `json:"target_id"`
	SummaryText        string          `json:"summary_text"`
	CoveredRevisionIDs []string        `json:"covered_revision_ids"`
	TokenCount         int             `json:"token_count"`
	Model              *string         `json:"model"`
	Status             string          `json:"status"`
	Metadata           json.RawMessage `json:"metadata"`
	CreatedAt          time.Time       `json:"created_at"`
}

type BlockSummarySource struct {
	TargetType         string
	TargetID           string
	Title              string
	CoveredRevisionIDs []string
	Content            string
}

func (req GenerateBlockSummaryRequest) normalized() (GenerateBlockSummaryRequest, error) {
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.ModelProfileID = strings.TrimSpace(req.ModelProfileID)
	if req.ProjectID == "" || req.ModelProfileID == "" {
		return req, ErrInvalidGenerationRequest
	}
	return req, nil
}

type TokenEvent struct {
	Type         string
	Content      string
	Reasoning    string
	Error        string
	InputTokens  int
	OutputTokens int
}

type ContextPreview struct {
	SystemPrompt     string        `json:"system_prompt"`
	UserPrompt       string        `json:"user_prompt"`
	FinalPrompt      string        `json:"final_prompt"`
	EstimatedTokens  int           `json:"estimated_tokens"`
	TokenBudget      int           `json:"token_budget"`
	Items            []ContextItem `json:"items"`
	ExcludedItemIDs  []string      `json:"excluded_item_ids"`
	PromptTemplateID *string       `json:"prompt_template_id"`
}

type ContextItem struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Title           string `json:"title"`
	Content         string `json:"content"`
	SourceID        string `json:"source_id,omitempty"`
	EstimatedTokens int    `json:"estimated_tokens"`
	Included        bool   `json:"included"`
	Required        bool   `json:"required"`
	Status          string `json:"status,omitempty"`
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
	if req.ConversationID != nil {
		trimmed := strings.TrimSpace(*req.ConversationID)
		if trimmed == "" {
			req.ConversationID = nil
		} else {
			req.ConversationID = &trimmed
		}
	}
	if req.ProjectID == "" || req.BlockID == "" || req.TaskType == "" || req.ModelProfileID == "" {
		return req, ErrInvalidGenerationRequest
	}
	if req.ContextNodeCount == nil {
		defaultCount := 1
		req.ContextNodeCount = &defaultCount
	} else if *req.ContextNodeCount < -1 {
		return req, ErrInvalidGenerationRequest
	}
	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return req, ErrInvalidGenerationRequest
	}
	if req.TopP != nil && (*req.TopP < 0 || *req.TopP > 1) {
		return req, ErrInvalidGenerationRequest
	}
	if req.MaxTokens != nil && *req.MaxTokens <= 0 {
		return req, ErrInvalidGenerationRequest
	}
	req.ExcludedContextItemIDs = normalizeStringSet(req.ExcludedContextItemIDs)
	return req, nil
}

func normalizeStringSet(values []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}
