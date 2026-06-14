package controller

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

const (
	EventReasonConnectionNotFound = "ConnectionNotFound"
	EventReasonSecretNotFound     = "SecretNotFound"
	EventReasonDeleted            = "Deleted"

	eventActionReconcile = "Reconcile"
)

func classifyReconcileError(err error) (reason, message string) {
	message = err.Error()

	var term *mqadmin.TerminalError
	if errors.As(err, &term) {
		reason = messagingv1alpha1.ReasonError
		if term.Reason != "" {
			reason = term.Reason
		}
		if term.Message != "" {
			message = term.Message
		}
		return reason, message
	}

	var connNF *mqadmin.ConnectionNotFoundError
	if errors.As(err, &connNF) {
		return EventReasonConnectionNotFound, message
	}

	var secretNF *mqadmin.SecretNotFoundError
	if errors.As(err, &secretNF) {
		return EventReasonSecretNotFound, message
	}

	return messagingv1alpha1.ReasonError, message
}

func recordReconcileWarning(recorder events.EventRecorder, obj runtime.Object, err error) {
	if recorder == nil || err == nil || errors.Is(err, mqadmin.ErrTransient) {
		return
	}
	reason, message := classifyReconcileError(err)
	recorder.Eventf(obj, nil, corev1.EventTypeWarning, reason, eventActionReconcile, "%s", message)
}

func recordNormalEvent(recorder events.EventRecorder, obj runtime.Object, reason, message string) {
	if recorder == nil {
		return
	}
	recorder.Eventf(obj, nil, corev1.EventTypeNormal, reason, eventActionReconcile, "%s", message)
}

func conditionChanged(
	conditions []metav1.Condition,
	condType string,
	newStatus metav1.ConditionStatus,
	newReason string,
) bool {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Status != newStatus || c.Reason != newReason
		}
	}
	return true
}
