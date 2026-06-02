package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/mqadmin"
)

func TestRequestsForConnection_EnqueuesDependents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
	}
	queue := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
		},
	}
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{Name: "retail", Namespace: ns},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
		},
	}
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(conn, queue, topic, channel).Build()
	reqs := requestsForConnection(ctx, cl, conn)
	if len(reqs) != 3 {
		t.Fatalf("requests = %d, want 3", len(reqs))
	}
}

func TestConnectionRefName(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
		},
	}
	name, err := connectionRefName(q)
	if err != nil || name != "qm1" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	topic := &messagingv1alpha1.Topic{
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm2"},
		},
	}
	name, err = connectionRefName(topic)
	if err != nil || name != "qm2" {
		t.Fatalf("topic name=%q err=%v", name, err)
	}
	ch := &messagingv1alpha1.Channel{
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm3"},
		},
	}
	name, err = connectionRefName(ch)
	if err != nil || name != "qm3" {
		t.Fatalf("channel name=%q err=%v", name, err)
	}
}

func TestConnectionReadyChanged(t *testing.T) {
	t.Parallel()
	old := &messagingv1alpha1.QueueManagerConnection{}
	newReady := &messagingv1alpha1.QueueManagerConnection{
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:   messagingv1alpha1.ConditionReady,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	if !connectionReadyChanged(old, newReady) {
		t.Fatal("expected ready transition")
	}
}

func TestResolveConnection_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(s).Build()
	_, err := resolveConnection(ctx, cl, "default", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIgnoreNotFound(t *testing.T) {
	t.Parallel()
	if !ignoreNotFound(apierrors.NewNotFound(schema.GroupResource{}, "x")) {
		t.Fatal("expected true for NotFound")
	}
	if ignoreNotFound(errors.New("other")) {
		t.Fatal("expected false")
	}
}

func TestWaitForConnectionReady_Requeues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	s := unitSchemeOrFatal(t)
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
	}
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	recorder := record.NewFakeRecorder(1)
	result, wait, err := waitForConnectionReady(ctx, cl.Status(), recorder, q, conn, 1)
	if err != nil || !wait || result.RequeueAfter != 15*time.Second {
		t.Fatalf("result=%+v wait=%v err=%v", result, wait, err)
	}
	updated := &messagingv1alpha1.Queue{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(q), updated); err != nil {
		t.Fatal(err)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != metav1.ConditionFalse {
		t.Fatalf("conditions = %v", updated.Status.Conditions)
	}
	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, corev1.EventTypeNormal) || !strings.Contains(ev, messagingv1alpha1.ReasonProgressing) {
			t.Fatalf("event = %q", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("expected progressing event")
	}
}

func TestPatchSyncedAvailable_Queue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	s := unitSchemeOrFatal(t)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 2},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	recorder := record.NewFakeRecorder(1)
	if err := patchSyncedAvailable(ctx, cl.Status(), recorder, q, 2, "synced"); err != nil {
		t.Fatal(err)
	}
	updated := &messagingv1alpha1.Queue{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(q), updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.ObservedGeneration != 2 {
		t.Fatalf("ObservedGeneration = %d", updated.Status.ObservedGeneration)
	}
	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, corev1.EventTypeNormal) || !strings.Contains(ev, messagingv1alpha1.ReasonAvailable) {
			t.Fatalf("event = %q", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("expected available event")
	}
}

func TestPatchSyncedProgressing_Channel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	s := unitSchemeOrFatal(t)
	ch := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(ch).WithObjects(ch).Build()
	if err := patchSyncedProgressing(ctx, cl.Status(), nil, ch, 1, "waiting"); err != nil {
		t.Fatal(err)
	}
}

func TestPatchSyncedDeleting_Topic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	s := unitSchemeOrFatal(t)
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{Name: "retail", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(topic).WithObjects(topic).Build()
	if err := patchSyncedDeleting(ctx, cl.Status(), nil, topic, 1, "deleting"); err != nil {
		t.Fatal(err)
	}
}

func TestSetSyncedError_TerminalQueue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	s := unitSchemeOrFatal(t)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	recorder := record.NewFakeRecorder(1)
	result, err := setSyncedError(
		ctx,
		cl.Status(),
		recorder,
		q,
		1,
		&mqadmin.TerminalError{Reason: "MQSCError", Message: "bad"},
	)
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, corev1.EventTypeWarning) || !strings.Contains(ev, "MQSCError") {
			t.Fatalf("event = %q", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("expected warning event")
	}
}

func TestSetSyncedError_TransientChannel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	s := unitSchemeOrFatal(t)
	ch := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(ch).WithObjects(ch).Build()
	result, err := setSyncedError(ctx, cl.Status(), nil, ch, 1, &mqadmin.TransientError{Message: "timeout"})
	if !errors.Is(err, mqadmin.ErrTransient) || result.RequeueAfter != 30*time.Second {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
