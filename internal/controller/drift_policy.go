package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/mqadmin"
)

func isObserveOnly(obj client.Object) bool {
	anns := obj.GetAnnotations()
	if anns == nil {
		return false
	}
	return anns[messagingv1alpha1.DriftPolicyAnnotation] == messagingv1alpha1.DriftPolicyObserveOnly
}

func reconcileMQObjectState(
	observeOnly bool,
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

	if !exists || mqadmin.AttributesNeedUpdate(desiredAttrs, observedAttrs, driftKeys) {
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
	recorder record.EventRecorder,
	obj client.Object,
	generation int64,
	message string,
	opts syncStatusOpts,
) error {
	emitSyncedTransitionEvent(recorder, obj, metav1.ConditionFalse, messagingv1alpha1.ReasonDriftDetected, message)

	switch o := obj.(type) {
	case *messagingv1alpha1.Queue:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, messagingv1alpha1.ReasonDriftDetected, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	case *messagingv1alpha1.Topic:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, messagingv1alpha1.ReasonDriftDetected, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	case *messagingv1alpha1.Channel:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, messagingv1alpha1.ReasonDriftDetected, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	case *messagingv1alpha1.ChannelAuthRule:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, messagingv1alpha1.ReasonDriftDetected, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	case *messagingv1alpha1.AuthorityRecord:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, messagingv1alpha1.ReasonDriftDetected, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	default:
		return fmt.Errorf("patchSyncedDrift: unsupported type %T", obj)
	}
}
