package validation

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func TestValidateManagedChannelRef(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)

	connName := "qm1"
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-app", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: connName},
			ChannelName:   "ORDERS.APP",
		},
	}
	deleting := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "legacy-app",
			Namespace:         "ns",
			DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
			Finalizers:        []string{messagingv1alpha1.ChannelFinalizer},
		},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: connName},
			ChannelName:   "LEGACY.APP",
		},
	}
	otherConn := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-only", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "other-qm"},
			ChannelName:   "REMOTE.ONLY",
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(channel, deleting, otherConn).Build()
	path := field.NewPath("spec").Child("channelName")

	tests := []struct {
		name        string
		connRef     string
		channelName string
		wantType    field.ErrorType
	}{
		{name: "not found", connRef: connName, channelName: "MISSING.APP", wantType: field.ErrorTypeNotFound},
		{name: "wrong connectionRef", connRef: connName, channelName: "REMOTE.ONLY", wantType: field.ErrorTypeNotFound},
		{name: "deleting channel", connRef: connName, channelName: "LEGACY.APP", wantType: field.ErrorTypeInvalid},
		{name: "ok", connRef: connName, channelName: "ORDERS.APP", wantType: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateManagedChannelRef(context.Background(), cl, "ns", tt.connRef, tt.channelName, path)
			if tt.wantType == "" {
				if len(errs) > 0 {
					t.Fatalf("unexpected errors: %v", errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("expected %s error", tt.wantType)
			}
			if errs[0].Type != tt.wantType {
				t.Fatalf("expected type %s, got %s: %v", tt.wantType, errs[0].Type, errs)
			}
		})
	}
}
