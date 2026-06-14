package health

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func TestMQConnectivityChecker_Check(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	ready := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm-up", Namespace: "default"},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "creds",
			},
		},
	}
	ready.Status.Conditions = []metav1.Condition{{
		Type:   messagingv1alpha1.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: messagingv1alpha1.ReasonAvailable,
	}}

	unready := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm-down", Namespace: "default"},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "creds",
			},
		},
	}
	unready.Status.Conditions = []metav1.Condition{{
		Type:   messagingv1alpha1.ConditionReady,
		Status: metav1.ConditionFalse,
		Reason: messagingv1alpha1.ReasonError,
	}}

	t.Run("no QMCs", func(t *testing.T) {
		t.Parallel()
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&messagingv1alpha1.QueueManagerConnection{}).
			Build()
		checker := &MQConnectivityChecker{Client: c}
		if err := checker.Check(nil); err != nil {
			t.Fatalf("Check() err = %v", err)
		}
	})

	t.Run("one ready", func(t *testing.T) {
		t.Parallel()
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&messagingv1alpha1.QueueManagerConnection{}).
			WithObjects(ready).
			Build()
		checker := &MQConnectivityChecker{Client: c}
		if err := checker.Check(nil); err != nil {
			t.Fatalf("Check() err = %v", err)
		}
	})

	t.Run("list error", func(t *testing.T) {
		t.Parallel()
		cl := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
		checker := &MQConnectivityChecker{Client: cl}
		if err := checker.Check(nil); err == nil {
			t.Fatal("expected list error")
		}
	})

	t.Run("constructor wraps checker", func(t *testing.T) {
		t.Parallel()
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&messagingv1alpha1.QueueManagerConnection{}).
			Build()
		check := NewMQConnectivityChecker(c)
		if err := check(nil); err != nil {
			t.Fatalf("Check() err = %v", err)
		}
	})

	t.Run("only unready", func(t *testing.T) {
		t.Parallel()
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&messagingv1alpha1.QueueManagerConnection{}).
			WithObjects(unready).
			Build()
		checker := &MQConnectivityChecker{Client: c}
		if err := checker.Check(nil); !errors.Is(err, ErrNoHealthyQMC) {
			t.Fatalf("Check() err = %v, want ErrNoHealthyQMC", err)
		}
	})
}
