package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func TestAdoptionBlockForExisting_FailIfExists(t *testing.T) {
	t.Parallel()
	block := adoptionBlockForExisting(
		messagingv1alpha1.AdoptionPolicyFailIfExists,
		true,
		true,
		false,
		`queue "APP.Q"`,
		"",
	)
	if block == nil || block.Reason != messagingv1alpha1.ReasonAlreadyExists {
		t.Fatalf("block = %#v", block)
	}
}

func TestAdoptionBlockForExisting_AdoptIfMatchingConflict(t *testing.T) {
	t.Parallel()
	block := adoptionBlockForExisting(
		messagingv1alpha1.AdoptionPolicyAdoptIfMatching,
		true,
		true,
		true,
		`queue "APP.Q"`,
		"drift",
	)
	if block == nil || block.Reason != messagingv1alpha1.ReasonAdoptionConflict {
		t.Fatalf("block = %#v", block)
	}
}

func TestAttributeMismatchMessage(t *testing.T) {
	t.Parallel()
	msg := attributeMismatchMessage(
		map[string]string{"maxdepth": "5000"},
		map[string]string{"maxdepth": "1000"},
		[]string{"maxdepth"},
		`queue "APP.Q"`,
	)
	if msg == "" || msg == `queue "APP.Q" differs from spec` {
		t.Fatalf("msg = %q", msg)
	}
}

func TestHandleAdoptionBlock_NilBlock(t *testing.T) {
	t.Parallel()
	result, err := handleAdoptionBlock(
		context.Background(), nil, nil, &messagingv1alpha1.Queue{}, 1, nil, syncStatusOpts{},
	)
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestHandleAdoptionBlock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	q := &messagingv1alpha1.Queue{ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "ns"}}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(q).WithObjects(q).Build()
	block := &AdoptionBlockedError{Reason: messagingv1alpha1.ReasonAlreadyExists, Message: "exists"}
	result, err := handleAdoptionBlock(ctx, cl.Status(), nil, q, 1, block, syncStatusOpts{})
	if err != nil || result.RequeueAfter == 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPatchSyncedAdoptionBlocked_AllKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := unitSchemeOrFatal(t)
	cases := []client.Object{
		&messagingv1alpha1.Queue{ObjectMeta: metav1.ObjectMeta{Name: "q", Namespace: "ns"}},
		&messagingv1alpha1.Topic{ObjectMeta: metav1.ObjectMeta{Name: "t", Namespace: "ns"}},
		&messagingv1alpha1.Channel{ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"}},
		&messagingv1alpha1.ChannelAuthRule{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"}},
		&messagingv1alpha1.AuthorityRecord{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns"}},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(cases...).WithObjects(cases...).Build()
	for _, obj := range cases {
		if err := patchSyncedAdoptionBlocked(
			ctx, cl.Status(), nil, obj, 1, messagingv1alpha1.ReasonAlreadyExists, "exists", syncStatusOpts{},
		); err != nil {
			t.Fatalf("%T: %v", obj, err)
		}
	}
}
