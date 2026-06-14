package controller

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
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

func TestFindCondition(t *testing.T) {
	t.Parallel()
	conds := []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}
	c, ok := findCondition(conds, "Ready")
	if !ok || c.Status != metav1.ConditionTrue {
		t.Fatalf("findCondition() = (%v, %v)", c, ok)
	}
	if _, ok := findCondition(conds, "Synced"); ok {
		t.Fatal("expected missing condition")
	}
}

func TestApplyMQObjectStatusFields(t *testing.T) {
	t.Parallel()
	now := metav1.NewTime(time.Now())
	exists := true
	opts := syncStatusOpts{mqObjectExists: &exists}

	t.Run("queue", func(t *testing.T) {
		t.Parallel()
		q := &messagingv1alpha1.Queue{}
		applyMQObjectStatusFields(q, opts, "synced", &now)
		if q.Status.Message != "synced" || q.Status.MQObjectExists == nil || !*q.Status.MQObjectExists ||
			q.Status.LastSyncTime == nil {
			t.Fatalf("status = %+v", q.Status)
		}
	})
	t.Run("topic", func(t *testing.T) {
		t.Parallel()
		o := &messagingv1alpha1.Topic{}
		applyMQObjectStatusFields(o, opts, "synced", &now)
		if o.Status.Message != "synced" || o.Status.LastSyncTime == nil {
			t.Fatalf("status = %+v", o.Status)
		}
	})
	t.Run("channel", func(t *testing.T) {
		t.Parallel()
		o := &messagingv1alpha1.Channel{}
		applyMQObjectStatusFields(o, opts, "synced", &now)
		if o.Status.Message != "synced" {
			t.Fatalf("status = %+v", o.Status)
		}
	})
	t.Run("channelauthrule", func(t *testing.T) {
		t.Parallel()
		o := &messagingv1alpha1.ChannelAuthRule{}
		applyMQObjectStatusFields(o, opts, "synced", nil)
		if o.Status.Message != "synced" || o.Status.LastSyncTime != nil {
			t.Fatalf("status = %+v", o.Status)
		}
	})
	t.Run("authorityrecord", func(t *testing.T) {
		t.Parallel()
		o := &messagingv1alpha1.AuthorityRecord{}
		applyMQObjectStatusFields(o, syncStatusOpts{}, "waiting", nil)
		if o.Status.Message != "waiting" || o.Status.MQObjectExists != nil {
			t.Fatalf("status = %+v", o.Status)
		}
	})
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
