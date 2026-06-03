package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/adapter/mqrest"
	"github.com/konih/kurator/internal/mqadmin"
	mqadmintest "github.com/konih/kurator/test/mocks/mqadmin"
)

func assertRecorderEventContains(t *testing.T, recorder *events.FakeRecorder, substr string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case ev := <-recorder.Events:
			if strings.Contains(ev, substr) {
				return
			}
		case <-deadline:
			t.Fatalf("expected event containing %q", substr)
		}
	}
}

func TestAuthorityRecordReconciler_DriftAppliesSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "app-orders-get-put",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.AuthorityRecordFinalizer},
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT", "INQ"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetAuthority(mock.Anything, spec).Return(&mqadmin.AuthorityState{
		Profile: spec.Profile, ObjectType: spec.ObjectType, Principal: spec.Principal,
		Authorities: []string{"GET", "PUT"},
	}, nil)
	mockAdmin.EXPECT().SetAuthority(mock.Anything, spec).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestAuthorityRecordReconciler_ObserveOnlyDrift(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "app-orders-get-put",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.AuthorityRecordFinalizer},
			Annotations: map[string]string{
				messagingv1alpha1.DriftPolicyAnnotation: messagingv1alpha1.DriftPolicyObserveOnly,
			},
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT", "INQ"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetAuthority(mock.Anything, spec).Return(&mqadmin.AuthorityState{
		Profile: spec.Profile, ObjectType: spec.ObjectType, Principal: spec.Principal,
		Authorities: []string{"GET", "PUT"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	updated := &messagingv1alpha1.AuthorityRecord{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) !=
		messagingv1alpha1.ReasonDriftDetected {
		t.Fatalf("reason = %q", conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced))
	}
}

func TestTopicReconciler_ObserveOnlyDrift(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "retail-orders"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "retail-orders",
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
		Attributes: map[string]string{"topstr": "other"},
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
	if conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) !=
		messagingv1alpha1.ReasonDriftDetected {
		t.Fatalf("reason = %q", conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced))
	}
}

func TestTopicReconciler_DeletionEmitsDeletedEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
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

	recorder := events.NewFakeRecorder(2)
	rec := &TopicReconciler{Client: cl, Scheme: s, MQFactory: mockFactory, Recorder: recorder}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertRecorderEventContains(t, recorder, EventReasonDeleted)
}

func TestQueueReconciler_DeletionEmitsDeletedEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
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

	recorder := events.NewFakeRecorder(2)
	rec := &QueueReconciler{Client: cl, Scheme: s, MQFactory: mockFactory, Recorder: recorder}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertRecorderEventContains(t, recorder, EventReasonDeleted)
}

func TestTopicReconciler_GetAPIError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cl := fake.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			return apierrors.NewForbidden(schema.GroupResource{Resource: "topics"}, "missing", errors.New("denied"))
		},
	}).Build()
	rec := &TopicReconciler{Client: cl, Scheme: s, MQFactory: mqadmintest.NewMockFactory(t)}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "missing"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthorityRecordReconciler_MissingConnection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "auth1"}
	s := unitSchemeOrFatal(t)

	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "auth1",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.AuthorityRecordFinalizer},
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "missing"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth).
		WithObjects(auth).
		Build()

	rec := &AuthorityRecordReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mqadmintest.NewMockFactory(t),
		Recorder:  events.NewFakeRecorder(1),
	}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	updated := &messagingv1alpha1.AuthorityRecord{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != EventReasonConnectionNotFound {
		t.Fatalf("reason = %q", conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced))
	}
}

func TestPatchSyncedProgressing_Topic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{Name: "t1", Namespace: "ns", Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(topic).WithObjects(topic).Build()
	if err := patchSyncedProgressing(ctx, cl.Status(), nil, topic, 1, "waiting"); err != nil {
		t.Fatalf("patchSyncedProgressing: %v", err)
	}
}

func TestQueueManagerConnectionReconciler_MissingSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "qm1"}
	s := unitSchemeOrFatal(t)

	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "missing-creds",
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(conn).
		WithObjects(conn).
		Build()

	rec := &QueueManagerConnectionReconciler{
		Client:    cl,
		Scheme:    s,
		MQFactory: mqrest.NewClientFactory(cl),
		Recorder:  events.NewFakeRecorder(2),
	}
	for range 2 {
		if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
	}
	updated := &messagingv1alpha1.QueueManagerConnection{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	msg := findConditionMessage(updated.Status.Conditions, messagingv1alpha1.ConditionReady)
	if !strings.Contains(msg, "credentials secret") {
		t.Fatalf("status message = %q", msg)
	}
}

func findConditionMessage(conditions []metav1.Condition, condType string) string {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Message
		}
	}
	return ""
}

func TestQueueManagerConnectionReconciler_GetError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cl := fake.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			return apierrors.NewForbidden(
				schema.GroupResource{Resource: "queuemanagerconnections"},
				"x",
				errors.New("denied"),
			)
		},
	}).Build()
	rec := &QueueManagerConnectionReconciler{Client: cl, Scheme: s}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "x"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChannelReconciler_InvalidMQSCSpec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "bad-channel"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "bad-channel",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelFinalizer},
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Type:          messagingv1alpha1.ChannelType("sender"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mqadmintest.NewMockAdmin(t), nil)

	rec := &ChannelReconciler{Client: cl, Scheme: s, MQFactory: mockFactory, Recorder: events.NewFakeRecorder(1)}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestResolveConnection_MissingObject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cl := fake.NewClientBuilder().WithScheme(s).Build()
	_, err := resolveConnection(ctx, cl, "ns", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "get connection") {
		t.Fatalf("err = %v", err)
	}
}

func TestQueueReconciler_ObserveOnlyDrift(t *testing.T) {
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
	if conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) !=
		messagingv1alpha1.ReasonDriftDetected {
		t.Fatalf("reason = %q", conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced))
	}
}

func TestChannelReconciler_DefineOnDrift(t *testing.T) {
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
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			Attributes:    map[string]string{"trptype": "tcp"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannel(mock.Anything, mock.Anything).Return(&mqadmin.ChannelState{
		Attributes: map[string]string{"trptype": "lu62"},
	}, nil)
	mockAdmin.EXPECT().DefineChannel(mock.Anything, mock.Anything).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestTopicReconciler_DefineOnDrift(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
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
		Attributes: map[string]string{"topstr": "other"},
	}, nil)
	mockAdmin.EXPECT().DefineTopic(mock.Anything, mock.Anything).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &TopicReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestChannelReconciler_DeletionEmitsDeletedEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
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
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(channel, conn).
		WithObjects(conn, channel).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteChannel(mock.Anything, mock.Anything).Return(nil)
	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	recorder := events.NewFakeRecorder(2)
	rec := &ChannelReconciler{Client: cl, Scheme: s, MQFactory: mockFactory, Recorder: recorder}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertRecorderEventContains(t, recorder, EventReasonDeleted)
}

func TestAuthorityRecordReconciler_DeletionEmitsDeletedEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	now := metav1.Now()
	conn := readyConnForUnit(ns)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "app-orders-get-put",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.AuthorityRecordFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteAuthority(mock.Anything, spec).Return(nil)
	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	recorder := events.NewFakeRecorder(2)
	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory, Recorder: recorder}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertRecorderEventContains(t, recorder, EventReasonDeleted)
}

func TestResolveConnection_APIError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cl := fake.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			return apierrors.NewForbidden(
				schema.GroupResource{Resource: "queuemanagerconnections"},
				"qm1",
				errors.New("denied"),
			)
		},
	}).Build()
	_, err := resolveConnection(ctx, cl, "ns", "qm1")
	if err == nil {
		t.Fatal("expected error")
	}
}
