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

func TestGenerateBlockSummaryRequestNormalizesSourceSelections(t *testing.T) {
	req, err := (GenerateBlockSummaryRequest{
		ProjectID:      "project-id",
		ModelProfileID: "profile-id",
		SourceSelections: []SummarySourceSelection{
			{BlockID: " block-1 ", Mode: "summary"},
			{BlockID: "block-2", Mode: "exclude"},
		},
	}).normalized()
	if err != nil {
		t.Fatalf("normalized() error = %v", err)
	}
	if req.SourceSelections[0].BlockID != "block-1" || req.SourceSelections[1].Mode != "exclude" {
		t.Fatalf("normalized source selections = %#v", req.SourceSelections)
	}
}

func TestGenerateBlockSummaryRequestRejectsInvalidSourceSelections(t *testing.T) {
	tests := []GenerateBlockSummaryRequest{
		{
			ProjectID:      "project-id",
			ModelProfileID: "profile-id",
			SourceSelections: []SummarySourceSelection{
				{BlockID: "block-1", Mode: "compressed"},
			},
		},
		{
			ProjectID:      "project-id",
			ModelProfileID: "profile-id",
			SourceSelections: []SummarySourceSelection{
				{BlockID: "block-1", Mode: "summary"},
				{BlockID: "block-1", Mode: "full_text"},
			},
		},
	}
	for _, req := range tests {
		if _, err := req.normalized(); !errors.Is(err, ErrInvalidGenerationRequest) {
			t.Fatalf("normalized() error = %v, want ErrInvalidGenerationRequest", err)
		}
	}
}

func TestManualSummaryRequestNormalized(t *testing.T) {
	req, err := (ManualSummaryRequest{
		ProjectID:   " project-id ",
		SummaryText: " 手写摘要 ",
	}).normalized()
	if err != nil {
		t.Fatalf("normalized() error = %v", err)
	}
	if req.ProjectID != "project-id" || req.SummaryText != "手写摘要" {
		t.Fatalf("normalized() = %#v", req)
	}

	if _, err := (ManualSummaryRequest{ProjectID: "project-id"}).normalized(); !errors.Is(err, ErrInvalidGenerationRequest) {
		t.Fatalf("empty summary error = %v, want ErrInvalidGenerationRequest", err)
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
