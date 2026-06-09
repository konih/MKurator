package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	messagingv1alpha1 "github.com/konih/mkurator/api/v1alpha1"
)

func TestWorkloadSuspended(t *testing.T) {
	t.Parallel()
	cases := []client.Object{
		&messagingv1alpha1.Queue{Spec: messagingv1alpha1.QueueSpec{Suspend: true}},
		&messagingv1alpha1.Topic{Spec: messagingv1alpha1.TopicSpec{Suspend: true}},
		&messagingv1alpha1.Channel{Spec: messagingv1alpha1.ChannelSpec{Suspend: true}},
		&messagingv1alpha1.ChannelAuthRule{Spec: messagingv1alpha1.ChannelAuthRuleSpec{Suspend: true}},
		&messagingv1alpha1.AuthorityRecord{Spec: messagingv1alpha1.AuthorityRecordSpec{Suspend: true}},
	}
	for _, obj := range cases {
		if !workloadSuspended(obj) {
			t.Fatalf("expected suspended for %T", obj)
		}
	}
	if workloadSuspended(&messagingv1alpha1.QueueManagerConnection{}) {
		t.Fatal("expected unsupported type not suspended")
	}
}

func TestPatchSyncedSuspended_AllKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	objects := []client.Object{
		&messagingv1alpha1.Queue{ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1}},
		&messagingv1alpha1.Topic{ObjectMeta: metav1.ObjectMeta{Name: "retail", Namespace: ns, Generation: 1}},
		&messagingv1alpha1.Channel{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, Generation: 1}},
		&messagingv1alpha1.ChannelAuthRule{ObjectMeta: metav1.ObjectMeta{Name: "car1", Namespace: ns, Generation: 1}},
		&messagingv1alpha1.AuthorityRecord{ObjectMeta: metav1.ObjectMeta{Name: "auth1", Namespace: ns, Generation: 1}},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(objects...).WithObjects(objects...).Build()
	recorder := events.NewFakeRecorder(len(objects))

	for _, obj := range objects {
		if err := patchSyncedSuspended(ctx, cl.Status(), recorder, obj, 1, suspendedMessage); err != nil {
			t.Fatalf("patch %T: %v", obj, err)
		}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			t.Fatalf("get %T: %v", obj, err)
		}
		if conditionReason(syncedConditions(obj), messagingv1alpha1.ConditionSynced) != messagingv1alpha1.ReasonSuspended {
			t.Fatalf("%T reason = %s", obj, conditionReason(syncedConditions(obj), messagingv1alpha1.ConditionSynced))
		}
	}
}

func TestPatchSyncedSuspended_UnsupportedType(t *testing.T) {
	t.Parallel()
	err := patchSyncedSuspended(context.Background(), nil, nil, &messagingv1alpha1.QueueManagerConnection{}, 1, "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReconcileWorkloadSuspended_NoHotLoop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 1},
		Spec: messagingv1alpha1.QueueSpec{
			Suspend: true,
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	recorder := events.NewFakeRecorder(2)

	result, err := reconcileWorkloadSuspended(ctx, cl.Status(), recorder, q, 1)
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}

	updated := &messagingv1alpha1.Queue{}
	if getErr := cl.Get(ctx, client.ObjectKeyFromObject(q), updated); getErr != nil {
		t.Fatal(getErr)
	}
	if conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) != metav1.ConditionFalse {
		t.Fatalf("status = %v", updated.Status.Conditions)
	}
	if conditionReason(
		updated.Status.Conditions,
		messagingv1alpha1.ConditionSynced,
	) != messagingv1alpha1.ReasonSuspended {
		t.Fatalf("reason = %s", conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced))
	}

	result, err = reconcileWorkloadSuspended(ctx, cl.Status(), recorder, updated, 1)
	if err != nil {
		t.Fatalf("second pass err=%v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("second pass result=%+v", result)
	}
}

func TestAnnotationValue(t *testing.T) {
	t.Parallel()
	if annotationValue(nil, "k") != "" {
		t.Fatal("expected empty for nil map")
	}
	if annotationValue(map[string]string{"k": "v"}, "k") != "v" {
		t.Fatal("expected value")
	}
}

func TestReconcileRequestedAnnotationChanged(t *testing.T) {
	t.Parallel()
	p := reconcileRequestedAnnotationChanged{}
	oldQ := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				messagingv1alpha1.ReconcileRequestedAtAnnotation: "2026-06-10T10:00:00Z",
			},
		},
	}
	newQ := oldQ.DeepCopy()
	newQ.Annotations[messagingv1alpha1.ReconcileRequestedAtAnnotation] = "2026-06-10T10:01:00Z"
	if !p.Update(event.UpdateEvent{ObjectOld: oldQ, ObjectNew: newQ}) {
		t.Fatal("expected reconcile on annotation change")
	}
	same := oldQ.DeepCopy()
	if p.Update(event.UpdateEvent{ObjectOld: oldQ, ObjectNew: same}) {
		t.Fatal("expected no reconcile when annotation unchanged")
	}
}

func TestReconcileWorkloadSuspended_EmitsEventOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	q := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: ns, Generation: 2},
		Spec:       messagingv1alpha1.QueueSpec{Suspend: true},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	recorder := events.NewFakeRecorder(2)

	if _, err := reconcileWorkloadSuspended(ctx, cl.Status(), recorder, q, 2); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, corev1.EventTypeNormal) || !strings.Contains(ev, messagingv1alpha1.ReasonSuspended) {
			t.Fatalf("event = %q", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("expected suspended event")
	}
}
