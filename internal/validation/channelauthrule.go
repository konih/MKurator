package validation

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/konih/mkurator/api/v1alpha1"
)

// ValidateChannelAuthRuleSpec runs stateful admission validation for ChannelAuthRule spec fields.
func ValidateChannelAuthRuleSpec(
	ctx context.Context,
	reader client.Reader,
	namespace, _ string,
	spec *messagingv1alpha1.ChannelAuthRuleSpec,
) field.ErrorList {
	return append(
		ValidateConnectionRef(ctx, reader, namespace, spec.ConnectionRef.Name,
			field.NewPath("spec").Child("connectionRef").Child("name")),
		ValidateManagedChannelRef(ctx, reader, namespace, spec.ConnectionRef.Name, spec.ChannelName,
			field.NewPath("spec").Child("channelName"))...,
	)
}
