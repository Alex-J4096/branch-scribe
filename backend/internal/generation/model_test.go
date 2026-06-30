package generation

import "testing"

func TestGenerateOnceRequestContextNodeCount(t *testing.T) {
	base := GenerateOnceRequest{
		ProjectID:      "project",
		BlockID:        "block",
		TaskType:       "continue",
		ModelProfileID: "profile",
	}

	normalized, err := base.normalized()
	if err != nil {
		t.Fatalf("normalize default request: %v", err)
	}
	if normalized.ContextNodeCount == nil || *normalized.ContextNodeCount != 1 {
		t.Fatalf("expected default context node count 1, got %v", normalized.ContextNodeCount)
	}

	all := -1
	base.ContextNodeCount = &all
	normalized, err = base.normalized()
	if err != nil {
		t.Fatalf("normalize all-nodes request: %v", err)
	}
	if *normalized.ContextNodeCount != -1 {
		t.Fatalf("expected all-nodes sentinel -1, got %d", *normalized.ContextNodeCount)
	}

	invalid := -2
	base.ContextNodeCount = &invalid
	if _, err := base.normalized(); err == nil {
		t.Fatal("expected context node count below -1 to be rejected")
	}
}

func TestGenerateOnceRequestRejectsConflictingConversationRetryTargets(t *testing.T) {
	conversationID := "conversation"
	assistantID := "assistant"
	userID := "user"
	request := GenerateOnceRequest{
		ProjectID:           "project",
		BlockID:             "block",
		TaskType:            "continue",
		ModelProfileID:      "profile",
		ConversationID:      &conversationID,
		RegenerateMessageID: &assistantID,
		RetryUserMessageID:  &userID,
	}
	if _, err := request.normalized(); err == nil {
		t.Fatal("expected conflicting retry targets to be rejected")
	}
}
