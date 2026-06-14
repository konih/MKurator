package mqrest

import (
	"errors"
	"testing"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func TestValidateQueueType(t *testing.T) {
	t.Parallel()
	for _, qType := range []mqadmin.QueueType{
		mqadmin.QueueTypeLocal,
		mqadmin.QueueTypeAlias,
		mqadmin.QueueTypeRemote,
		"",
	} {
		if err := validateQueueType(qType); err != nil {
			t.Fatalf("validateQueueType(%q) = %v", qType, err)
		}
	}
	err := validateQueueType("bogus")
	if err == nil || !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}
