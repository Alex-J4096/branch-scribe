package generation

import "testing"

func TestParseCharacterCardProposal(t *testing.T) {
	proposal, err := parseCharacterCardProposal("```json\n" +
		`{"description":"经历冲突后更加谨慎","attributes":{"mood":"警惕"},"change_summary":"由轻信变为警惕"}` +
		"\n```")
	if err != nil {
		t.Fatalf("parse proposal: %v", err)
	}
	if proposal.Description != "经历冲突后更加谨慎" || proposal.ChangeSummary != "由轻信变为警惕" {
		t.Fatalf("unexpected proposal: %#v", proposal)
	}
}

func TestParseCharacterCardProposalRejectsInvalidContent(t *testing.T) {
	if _, err := parseCharacterCardProposal(`{"description":"","attributes":{}}`); err == nil {
		t.Fatal("expected invalid proposal error")
	}
}
