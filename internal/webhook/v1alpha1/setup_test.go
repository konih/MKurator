package webhookv1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/conduit-ops/mkurator/internal/validation"
)

func TestAdmissionResult_WithErrors(t *testing.T) {
	t.Parallel()
	errs := field.ErrorList{
		field.Required(field.NewPath("spec").Child("name"), "name is required"),
	}
	_, err := admissionResult(nil, errs, validation.QueueInvalid, "orders")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAdmissionResult_NoErrors(t *testing.T) {
	t.Parallel()
	warnings, err := admissionResult([]string{"deprecated field"}, nil, validation.QueueInvalid, "orders")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 || warnings[0] != "deprecated field" {
		t.Fatalf("warnings = %v", warnings)
	}
}
