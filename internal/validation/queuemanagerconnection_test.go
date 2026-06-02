package validation

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
)

func TestValidateQueueManagerConnectionSpecRequiredFields(t *testing.T) {
	t.Parallel()
	cl := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()

	spec := &messagingv1alpha1.QueueManagerConnectionSpec{}
	if errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", nil, spec); len(errs) < 3 {
		t.Fatalf("expected required field errors, got %v", errs)
	}
}

func TestValidateQueueManagerConnectionDeleteWithChannel(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)
	conn := sampleConnection("ns", "qm1")
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn, channel).Build()
	if errs := ValidateQueueManagerConnectionDelete(context.Background(), cl, conn); len(errs) == 0 {
		t.Fatal("expected delete blocked when channel dependent exists")
	}
}

func TestValidateQueueManagerConnectionDeleteWithAuthDependents(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)
	conn := sampleConnection("ns", "qm1")
	car := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{Name: "car1", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
		},
	}
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auth1", Namespace: "ns"},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn, car, auth).Build()
	if errs := ValidateQueueManagerConnectionDelete(context.Background(), cl, conn); len(errs) == 0 {
		t.Fatal("expected delete blocked when auth dependents exist")
	}
}

func TestValidateQueueManagerConnectionSpec(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	t.Run("http endpoint rejected", func(t *testing.T) {
		t.Parallel()
		spec := &messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "http://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "creds"},
		}
		if errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", nil, spec); len(errs) == 0 {
			t.Fatal("expected endpoint error")
		}
	})
	t.Run("missing credentials secret", func(t *testing.T) {
		t.Parallel()
		spec := &messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "missing"},
		}
		if errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", nil, spec); len(errs) == 0 {
			t.Fatal("expected secret not found error")
		}
	})
}

func TestValidateQueueManagerConnectionDelete(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)
	conn := sampleConnection("ns", "qm1")
	queue := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "ns"},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
		},
	}
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{Name: "events", Namespace: "ns"},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn, queue, topic).Build()

	t.Run("deny with dependents", func(t *testing.T) {
		t.Parallel()
		if errs := ValidateQueueManagerConnectionDelete(context.Background(), cl, conn); len(errs) == 0 {
			t.Fatal("expected delete blocked when dependents exist")
		}
	})
	t.Run("allow without dependents", func(t *testing.T) {
		t.Parallel()
		empty := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn).Build()
		if errs := ValidateQueueManagerConnectionDelete(context.Background(), empty, conn); len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})
}

func TestValidateQueueManagerConnectionInsecureTLS(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	baseSpec := func() *messagingv1alpha1.QueueManagerConnectionSpec {
		return &messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "creds"},
			TLS:                  &messagingv1alpha1.TLSConfig{InsecureSkipVerify: true},
		}
	}

	tests := []struct {
		name        string
		annotations map[string]string
		wantErr     bool
	}{
		{
			name:        "deny without opt-in annotation",
			annotations: nil,
			wantErr:     true,
		},
		{
			name:        "deny with false annotation",
			annotations: map[string]string{messagingv1alpha1.AllowInsecureTLSAnnotation: "false"},
			wantErr:     true,
		},
		{
			name:        "deny with empty annotation",
			annotations: map[string]string{messagingv1alpha1.AllowInsecureTLSAnnotation: ""},
			wantErr:     true,
		},
		{
			name:        "allow with true annotation",
			annotations: map[string]string{messagingv1alpha1.AllowInsecureTLSAnnotation: "true"},
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", tt.annotations, baseSpec())
			if tt.wantErr && len(errs) == 0 {
				t.Fatal("expected insecure TLS error")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestValidateQueueManagerConnectionCASecret(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = messagingv1alpha1.AddToScheme(scheme)
	creds := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"}}
	ca := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: "ns"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(creds, ca).Build()

	spec := &messagingv1alpha1.QueueManagerConnectionSpec{
		QueueManager:         "QM1",
		Endpoint:             "https://mq.example:9443",
		CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "creds"},
		TLS: &messagingv1alpha1.TLSConfig{
			CASecretRef: &messagingv1alpha1.SecretReference{Name: "ca"},
		},
	}
	if errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", nil, spec); len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}
