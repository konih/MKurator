package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/mqadmin"
	mqadmintest "github.com/konih/kurator/test/mocks/mqadmin"
)

func TestIsObserveOnly(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{}
	if isObserveOnly(q) {
		t.Fatal("expected default false")
	}
	q.Annotations = map[string]string{
		messagingv1alpha1.DriftPolicyAnnotation: messagingv1alpha1.DriftPolicyObserveOnly,
	}
	if !isObserveOnly(q) {
		t.Fatal("expected observe-only")
	}
}

func TestReconcileMQObjectState_ObserveOnlyDrift(t *testing.T) {
	t.Parallel()
	exists, msg, err := reconcileMQObjectState(
		true,
		true,
		map[string]string{"maxdepth": "1000"},
		map[string]string{"maxdepth": "5000"},
		[]string{"maxdepth"},
		"queue \"APP.Q\"",
		func() error { return errors.New("define should not run") },
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !exists || msg == "" {
		t.Fatalf("exists=%v msg=%q", exists, msg)
	}
}

func TestReconcileMQObjectState_ObserveOnlyNotFound(t *testing.T) {
	t.Parallel()
	exists, msg, err := reconcileMQObjectState(
		true,
		false,
		nil,
		map[string]string{"maxdepth": "5000"},
		[]string{"maxdepth"},
		"queue \"APP.Q\"",
		func() error { return errors.New("define should not run") },
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if exists || msg != `queue "APP.Q" not found on queue manager` {
		t.Fatalf("exists=%v msg=%q", exists, msg)
	}
}

func TestReconcileMQObjectState_DefaultDefinesOnDrift(t *testing.T) {
	t.Parallel()
	called := false
	exists, msg, err := reconcileMQObjectState(
		false,
		true,
		map[string]string{"maxdepth": "1000"},
		map[string]string{"maxdepth": "5000"},
		[]string{"maxdepth"},
		"queue \"APP.Q\"",
		func() error {
			called = true
			return nil
		},
	)
	if err != nil || msg != "" || !exists || !called {
		t.Fatalf("exists=%v msg=%q err=%v called=%v", exists, msg, err, called)
	}
}

func TestQueueReconciler_ObserveOnlyReportsDriftWithoutDefine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "orders",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.QueueFinalizer},
			Annotations: map[string]string{
				messagingv1alpha1.DriftPolicyAnnotation: messagingv1alpha1.DriftPolicyObserveOnly,
			},
		},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
			Type:          messagingv1alpha1.QueueTypeLocal,
			Attributes:    map[string]string{"maxdepth": "5000"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(q, conn).
		WithObjects(conn, q).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetQueue(mock.Anything, mock.Anything).Return(&mqadmin.QueueState{
		Attributes: map[string]string{"maxdepth": "1000"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.Queue{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != metav1.ConditionFalse {
		t.Fatalf("Synced = %v", updated.Status.Conditions)
	}
	reason := conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)
	if reason != messagingv1alpha1.ReasonDriftDetected {
		t.Fatalf("reason = %q", reason)
	}
}

func TestTopicReconciler_ObserveOnlySyncedWithoutDefine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "retail"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "retail",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.TopicFinalizer},
			Annotations: map[string]string{
				messagingv1alpha1.DriftPolicyAnnotation: messagingv1alpha1.DriftPolicyObserveOnly,
			},
		},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
			Attributes:    map[string]string{"topstr": "retail/orders"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(topic, conn).
		WithObjects(conn, topic).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetTopic(mock.Anything, "RETAIL.ORDERS").Return(&mqadmin.TopicState{
		Attributes: map[string]string{"topstr": "retail/orders"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &TopicReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.Topic{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != metav1.ConditionTrue {
		t.Fatalf("Synced = %v", updated.Status.Conditions)
	}
}

func TestChannelReconciler_ObserveOnlyReportsDriftWithoutDefine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders-app"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "orders-app",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelFinalizer},
			Annotations: map[string]string{
				messagingv1alpha1.DriftPolicyAnnotation: messagingv1alpha1.DriftPolicyObserveOnly,
			},
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Attributes:    map[string]string{"sslciph": "NULL"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannel(mock.Anything, mock.Anything).Return(&mqadmin.ChannelState{
		Attributes: map[string]string{"sslciph": "TLS_RSA_WITH_AES_128_CBC_SHA256"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.Channel{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	reason := conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)
	if reason != messagingv1alpha1.ReasonDriftDetected {
		t.Fatalf("reason = %q", reason)
	}
}
