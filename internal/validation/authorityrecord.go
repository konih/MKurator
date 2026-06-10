package validation

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/konih/mkurator/api/v1alpha1"
)

// ValidateAuthorityRecordSpec runs stateful admission validation for AuthorityRecord spec fields.
func ValidateAuthorityRecordSpec(
	ctx context.Context,
	reader client.Reader,
	namespace, _ string,
	spec *messagingv1alpha1.AuthorityRecordSpec,
) field.ErrorList {
	return ValidateConnectionRef(ctx, reader, namespace, spec.ConnectionRef.Name,
		field.NewPath("spec").Child("connectionRef").Child("name"))
}
