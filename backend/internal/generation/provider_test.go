package generation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleProviderGenerateOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "生成结果", "reasoning_content": "推理内容"}}],
			"usage": {"prompt_tokens": 12, "completion_tokens": 34}
		}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider()
	result, err := provider.GenerateOnce(context.Background(), GenerateRequest{
		Model:       "test-model",
		BaseURL:     server.URL + "/v1",
		APIKey:      "test-key",
		Messages:    []ChatMessage{{Role: "user", Content: "继续写"}},
		Temperature: 0.8,
		TopP:        0.9,
		MaxTokens:   1024,
	})
	if err != nil {
		t.Fatalf("GenerateOnce returned error: %v", err)
	}
	if result.Content != "生成结果" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if result.Reasoning != "推理内容" {
		t.Fatalf("unexpected reasoning: %q", result.Reasoning)
	}
	if result.InputTokens != 12 || result.OutputTokens != 34 {
		t.Fatalf("unexpected usage: %+v", result)
	}
}

func TestOpenAICompatibleProviderGenerateStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("missing stream accept header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"先分析\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"第一段\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"第二段\"}}],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":22}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider()
	events, err := provider.GenerateStream(context.Background(), GenerateRequest{
		Model:       "test-model",
		BaseURL:     server.URL + "/v1",
		APIKey:      "test-key",
		Messages:    []ChatMessage{{Role: "user", Content: "继续写"}},
		Temperature: 0.8,
		TopP:        0.9,
		MaxTokens:   1024,
	})
	if err != nil {
		t.Fatalf("GenerateStream returned error: %v", err)
	}

	var output string
	var reasoning string
	var done TokenEvent
	for event := range events {
		if event.Type == "delta" {
			output += event.Content
		}
		if event.Type == "reasoning" {
			reasoning += event.Reasoning
		}
		if event.Type == "done" {
			done = event
		}
	}
	if output != "第一段第二段" {
		t.Fatalf("unexpected stream output: %q", output)
	}
	if reasoning != "先分析" {
		t.Fatalf("unexpected stream reasoning: %q", reasoning)
	}
	if done.InputTokens != 11 || done.OutputTokens != 22 {
		t.Fatalf("unexpected usage: %+v", done)
	}
}

func TestOpenAICompatibleProviderReturnsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad api key"}}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider()
	_, err := provider.GenerateOnce(context.Background(), GenerateRequest{
		Model:     "test-model",
		BaseURL:   server.URL,
		APIKey:    "test-key",
		Messages:  []ChatMessage{{Role: "user", Content: "继续写"}},
		MaxTokens: 1024,
	})
	if !errors.Is(err, ErrProviderRequestFailed) {
		t.Fatalf("expected provider error, got %v", err)
	}
	if !strings.Contains(err.Error(), "bad api key") {
		t.Fatalf("expected provider message, got %v", err)
	}
}
