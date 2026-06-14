package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
	mqadmintest "github.com/conduit-ops/mkurator/test/mocks/mqadmin"
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
		messagingv1alpha1.AdoptionPolicyAdopt,
		false,
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

func TestObserveOnlyAuthDriftMessage(t *testing.T) {
	t.Parallel()
	if got := observeOnlyAuthDriftMessage(false, "APP.ORDERS", "authority record"); got !=
		`authority record for "APP.ORDERS" not found on queue manager (observe-only; not applying)` {
		t.Fatalf("got %q", got)
	}
	if got := observeOnlyAuthDriftMessage(true, "DEV.APP", "CHLAUTH rule"); got !=
		"CHLAUTH on IBM MQ differs from spec (observe-only; not applying)" {
		t.Fatalf("got %q", got)
	}
}

func TestReconcileMQObjectState_ObserveOnlyNotFound(t *testing.T) {
	t.Parallel()
	exists, msg, err := reconcileMQObjectState(
		true,
		messagingv1alpha1.AdoptionPolicyAdopt,
		false,
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

func TestReconcileMQObjectState_NoDefineWhenAttributesMatch(t *testing.T) {
	t.Parallel()
	called := false
	exists, msg, err := reconcileMQObjectState(
		false,
		messagingv1alpha1.AdoptionPolicyAdopt,
		false,
		true,
		map[string]string{"maxdepth": "5000"},
		map[string]string{"maxdepth": "5000"},
		[]string{"maxdepth"},
		"queue \"APP.Q\"",
		func() error {
			called = true
			return nil
		},
	)
	if err != nil || msg != "" || !exists || called {
		t.Fatalf("exists=%v msg=%q err=%v called=%v", exists, msg, err, called)
	}
}

func TestReconcileMQObjectState_DefinesWhenMissing(t *testing.T) {
	t.Parallel()
	called := false
	exists, msg, err := reconcileMQObjectState(
		false,
		messagingv1alpha1.AdoptionPolicyAdopt,
		false,
		false,
		nil,
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

func TestReconcileMQObjectState_PubSubCaseInsensitive(t *testing.T) {
	t.Parallel()
	called := false
	exists, msg, err := reconcileMQObjectState(
		false,
		messagingv1alpha1.AdoptionPolicyAdopt,
		false,
		true,
		map[string]string{"pub": "enabled", "sub": "enabled"},
		map[string]string{"pub": "ENABLED", "sub": "ENABLED"},
		[]string{"pub", "sub"},
		"topic \"RETAIL.ORDERS\"",
		func() error {
			called = true
			return nil
		},
	)
	if err != nil || msg != "" || !exists || called {
		t.Fatalf("exists=%v msg=%q err=%v called=%v", exists, msg, err, called)
	}
}

func TestReconcileMQObjectState_ChannelTrptypeDrift(t *testing.T) {
	t.Parallel()
	called := false
	exists, msg, err := reconcileMQObjectState(
		false,
		messagingv1alpha1.AdoptionPolicyAdopt,
		false,
		true,
		map[string]string{"trptype": "tcp"},
		map[string]string{"trptype": "lu62"},
		[]string{"trptype"},
		"channel \"ORDERS.APP\"",
		func() error {
			called = true
			return nil
		},
	)
	if err != nil || msg != "" || !exists || !called {
		t.Fatalf("exists=%v msg=%q err=%v called=%v", exists, msg, err, called)
	}
}

func TestReconcileMQObjectState_ObserveOnlyNoDrift(t *testing.T) {
	t.Parallel()
	exists, msg, err := reconcileMQObjectState(
		true,
		messagingv1alpha1.AdoptionPolicyAdopt,
		false,
		true,
		map[string]string{"maxdepth": "5000"},
		map[string]string{"maxdepth": "5000"},
		[]string{"maxdepth"},
		"queue \"APP.Q\"",
		func() error { return errors.New("define should not run") },
	)
	if err != nil || msg != "" || !exists {
		t.Fatalf("exists=%v msg=%q err=%v", exists, msg, err)
	}
}

func TestReconcileMQObjectState_DefaultDefinesOnDrift(t *testing.T) {
	t.Parallel()
	called := false
	exists, msg, err := reconcileMQObjectState(
		false,
		messagingv1alpha1.AdoptionPolicyAdopt,
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
	ns := "mkurator-system"
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
	ns := "mkurator-system"
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
	ns := "mkurator-system"
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

func TestQueueReconciler_PeriodicResyncDetectsDrift(t *testing.T) {
	t.Parallel()
	t.Cleanup(func() {
		SetDriftResyncInterval(defaultDriftResyncLower, defaultDriftResyncUpper)
	})
	SetDriftResyncInterval(7*time.Minute, 7*time.Minute)

	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "orders",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.QueueFinalizer},
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

	queueSpec := mqadmin.QueueSpec{
		Name:       "APP.ORDERS",
		Type:       mqadmin.QueueTypeLocal,
		Attributes: map[string]string{"maxdepth": "5000"},
	}

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetQueue(mock.Anything, queueSpec).Return(&mqadmin.QueueState{
		Attributes: map[string]string{"maxdepth": "5000"},
	}, nil).Once()
	mockAdmin.EXPECT().GetQueue(mock.Anything, queueSpec).Return(&mqadmin.QueueState{
		Attributes: map[string]string{"maxdepth": "1000"},
	}, nil).Once()
	mockAdmin.EXPECT().DefineQueue(mock.Anything, queueSpec).Return(nil).Once()

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}

	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	assertDriftResyncRequeue(t, result)

	result, err = rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("resync reconcile: %v", err)
	}
	assertDriftResyncRequeue(t, result)

	updated := &messagingv1alpha1.Queue{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != metav1.ConditionTrue {
		t.Fatalf("Synced = %v", updated.Status.Conditions)
	}
}

func TestReconcileMQObjectState_FailIfExistsBlocks(t *testing.T) {
	t.Parallel()
	_, _, err := reconcileMQObjectState(
		false,
		messagingv1alpha1.AdoptionPolicyFailIfExists,
		true,
		true,
		map[string]string{"maxdepth": "5000"},
		map[string]string{"maxdepth": "1000"},
		[]string{"maxdepth"},
		`queue "APP.Q"`,
		func() error { t.Fatal("define should not run"); return nil },
	)
	var block *AdoptionBlockedError
	if !errors.As(err, &block) || block.Reason != messagingv1alpha1.ReasonAlreadyExists {
		t.Fatalf("err = %v", err)
	}
}
