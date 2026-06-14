package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func isObserveOnly(obj client.Object) bool {
	anns := obj.GetAnnotations()
	if anns == nil {
		return false
	}
	return anns[messagingv1alpha1.DriftPolicyAnnotation] == messagingv1alpha1.DriftPolicyObserveOnly
}

func observeOnlyAuthDriftMessage(exists bool, objectName, objectLabel string) string {
	if !exists {
		return fmt.Sprintf(
			`%s for %q not found on queue manager (observe-only; not applying)`,
			objectLabel,
			objectName,
		)
	}
	switch objectLabel {
	case "CHLAUTH rule":
		return "CHLAUTH on IBM MQ differs from spec (observe-only; not applying)"
	case "authority record":
		return "AUTHREC on IBM MQ differs from spec (observe-only; not applying)"
	default:
		return fmt.Sprintf("%s on IBM MQ differs from spec (observe-only; not applying)", objectLabel)
	}
}

func reconcileMQObjectState(
	observeOnly bool,
	adoptionPolicy messagingv1alpha1.AdoptionPolicy,
	firstAdoption bool,
	exists bool,
	observedAttrs map[string]string,
	desiredAttrs map[string]string,
	driftKeys []string,
	objectLabel string,
	defineFn func() error,
) (bool, string, error) {
	if observeOnly {
		if !exists {
			return false, fmt.Sprintf("%s not found on queue manager", objectLabel), nil
		}
		if drifts := mqadmin.AttributeDriftsForKeys(desiredAttrs, observedAttrs, driftKeys); len(drifts) > 0 {
			return true, mqadmin.FormatAttributeDriftMessage(drifts), nil
		}
		return true, "", nil
	}

	needsUpdate := !exists || mqadmin.AttributesNeedUpdate(desiredAttrs, observedAttrs, driftKeys)
	if blocked := adoptionBlockForExisting(
		adoptionPolicy,
		firstAdoption,
		exists,
		needsUpdate,
		objectLabel,
		attributeMismatchMessage(desiredAttrs, observedAttrs, driftKeys, objectLabel),
	); blocked != nil {
		return exists, "", blocked
	}

	if needsUpdate {
		if err := defineFn(); err != nil {
			return exists, "", err
		}
		return true, "", nil
	}
	return exists, "", nil
}

func patchSyncedDrift(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	message string,
	opts syncStatusOpts,
) error {
	return patchSyncedStatus(ctx, status, recorder, obj, syncedStatusPatch{
		conditionStatus: metav1.ConditionFalse,
		reason:          messagingv1alpha1.ReasonDriftDetected,
		generation:      generation,
		message:         message,
		opts:            opts,
		emitEvent:       true,
	})
}
