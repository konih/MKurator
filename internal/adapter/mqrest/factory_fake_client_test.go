package mqrest_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
)

func TestClientFactory_ForConnection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "mkurator-system"
	if err := messagingv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns, Generation: 1},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://ibm-mq.ibm-mq.svc:9443",
			TLS:          &messagingv1alpha1.TLSConfig{InsecureSkipVerify: true},
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "mq-credentials",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret, conn).Build()
	factory := mqrest.NewClientFactory(cl)

	admin1, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection: %v", err)
	}
	admin2, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection cached: %v", err)
	}
	if admin1 != admin2 {
		t.Fatal("expected cached client instance")
	}
}
