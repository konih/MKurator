package mqrest

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

type idleTrackingTransport struct {
	base       http.RoundTripper
	closeCalls atomic.Int32
}

func (t *idleTrackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
}

func (t *idleTrackingTransport) CloseIdleConnections() {
	t.closeCalls.Add(1)
}

func testConnAndSecret(
	ns string,
	generation int64,
	credRV string,
) (*messagingv1alpha1.QueueManagerConnection, *corev1.Secret) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns, ResourceVersion: credRV},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns, Generation: generation},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://ibm-mq.ibm-mq.svc:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}
	return conn, secret
}

func newRotationTestFactory(t *testing.T, cl client.Client) (*ClientFactory, *idleTrackingTransport) {
	t.Helper()
	tr := &idleTrackingTransport{}
	factory := NewClientFactory(cl).(*ClientFactory)
	factory.newClient = func(cfg Config) (mqadmin.Admin, error) {
		cfg.HTTPClient = &http.Client{Transport: tr}
		return NewClient(cfg)
	}
	return factory, tr
}

func rotateCredSecret(
	ctx context.Context,
	t *testing.T,
	cl client.Client,
	secret *corev1.Secret,
	stamp string,
) {
	t.Helper()
	latest := &corev1.Secret{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(secret), latest); err != nil {
		t.Fatalf("get secret before rotation: %v", err)
	}
	if latest.Data == nil {
		latest.Data = map[string][]byte{}
	}
	latest.Data["rotation"] = []byte(stamp)
	if err := cl.Update(ctx, latest); err != nil {
		t.Fatalf("update secret stamp %q: %v", stamp, err)
	}
}

func TestClientFactory_ReplaceOnRotationClosesOldTransport(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	conn, secret := testConnAndSecret(ns, 1, "1")
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	factory, tr := newRotationTestFactory(t, cl)

	c1, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection first: %v", err)
	}

	rotateCredSecret(ctx, t, cl, secret, "2")

	c2, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection after rotation: %v", err)
	}
	if c1 == c2 {
		t.Fatal("expected new client after secret rotation")
	}
	if got := tr.closeCalls.Load(); got != 1 {
		t.Fatalf("CloseIdleConnections calls = %d, want 1", got)
	}

	factory.mu.Lock()
	n := len(factory.cache)
	factory.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected single cache entry after rotation, got %d", n)
	}
}

func TestClientFactory_CacheBoundedAcrossRotations(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	conn, secret := testConnAndSecret(ns, 1, "0")
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	factory, tr := newRotationTestFactory(t, cl)

	const rotations = 25
	for i := 0; i < rotations; i++ {
		rotateCredSecret(ctx, t, cl, secret, fmt.Sprintf("%d", i+1))
		if _, err := factory.ForConnection(ctx, conn); err != nil {
			t.Fatalf("ForConnection rotation %d: %v", i, err)
		}
	}

	factory.mu.Lock()
	n := len(factory.cache)
	factory.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected cache size 1 after %d rotations, got %d", rotations, n)
	}
	if got := tr.closeCalls.Load(); got != rotations-1 {
		t.Fatalf("CloseIdleConnections calls = %d, want %d", got, rotations-1)
	}
}

func TestClientFactory_ReplaceOnGenerationChange(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	conn, secret := testConnAndSecret(ns, 1, "1")
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	factory, tr := newRotationTestFactory(t, cl)

	c1, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection first: %v", err)
	}

	conn.Generation = 2
	c2, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection after generation bump: %v", err)
	}
	if c1 == c2 {
		t.Fatal("expected new client after generation change")
	}
	if got := tr.closeCalls.Load(); got != 1 {
		t.Fatalf("CloseIdleConnections calls = %d, want 1", got)
	}
}
