package controller

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
)

func TestConnectionWaitMessage_ReadyCondition(t *testing.T) {
	t.Parallel()
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1"},
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:    messagingv1alpha1.ConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  messagingv1alpha1.ReasonError,
				Message: "mqweb ping failed: connection refused",
			}},
		},
	}
	msg := connectionWaitMessage(conn)
	if !strings.Contains(msg, "qm1") || !strings.Contains(msg, "Error") ||
		!strings.Contains(msg, "connection refused") {
		t.Fatalf("message = %q", msg)
	}
}

func TestConnectionWaitMessage_NoReadyCondition(t *testing.T) {
	t.Parallel()
	conn := &messagingv1alpha1.QueueManagerConnection{ObjectMeta: metav1.ObjectMeta{Name: "qm1"}}
	msg := connectionWaitMessage(conn)
	if msg != `waiting for connection "qm1" to become Ready` {
		t.Fatalf("message = %q", msg)
	}
}

func TestConnectionWaitMessage_ReadyTrueStillWaiting(t *testing.T) {
	t.Parallel()
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1"},
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:   messagingv1alpha1.ConditionReady,
				Status: metav1.ConditionTrue,
				Reason: messagingv1alpha1.ReasonAvailable,
			}},
		},
	}
	msg := connectionWaitMessage(conn)
	if msg != `waiting for connection "qm1" to become Ready` {
		t.Fatalf("message = %q", msg)
	}
}

func TestConnectionWaitMessage_ReasonOnly(t *testing.T) {
	t.Parallel()
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1"},
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:   messagingv1alpha1.ConditionReady,
				Status: metav1.ConditionFalse,
				Reason: messagingv1alpha1.ReasonProgressing,
			}},
		},
	}
	msg := connectionWaitMessage(conn)
	if !strings.Contains(msg, "Progressing") {
		t.Fatalf("message = %q", msg)
	}
}

func TestConnectionWaitMessage_MessageOnly(t *testing.T) {
	t.Parallel()
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1"},
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:    messagingv1alpha1.ConditionReady,
				Status:  metav1.ConditionFalse,
				Message: "still connecting",
			}},
		},
	}
	msg := connectionWaitMessage(conn)
	if !strings.Contains(msg, "still connecting") {
		t.Fatalf("message = %q", msg)
	}
}
