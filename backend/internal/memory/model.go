package memory

import (
	"encoding/json"
	"errors"
	"html"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidMemoryChunk  = errors.New("invalid memory chunk")
	ErrMemoryChunkNotFound = errors.New("memory chunk not found")
)

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

type Chunk struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"project_id"`
	SourceType string          `json:"source_type"`
	SourceID   *string         `json:"source_id"`
	ChunkText  string          `json:"chunk_text"`
	ChunkKind  string          `json:"chunk_kind"`
	Tags       []string        `json:"tags"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
	Similarity *float64        `json:"similarity,omitempty"`
}

type CreateChunkRequest struct {
	SourceType string          `json:"source_type"`
	SourceID   *string         `json:"source_id"`
	ChunkText  string          `json:"chunk_text"`
	ChunkKind  string          `json:"chunk_kind"`
	Tags       []string        `json:"tags"`
	Metadata   json.RawMessage `json:"metadata"`
}

type UpdateChunkRequest struct {
	SourceType *string         `json:"source_type"`
	SourceID   *string         `json:"source_id"`
	ChunkText  *string         `json:"chunk_text"`
	ChunkKind  *string         `json:"chunk_kind"`
	Tags       []string        `json:"tags"`
	Metadata   json.RawMessage `json:"metadata"`
}

type CreateFromBlockRequest struct {
	ChunkKind string          `json:"chunk_kind"`
	Tags      []string        `json:"tags"`
	Metadata  json.RawMessage `json:"metadata"`
}

type SearchRequest struct {
	Query          string `json:"q"`
	SourceType     string `json:"source_type"`
	ChunkKind      string `json:"chunk_kind"`
	Tag            string `json:"tag"`
	Mode           string `json:"mode"`
	ModelProfileID string `json:"model_profile_id"`
	Limit          int    `json:"limit"`
}

type ListFilter struct {
	SourceType string
	ChunkKind  string
	Tag        string
	Query      string
}

type ReindexRequest struct {
	ModelProfileID string `json:"model_profile_id"`
}

type ReindexResult struct {
	MemoryIndexed int    `json:"memory_indexed"`
	CanonIndexed  int    `json:"canon_indexed"`
	Model         string `json:"model"`
	Dimensions    int    `json:"dimensions"`
}

type EmbeddingProfile struct {
	ID         string
	Provider   string
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int
}

type EmbeddingDocument struct {
	ID   string
	Text string
}

func (req CreateChunkRequest) normalized() (CreateChunkRequest, error) {
	req.SourceType = normalizeRequiredText(req.SourceType)
	req.ChunkText = strings.TrimSpace(req.ChunkText)
	req.ChunkKind = normalizeRequiredText(req.ChunkKind)
	req.Tags = normalizeTags(req.Tags)
	req.Metadata = normalizeJSON(req.Metadata)
	req.SourceID = normalizeOptionalString(req.SourceID)
	if req.SourceType == "" || req.ChunkText == "" || req.ChunkKind == "" {
		return req, ErrInvalidMemoryChunk
	}
	return req, nil
}

func normalizeRequiredText(value string) string {
	return strings.TrimSpace(value)
}

func normalizeTags(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	tags := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		tags = append(tags, value)
	}
	return tags
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

func normalizeContent(content string, format string) string {
	if format == "html" {
		content = htmlTagPattern.ReplaceAllString(content, " ")
		content = html.UnescapeString(content)
	}
	return strings.Join(strings.Fields(content), " ")
}
