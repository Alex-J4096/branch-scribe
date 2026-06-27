package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrEmbeddingNotConfigured = errors.New("embedding is not configured")
	ErrEmbeddingRequestFailed = errors.New("embedding request failed")
)

type EmbeddingProvider interface {
	Embed(ctx context.Context, profile EmbeddingProfile, inputs []string) ([][]float64, error)
}

type OpenAICompatibleEmbeddingProvider struct {
	client *http.Client
}

func NewOpenAICompatibleEmbeddingProvider() *OpenAICompatibleEmbeddingProvider {
	return &OpenAICompatibleEmbeddingProvider{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAICompatibleEmbeddingProvider) Embed(ctx context.Context, profile EmbeddingProfile, inputs []string) ([][]float64, error) {
	if profile.Model == "" || profile.APIKey == "" || len(inputs) == 0 {
		return nil, ErrEmbeddingNotConfigured
	}
	payload := map[string]any{
		"model": profile.Model,
		"input": inputs,
	}
	if profile.Dimensions > 0 {
		payload["dimensions"] = profile.Dimensions
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(profile.BaseURL, "/") + "/embeddings"
	if profile.BaseURL == "" {
		endpoint = "https://api.openai.com/v1/embeddings"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+profile.APIKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEmbeddingRequestFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var providerError struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&providerError)
		message := strings.TrimSpace(providerError.Error.Message)
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("%w: %s", ErrEmbeddingRequestFailed, message)
	}
	var decoded struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("%w: invalid provider response", ErrEmbeddingRequestFailed)
	}
	if len(decoded.Data) != len(inputs) {
		return nil, fmt.Errorf("%w: provider returned %d vectors for %d inputs", ErrEmbeddingRequestFailed, len(decoded.Data), len(inputs))
	}
	vectors := make([][]float64, len(inputs))
	for _, item := range decoded.Data {
		if item.Index < 0 || item.Index >= len(vectors) || len(item.Embedding) == 0 {
			return nil, fmt.Errorf("%w: invalid embedding index", ErrEmbeddingRequestFailed)
		}
		vectors[item.Index] = item.Embedding
	}
	for _, vector := range vectors {
		if len(vector) == 0 {
			return nil, fmt.Errorf("%w: missing embedding vector", ErrEmbeddingRequestFailed)
		}
		if profile.Dimensions > 0 && len(vector) != profile.Dimensions {
			return nil, fmt.Errorf("%w: expected %d dimensions, got %d", ErrEmbeddingRequestFailed, profile.Dimensions, len(vector))
		}
	}
	return vectors, nil
}
