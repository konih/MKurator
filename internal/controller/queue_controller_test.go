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
