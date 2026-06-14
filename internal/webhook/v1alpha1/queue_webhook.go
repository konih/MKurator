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
// +kubebuilder:webhook:path=/validate-messaging-mkurator-dev-v1alpha1-queue,mutating=false,failurePolicy=fail,sideEffects=None,groups=messaging.mkurator.dev,resources=queues,verbs=create;update,versions=v1alpha1,name=vqueue.kb.io,admissionReviewVersions=v1

type queueCustomValidator struct {
	Client client.Reader
}

var _ admission.Validator[*messagingv1alpha1.Queue] = &queueCustomValidator{}

func setupQueueWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &messagingv1alpha1.Queue{}).
		WithValidator(&queueCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

func (v *queueCustomValidator) ValidateCreate(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
) (admission.Warnings, error) {
	return validateCreateUpdate(ctx, v.Client, queue, v.validateQueue, validation.QueueInvalid)
}

func (v *queueCustomValidator) ValidateUpdate(
	ctx context.Context,
	_ *messagingv1alpha1.Queue,
	newQueue *messagingv1alpha1.Queue,
) (admission.Warnings, error) {
	return validateCreateUpdate(ctx, v.Client, newQueue, v.validateQueue, validation.QueueInvalid)
}

func (v *queueCustomValidator) ValidateDelete(
	_ context.Context,
	_ *messagingv1alpha1.Queue,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *queueCustomValidator) validateQueue(
	ctx context.Context,
	reader client.Reader,
	queue *messagingv1alpha1.Queue,
) ([]string, field.ErrorList) {
	return validation.ValidateQueueSpec(ctx, reader, queue.Namespace, queue.Name, &queue.Spec)
}
