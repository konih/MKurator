package mqrest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func TestCredentialsFromSecret(t *testing.T) {
	t.Parallel()
	user, pass, err := credentialsFromSecret(map[string][]byte{
		"username":        []byte("mquser"),
		"mqAdminPassword": []byte("secret"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if user != "mquser" || pass != "secret" {
		t.Fatalf("user=%q pass=%q", user, pass)
	}

	_, _, err = credentialsFromSecret(map[string][]byte{"username": []byte("u")})
	if err == nil {
		t.Fatal("expected error when password missing")
	}

	user, pass, err = credentialsFromSecret(map[string][]byte{"password": []byte("p")})
	if err != nil {
		t.Fatal(err)
	}
	if user != "admin" || pass != "p" {
		t.Fatalf("defaults user=%q pass=%q", user, pass)
	}
}

func TestCaPoolFromSecret(t *testing.T) {
	t.Parallel()
	pem := []byte(`-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHBfpL1x5jTMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWxv
Y2FsaG9zdDAeFw0yNDAxMDEwMDAwMDBaFw0yNTAxMDEwMDAwMDBaMBQxEjAQBgNV
BAMMCWxvY2FsaG9zdDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABG1234567890
-----END CERTIFICATE-----`)
	_, err := caPoolFromSecret(map[string][]byte{"ca.crt": pem})
	if err == nil {
		t.Fatal("expected parse error for invalid PEM")
	}

	pool, err := caPoolFromSecret(map[string][]byte{"ca.crt": testCAPEM(t)})
	if err != nil {
		t.Fatalf("valid CA: %v", err)
	}
	if pool == nil {
		t.Fatal("expected cert pool")
	}
}

func TestClientFactory_BuildConfigWithCA(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	caPEM := testCAPEM(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-ca", Namespace: ns},
		Data:       map[string][]byte{"ca.crt": caPEM},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns, Generation: 2},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://ibm-mq.ibm-mq.svc:9443",
			TLS: &messagingv1alpha1.TLSConfig{
				CASecretRef: &messagingv1alpha1.SecretReference{Name: "mq-ca"},
			},
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, caSecret, conn).Build()
	factory := NewClientFactory(cl)
	if _, err := factory.ForConnection(ctx, conn); err != nil {
		t.Fatalf("ForConnection: %v", err)
	}
}

func TestClientFactory_CacheFingerprintChangesWithSecretResourceVersion(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns, Generation: 1},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://ibm-mq.ibm-mq.svc:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}
	secretData := map[string][]byte{
		"username":        []byte("admin"),
		"mqAdminPassword": []byte("passw0rd"),
	}
	secretV1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns, ResourceVersion: "1"},
		Data:       secretData,
	}
	secretV2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns, ResourceVersion: "2"},
		Data:       secretData,
	}

	cl1 := fake.NewClientBuilder().WithScheme(s).WithObjects(secretV1, conn).Build()
	cl2 := fake.NewClientBuilder().WithScheme(s).WithObjects(secretV2, conn).Build()

	fp1, err := NewClientFactory(cl1).(*ClientFactory).cacheFingerprint(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := NewClientFactory(cl2).(*ClientFactory).cacheFingerprint(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if fp1.credRV == fp2.credRV {
		t.Fatalf("fingerprints should differ when secret ResourceVersion changes: %+v", fp1)
	}
	if connectionCacheKey(conn) != fmt.Sprintf("%s/%s", ns, conn.Name) {
		t.Fatal("cache key should be namespace/name identity only")
	}
}

func TestClientFactory_BuildConfigMissingCASecret(t *testing.T) {
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
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://ibm-mq.ibm-mq.svc:9443",
			TLS: &messagingv1alpha1.TLSConfig{
				CASecretRef: &messagingv1alpha1.SecretReference{Name: "mq-ca"},
			},
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	factory := NewClientFactory(cl).(*ClientFactory)
	if _, err := factory.buildConfig(ctx, conn); err == nil {
		t.Fatal("expected error when CA secret is missing")
	}
}

func TestClientFactory_BuildConfigInvalidEndpoint(t *testing.T) {
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
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "://bad-url",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	factory := NewClientFactory(cl).(*ClientFactory)
	if _, err := factory.buildConfig(ctx, conn); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestClientFactory_BuildConfigInsecureTLS(t *testing.T) {
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
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns},
		Data: map[string][]byte{
			"username":        []byte("admin"),
			"mqAdminPassword": []byte("passw0rd"),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://ibm-mq.ibm-mq.svc:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
			TLS:                  &messagingv1alpha1.TLSConfig{InsecureSkipVerify: true},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	cfg, err := NewClientFactory(cl).(*ClientFactory).buildConfig(ctx, conn)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.TLSConfig == nil || !cfg.TLSConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify on TLS config")
	}
}

func TestCaPoolFromSecretMissingKey(t *testing.T) {
	t.Parallel()
	_, err := caPoolFromSecret(map[string][]byte{"other": []byte("x")})
	if err == nil {
		t.Fatal("expected error when CA PEM key is missing")
	}
}

func TestClientFactory_CacheFingerprintMissingCredSecret(t *testing.T) {
	ctx := context.Background()
	ns := "mkurator-system"
	s := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "missing"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(conn).Build()
	_, err := NewClientFactory(cl).(*ClientFactory).cacheFingerprint(ctx, conn)
	if err == nil {
		t.Fatal("expected error when credentials secret is missing")
	}
}

func TestFirstBytes_PrefersFirstKey(t *testing.T) {
	t.Parallel()
	got := firstBytes(map[string][]byte{"ca.crt": []byte("a"), "tls.crt": []byte("b")}, "tls.crt", "ca.crt")
	if string(got) != "b" {
		t.Fatalf("got %q", got)
	}
	if firstBytes(map[string][]byte{}, "ca.crt") != nil {
		t.Fatal("expected nil when no keys match")
	}
}

func testCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
