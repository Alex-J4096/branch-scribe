package generation

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type debugTestSink struct {
	mu     sync.Mutex
	events []DebugEvent
}

func (s *debugTestSink) Emit(event DebugEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

type debugTestProvider struct {
	onceResult CompletionResult
	onceErr    error
	stream     []TokenEvent
}

func (p *debugTestProvider) GenerateOnce(context.Context, GenerateRequest) (CompletionResult, error) {
	return p.onceResult, p.onceErr
}

func (p *debugTestProvider) GenerateStream(context.Context, GenerateRequest) (<-chan TokenEvent, error) {
	events := make(chan TokenEvent, len(p.stream))
	for _, event := range p.stream {
		events <- event
	}
	close(events)
	return events, nil
}

func TestDebugProviderReportsFinalMessagesAndOnceResponse(t *testing.T) {
	sink := &debugTestSink{}
	provider := NewDebugProvider(&debugTestProvider{
		onceResult: CompletionResult{Content: "answer", Reasoning: "thought", InputTokens: 12, OutputTokens: 3},
	}, sink)
	request := GenerateRequest{
		Provider: "openai-compatible", BaseURL: "http://provider.test/v1", Model: "test-model",
		APIKey: "must-not-be-reported", Messages: []ChatMessage{{Role: "user", Content: "assembled context"}},
		Temperature: 0.7, TopP: 0.9, MaxTokens: 100,
	}

	if _, err := provider.GenerateOnce(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("got %d debug events, want 2", len(sink.events))
	}
	if sink.events[0].Type != "request" || sink.events[0].Messages[0].Content != "assembled context" {
		t.Fatalf("unexpected request event: %#v", sink.events[0])
	}
	if sink.events[1].Type != "response" || sink.events[1].Content != "answer" || sink.events[1].Reasoning != "thought" {
		t.Fatalf("unexpected response event: %#v", sink.events[1])
	}
}

func TestDebugProviderPassesThroughAndReportsStream(t *testing.T) {
	sink := &debugTestSink{}
	expected := []TokenEvent{
		{Type: "reasoning", Reasoning: "think"},
		{Type: "delta", Content: "hello"},
		{Type: "done", InputTokens: 4, OutputTokens: 2},
	}
	provider := NewDebugProvider(&debugTestProvider{stream: expected}, sink)

	stream, err := provider.GenerateStream(context.Background(), GenerateRequest{
		Model: "test-model", Messages: []ChatMessage{{Role: "user", Content: "prompt"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var actual []TokenEvent
	for event := range stream {
		actual = append(actual, event)
	}
	if len(actual) != len(expected) {
		t.Fatalf("got %d stream events, want %d", len(actual), len(expected))
	}
	if len(sink.events) != 1+len(expected) {
		t.Fatalf("got %d debug events, want %d", len(sink.events), 1+len(expected))
	}
	for index, event := range expected {
		if sink.events[index+1].Type != event.Type {
			t.Fatalf("event %d type = %q, want %q", index, sink.events[index+1].Type, event.Type)
		}
	}
}

func TestDebugProviderReportsProviderError(t *testing.T) {
	sink := &debugTestSink{}
	provider := NewDebugProvider(&debugTestProvider{onceErr: errors.New("provider unavailable")}, sink)

	_, err := provider.GenerateOnce(context.Background(), GenerateRequest{Model: "test", Messages: []ChatMessage{{Role: "user"}}})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if len(sink.events) != 2 || sink.events[1].Type != "error" {
		t.Fatalf("unexpected debug events: %#v", sink.events)
	}
}
