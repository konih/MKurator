package webhookv1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/konih/kurator/internal/validation"
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
