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
// +kubebuilder:webhook:path=/validate-messaging-mkurator-dev-v1alpha1-channelauthrule,mutating=false,failurePolicy=fail,sideEffects=None,groups=messaging.mkurator.dev,resources=channelauthrules,verbs=create;update,versions=v1alpha1,name=vchannelauthrule.kb.io,admissionReviewVersions=v1

type channelAuthRuleCustomValidator struct {
	Client client.Reader
}

var _ admission.Validator[*messagingv1alpha1.ChannelAuthRule] = &channelAuthRuleCustomValidator{}

func setupChannelAuthRuleWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &messagingv1alpha1.ChannelAuthRule{}).
		WithValidator(&channelAuthRuleCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

func (v *channelAuthRuleCustomValidator) ValidateCreate(
	ctx context.Context,
	rule *messagingv1alpha1.ChannelAuthRule,
) (admission.Warnings, error) {
	return validateCreateUpdate(ctx, v.Client, rule, v.validateRule, validation.ChannelAuthRuleInvalid)
}

func (v *channelAuthRuleCustomValidator) ValidateUpdate(
	ctx context.Context,
	_ *messagingv1alpha1.ChannelAuthRule,
	newRule *messagingv1alpha1.ChannelAuthRule,
) (admission.Warnings, error) {
	// Finalizer removal during delete only changes metadata; skip spec checks so
	// deletion is not blocked when the managed Channel CR is already gone.
	if newRule.DeletionTimestamp != nil {
		return nil, nil
	}
	return validateCreateUpdate(ctx, v.Client, newRule, v.validateRule, validation.ChannelAuthRuleInvalid)
}

func (v *channelAuthRuleCustomValidator) ValidateDelete(
	_ context.Context,
	_ *messagingv1alpha1.ChannelAuthRule,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *channelAuthRuleCustomValidator) validateRule(
	ctx context.Context,
	reader client.Reader,
	rule *messagingv1alpha1.ChannelAuthRule,
) ([]string, field.ErrorList) {
	return nil, validation.ValidateChannelAuthRuleSpec(ctx, reader, rule.Namespace, rule.Name, &rule.Spec)
}
