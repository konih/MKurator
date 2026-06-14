package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func TestToMQQueueSpec(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			QueueName: "APP.ORDERS",
			Type:      messagingv1alpha1.QueueTypeLocal,
			Attributes: map[string]string{
				"MaxDepth": "5000",
			},
		},
	}
	spec := toMQQueueSpec(q)
	if spec.Name != "APP.ORDERS" || spec.Type != mqadmin.QueueTypeLocal {
		t.Fatalf("spec = %+v", spec)
	}
	if spec.Attributes["maxdepth"] != "5000" {
		t.Fatalf("attrs = %v", spec.Attributes)
	}
}

func TestToMQQueueSpecTypedMaxDepth(t *testing.T) {
	t.Parallel()
	depth := int32(10000)
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			QueueName: "APP.ORDERS",
			Type:      messagingv1alpha1.QueueTypeLocal,
			MaxDepth:  &depth,
		},
	}
	spec := toMQQueueSpec(q)
	if spec.Attributes["maxdepth"] != "10000" {
		t.Fatalf("attrs = %v", spec.Attributes)
	}
}

func TestToMQQueueSpecTypedDescription(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			QueueName:   "APP.ORDERS",
			Type:        messagingv1alpha1.QueueTypeLocal,
			Description: "Order processing queue",
		},
	}
	spec := toMQQueueSpec(q)
	if spec.Attributes["descr"] != "Order processing queue" {
		t.Fatalf("attrs = %v", spec.Attributes)
	}
}

func TestToMQQueueSpecTypedDefPersistence(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			QueueName:      "APP.ORDERS",
			Type:           messagingv1alpha1.QueueTypeLocal,
			DefPersistence: messagingv1alpha1.QueueDefaultPersistenceYes,
		},
	}
	spec := toMQQueueSpec(q)
	if spec.Attributes["defpsist"] != "yes" {
		t.Fatalf("attrs = %v", spec.Attributes)
	}
}

func TestToMQQueueSpecTypedTargetQueue(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			QueueName:   "APP.ORDERS.ALIAS",
			Type:        messagingv1alpha1.QueueTypeAlias,
			TargetQueue: "APP.ORDERS",
		},
	}
	spec := toMQQueueSpec(q)
	if spec.Attributes["targq"] != "APP.ORDERS" {
		t.Fatalf("attrs = %v", spec.Attributes)
	}
}

func TestToMQQueueSpecTypedXmitQueue(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			QueueName: "APP.ORDERS.REMOTE",
			Type:      messagingv1alpha1.QueueTypeRemote,
			XmitQueue: "SYSTEM.DEFAULT.XMIT.QUEUE",
		},
	}
	spec := toMQQueueSpec(q)
	if spec.Attributes["xmitq"] != "SYSTEM.DEFAULT.XMIT.QUEUE" {
		t.Fatalf("attrs = %v", spec.Attributes)
	}
}

func TestToMQQueueSpecTypedRemoteQueueManager(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			QueueName:          "APP.ORDERS.REMOTE",
			Type:               messagingv1alpha1.QueueTypeRemote,
			RemoteQueueManager: "QM2",
		},
	}
	spec := toMQQueueSpec(q)
	if spec.Attributes["rqmname"] != "QM2" {
		t.Fatalf("attrs = %v", spec.Attributes)
	}
}

func TestToMQQueueSpecTypedGetPut(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			QueueName: "APP.ORDERS",
			Type:      messagingv1alpha1.QueueTypeLocal,
			Get:       messagingv1alpha1.QueueAccessEnabledEnabled,
			Put:       messagingv1alpha1.QueueAccessEnabledDisabled,
		},
	}
	spec := toMQQueueSpec(q)
	if spec.Attributes["get"] != "enabled" {
		t.Fatalf("get = %q", spec.Attributes["get"])
	}
	if spec.Attributes["put"] != "disabled" {
		t.Fatalf("put = %q", spec.Attributes["put"])
	}
}

func TestConnectionReady(t *testing.T) {
	t.Parallel()
	ready := &messagingv1alpha1.QueueManagerConnection{
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:   messagingv1alpha1.ConditionReady,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	if !connectionReady(ready) {
		t.Fatal("expected ready")
	}
	pending := &messagingv1alpha1.QueueManagerConnection{}
	if connectionReady(pending) {
		t.Fatal("expected not ready")
	}
}
