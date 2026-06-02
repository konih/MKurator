package webhookv1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/validation"
)

//nolint:lll // kubebuilder webhook marker is a single line
// +kubebuilder:webhook:path=/validate-messaging-kurator-dev-v1alpha1-queuemanagerconnection,mutating=false,failurePolicy=fail,sideEffects=None,groups=messaging.kurator.dev,resources=queuemanagerconnections,verbs=create;update;delete,versions=v1alpha1,name=vqueuemanagerconnection.kb.io,admissionReviewVersions=v1

type queueManagerConnectionCustomValidator struct {
	Client client.Reader
}

var _ admission.Validator[*messagingv1alpha1.QueueManagerConnection] = &queueManagerConnectionCustomValidator{}

func setupQueueManagerConnectionWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &messagingv1alpha1.QueueManagerConnection{}).
		WithValidator(&queueManagerConnectionCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

func (v *queueManagerConnectionCustomValidator) ValidateCreate(
	ctx context.Context,
	conn *messagingv1alpha1.QueueManagerConnection,
) (admission.Warnings, error) {
	return v.validate(ctx, conn)
}

func (v *queueManagerConnectionCustomValidator) ValidateUpdate(
	ctx context.Context,
	_ *messagingv1alpha1.QueueManagerConnection,
	newConn *messagingv1alpha1.QueueManagerConnection,
) (admission.Warnings, error) {
	return v.validate(ctx, newConn)
}

func (v *queueManagerConnectionCustomValidator) ValidateDelete(
	ctx context.Context,
	conn *messagingv1alpha1.QueueManagerConnection,
) (admission.Warnings, error) {
	errs := validation.ValidateQueueManagerConnectionDelete(ctx, v.Client, conn)
	if len(errs) > 0 {
		return nil, validation.QueueManagerConnectionInvalid(validation.ObjectNameFromMeta(conn), errs)
	}
	return nil, nil
}

func (v *queueManagerConnectionCustomValidator) validate(
	ctx context.Context,
	conn *messagingv1alpha1.QueueManagerConnection,
) (admission.Warnings, error) {
	errs := validation.ValidateQueueManagerConnectionSpec(ctx, v.Client, conn.Namespace, conn.Annotations, &conn.Spec)
	if len(errs) > 0 {
		return nil, validation.QueueManagerConnectionInvalid(validation.ObjectNameFromMeta(conn), errs)
	}
	return nil, nil
}
