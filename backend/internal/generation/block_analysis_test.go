package generation

import "testing"

func TestParseConsistencyCheck(t *testing.T) {
	facts := []CanonFact{{ID: "canon-1", Name: "月门"}}
	result, err := parseConsistencyCheck(`{"consistent":true,"summary":"发现冲突","conflicts":[{"canon_entity_id":"canon-1","canon_name":"月门","severity":"error","claim":"白日开启","canon_fact":"只在夜晚开启","explanation":"开启时间冲突","suggestion":"改为夜晚"}]}`, facts)
	if err != nil {
		t.Fatalf("parseConsistencyCheck returned error: %v", err)
	}
	if result.Consistent || len(result.Conflicts) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseConsistencyCheckRejectsUnknownCanon(t *testing.T) {
	_, err := parseConsistencyCheck(`{"consistent":false,"conflicts":[{"canon_entity_id":"invented","severity":"warning","explanation":"冲突"}]}`, nil)
	if err == nil {
		t.Fatal("expected unknown canon id to be rejected")
	}
}

func TestParseTimelineExtraction(t *testing.T) {
	canonID := "canon-1"
	result, err := parseTimelineExtraction(`{"events":[{"title":"抵达月门","description":"众人抵达月门","event_time":"午夜","sort_order":9,"canon_entity_id":"canon-1"}]}`, []CanonFact{{ID: canonID}})
	if err != nil {
		t.Fatalf("parseTimelineExtraction returned error: %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].SortOrder != 0 || result.Events[0].CanonEntityID == nil {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseTimelineExtractionDropsUnknownCanon(t *testing.T) {
	result, err := parseTimelineExtraction(`{"events":[{"title":"启程","description":"主角离开村庄","event_time":null,"canon_entity_id":"invented"}]}`, nil)
	if err != nil {
		t.Fatalf("parseTimelineExtraction returned error: %v", err)
	}
	if result.Events[0].CanonEntityID != nil {
		t.Fatalf("expected unknown canon link to be removed: %#v", result.Events[0])
	}
}
