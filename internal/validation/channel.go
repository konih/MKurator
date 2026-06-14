package validation

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

// ValidateChannelSpec runs stateful admission validation for Channel spec fields.
func ValidateChannelSpec(
	ctx context.Context,
	reader client.Reader,
	namespace, _ string,
	spec *messagingv1alpha1.ChannelSpec,
) ([]string, field.ErrorList) {
	errs := ValidateConnectionRef(ctx, reader, namespace, spec.ConnectionRef.Name,
		field.NewPath("spec").Child("connectionRef").Child("name"))
	warnings := unknownChannelAttributeWarnings(spec.Attributes)
	return warnings, errs
}
