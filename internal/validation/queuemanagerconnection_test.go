package validation

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func TestValidateQueueManagerConnectionDeleteWithTopic(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = messagingv1alpha1.AddToScheme(scheme)
	conn := sampleConnection("ns", "qm1")
	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{Name: "retail", Namespace: "ns"},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn, topic).Build()
	errs := ValidateQueueManagerConnectionDelete(context.Background(), cl, conn)
	if len(errs) == 0 {
		t.Fatal("expected delete blocked when topic dependent exists")
	}
	if !strings.Contains(errs[0].Detail, "retail") {
		t.Fatalf("detail = %q", errs[0].Detail)
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

	t.Run("missing credentials secret", func(t *testing.T) {
		t.Parallel()
		spec := &messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "missing"},
		}
		if _, errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", nil, spec); len(errs) == 0 {
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
			_, errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", tt.annotations, baseSpec())
			if tt.wantErr && len(errs) == 0 {
				t.Fatal("expected insecure TLS error")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestValidateQueueManagerConnectionDeleteMultipleDependents(t *testing.T) {
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
	errs := ValidateQueueManagerConnectionDelete(context.Background(), cl, conn)
	if len(errs) == 0 {
		t.Fatal("expected delete blocked")
	}
	detail := errs[0].Detail
	if !strings.Contains(detail, "Queue") || !strings.Contains(detail, "Topic") {
		t.Fatalf("detail = %q", detail)
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
	if _, errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", nil, spec); len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateQueueManagerConnectionCredentialsUsernameWarning(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	t.Run("warn when username key missing", func(t *testing.T) {
		t.Parallel()
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"},
			Data:       map[string][]byte{"password": []byte("x")},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
		spec := sampleConnection("ns", "qm1").Spec
		warnings, errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", nil, &spec)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], `default to "admin"`) {
			t.Fatalf("warnings = %v", warnings)
		}
	})
	t.Run("no warning when username present", func(t *testing.T) {
		t.Parallel()
		for _, key := range []string{"username", "user", "mqAdminUser"} {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"},
				Data:       map[string][]byte{key: []byte("mquser"), "password": []byte("x")},
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
			spec := sampleConnection("ns", "qm1").Spec
			warnings, errs := ValidateQueueManagerConnectionSpec(context.Background(), cl, "ns", nil, &spec)
			if len(errs) > 0 {
				t.Fatalf("key %q: unexpected errors: %v", key, errs)
			}
			if len(warnings) != 0 {
				t.Fatalf("key %q: warnings = %v", key, warnings)
			}
		}
	})
}
