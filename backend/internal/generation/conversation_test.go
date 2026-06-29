package generation

import (
	"errors"
	"reflect"
	"testing"
)

func TestConversationHistoryBeforeRegenerationExcludesTargetUserAndLaterRounds(t *testing.T) {
	history := []ConversationMessage{
		{ID: "user-a", Role: "user", Content: "a"},
		{ID: "assistant-a", Role: "assistant", Content: "reply-a"},
		{ID: "user-b", Role: "user", Content: "b"},
		{ID: "assistant-b", Role: "assistant", Content: "reply-b"},
		{ID: "user-c", Role: "user", Content: "c"},
		{ID: "assistant-c", Role: "assistant", Content: "reply-c"},
	}

	got, err := conversationHistoryBeforeRegeneration(history, "assistant-a")
	if err != nil {
		t.Fatalf("history before regeneration: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("regenerating a must not include a or later messages, got %#v", got)
	}

	got, err = conversationHistoryBeforeRegeneration(history, "assistant-b")
	if err != nil {
		t.Fatalf("history before second regeneration: %v", err)
	}
	want := history[:2]
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("regenerating b history = %#v, want %#v", got, want)
	}
}

func TestConversationHistoryBeforeRegenerationValidatesTarget(t *testing.T) {
	history := []ConversationMessage{
		{ID: "user-a", Role: "user", Content: "a"},
		{ID: "assistant-a", Role: "assistant", Content: "reply-a"},
	}

	if _, err := conversationHistoryBeforeRegeneration(history, "missing"); !errors.Is(err, ErrGenerationResourceNotFound) {
		t.Fatalf("missing target error = %v", err)
	}
	if _, err := conversationHistoryBeforeRegeneration(history, "user-a"); !errors.Is(err, ErrInvalidGenerationRequest) {
		t.Fatalf("user target error = %v", err)
	}
	if _, err := conversationHistoryBeforeRegeneration(
		[]ConversationMessage{{ID: "assistant-a", Role: "assistant", Content: "reply-a"}},
		"assistant-a",
	); !errors.Is(err, ErrInvalidGenerationRequest) {
		t.Fatalf("assistant without source user error = %v", err)
	}
}

func TestNormalizeMessageIDs(t *testing.T) {
	got, err := normalizeMessageIDs([]string{" message-a ", "message-b", "message-a"})
	if err != nil {
		t.Fatalf("normalize message ids: %v", err)
	}
	if want := []string{"message-a", "message-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized ids = %#v, want %#v", got, want)
	}
	if _, err := normalizeMessageIDs(nil); !errors.Is(err, ErrInvalidGenerationRequest) {
		t.Fatalf("empty ids error = %v", err)
	}
	if _, err := normalizeMessageIDs([]string{" "}); !errors.Is(err, ErrInvalidGenerationRequest) {
		t.Fatalf("blank id error = %v", err)
	}
}
