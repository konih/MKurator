package validation

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

// ValidateConnectionRef ensures the referenced QueueManagerConnection exists in the same namespace and is not deleting.
func ValidateConnectionRef(
	ctx context.Context,
	reader client.Reader,
	namespace, refName string,
	path *field.Path,
) field.ErrorList {
	var errs field.ErrorList
	if refName == "" {
		return append(errs, field.Required(path, "connectionRef.name is required"))
	}

	conn := &messagingv1alpha1.QueueManagerConnection{}
	key := client.ObjectKey{Namespace: namespace, Name: refName}
	if err := reader.Get(ctx, key, conn); err != nil {
		if apierrors.IsNotFound(err) {
			return append(errs, field.NotFound(path, fmt.Sprintf(
				"QueueManagerConnection %q not found in namespace %q", refName, namespace)))
		}
		return append(errs, field.InternalError(path, fmt.Errorf("get QueueManagerConnection %q: %w", refName, err)))
	}
	if conn.DeletionTimestamp != nil {
		return append(errs, field.Invalid(path, refName, fmt.Sprintf(
			"QueueManagerConnection %q is deleting; remove or wait for deletion to finish before creating dependents",
			refName,
		)))
	}
	return errs
}
