package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

type AdoptionBlockedError struct {
	Reason  string
	Message string
}

func (e *AdoptionBlockedError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func adoptionBlockForExisting(
	policy messagingv1alpha1.AdoptionPolicy,
	firstAdoption bool,
	exists bool,
	needsUpdate bool,
	objectLabel string,
	mismatchMessage string,
) *AdoptionBlockedError {
	if !firstAdoption || !exists {
		return nil
	}
	switch policy {
	case messagingv1alpha1.AdoptionPolicyFailIfExists:
		return &AdoptionBlockedError{
			Reason:  messagingv1alpha1.ReasonAlreadyExists,
			Message: fmt.Sprintf("%s already exists on queue manager", objectLabel),
		}
	case messagingv1alpha1.AdoptionPolicyAdoptIfMatching:
		if needsUpdate {
			msg := mismatchMessage
			if msg == "" {
				msg = fmt.Sprintf("%s differs from spec", objectLabel)
			}
			return &AdoptionBlockedError{
				Reason:  messagingv1alpha1.ReasonAdoptionConflict,
				Message: msg,
			}
		}
	}
	return nil
}

func handleAdoptionBlock(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	block *AdoptionBlockedError,
	opts syncStatusOpts,
) (ctrl.Result, error) {
	if block == nil {
		return ctrl.Result{}, nil
	}
	if err := patchSyncedAdoptionBlocked(
		ctx, status, recorder, obj, generation, block.Reason, block.Message, opts,
	); err != nil {
		return ctrl.Result{}, err
	}
	return workloadDriftResyncResult(), nil
}

func patchSyncedAdoptionBlocked(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	reason, message string,
	opts syncStatusOpts,
) error {
	emitSyncedTransitionEvent(recorder, obj, metav1.ConditionFalse, reason, message)

	switch o := obj.(type) {
	case *messagingv1alpha1.Queue:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, reason, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	case *messagingv1alpha1.Topic:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, reason, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	case *messagingv1alpha1.Channel:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, reason, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	case *messagingv1alpha1.ChannelAuthRule:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, reason, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	case *messagingv1alpha1.AuthorityRecord:
		setCondition(&o.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, reason, message, generation)
		applyMQObjectStatusFields(o, opts, message, nil)
		return status.Update(ctx, o)
	default:
		return fmt.Errorf("patchSyncedAdoptionBlocked: unsupported type %T", obj)
	}
}

func attributeMismatchMessage(
	desiredAttrs map[string]string,
	observedAttrs map[string]string,
	driftKeys []string,
	objectLabel string,
) string {
	if drifts := mqadmin.AttributeDriftsForKeys(desiredAttrs, observedAttrs, driftKeys); len(drifts) > 0 {
		return mqadmin.FormatAttributeDriftMessage(drifts)
	}
	return fmt.Sprintf("%s differs from spec", objectLabel)
}
