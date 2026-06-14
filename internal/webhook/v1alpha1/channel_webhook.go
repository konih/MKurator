//nolint:dupl // workload webhook validators share the same controller-runtime shape
package webhookv1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/validation"
)

//nolint:lll // kubebuilder webhook marker is a single line
// +kubebuilder:webhook:path=/validate-messaging-mkurator-dev-v1alpha1-channel,mutating=false,failurePolicy=fail,sideEffects=None,groups=messaging.mkurator.dev,resources=channels,verbs=create;update,versions=v1alpha1,name=vchannel.kb.io,admissionReviewVersions=v1

type channelCustomValidator struct {
	Client client.Reader
}

var _ admission.Validator[*messagingv1alpha1.Channel] = &channelCustomValidator{}

func setupChannelWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &messagingv1alpha1.Channel{}).
		WithValidator(&channelCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

func (v *channelCustomValidator) ValidateCreate(
	ctx context.Context,
	channel *messagingv1alpha1.Channel,
) (admission.Warnings, error) {
	return validateCreateUpdate(ctx, v.Client, channel, v.validateChannel, validation.ChannelInvalid)
}

func (v *channelCustomValidator) ValidateUpdate(
	ctx context.Context,
	_ *messagingv1alpha1.Channel,
	newChannel *messagingv1alpha1.Channel,
) (admission.Warnings, error) {
	return validateCreateUpdate(ctx, v.Client, newChannel, v.validateChannel, validation.ChannelInvalid)
}

func (v *channelCustomValidator) ValidateDelete(
	_ context.Context,
	_ *messagingv1alpha1.Channel,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *channelCustomValidator) validateChannel(
	ctx context.Context,
	reader client.Reader,
	channel *messagingv1alpha1.Channel,
) ([]string, field.ErrorList) {
	return validation.ValidateChannelSpec(ctx, reader, channel.Namespace, channel.Name, &channel.Spec)
}
