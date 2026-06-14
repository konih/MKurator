package mqrest

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func TestClientFactory_ReleaseConnection(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns, ResourceVersion: "1"},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns, Generation: 1},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://ibm-mq.ibm-mq.svc:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	factory := NewClientFactory(cl)

	if _, err := factory.ForConnection(ctx, conn); err != nil {
		t.Fatalf("ForConnection: %v", err)
	}
	if err := factory.ReleaseConnection(ctx, conn); err != nil {
		t.Fatalf("ReleaseConnection: %v", err)
	}

	f := factory.(*ClientFactory)
	f.mu.Lock()
	n := len(f.cache)
	f.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected empty cache after release, got %d entries", n)
	}
}

func TestClientFactory_ReleaseConnectionMissingSecret(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns, ResourceVersion: "1"},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns, Generation: 1},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://ibm-mq.ibm-mq.svc:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	factory := NewClientFactory(cl)

	if _, err := factory.ForConnection(ctx, conn); err != nil {
		t.Fatalf("ForConnection: %v", err)
	}

	if err := cl.Delete(ctx, secret); err != nil {
		t.Fatalf("delete secret: %v", err)
	}

	if err := factory.ReleaseConnection(ctx, conn); err != nil {
		t.Fatalf("ReleaseConnection with missing secret: %v", err)
	}

	f := factory.(*ClientFactory)
	f.mu.Lock()
	n := len(f.cache)
	f.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected empty cache after release, got %d entries", n)
	}
}

func TestClientFactory_ForConnectionUsesCache(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns, ResourceVersion: "1"},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns, Generation: 1},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://ibm-mq.ibm-mq.svc:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	factory := NewClientFactory(cl)

	c1, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection first: %v", err)
	}
	c2, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection second: %v", err)
	}
	if c1 != c2 {
		t.Fatal("expected cached client instance")
	}
}
