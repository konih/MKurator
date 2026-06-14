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

func TestValidateConnectionRef(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)

	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: "ns"},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "creds"},
		},
	}
	deleting := conn.DeepCopy()
	deleting.Name = "qm-deleting"
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	deleting.Finalizers = []string{messagingv1alpha1.QueueManagerConnectionFinalizer}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn, deleting).Build()
	path := fieldRoot("connectionRef").Child("name")

	t.Run("missing ref name", func(t *testing.T) {
		t.Parallel()
		if errs := ValidateConnectionRef(context.Background(), cl, "ns", "", path); len(errs) == 0 {
			t.Fatal("expected required error")
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		if errs := ValidateConnectionRef(context.Background(), cl, "ns", "missing", path); len(errs) == 0 {
			t.Fatal("expected not found error")
		}
	})
	t.Run("deleting", func(t *testing.T) {
		t.Parallel()
		if errs := ValidateConnectionRef(context.Background(), cl, "ns", "qm-deleting", path); len(errs) == 0 {
			t.Fatal("expected deleting error")
		}
	})
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		if errs := ValidateConnectionRef(context.Background(), cl, "ns", "qm1", path); len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})
}

func TestValidateQueueSpec(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)
	conn := sampleConnection("ns", "qm1")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn).Build()

	t.Run("unknown attribute warning", func(t *testing.T) {
		t.Parallel()
		spec := &messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.Q",
			Attributes:    map[string]string{"notreal": "x"},
		}
		warnings, errs := ValidateQueueSpec(context.Background(), cl, "ns", "app-queue", spec)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if len(warnings) != 1 {
			t.Fatalf("expected one warning, got %v", warnings)
		}
	})
	t.Run("missing connection ref", func(t *testing.T) {
		t.Parallel()
		spec := &messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "missing"},
			QueueName:     "APP.Q",
		}
		_, errs := ValidateQueueSpec(context.Background(), cl, "ns", "app-queue", spec)
		if len(errs) == 0 {
			t.Fatal("expected connection ref error")
		}
	})
}

func TestValidateChannelSpec(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)
	conn := sampleConnection("ns", "qm1")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn).Build()

	spec := &messagingv1alpha1.ChannelSpec{
		ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
		ChannelName:   "ORDERS.APP",
	}
	warnings, errs := ValidateChannelSpec(context.Background(), cl, "ns", "orders-app", spec)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestValidateTopicSpec(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)
	conn := sampleConnection("ns", "qm1")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn).Build()

	spec := &messagingv1alpha1.TopicSpec{
		ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
		TopicName:     "RETAIL.ORDERS",
		Attributes:    map[string]string{"topstr": "A.B"},
	}
	warnings, errs := ValidateTopicSpec(context.Background(), cl, "ns", "retail-orders", spec)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestInvalidHelpers(t *testing.T) {
	t.Parallel()
	errs := field.ErrorList{field.Required(field.NewPath("spec"), "required")}
	if err := QueueInvalid("q1", errs); err == nil {
		t.Fatal("expected QueueInvalid error")
	}
	if err := TopicInvalid("t1", errs); err == nil {
		t.Fatal("expected TopicInvalid error")
	}
	if err := ChannelInvalid("c1", errs); err == nil {
		t.Fatal("expected ChannelInvalid error")
	}
	if err := QueueManagerConnectionInvalid("conn", errs); err == nil {
		t.Fatal("expected QueueManagerConnectionInvalid error")
	}
	if got := ObjectNameFromMeta(&metav1.ObjectMeta{GenerateName: "gen-"}); got != "gen-" {
		t.Fatalf("ObjectNameFromMeta = %q", got)
	}
}

func sampleConnection(ns, name string) *messagingv1alpha1.QueueManagerConnection {
	return &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "creds"},
		},
	}
}

func sampleManagedChannel(ns, name, connName, channelName string) *messagingv1alpha1.Channel {
	return &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: connName},
			ChannelName:   channelName,
		},
	}
}

func fieldRoot(name string) *field.Path {
	return field.NewPath("spec").Child(name)
}
