package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
	mqadmintest "github.com/conduit-ops/mkurator/test/mocks/mqadmin"
)

func TestToMQTopicSpec(t *testing.T) {
	t.Parallel()
	topic := &messagingv1alpha1.Topic{
		Spec: messagingv1alpha1.TopicSpec{
			TopicName: "RETAIL.ORDERS",
			Attributes: map[string]string{
				"TopStr": "retail/orders",
			},
		},
	}
	spec := toMQTopicSpec(topic)
	if spec.Name != "RETAIL.ORDERS" || spec.Attributes["topstr"] != "retail/orders" {
		t.Fatalf("spec = %+v", spec)
	}
}

func TestTopicReconciler_SyncedWithoutDefine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "retail-orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "retail-orders",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.TopicFinalizer},
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

	rec := &TopicReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertDriftResyncRequeue(t, result)

	updated := &messagingv1alpha1.Topic{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != metav1.ConditionTrue {
		t.Fatalf("Synced = %v", updated.Status.Conditions)
	}
}

func TestTopicReconciler_SetsDesiredMQSCInStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "retail-orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "retail-orders",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.TopicFinalizer},
		},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
			Attributes:    map[string]string{"topstr": "retail/orders", "descr": "orders topic"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(topic, conn).
		WithObjects(conn, topic).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetTopic(mock.Anything, "RETAIL.ORDERS").Return(&mqadmin.TopicState{
		Attributes: map[string]string{"topstr": "retail/orders", "descr": "orders topic"},
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
	want := "DEFINE TOPIC('RETAIL.ORDERS') REPLACE DESCR('orders topic') TOPICSTR('retail/orders')"
	if updated.Status.DesiredMQSC != want {
		t.Fatalf("DesiredMQSC = %q, want %q", updated.Status.DesiredMQSC, want)
	}
}

func TestToMQChannelSpec(t *testing.T) {
	t.Parallel()
	channel := &messagingv1alpha1.Channel{
		Spec: messagingv1alpha1.ChannelSpec{
			ChannelName: "ORDERS.APP",
			Type:        messagingv1alpha1.ChannelTypeSvrconn,
			Attributes: map[string]string{
				"MaxMsgl": "4194304",
			},
		},
	}
	spec := toMQChannelSpec(channel)
	if spec.Name != "ORDERS.APP" || spec.Type != mqadmin.ChannelTypeSvrconn {
		t.Fatalf("spec = %+v", spec)
	}
	if spec.Attributes["maxmsgl"] != "4194304" {
		t.Fatalf("attrs = %v", spec.Attributes)
	}
}

func TestChannelReconciler_SyncedWithoutDefine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders-app"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	spec := mqadmin.ChannelSpec{
		Name:       "ORDERS.APP",
		Type:       mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{"trptype": "tcp"},
	}
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "orders-app",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelFinalizer},
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Type:          messagingv1alpha1.ChannelTypeSvrconn,
			Attributes:    map[string]string{"trptype": "tcp"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannel(mock.Anything, spec).Return(&mqadmin.ChannelState{
		Attributes: map[string]string{"trptype": "tcp"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mockFactory,
	}

	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertDriftResyncRequeue(t, result)

	updated := &messagingv1alpha1.Channel{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != metav1.ConditionTrue {
		t.Fatalf("Synced = %v", updated.Status.Conditions)
	}
}

func TestChannelReconciler_SetsDesiredMQSCInStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders-app"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	spec := mqadmin.ChannelSpec{
		Name:       "ORDERS.APP",
		Type:       mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{"trptype": "tcp", "descr": "app channel"},
	}
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "orders-app",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelFinalizer},
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Type:          messagingv1alpha1.ChannelTypeSvrconn,
			Attributes:    map[string]string{"trptype": "tcp", "descr": "app channel"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannel(mock.Anything, spec).Return(&mqadmin.ChannelState{
		Attributes: map[string]string{"trptype": "tcp", "descr": "app channel"},
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
	want := "DEFINE CHANNEL('ORDERS.APP') REPLACE CHLTYPE('svrconn') DESCR('app channel') TRPTYPE('tcp')"
	if updated.Status.DesiredMQSC != want {
		t.Fatalf("DesiredMQSC = %q, want %q", updated.Status.DesiredMQSC, want)
	}
}

func TestTopicReconciler_DefinesWhenMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "retail-orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "retail-orders",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.TopicFinalizer},
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

	spec := toMQTopicSpec(topic)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetTopic(mock.Anything, "RETAIL.ORDERS").Return(nil, mqadmin.ErrNotFound).Once()
	mockAdmin.EXPECT().DefineTopic(mock.Anything, spec).Return(nil).Once()

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &TopicReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestTopicReconciler_DeletionDeleteFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "retail-orders"}
	s := unitSchemeOrFatal(t)

	now := metav1.Now()
	conn := readyConnForUnit(ns)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "retail-orders",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.TopicFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(topic, conn).
		WithObjects(conn, topic).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().
		DeleteTopic(mock.Anything, "RETAIL.ORDERS").
		Return(&mqadmin.TerminalError{Message: "delete denied"})

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
	if len(updated.Finalizers) == 0 {
		t.Fatal("finalizer should remain when delete fails")
	}
}

func TestTopicReconciler_Deletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "retail-orders"}
	s := unitSchemeOrFatal(t)

	now := metav1.Now()
	conn := readyConnForUnit(ns)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "retail-orders",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.TopicFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(topic, conn).
		WithObjects(conn, topic).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteTopic(mock.Anything, "RETAIL.ORDERS").Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &TopicReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.Topic{}
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

func TestChannelReconciler_DefinesWhenMissing(t *testing.T) {
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
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Type:          messagingv1alpha1.ChannelTypeSvrconn,
			Attributes:    map[string]string{"trptype": "tcp"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	spec := toMQChannelSpec(channel)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannel(mock.Anything, spec).Return(nil, mqadmin.ErrNotFound).Once()
	mockAdmin.EXPECT().DefineChannel(mock.Anything, spec).Return(nil).Once()

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestChannelReconciler_DeletionDeleteFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders-app"}
	s := unitSchemeOrFatal(t)

	now := metav1.Now()
	conn := readyConnForUnit(ns)
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "orders-app",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.ChannelFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Type:          messagingv1alpha1.ChannelTypeSvrconn,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	spec := toMQChannelSpec(channel)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteChannel(mock.Anything, spec).Return(&mqadmin.TerminalError{Message: "delete denied"})

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
	if len(updated.Finalizers) == 0 {
		t.Fatal("finalizer should remain when delete fails")
	}
}

func TestChannelReconciler_Deletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "orders-app"}
	s := unitSchemeOrFatal(t)

	now := metav1.Now()
	conn := readyConnForUnit(ns)
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "orders-app",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.ChannelFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Type:          messagingv1alpha1.ChannelTypeSvrconn,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	spec := toMQChannelSpec(channel)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteChannel(mock.Anything, spec).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.Channel{}
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

func TestTopicReconciler_TransientError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "retail-orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "retail-orders",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.TopicFinalizer},
		},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(topic, conn).
		WithObjects(conn, topic).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetTopic(mock.Anything, "RETAIL.ORDERS").Return(nil, &mqadmin.TransientError{Message: "timeout"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &TopicReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("transient reconcile should requeue without error, got result=%+v err=%v", result, err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("RequeueAfter = %v", result.RequeueAfter)
	}
}

func TestTopicReconciler_ReconcileNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cl := fake.NewClientBuilder().WithScheme(s).Build()
	rec := &TopicReconciler{Client: cl, Scheme: s}
	result, err := rec.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "mkurator-system", Name: "missing"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	_ = result
}

func TestChannelReconciler_ReconcileNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cl := fake.NewClientBuilder().WithScheme(s).Build()
	rec := &ChannelReconciler{Client: cl, Scheme: s}
	result, err := rec.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "mkurator-system", Name: "missing"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	_ = result
}

func TestChannelReconciler_TransientError(t *testing.T) {
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
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Type:          messagingv1alpha1.ChannelTypeSvrconn,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	spec := toMQChannelSpec(channel)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannel(mock.Anything, spec).Return(nil, &mqadmin.TransientError{Message: "timeout"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("transient reconcile should requeue without error, got result=%+v err=%v", result, err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("RequeueAfter = %v", result.RequeueAfter)
	}
}
