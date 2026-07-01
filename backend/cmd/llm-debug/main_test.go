package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"branchscribe/backend/internal/generation"
)

func postEvent(t *testing.T, hub *debugHub, event generation.DebugEvent) {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	response := httptest.NewRecorder()
	hub.handleEvent(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("event response status = %d", response.Code)
	}
}

func TestDebugHubBuildsStreamedSession(t *testing.T) {
	hub := newDebugHub()
	now := time.Now()
	postEvent(t, hub, generation.DebugEvent{
		Type: "request", RequestID: "request-1", Timestamp: now, Provider: "test",
		Model: "model", Messages: []generation.ChatMessage{{Role: "user", Content: "final prompt"}},
		Temperature: 0.7, TopP: 0.9, MaxTokens: 512, Stream: true,
	})
	postEvent(t, hub, generation.DebugEvent{Type: "reasoning", RequestID: "request-1", Reasoning: "first "})
	postEvent(t, hub, generation.DebugEvent{Type: "reasoning", RequestID: "request-1", Reasoning: "second"})
	postEvent(t, hub, generation.DebugEvent{Type: "delta", RequestID: "request-1", Content: "hello "})
	postEvent(t, hub, generation.DebugEvent{Type: "delta", RequestID: "request-1", Content: "world"})
	postEvent(t, hub, generation.DebugEvent{Type: "done", RequestID: "request-1", InputTokens: 10, OutputTokens: 4})

	response := httptest.NewRecorder()
	hub.handleSessions(response, httptest.NewRequest(http.MethodGet, "/api/sessions", nil))
	var sessions []debugSession
	if err := json.Unmarshal(response.Body.Bytes(), &sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	session := sessions[0]
	if session.Messages[0].Content != "final prompt" || session.Reasoning != "first second" || session.Content != "hello world" {
		t.Fatalf("unexpected aggregated session: %#v", session)
	}
	if session.Status != "done" || session.InputTokens != 10 || session.OutputTokens != 4 {
		t.Fatalf("unexpected completion state: %#v", session)
	}
}

func TestDebugHubClearRemovesHistory(t *testing.T) {
	hub := newDebugHub()
	postEvent(t, hub, generation.DebugEvent{Type: "request", RequestID: "request-1", Model: "model"})

	response := httptest.NewRecorder()
	hub.handleClear(response, httptest.NewRequest(http.MethodDelete, "/api/sessions", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("clear response status = %d", response.Code)
	}
	if len(hub.sessions) != 0 || len(hub.order) != 0 {
		t.Fatal("clear did not remove session history")
	}
}

func TestDebugWebUIIncludesTaggedPromptFolding(t *testing.T) {
	content, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(content)
	for _, expected := range []string{"promptTagPattern", "renderMessageContent", `class="prompt-block"`} {
		if !strings.Contains(page, expected) {
			t.Fatalf("debug UI missing %q", expected)
		}
	}
}
