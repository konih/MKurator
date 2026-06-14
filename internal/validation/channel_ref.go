package validation

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

// ValidateManagedChannelRef ensures a Channel CR exists in the same namespace with matching
// spec.channelName and spec.connectionRef.name. CHLAUTH rules target channels MKurator manages
// via Channel CRs; pre-existing MQ-only channels are out of scope for this check.
func ValidateManagedChannelRef(
	ctx context.Context,
	reader client.Reader,
	namespace, connectionRefName, channelName string,
	path *field.Path,
) field.ErrorList {
	var errs field.ErrorList
	if channelName == "" {
		return errs
	}

	var channels messagingv1alpha1.ChannelList
	if err := reader.List(ctx, &channels, client.InNamespace(namespace)); err != nil {
		return field.ErrorList{
			field.InternalError(path, fmt.Errorf("list Channels: %w", err)),
		}
	}

	var match *messagingv1alpha1.Channel
	for i := range channels.Items {
		ch := &channels.Items[i]
		if ch.Spec.ChannelName != channelName {
			continue
		}
		if connectionRefName != "" && ch.Spec.ConnectionRef.Name != connectionRefName {
			continue
		}
		match = ch
		break
	}
	if match == nil {
		return field.ErrorList{
			field.NotFound(path, fmt.Sprintf(
				"Channel with channelName %q and connectionRef %q not found in namespace %q; create a Channel CR first",
				channelName, connectionRefName, namespace,
			)),
		}
	}
	if match.DeletionTimestamp != nil {
		return field.ErrorList{
			field.Invalid(path, channelName, fmt.Sprintf(
				"Channel %q is deleting; wait for deletion to finish or point channelName at another Channel",
				match.Name,
			)),
		}
	}
	return errs
}
