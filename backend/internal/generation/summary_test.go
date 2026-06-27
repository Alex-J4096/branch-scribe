package generation

import (
	"errors"
	"testing"
)

func TestGenerateBlockSummaryRequestNormalized(t *testing.T) {
	req, err := (GenerateBlockSummaryRequest{
		ProjectID:      " project-id ",
		ModelProfileID: " profile-id ",
	}).normalized()
	if err != nil {
		t.Fatalf("normalized() error = %v", err)
	}
	if req.ProjectID != "project-id" || req.ModelProfileID != "profile-id" {
		t.Fatalf("normalized() = %#v", req)
	}
}

func TestGenerateBlockSummaryRequestRequiresIDs(t *testing.T) {
	_, err := (GenerateBlockSummaryRequest{}).normalized()
	if !errors.Is(err, ErrInvalidGenerationRequest) {
		t.Fatalf("normalized() error = %v, want ErrInvalidGenerationRequest", err)
	}
}

func TestApplyContextBudgetAllowsStaleSummary(t *testing.T) {
	items := applyContextBudget([]ContextItem{
		{
			ID:      "summary:stale",
			Type:    "chapter_summary",
			Content: "outdated",
			Status:  "stale",
		},
		{
			ID:       "recent",
			Type:     "recent_block",
			Content:  "current",
			Required: true,
		},
	}, nil, 100)

	if !items[0].Included {
		t.Fatal("stale summary should remain available for context")
	}
	if !items[1].Included {
		t.Fatal("required current context must be included")
	}
}
