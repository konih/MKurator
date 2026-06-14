package validation

import (
	"testing"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func TestUnknownAttributeWarnings(t *testing.T) {
	t.Parallel()
	warnings := unknownQueueAttributeWarnings("", map[string]string{"bogus": "1"})
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}

	warnings = unknownQueueAttributeWarnings(
		"alias",
		map[string]string{"targq": "Q", "unknown": "x"},
	)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for alias, got %v", warnings)
	}

	warnings = unknownTopicAttributeWarnings(map[string]string{"topstr": "A.B"})
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	warnings = unknownChannelAttributeWarnings(map[string]string{"sslciph": "NULL"})
	if len(warnings) != 0 {
		t.Fatalf("expected sslciph allowed, got %v", warnings)
	}

	_ = mqadmin.NormalizeAttrKey("TopicStr")
}
