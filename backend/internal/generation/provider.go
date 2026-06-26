package generation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Provider interface {
	GenerateOnce(ctx context.Context, req GenerateRequest) (CompletionResult, error)
	GenerateStream(ctx context.Context, req GenerateRequest) (<-chan TokenEvent, error)
}

type OpenAICompatibleProvider struct {
	client *http.Client
}

func NewOpenAICompatibleProvider() *OpenAICompatibleProvider {
	return &OpenAICompatibleProvider{
		client: &http.Client{},
	}
}

func (p *OpenAICompatibleProvider) GenerateOnce(ctx context.Context, req GenerateRequest) (CompletionResult, error) {
	if req.Model == "" || req.APIKey == "" || len(req.Messages) == 0 {
		return CompletionResult{}, ErrInvalidGenerationRequest
	}

	body, err := json.Marshal(openAIChatCompletionRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	})
	if err != nil {
		return CompletionResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(req.BaseURL), bytes.NewReader(body))
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return CompletionResult{}, providerError(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CompletionResult{}, providerError(readProviderError(resp))
	}

	var decoded openAIChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return CompletionResult{}, providerError("invalid provider response")
	}
	if len(decoded.Choices) == 0 {
		return CompletionResult{}, providerError("provider returned no choices")
	}

	return CompletionResult{
		Content:      decoded.Choices[0].Message.Content,
		InputTokens:  decoded.Usage.PromptTokens,
		OutputTokens: decoded.Usage.CompletionTokens,
	}, nil
}

func (p *OpenAICompatibleProvider) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan TokenEvent, error) {
	if req.Model == "" || req.APIKey == "" || len(req.Messages) == 0 {
		return nil, ErrInvalidGenerationRequest
	}

	body, err := json.Marshal(openAIChatCompletionRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
		StreamOptions: map[string]bool{
			"include_usage": true,
		},
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(req.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, providerError(err.Error())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, providerError(readProviderError(resp))
	}

	events := make(chan TokenEvent)
	go func() {
		defer close(events)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var inputTokens int
		var outputTokens int
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				events <- TokenEvent{Type: "done", InputTokens: inputTokens, OutputTokens: outputTokens}
				return
			}

			var decoded openAIChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
				events <- TokenEvent{Type: "error", Error: "invalid provider stream response"}
				return
			}
			if decoded.Usage != nil {
				inputTokens = decoded.Usage.PromptTokens
				outputTokens = decoded.Usage.CompletionTokens
			}
			for _, choice := range decoded.Choices {
				if choice.Delta.Content != "" {
					events <- TokenEvent{Type: "delta", Content: choice.Delta.Content}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			events <- TokenEvent{Type: "error", Error: err.Error()}
			return
		}
		events <- TokenEvent{Type: "done", InputTokens: inputTokens, OutputTokens: outputTokens}
	}()

	return events, nil
}

type openAIChatCompletionRequest struct {
	Model         string          `json:"model"`
	Messages      []ChatMessage   `json:"messages"`
	Temperature   float64         `json:"temperature"`
	TopP          float64         `json:"top_p"`
	MaxTokens     int             `json:"max_tokens"`
	Stream        bool            `json:"stream"`
	StreamOptions map[string]bool `json:"stream_options,omitempty"`
}

type openAIChatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type openAIChatCompletionStreamResponse struct {
	Choices []struct {
		Delta ChatMessage `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

func chatCompletionsURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return baseURL + "/chat/completions"
}

func readProviderError(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Sprintf("provider returned HTTP %d", resp.StatusCode)
	}
	var decoded openAIErrorResponse
	if err := json.Unmarshal(body, &decoded); err == nil && decoded.Error.Message != "" {
		return decoded.Error.Message
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Sprintf("provider returned HTTP %d", resp.StatusCode)
	}
	return message
}
