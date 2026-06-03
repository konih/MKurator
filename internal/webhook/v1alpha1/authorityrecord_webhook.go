//nolint:dupl // workload webhook validators share the same controller-runtime shape
package webhookv1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/validation"
)

//nolint:lll // kubebuilder webhook marker is a single line
// +kubebuilder:webhook:path=/validate-messaging-kurator-dev-v1alpha1-authorityrecord,mutating=false,failurePolicy=fail,sideEffects=None,groups=messaging.kurator.dev,resources=authorityrecords,verbs=create;update,versions=v1alpha1,name=vauthorityrecord.kb.io,admissionReviewVersions=v1

type authorityRecordCustomValidator struct {
	Client client.Reader
}

var _ admission.Validator[*messagingv1alpha1.AuthorityRecord] = &authorityRecordCustomValidator{}

func setupAuthorityRecordWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &messagingv1alpha1.AuthorityRecord{}).
		WithValidator(&authorityRecordCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

func (v *authorityRecordCustomValidator) ValidateCreate(
	ctx context.Context,
	auth *messagingv1alpha1.AuthorityRecord,
) (admission.Warnings, error) {
	return validateCreateUpdate(ctx, v.Client, auth, v.validateRecord, validation.AuthorityRecordInvalid)
}

func (v *authorityRecordCustomValidator) ValidateUpdate(
	ctx context.Context,
	_ *messagingv1alpha1.AuthorityRecord,
	newAuth *messagingv1alpha1.AuthorityRecord,
) (admission.Warnings, error) {
	if newAuth.DeletionTimestamp != nil {
		return nil, nil
	}
	return validateCreateUpdate(ctx, v.Client, newAuth, v.validateRecord, validation.AuthorityRecordInvalid)
}

func (v *authorityRecordCustomValidator) ValidateDelete(
	_ context.Context,
	_ *messagingv1alpha1.AuthorityRecord,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *authorityRecordCustomValidator) validateRecord(
	ctx context.Context,
	reader client.Reader,
	auth *messagingv1alpha1.AuthorityRecord,
) ([]string, field.ErrorList) {
	return nil, validation.ValidateAuthorityRecordSpec(ctx, reader, auth.Namespace, auth.Name, &auth.Spec)
}
