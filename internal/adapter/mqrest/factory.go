package mqrest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"sync"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

// ClientFactory resolves Secrets and caches mqrest clients per connection.
type ClientFactory struct {
	K8s client.Client

	mu        sync.Mutex
	cache     map[string]*cacheEntry
	newClient func(Config) (mqadmin.Admin, error)
}

type cacheEntry struct {
	admin      mqadmin.Admin
	generation int64
	credRV     string
	caRV       string
}

type cacheFingerprint struct {
	generation int64
	credRV     string
	caRV       string
}

func (e *cacheEntry) matches(fp cacheFingerprint) bool {
	return e.generation == fp.generation &&
		e.credRV == fp.credRV &&
		e.caRV == fp.caRV
}

// NewClientFactory returns a mqadmin.Factory that caches clients by QMC identity (ADR-0023).
func NewClientFactory(k8s client.Client) mqadmin.Factory {
	return &ClientFactory{
		K8s:   k8s,
		cache: make(map[string]*cacheEntry),
	}
}

func (f *ClientFactory) createClient(cfg Config) (mqadmin.Admin, error) {
	if f.newClient != nil {
		return f.newClient(cfg)
	}
	return NewClient(cfg)
}

// ForConnection implements mqadmin.Factory.
func (f *ClientFactory) ForConnection(
	ctx context.Context,
	conn *messagingv1alpha1.QueueManagerConnection,
) (mqadmin.Admin, error) {
	key := connectionCacheKey(conn)

	fp, err := f.cacheFingerprint(ctx, conn)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	if entry, ok := f.cache[key]; ok && entry.matches(fp) {
		admin := entry.admin
		f.mu.Unlock()
		return admin, nil
	}
	f.mu.Unlock()

	cfg, err := f.buildConfig(ctx, conn)
	if err != nil {
		return nil, err
	}

	c, err := f.createClient(cfg)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	var old mqadmin.Admin
	if entry, ok := f.cache[key]; ok {
		if entry.matches(fp) {
			closeClientIdleConnections(c)
			return entry.admin, nil
		}
		old = entry.admin
	}

	f.cache[key] = &cacheEntry{
		admin:      c,
		generation: fp.generation,
		credRV:     fp.credRV,
		caRV:       fp.caRV,
	}
	closeClientIdleConnections(old)
	return c, nil
}

// ReleaseConnection implements mqadmin.Factory.
// Eviction is keyed by QMC identity (namespace/name) and does not read Secrets,
// so deletion succeeds when credentials were removed first (ADR-0023).
func (f *ClientFactory) ReleaseConnection(
	_ context.Context,
	conn *messagingv1alpha1.QueueManagerConnection,
) error {
	key := connectionCacheKey(conn)

	f.mu.Lock()
	entry, ok := f.cache[key]
	if ok {
		delete(f.cache, key)
	}
	f.mu.Unlock()

	if ok {
		closeClientIdleConnections(entry.admin)
	}
	return nil
}

func connectionCacheKey(conn *messagingv1alpha1.QueueManagerConnection) string {
	return fmt.Sprintf("%s/%s", conn.Namespace, conn.Name)
}

func (f *ClientFactory) cacheFingerprint(
	ctx context.Context,
	conn *messagingv1alpha1.QueueManagerConnection,
) (cacheFingerprint, error) {
	credSecret := &corev1.Secret{}
	if err := f.K8s.Get(ctx, client.ObjectKey{
		Namespace: conn.Namespace,
		Name:      conn.Spec.CredentialsSecretRef.Name,
	}, credSecret); err != nil {
		return cacheFingerprint{}, secretLookupError(
			conn.Spec.CredentialsSecretRef.Name,
			"credentials",
			"cache fingerprint",
			err,
		)
	}

	fp := cacheFingerprint{
		generation: conn.Generation,
		credRV:     credSecret.ResourceVersion,
	}

	if conn.Spec.TLS != nil && conn.Spec.TLS.CASecretRef != nil {
		caSecret := &corev1.Secret{}
		if err := f.K8s.Get(ctx, client.ObjectKey{
			Namespace: conn.Namespace,
			Name:      conn.Spec.TLS.CASecretRef.Name,
		}, caSecret); err != nil {
			return cacheFingerprint{}, secretLookupError(
				conn.Spec.TLS.CASecretRef.Name,
				"CA",
				"cache fingerprint",
				err,
			)
		}
		fp.caRV = caSecret.ResourceVersion
	}

	return fp, nil
}

func closeClientIdleConnections(admin mqadmin.Admin) {
	if admin == nil {
		return
	}
	if c, ok := admin.(*Client); ok {
		c.CloseIdleConnections()
	}
}

func (f *ClientFactory) buildConfig(
	ctx context.Context,
	conn *messagingv1alpha1.QueueManagerConnection,
) (Config, error) {
	ns := conn.Namespace
	credSecret := &corev1.Secret{}
	if err := f.K8s.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      conn.Spec.CredentialsSecretRef.Name,
	}, credSecret); err != nil {
		return Config{}, secretLookupError(
			conn.Spec.CredentialsSecretRef.Name,
			"credentials",
			"",
			err,
		)
	}

	user, pass, err := credentialsFromSecret(credSecret.Data)
	if err != nil {
		return Config{}, err
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if conn.Spec.TLS != nil && conn.Spec.TLS.InsecureSkipVerify {
		tlsCfg.InsecureSkipVerify = true
	}

	if conn.Spec.TLS != nil && conn.Spec.TLS.CASecretRef != nil {
		caSecret := &corev1.Secret{}
		if getErr := f.K8s.Get(ctx, client.ObjectKey{
			Namespace: ns,
			Name:      conn.Spec.TLS.CASecretRef.Name,
		}, caSecret); getErr != nil {
			return Config{}, secretLookupError(
				conn.Spec.TLS.CASecretRef.Name,
				"CA",
				"",
				getErr,
			)
		}
		pool, poolErr := caPoolFromSecret(caSecret.Data)
		if poolErr != nil {
			return Config{}, poolErr
		}
		tlsCfg.RootCAs = pool
	}

	endpoint, err := url.Parse(conn.Spec.Endpoint)
	if err != nil {
		return Config{}, fmt.Errorf("parse endpoint: %w", err)
	}

	prefix := conn.Spec.RESTPrefix
	if prefix == "" {
		prefix = DefaultRESTPrefix
	}

	return Config{
		Endpoint:     endpoint,
		RESTPrefix:   prefix,
		QueueManager: conn.Spec.QueueManager,
		Username:     user,
		Password:     pass,
		TLSConfig:    tlsCfg,
	}, nil
}

func credentialsFromSecret(data map[string][]byte) (string, string, error) {
	user := firstKey(data, "username", "user", "mqAdminUser")
	pass := firstKey(data, "password", "mqAdminPassword")
	if user == "" {
		// IBM MQ dev images often use admin; admission warns when username keys are absent (ARCH-12).
		user = "admin"
	}
	if pass == "" {
		return "", "", fmt.Errorf("credentials secret missing password (expected key password or mqAdminPassword)")
	}
	return user, pass, nil
}

func firstKey(data map[string][]byte, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k]; ok && len(v) > 0 {
			return string(v)
		}
	}
	return ""
}

func caPoolFromSecret(data map[string][]byte) (*x509.CertPool, error) {
	pemBytes := firstBytes(data, "tls.crt", "ca.crt", "ca.pem")
	if len(pemBytes) == 0 {
		return nil, fmt.Errorf("CA secret missing tls.crt or ca.crt")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("parse CA certificate PEM")
	}
	return pool, nil
}

func firstBytes(data map[string][]byte, keys ...string) []byte {
	for _, k := range keys {
		if v, ok := data[k]; ok && len(v) > 0 {
			return v
		}
	}
	return nil
}

func secretLookupError(name, role, context string, err error) error {
	if k8serrors.IsNotFound(err) {
		return &mqadmin.SecretNotFoundError{Name: name, Role: role, Cause: err}
	}
	if context != "" {
		return fmt.Errorf("get %s secret for %s: %w", role, context, err)
	}
	return fmt.Errorf("get %s secret: %w", role, err)
}
