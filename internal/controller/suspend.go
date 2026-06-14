package controller

import (
	"context"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

const suspendedMessage = "Reconciliation suspended"

func workloadSuspended(obj client.Object) bool {
	switch o := obj.(type) {
	case *messagingv1alpha1.Queue:
		return o.Spec.Suspend
	case *messagingv1alpha1.Topic:
		return o.Spec.Suspend
	case *messagingv1alpha1.Channel:
		return o.Spec.Suspend
	case *messagingv1alpha1.ChannelAuthRule:
		return o.Spec.Suspend
	case *messagingv1alpha1.AuthorityRecord:
		return o.Spec.Suspend
	default:
		return false
	}
}

func workloadAlreadySuspended(obj client.Object, generation int64) bool {
	for _, c := range syncedConditions(obj) {
		if c.Type == messagingv1alpha1.ConditionSynced &&
			c.Status == metav1.ConditionFalse &&
			c.Reason == messagingv1alpha1.ReasonSuspended &&
			c.ObservedGeneration == generation {
			return true
		}
	}
	return false
}

func reconcileWorkloadSuspended(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
) (ctrl.Result, error) {
	if workloadAlreadySuspended(obj, generation) {
		return ctrl.Result{}, nil
	}
	if err := patchSyncedSuspended(ctx, status, recorder, obj, generation, suspendedMessage); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func patchSyncedSuspended(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	message string,
) error {
	return patchSyncedStatus(ctx, status, recorder, obj, syncedStatusPatch{
		conditionStatus: metav1.ConditionFalse,
		reason:          messagingv1alpha1.ReasonSuspended,
		generation:      generation,
		message:         message,
		emitEvent:       true,
	})
}

func workloadReconcilePredicates() predicate.Predicate {
	return predicate.Or(
		predicate.GenerationChangedPredicate{},
		reconcileRequestedAnnotationChanged{},
		workloadLifecycleChanged{},
	)
}

// workloadLifecycleChanged enqueues when finalizers or deletionTimestamp change.
// GenerationChangedPredicate alone skips finalizer-only updates (e2e first-sync stall).
type workloadLifecycleChanged struct {
	predicate.Funcs
}

func (workloadLifecycleChanged) Update(e event.UpdateEvent) bool {
	oldMeta, okOld := e.ObjectOld.(metav1.Object)
	newMeta, okNew := e.ObjectNew.(metav1.Object)
	if !okOld || !okNew {
		return false
	}
	if !oldMeta.GetDeletionTimestamp().Equal(newMeta.GetDeletionTimestamp()) {
		return true
	}
	return !slices.Equal(oldMeta.GetFinalizers(), newMeta.GetFinalizers())
}

type reconcileRequestedAnnotationChanged struct {
	predicate.Funcs
}

func (reconcileRequestedAnnotationChanged) Update(e event.UpdateEvent) bool {
	oldMeta, okOld := e.ObjectOld.(metav1.Object)
	newMeta, okNew := e.ObjectNew.(metav1.Object)
	if !okOld || !okNew {
		return false
	}
	oldAnn := annotationValue(oldMeta.GetAnnotations(), messagingv1alpha1.ReconcileRequestedAtAnnotation)
	newAnn := annotationValue(newMeta.GetAnnotations(), messagingv1alpha1.ReconcileRequestedAtAnnotation)
	return oldAnn != newAnn
}

func annotationValue(annotations map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	return annotations[key]
}
