package webhookv1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/conduit-ops/mkurator/internal/validation"
)

func admissionResult(
	warnings []string,
	errs field.ErrorList,
	invalid func(string, field.ErrorList) error,
	name string,
) (admission.Warnings, error) {
	if len(errs) > 0 {
		return warnings, invalid(name, errs)
	}
	return warnings, nil
}

func validateCreateUpdate[T client.Object](
	ctx context.Context,
	reader client.Reader,
	obj T,
	validate func(context.Context, client.Reader, T) ([]string, field.ErrorList),
	invalid func(string, field.ErrorList) error,
) (admission.Warnings, error) {
	warnings, errs := validate(ctx, reader, obj)
	return admissionResult(warnings, errs, invalid, validation.ObjectNameFromMeta(obj))
}

// SetupWithManager registers all validating webhooks with the manager.
func SetupWithManager(mgr ctrl.Manager) error {
	if err := setupQueueManagerConnectionWebhook(mgr); err != nil {
		return fmt.Errorf("setup QueueManagerConnection webhook: %w", err)
	}
	if err := setupQueueWebhook(mgr); err != nil {
		return fmt.Errorf("setup Queue webhook: %w", err)
	}
	if err := setupTopicWebhook(mgr); err != nil {
		return fmt.Errorf("setup Topic webhook: %w", err)
	}
	if err := setupChannelWebhook(mgr); err != nil {
		return fmt.Errorf("setup Channel webhook: %w", err)
	}
	if err := setupChannelAuthRuleWebhook(mgr); err != nil {
		return fmt.Errorf("setup ChannelAuthRule webhook: %w", err)
	}
	if err := setupAuthorityRecordWebhook(mgr); err != nil {
		return fmt.Errorf("setup AuthorityRecord webhook: %w", err)
	}
	return nil
}
