package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type DebugEvent struct {
	Type         string        `json:"type"`
	RequestID    string        `json:"request_id"`
	Timestamp    time.Time     `json:"timestamp"`
	Provider     string        `json:"provider,omitempty"`
	BaseURL      string        `json:"base_url,omitempty"`
	Model        string        `json:"model,omitempty"`
	Messages     []ChatMessage `json:"messages,omitempty"`
	Temperature  float64       `json:"temperature,omitempty"`
	TopP         float64       `json:"top_p,omitempty"`
	MaxTokens    int           `json:"max_tokens,omitempty"`
	Stream       bool          `json:"stream,omitempty"`
	Content      string        `json:"content,omitempty"`
	Reasoning    string        `json:"reasoning,omitempty"`
	InputTokens  int           `json:"input_tokens,omitempty"`
	OutputTokens int           `json:"output_tokens,omitempty"`
	FinishReason string        `json:"finish_reason,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type DebugSink interface {
	Emit(DebugEvent)
}

type HTTPDebugSink struct {
	url    string
	client *http.Client
	events chan DebugEvent
}

func NewHTTPDebugSink(url string) *HTTPDebugSink {
	sink := &HTTPDebugSink{
		url:    strings.TrimRight(strings.TrimSpace(url), "/") + "/events",
		client: &http.Client{Timeout: time.Second},
		events: make(chan DebugEvent, 256),
	}
	go sink.run()
	return sink
}

func (s *HTTPDebugSink) Emit(event DebugEvent) {
	select {
	case s.events <- event:
	default:
		// Debug output is best-effort and must never hold up an LLM request.
	}
}

func (s *HTTPDebugSink) run() {
	for event := range s.events {
		body, err := json.Marshal(event)
		if err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.url, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}
}

type DebugProvider struct {
	next Provider
	sink DebugSink
	seq  atomic.Uint64
}

func NewDebugProvider(next Provider, sink DebugSink) *DebugProvider {
	return &DebugProvider{next: next, sink: sink}
}

func (p *DebugProvider) GenerateOnce(ctx context.Context, req GenerateRequest) (CompletionResult, error) {
	requestID := p.requestID()
	p.emitRequest(requestID, req, false)
	result, err := p.next.GenerateOnce(ctx, req)
	if err != nil {
		p.sink.Emit(DebugEvent{Type: "error", RequestID: requestID, Timestamp: time.Now(), Error: err.Error()})
		return result, err
	}
	p.sink.Emit(DebugEvent{
		Type: "response", RequestID: requestID, Timestamp: time.Now(),
		Content: result.Content, Reasoning: result.Reasoning,
		InputTokens: result.InputTokens, OutputTokens: result.OutputTokens,
		FinishReason: result.FinishReason,
	})
	return result, nil
}

func (p *DebugProvider) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan TokenEvent, error) {
	requestID := p.requestID()
	p.emitRequest(requestID, req, true)
	source, err := p.next.GenerateStream(ctx, req)
	if err != nil {
		p.sink.Emit(DebugEvent{Type: "error", RequestID: requestID, Timestamp: time.Now(), Error: err.Error()})
		return nil, err
	}

	output := make(chan TokenEvent)
	go func() {
		defer close(output)
		for event := range source {
			debugEvent := DebugEvent{
				Type: event.Type, RequestID: requestID, Timestamp: time.Now(),
				Content: event.Content, Reasoning: event.Reasoning,
				InputTokens: event.InputTokens, OutputTokens: event.OutputTokens, Error: event.Error,
				FinishReason: event.FinishReason,
			}
			p.sink.Emit(debugEvent)
			select {
			case output <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return output, nil
}

func (p *DebugProvider) emitRequest(requestID string, req GenerateRequest, stream bool) {
	p.sink.Emit(DebugEvent{
		Type: "request", RequestID: requestID, Timestamp: time.Now(),
		Provider: req.Provider, BaseURL: req.BaseURL, Model: req.Model,
		Messages: req.Messages, Temperature: req.Temperature, TopP: req.TopP,
		MaxTokens: req.MaxTokens, Stream: stream,
	})
}

func (p *DebugProvider) requestID() string {
	return time.Now().Format("20060102T150405.000000000") + "-" + stringID(p.seq.Add(1))
}

func stringID(value uint64) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if value == 0 {
		return "0"
	}
	var result [13]byte
	index := len(result)
	for value > 0 {
		index--
		result[index] = digits[value%uint64(len(digits))]
		value /= uint64(len(digits))
	}
	return string(result[index:])
}
