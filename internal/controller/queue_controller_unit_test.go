package controller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
	mqadmintest "github.com/conduit-ops/mkurator/test/mocks/mqadmin"
)

var (
	unitScheme     *runtime.Scheme
	unitSchemeOnce sync.Once
)

func unitSchemeOrFatal(t *testing.T) *runtime.Scheme {
	t.Helper()
	unitSchemeOnce.Do(func() {
		s := runtime.NewScheme()
		if err := messagingv1alpha1.AddToScheme(s); err != nil {
			t.Fatalf("AddToScheme: %v", err)
		}
		unitScheme = s
	})
	return unitScheme
}

func TestQueueReconciler_SyncedWithoutDefine(t *testing.T) {
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
		Attributes: map[string]string{"maxdepth": "5000"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
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

func TestQueueReconciler_SetsDesiredMQSCInStatus(t *testing.T) {
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
		},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
			Type:          messagingv1alpha1.QueueTypeLocal,
			Attributes:    map[string]string{"maxdepth": "5000", "descr": "orders"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(q, conn).
		WithObjects(conn, q).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetQueue(mock.Anything, mock.Anything).Return(&mqadmin.QueueState{
		Attributes: map[string]string{"maxdepth": "5000", "descr": "orders"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.Queue{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	want := "DEFINE QLOCAL('APP.ORDERS') REPLACE DESCR('orders') MAXDEPTH(5000)"
	if updated.Status.DesiredMQSC != want {
		t.Fatalf("DesiredMQSC = %q, want %q", updated.Status.DesiredMQSC, want)
	}
}

func TestQueueReconciler_AddsFinalizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
			Type:          messagingv1alpha1.QueueTypeLocal,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(q, conn).
		WithObjects(conn, q).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	updated := &messagingv1alpha1.Queue{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Finalizers) != 1 {
		t.Fatalf("finalizers = %v", updated.Finalizers)
	}
}

func TestQueueReconciler_DeletionDeleteFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders"}
	s := unitSchemeOrFatal(t)

	now := metav1.Now()
	conn := readyConnForUnit(ns)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "orders",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.QueueFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
			Type:          messagingv1alpha1.QueueTypeLocal,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(q, conn).
		WithObjects(conn, q).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().
		DeleteQueue(mock.Anything, mock.Anything).
		Return(&mqadmin.TerminalError{Message: "delete denied"})

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
	if len(updated.Finalizers) == 0 {
		t.Fatal("finalizer should remain when delete fails")
	}
}

func TestQueueReconciler_Deletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders"}
	s := unitSchemeOrFatal(t)

	now := metav1.Now()
	conn := readyConnForUnit(ns)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "orders",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.QueueFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
			Type:          messagingv1alpha1.QueueTypeLocal,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(q, conn).
		WithObjects(conn, q).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteQueue(mock.Anything, mock.Anything).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.Queue{}
	err := cl.Get(ctx, key, updated)
	if apierrors.IsNotFound(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Finalizers) != 0 {
		t.Fatalf("finalizers = %v", updated.Finalizers)
	}
}

func TestQueueReconciler_TransientError(t *testing.T) {
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
		},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
			Type:          messagingv1alpha1.QueueTypeLocal,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(q, conn).
		WithObjects(conn, q).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetQueue(mock.Anything, mock.Anything).Return(nil, &mqadmin.TransientError{Message: "timeout"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("transient reconcile should requeue without error, got result=%+v err=%v", result, err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("RequeueAfter = %v", result.RequeueAfter)
	}
}

func TestQueueManagerConnectionReconciler_PingFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "qm1"}
	s := unitSchemeOrFatal(t)

	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "qm1",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.QueueManagerConnectionFinalizer},
		},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "mq-credentials",
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(conn).
		WithObjects(conn).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().Ping(mock.Anything).Return(&mqadmin.TerminalError{Message: "auth failed"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueManagerConnectionReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.QueueManagerConnection{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionReady) != metav1.ConditionFalse {
		t.Fatalf("Ready = %v", updated.Status.Conditions)
	}
}

func TestQueueReconciler_UnsupportedType(t *testing.T) {
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
		},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
			Type:          messagingv1alpha1.QueueType("model"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(q, conn).
		WithObjects(conn, q).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

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
	if updated.Status.DesiredMQSC != "" {
		t.Fatalf("DesiredMQSC should be empty on format error, got %q", updated.Status.DesiredMQSC)
	}
}

func TestQueueReconciler_DefineOnDrift(t *testing.T) {
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
	mockAdmin.EXPECT().DefineQueue(mock.Anything, mock.Anything).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestQueueManagerConnectionReconciler_TransientPingFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "qm1"}
	s := unitSchemeOrFatal(t)

	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "qm1",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.QueueManagerConnectionFinalizer},
		},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "mq-credentials",
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(conn).
		WithObjects(conn).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().Ping(mock.Anything).Return(&mqadmin.TransientError{Message: "timeout"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueManagerConnectionReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("transient reconcile should requeue without error, got result=%+v err=%v", result, err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("RequeueAfter = %v", result.RequeueAfter)
	}
}

func TestQueueManagerConnectionReconciler_SteadyStateTransientPingPreservesReady(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "qm1"}
	s := unitSchemeOrFatal(t)

	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "qm1",
			Namespace:  ns,
			Generation: 1,
			Finalizers: []string{messagingv1alpha1.QueueManagerConnectionFinalizer},
		},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "mq-credentials",
			},
		},
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			ObservedGeneration: 1,
			Conditions: []metav1.Condition{{
				Type:               messagingv1alpha1.ConditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             messagingv1alpha1.ReasonAvailable,
				LastTransitionTime: metav1.Now(),
			}},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(conn).
		WithObjects(conn).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().Ping(mock.Anything).Return(&mqadmin.TransientError{Message: "timeout"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &QueueManagerConnectionReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if result.RequeueAfter != TransientRequeueInterval() {
		t.Fatalf("RequeueAfter = %v", result.RequeueAfter)
	}

	updated := &messagingv1alpha1.QueueManagerConnection{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionReady) != metav1.ConditionTrue {
		t.Fatalf("Ready should stay True on transient ping, got %v", updated.Status.Conditions)
	}
}

func TestQueueReconciler_ReconcileNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cl := fake.NewClientBuilder().WithScheme(s).Build()
	rec := &QueueReconciler{Client: cl, Scheme: s}
	result, err := rec.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "mkurator-system", Name: "missing"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	_ = result
}

func TestQueueManagerConnectionReconciler_ReconcileNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cl := fake.NewClientBuilder().WithScheme(s).Build()
	rec := &QueueManagerConnectionReconciler{Client: cl, Scheme: s}
	result, err := rec.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "mkurator-system", Name: "missing"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	_ = result
}

func TestQueueReconciler_FirstPassAddsFinalizerWithoutSynced(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
			Type:          messagingv1alpha1.QueueTypeLocal,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(q, conn).
		WithObjects(conn, q).
		Build()

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mqadmintest.NewMockAdmin(t), nil).Once()

	rec := &QueueReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.Queue{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(updated, messagingv1alpha1.QueueFinalizer) {
		t.Fatal("expected finalizer added")
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != "" {
		t.Fatalf("expected empty Synced on first pass, got %v", updated.Status.Conditions)
	}
}

func readyConnForUnit(ns string) *messagingv1alpha1.QueueManagerConnection {
	return &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "mq-credentials",
			},
		},
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:               messagingv1alpha1.ConditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             messagingv1alpha1.ReasonAvailable,
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
}

func TestQueueReconciler_AdoptionConflict(t *testing.T) {
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
		},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS", Type: messagingv1alpha1.QueueTypeLocal,
			Attributes: map[string]string{"maxdepth": "5000"},
			WorkloadLifecyclePolicies: messagingv1alpha1.WorkloadLifecyclePolicies{
				AdoptionPolicy: messagingv1alpha1.AdoptionPolicyAdoptIfMatching,
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q, conn).WithObjects(conn, q).Build()
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().
		GetQueue(mock.Anything, mock.Anything).
		Return(&mqadmin.QueueState{Attributes: map[string]string{"maxdepth": "1000"}}, nil)
	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)
	rec := &QueueReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	updated := &messagingv1alpha1.Queue{}
	_ = cl.Get(ctx, key, updated)
	if conditionReason(
		updated.Status.Conditions,
		messagingv1alpha1.ConditionSynced,
	) != messagingv1alpha1.ReasonAdoptionConflict {
		t.Fatalf("reason = %q", conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced))
	}
}

func TestQueueReconciler_OrphanDeleteWithoutConnection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders"}
	s := unitSchemeOrFatal(t)
	now := metav1.Now()
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "orders",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.QueueFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "missing-qmc"},
			QueueName:     "APP.ORDERS", Type: messagingv1alpha1.QueueTypeLocal,
			WorkloadLifecyclePolicies: messagingv1alpha1.WorkloadLifecyclePolicies{
				DeletionPolicy: messagingv1alpha1.DeletionPolicyOrphan,
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	rec := &QueueReconciler{Client: cl, Scheme: s, MQFactory: mqadmintest.NewMockFactory(t)}
	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	updated := &messagingv1alpha1.Queue{}
	_ = cl.Get(ctx, key, updated)
	if controllerutil.ContainsFinalizer(updated, messagingv1alpha1.QueueFinalizer) {
		t.Fatal("finalizer should be removed")
	}
}
