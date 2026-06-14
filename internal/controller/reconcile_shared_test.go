package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
	mqadmintest "github.com/conduit-ops/mkurator/test/mocks/mqadmin"
)

func allWorkloadKinds(ns string, generation int64) []client.Object {
	return []client.Object{
		&messagingv1alpha1.Queue{ObjectMeta: metav1.ObjectMeta{Name: "q1", Namespace: ns, Generation: generation}},
		&messagingv1alpha1.Topic{ObjectMeta: metav1.ObjectMeta{Name: "t1", Namespace: ns, Generation: generation}},
		&messagingv1alpha1.Channel{ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: ns, Generation: generation}},
		&messagingv1alpha1.ChannelAuthRule{
			ObjectMeta: metav1.ObjectMeta{Name: "car1", Namespace: ns, Generation: generation},
		},
		&messagingv1alpha1.AuthorityRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "auth1", Namespace: ns, Generation: generation},
		},
	}
}

type patchedStatusExpect struct {
	syncedStatus      metav1.ConditionStatus
	syncedReason      string
	message           string
	observedGen       int64
	wantLastSync      bool
	mqObjectExists    *bool
	statusObservedGen int64
}

func unitConditionStatus(conditions []metav1.Condition, condType string) metav1.ConditionStatus {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Status
		}
	}
	return ""
}

func unitConditionReason(conditions []metav1.Condition, condType string) string {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Reason
		}
	}
	return ""
}

func unitConditionObservedGeneration(conditions []metav1.Condition, condType string) int64 {
	for _, c := range conditions {
		if c.Type == condType {
			return c.ObservedGeneration
		}
	}
	return 0
}

func workloadStatusFields(obj client.Object) (
	message string,
	lastSync *metav1.Time,
	mqExists *bool,
	observedGen int64,
) {
	switch o := obj.(type) {
	case *messagingv1alpha1.Queue:
		return o.Status.Message, o.Status.LastSyncTime, o.Status.MQObjectExists, o.Status.ObservedGeneration
	case *messagingv1alpha1.Topic:
		return o.Status.Message, o.Status.LastSyncTime, o.Status.MQObjectExists, o.Status.ObservedGeneration
	case *messagingv1alpha1.Channel:
		return o.Status.Message, o.Status.LastSyncTime, o.Status.MQObjectExists, o.Status.ObservedGeneration
	case *messagingv1alpha1.ChannelAuthRule:
		return o.Status.Message, o.Status.LastSyncTime, o.Status.MQObjectExists, o.Status.ObservedGeneration
	case *messagingv1alpha1.AuthorityRecord:
		return o.Status.Message, o.Status.LastSyncTime, o.Status.MQObjectExists, o.Status.ObservedGeneration
	default:
		return "", nil, nil, 0
	}
}

func rereadWorkload(ctx context.Context, t *testing.T, cl client.Client, obj client.Object) client.Object {
	t.Helper()
	key := client.ObjectKeyFromObject(obj)
	switch obj.(type) {
	case *messagingv1alpha1.Queue:
		got := &messagingv1alpha1.Queue{}
		if err := cl.Get(ctx, key, got); err != nil {
			t.Fatal(err)
		}
		return got
	case *messagingv1alpha1.Topic:
		got := &messagingv1alpha1.Topic{}
		if err := cl.Get(ctx, key, got); err != nil {
			t.Fatal(err)
		}
		return got
	case *messagingv1alpha1.Channel:
		got := &messagingv1alpha1.Channel{}
		if err := cl.Get(ctx, key, got); err != nil {
			t.Fatal(err)
		}
		return got
	case *messagingv1alpha1.ChannelAuthRule:
		got := &messagingv1alpha1.ChannelAuthRule{}
		if err := cl.Get(ctx, key, got); err != nil {
			t.Fatal(err)
		}
		return got
	case *messagingv1alpha1.AuthorityRecord:
		got := &messagingv1alpha1.AuthorityRecord{}
		if err := cl.Get(ctx, key, got); err != nil {
			t.Fatal(err)
		}
		return got
	default:
		t.Fatalf("unsupported type %T", obj)
		return nil
	}
}

func assertPatchedWorkloadStatus(t *testing.T, obj client.Object, want patchedStatusExpect) {
	t.Helper()
	conds := syncedConditions(obj)
	if status := unitConditionStatus(conds, messagingv1alpha1.ConditionSynced); status != want.syncedStatus {
		t.Fatalf("%T Synced status = %q, want %q", obj, status, want.syncedStatus)
	}
	if reason := unitConditionReason(conds, messagingv1alpha1.ConditionSynced); reason != want.syncedReason {
		t.Fatalf("%T Synced reason = %q, want %q", obj, reason, want.syncedReason)
	}
	if gen := unitConditionObservedGeneration(conds, messagingv1alpha1.ConditionSynced); gen != want.observedGen {
		t.Fatalf("%T condition ObservedGeneration = %d, want %d", obj, gen, want.observedGen)
	}
	msg, lastSync, mqExists, statusGen := workloadStatusFields(obj)
	if msg != want.message {
		t.Fatalf("%T message = %q, want %q", obj, msg, want.message)
	}
	if want.wantLastSync && lastSync == nil {
		t.Fatalf("%T LastSyncTime nil, want set", obj)
	}
	if !want.wantLastSync && lastSync != nil {
		t.Fatalf("%T LastSyncTime = %v, want nil", obj, lastSync)
	}
	if want.mqObjectExists != nil {
		if mqExists == nil || *mqExists != *want.mqObjectExists {
			t.Fatalf("%T mqObjectExists = %v, want %v", obj, mqExists, want.mqObjectExists)
		}
	}
	if want.statusObservedGen != 0 && statusGen != want.statusObservedGen {
		t.Fatalf("%T Status.ObservedGeneration = %d, want %d", obj, statusGen, want.statusObservedGen)
	}
}

func TestRequestsForConnection_EnqueuesDependents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
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
	car := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{Name: "car1", Namespace: ns},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
		},
	}
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auth1", Namespace: ns},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(conn, queue, topic, channel, car, auth).Build()
	reqs := requestsForConnection(ctx, cl, conn)
	if len(reqs) != 5 {
		t.Fatalf("requests = %d, want 5", len(reqs))
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

func TestWaitForConnectionReady_Requeues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:    messagingv1alpha1.ConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  messagingv1alpha1.ReasonError,
				Message: "credentials secret not found",
			}},
		},
	}
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	recorder := events.NewFakeRecorder(1)
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
	if !strings.Contains(updated.Status.Message, "credentials secret not found") {
		t.Fatalf("status.message = %q", updated.Status.Message)
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

func TestWaitForConnectionReady_AlreadyReady(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	conn := readyConnForUnit(ns)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1},
	}
	result, wait, err := waitForConnectionReady(ctx, nil, nil, q, conn, 1)
	if err != nil || wait || result != (ctrl.Result{}) {
		t.Fatalf("result=%+v wait=%v err=%v", result, wait, err)
	}
}

func TestPatchSyncedAvailable_AllKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	const generation int64 = 2
	const message = "synced"
	exists := true
	opts := syncStatusOpts{mqObjectExists: &exists}
	want := patchedStatusExpect{
		syncedStatus:      metav1.ConditionTrue,
		syncedReason:      messagingv1alpha1.ReasonAvailable,
		message:           message,
		observedGen:       generation,
		wantLastSync:      true,
		mqObjectExists:    &exists,
		statusObservedGen: generation,
	}

	for _, obj := range allWorkloadKinds(ns, generation) {
		t.Run(client.ObjectKeyFromObject(obj).Name, func(t *testing.T) {
			t.Parallel()
			cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(obj).WithObjects(obj).Build()
			recorder := events.NewFakeRecorder(1)
			if err := patchSyncedAvailable(ctx, cl.Status(), recorder, obj, generation, message, opts); err != nil {
				t.Fatal(err)
			}
			got := rereadWorkload(ctx, t, cl, obj)
			assertPatchedWorkloadStatus(t, got, want)
			select {
			case ev := <-recorder.Events:
				if !strings.Contains(ev, corev1.EventTypeNormal) ||
					!strings.Contains(ev, messagingv1alpha1.ReasonAvailable) {
					t.Fatalf("event = %q", ev)
				}
			case <-time.After(time.Second):
				t.Fatal("expected available event")
			}
		})
	}
}

func TestPatchSyncedProgressing_AllKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	const generation int64 = 1
	const message = "waiting"
	want := patchedStatusExpect{
		syncedStatus: metav1.ConditionFalse,
		syncedReason: messagingv1alpha1.ReasonProgressing,
		message:      message,
		observedGen:  generation,
	}

	for _, obj := range allWorkloadKinds(ns, generation) {
		t.Run(client.ObjectKeyFromObject(obj).Name, func(t *testing.T) {
			t.Parallel()
			cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(obj).WithObjects(obj).Build()
			if err := patchSyncedProgressing(ctx, cl.Status(), nil, obj, generation, message); err != nil {
				t.Fatal(err)
			}
			got := rereadWorkload(ctx, t, cl, obj)
			assertPatchedWorkloadStatus(t, got, want)
		})
	}
}

func TestPatchSyncedDeleting_AllKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	const generation int64 = 1
	const message = "deleting"
	want := patchedStatusExpect{
		syncedStatus: metav1.ConditionFalse,
		syncedReason: messagingv1alpha1.ReasonDeleting,
		message:      message,
		observedGen:  generation,
	}

	for _, obj := range allWorkloadKinds(ns, generation) {
		t.Run(client.ObjectKeyFromObject(obj).Name, func(t *testing.T) {
			t.Parallel()
			cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(obj).WithObjects(obj).Build()
			if err := patchSyncedDeleting(ctx, cl.Status(), nil, obj, generation, message); err != nil {
				t.Fatal(err)
			}
			got := rereadWorkload(ctx, t, cl, obj)
			assertPatchedWorkloadStatus(t, got, want)
		})
	}
}

func TestSetSyncedError_TerminalQueue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	recorder := events.NewFakeRecorder(1)
	result, err := setSyncedError(
		ctx,
		cl.Status(),
		recorder,
		q,
		1,
		&mqadmin.TerminalError{Reason: "MQSCError", Message: "bad"},
		syncStatusOpts{},
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

func TestConnectionWatchPredicates(t *testing.T) {
	t.Parallel()
	pred := connectionWatchPredicates()
	ready := readyConnForUnit("ns")
	notReady := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: "ns"},
	}

	if !pred.Create(event.CreateEvent{Object: ready}) {
		t.Fatal("expected create for ready connection")
	}
	if pred.Create(event.CreateEvent{Object: notReady}) {
		t.Fatal("expected create skip when connection not ready")
	}
	if pred.Create(event.CreateEvent{Object: &messagingv1alpha1.Queue{}}) {
		t.Fatal("expected create skip for non-connection object")
	}

	old := notReady.DeepCopy()
	newReady := ready.DeepCopy()
	newReady.Generation = 2
	if !pred.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: newReady}) {
		t.Fatal("expected update on ready transition")
	}
	prev := ready.DeepCopy()
	prev.Generation = 2
	next := ready.DeepCopy()
	next.Generation = 3
	if !pred.Update(event.UpdateEvent{ObjectOld: prev, ObjectNew: next}) {
		t.Fatal("expected update on generation change")
	}
	if pred.Update(event.UpdateEvent{ObjectOld: &messagingv1alpha1.Queue{}, ObjectNew: next}) {
		t.Fatal("expected update skip for wrong types")
	}
}

func TestWatchConnectionStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)

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
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(conn, queue).Build()

	mapFn := connectionEnqueueMapper(cl)
	reqs := mapFn(ctx, conn)
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	if reqs := mapFn(ctx, queue); len(reqs) != 0 {
		t.Fatalf("non-connection object should yield no requests, got %d", len(reqs))
	}
	_ = watchConnectionStatus(cl)
}

func TestAppendDependentsOrLog_ListError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := runtime.NewScheme()
	cl := fake.NewClientBuilder().WithScheme(s).Build()
	reqs := appendDependentsOrLog(ctx, cl, logr.Discard(), "ns", "qm1", "Queue",
		func() *messagingv1alpha1.QueueList { return &messagingv1alpha1.QueueList{} },
		func(l *messagingv1alpha1.QueueList) []*messagingv1alpha1.Queue { return nil },
		nil,
	)
	if len(reqs) != 0 {
		t.Fatalf("requests = %d, want 0 when list fails", len(reqs))
	}
}

func TestMQObjectFrom_Unsupported(t *testing.T) {
	t.Parallel()
	_, err := mqObjectFrom(&messagingv1alpha1.QueueManagerConnection{})
	if err == nil {
		t.Fatal("expected error for non-workload type")
	}
}

func TestSetSyncedError_UnsupportedType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: "ns"},
	}
	_, err := setSyncedError(ctx, nil, nil, conn, 1, fmt.Errorf("fail"), syncStatusOpts{})
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestConnectionRefName_Unsupported(t *testing.T) {
	t.Parallel()
	if _, err := connectionRefName(&messagingv1alpha1.QueueManagerConnection{}); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestSetSyncedError_TransientChannel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	ch := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(ch).WithObjects(ch).Build()
	result, err := setSyncedError(
		ctx, cl.Status(), nil, ch, 1, &mqadmin.TransientError{Message: "timeout"}, syncStatusOpts{},
	)
	if err != nil || result.RequeueAfter != 30*time.Second {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPatchSyncedDrift_AllKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	const generation int64 = 1
	const message = "drift"
	exists := true
	opts := syncStatusOpts{mqObjectExists: &exists}
	want := patchedStatusExpect{
		syncedStatus:   metav1.ConditionFalse,
		syncedReason:   messagingv1alpha1.ReasonDriftDetected,
		message:        message,
		observedGen:    generation,
		mqObjectExists: &exists,
	}

	for _, obj := range allWorkloadKinds(ns, generation) {
		t.Run(client.ObjectKeyFromObject(obj).Name, func(t *testing.T) {
			t.Parallel()
			cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(obj).WithObjects(obj).Build()
			if err := patchSyncedDrift(ctx, cl.Status(), nil, obj, generation, message, opts); err != nil {
				t.Fatal(err)
			}
			got := rereadWorkload(ctx, t, cl, obj)
			assertPatchedWorkloadStatus(t, got, want)
		})
	}
}

func TestPatchSyncedDrift_UnsupportedType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: "ns"},
	}
	if err := patchSyncedDrift(ctx, nil, nil, conn, 1, "drift", syncStatusOpts{}); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestSetSyncedError_AuthorityRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auth1", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(auth).WithObjects(auth).Build()
	_, err := setSyncedError(
		ctx,
		cl.Status(),
		nil,
		auth,
		1,
		&mqadmin.TerminalError{Message: "denied"},
		syncStatusOpts{},
	)
	if err != nil {
		t.Fatalf("setSyncedError: %v", err)
	}
}

func TestPatchSyncedOrphaned_AllKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	const generation int64 = 1
	const message = "orphaned"
	want := patchedStatusExpect{
		syncedStatus: metav1.ConditionFalse,
		syncedReason: messagingv1alpha1.ReasonOrphaned,
		message:      message,
		observedGen:  generation,
	}

	for _, obj := range allWorkloadKinds(ns, generation) {
		t.Run(client.ObjectKeyFromObject(obj).Name, func(t *testing.T) {
			t.Parallel()
			cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(obj).WithObjects(obj).Build()
			if err := patchSyncedOrphaned(ctx, cl.Status(), nil, obj, generation, message); err != nil {
				t.Fatal(err)
			}
			got := rereadWorkload(ctx, t, cl, obj)
			assertPatchedWorkloadStatus(t, got, want)
		})
	}
}

func TestSyncedConditions_AuthTypes(t *testing.T) {
	t.Parallel()
	car := &messagingv1alpha1.ChannelAuthRule{Status: messagingv1alpha1.ChannelAuthRuleStatus{
		Conditions: []metav1.Condition{{Type: messagingv1alpha1.ConditionSynced, Status: metav1.ConditionTrue}},
	}}
	if len(syncedConditions(car)) != 1 {
		t.Fatal("expected channel auth rule conditions")
	}
	auth := &messagingv1alpha1.AuthorityRecord{Status: messagingv1alpha1.AuthorityRecordStatus{
		Conditions: []metav1.Condition{{Type: messagingv1alpha1.ConditionSynced, Status: metav1.ConditionTrue}},
	}}
	if len(syncedConditions(auth)) != 1 {
		t.Fatal("expected authority record conditions")
	}
}

func TestConnectionRefName_AuthTypes(t *testing.T) {
	t.Parallel()
	car := &messagingv1alpha1.ChannelAuthRule{
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
		},
	}
	name, err := connectionRefName(car)
	if err != nil || name != "qm1" {
		t.Fatalf("car name=%q err=%v", name, err)
	}
	auth := &messagingv1alpha1.AuthorityRecord{
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm2"},
		},
	}
	name, err = connectionRefName(auth)
	if err != nil || name != "qm2" {
		t.Fatalf("auth name=%q err=%v", name, err)
	}
}

func TestForceOrphanRequested(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{}
	if forceOrphanRequested(q) {
		t.Fatal("expected false without annotation")
	}
	q.Annotations = map[string]string{messagingv1alpha1.ForceOrphanAnnotation: "true"}
	if !forceOrphanRequested(q) {
		t.Fatal("expected true with force-orphan annotation")
	}
}

func TestDeletionAwaitingConnection_Requeues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	notFound := apierrors.NewNotFound(
		schema.GroupResource{Group: "messaging.mkurator.dev", Resource: "queuemanagerconnections"},
		"qm1",
	)
	result, err := deletionAwaitingConnection(ctx, cl.Status(), nil, q, 1, notFound)
	if err != nil || result.RequeueAfter != 15*time.Second {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestOrphanFinalizeWorkload_AllKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	type tc struct {
		obj       client.Object
		finalizer string
	}
	cases := []tc{
		{
			&messagingv1alpha1.Queue{ObjectMeta: metav1.ObjectMeta{
				Name: "q1", Namespace: ns, Finalizers: []string{messagingv1alpha1.QueueFinalizer},
			}},
			messagingv1alpha1.QueueFinalizer,
		},
		{
			&messagingv1alpha1.Topic{ObjectMeta: metav1.ObjectMeta{
				Name: "t1", Namespace: ns, Finalizers: []string{messagingv1alpha1.TopicFinalizer},
			}},
			messagingv1alpha1.TopicFinalizer,
		},
		{
			&messagingv1alpha1.Channel{ObjectMeta: metav1.ObjectMeta{
				Name: "c1", Namespace: ns, Finalizers: []string{messagingv1alpha1.ChannelFinalizer},
			}},
			messagingv1alpha1.ChannelFinalizer,
		},
		{
			&messagingv1alpha1.ChannelAuthRule{ObjectMeta: metav1.ObjectMeta{
				Name: "car1", Namespace: ns, Finalizers: []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
			}},
			messagingv1alpha1.ChannelAuthRuleFinalizer,
		},
		{
			&messagingv1alpha1.AuthorityRecord{ObjectMeta: metav1.ObjectMeta{
				Name: "auth1", Namespace: ns, Finalizers: []string{messagingv1alpha1.AuthorityRecordFinalizer},
			}},
			messagingv1alpha1.AuthorityRecordFinalizer,
		},
	}
	for _, c := range cases {
		cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(c.obj).WithObjects(c.obj).Build()
		result, err := orphanFinalizeWorkload(ctx, cl, cl.Status(), nil, c.obj, 1, c.finalizer, "orphaned")
		if err != nil || result != (ctrl.Result{}) {
			t.Fatalf("%T: result=%+v err=%v", c.obj, result, err)
		}
	}
}

func TestReconcileWorkloadDeletion_NoFinalizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "ns"},
	}
	result, err := reconcileWorkloadDeletion(
		ctx, fake.NewClientBuilder().WithScheme(unitSchemeOrFatal(t)).WithObjects(q).Build(),
		nil, nil, mqadmintest.NewMockFactory(t), q, 1, messagingv1alpha1.QueueFinalizer, "orphaned",
		func(context.Context, mqadmin.Admin) (ctrl.Result, error) {
			t.Fatal("deleteFn should not run without finalizer")
			return ctrl.Result{}, nil
		},
	)
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestOrphanFinalizeWorkload_Queue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "orders",
			Namespace:  ns,
			Generation: 1,
			Finalizers: []string{messagingv1alpha1.QueueFinalizer},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	result, err := orphanFinalizeWorkload(
		ctx, cl, cl.Status(), nil, q, 1, messagingv1alpha1.QueueFinalizer, "orphaned",
	)
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	got := &messagingv1alpha1.Queue{}
	if getErr := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: "orders"}, got); getErr != nil {
		t.Fatal(getErr)
	}
	if len(got.Finalizers) != 0 {
		t.Fatalf("finalizers = %v, want none", got.Finalizers)
	}
}

func TestSetSyncedError_TransientAuthorityRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := unitSchemeOrFatal(t)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auth1", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(auth).WithObjects(auth).Build()
	result, err := setSyncedError(
		ctx, cl.Status(), nil, auth, 1, &mqadmin.TransientError{Message: "timeout"}, syncStatusOpts{},
	)
	if err != nil || result.RequeueAfter != 30*time.Second {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
