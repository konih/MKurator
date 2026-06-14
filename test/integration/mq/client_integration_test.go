//go:build integration

package mq

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if !integrationEnabled() {
		t.Skip("IBM MQ integration disabled; set KURATOR_INTEGRATION_MQ=1 and start MQ (task mq:integration:up)")
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestIntegration_Ping(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestIntegration_Queue_CreateGetDelete(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := queueNameForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: name, Type: mqadmin.QueueTypeLocal})
	})

	spec := mqadmin.QueueSpec{
		Name: name,
		Type: mqadmin.QueueTypeLocal,
		Attributes: map[string]string{
			"maxdepth": "5000",
			"descr":    "mkurator integration",
		},
	}
	if err := c.DefineQueue(ctx, spec); err != nil {
		t.Fatalf("DefineQueue: %v", err)
	}

	state, err := c.GetQueue(ctx, spec)
	if err != nil {
		t.Fatalf("GetQueue: %v", err)
	}
	if state.Name != name {
		t.Fatalf("name = %q", state.Name)
	}
	if state.Attributes["maxdepth"] != "5000" {
		t.Fatalf("maxdepth = %q", state.Attributes["maxdepth"])
	}

	if err := c.DeleteQueue(ctx, mqadmin.QueueSpec{Name: name, Type: mqadmin.QueueTypeLocal}); err != nil {
		t.Fatalf("DeleteQueue: %v", err)
	}

	_, err = c.GetQueue(ctx, spec)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_Queue_UpdateViaReplace(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := queueNameForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: name, Type: mqadmin.QueueTypeLocal})
	})

	define := func(maxdepth string) {
		t.Helper()
		err := c.DefineQueue(ctx, mqadmin.QueueSpec{
			Name: name,
			Type: mqadmin.QueueTypeLocal,
			Attributes: map[string]string{
				"maxdepth": maxdepth,
			},
		})
		if err != nil {
			t.Fatalf("DefineQueue maxdepth=%s: %v", maxdepth, err)
		}
	}

	spec := mqadmin.QueueSpec{Name: name, Type: mqadmin.QueueTypeLocal}

	define("100")
	state, err := c.GetQueue(ctx, spec)
	if err != nil {
		t.Fatalf("GetQueue: %v", err)
	}
	if state.Attributes["maxdepth"] != "100" {
		t.Fatalf("maxdepth after first define = %q", state.Attributes["maxdepth"])
	}

	define("200")
	state, err = c.GetQueue(ctx, spec)
	if err != nil {
		t.Fatalf("GetQueue after update: %v", err)
	}
	if state.Attributes["maxdepth"] != "200" {
		t.Fatalf("maxdepth after replace = %q", state.Attributes["maxdepth"])
	}
}

func TestIntegration_GetQueue_NotFound(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := queueNameForTest(t.Name() + ".MISSING")

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.GetQueue(ctx, mqadmin.QueueSpec{Name: name, Type: mqadmin.QueueTypeLocal})
	if err == nil {
		t.Fatal("expected not found")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_DeleteQueue_Idempotent(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := queueNameForTest(t.Name() + ".GONE")

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	if err := c.DeleteQueue(ctx, mqadmin.QueueSpec{Name: name, Type: mqadmin.QueueTypeLocal}); err != nil {
		t.Fatalf("DeleteQueue on missing queue: %v", err)
	}
}

func TestIntegration_Queue_Alias(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	target := queueNameForTest(t.Name() + ".TARGET")
	alias := queueNameForTest(t.Name() + ".ALIAS")

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: alias, Type: mqadmin.QueueTypeAlias})
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: target, Type: mqadmin.QueueTypeLocal})
	})

	if err := c.DefineQueue(ctx, mqadmin.QueueSpec{
		Name: target, Type: mqadmin.QueueTypeLocal, Attributes: map[string]string{"maxdepth": "100"},
	}); err != nil {
		t.Fatalf("define target: %v", err)
	}

	aliasSpec := mqadmin.QueueSpec{
		Name: alias, Type: mqadmin.QueueTypeAlias,
		Attributes: map[string]string{"targq": target, "descr": "integration alias"},
	}
	if err := c.DefineQueue(ctx, aliasSpec); err != nil {
		t.Fatalf("define alias: %v", err)
	}

	state, err := c.GetQueue(ctx, aliasSpec)
	if err != nil {
		t.Fatalf("GetQueue alias: %v", err)
	}
	if state.Attributes["targq"] != target {
		t.Fatalf("targq = %q", state.Attributes["targq"])
	}
}

func TestIntegration_Queue_Alias_UpdateTargetViaReplace(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	hash := testNameHash(t.Name()) % 100000
	target1 := fmt.Sprintf("KIT.Q.%05d.T1", hash)
	target2 := fmt.Sprintf("KIT.Q.%05d.T2", hash)
	alias := fmt.Sprintf("KIT.Q.%05d.A", hash)

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: alias, Type: mqadmin.QueueTypeAlias})
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: target1, Type: mqadmin.QueueTypeLocal})
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: target2, Type: mqadmin.QueueTypeLocal})
	})

	for _, target := range []string{target1, target2} {
		if err := c.DefineQueue(ctx, mqadmin.QueueSpec{
			Name: target, Type: mqadmin.QueueTypeLocal, Attributes: map[string]string{"maxdepth": "100"},
		}); err != nil {
			t.Fatalf("define target %s: %v", target, err)
		}
	}

	defineAlias := func(target string) {
		t.Helper()
		spec := mqadmin.QueueSpec{
			Name: alias, Type: mqadmin.QueueTypeAlias,
			Attributes: map[string]string{"targq": target, "descr": "integration alias replace"},
		}
		if err := c.DefineQueue(ctx, spec); err != nil {
			t.Fatalf("define alias target=%s: %v", target, err)
		}
	}

	spec := mqadmin.QueueSpec{Name: alias, Type: mqadmin.QueueTypeAlias}
	defineAlias(target1)
	state, err := c.GetQueue(ctx, spec)
	if err != nil {
		t.Fatalf("GetQueue alias v1: %v", err)
	}
	if state.Attributes["targq"] != target1 {
		t.Fatalf("targq v1 = %q", state.Attributes["targq"])
	}

	defineAlias(target2)
	state, err = c.GetQueue(ctx, spec)
	if err != nil {
		t.Fatalf("GetQueue alias v2: %v", err)
	}
	if state.Attributes["targq"] != target2 {
		t.Fatalf("targq v2 = %q", state.Attributes["targq"])
	}
}

func TestIntegration_Queue_Remote(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	target := queueNameForTest(t.Name() + ".TARGET")
	remote := queueNameForTest(t.Name() + ".REMOTE")

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: remote, Type: mqadmin.QueueTypeRemote})
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: target, Type: mqadmin.QueueTypeLocal})
	})

	if err := c.DefineQueue(ctx, mqadmin.QueueSpec{
		Name: target, Type: mqadmin.QueueTypeLocal, Attributes: map[string]string{"maxdepth": "100"},
	}); err != nil {
		t.Fatalf("define target: %v", err)
	}

	remoteSpec := mqadmin.QueueSpec{
		Name: remote, Type: mqadmin.QueueTypeRemote,
		Attributes: map[string]string{
			"rname":   target,
			"rqmname": integrationQueueManager(),
			"xmitq":   "SYSTEM.DEFAULT.XMIT.QUEUE",
			"descr":   "integration remote",
		},
	}
	if err := c.DefineQueue(ctx, remoteSpec); err != nil {
		t.Fatalf("define remote: %v", err)
	}

	state, err := c.GetQueue(ctx, remoteSpec)
	if err != nil {
		t.Fatalf("GetQueue remote: %v", err)
	}
	if state.Attributes["rname"] != target {
		t.Fatalf("rname = %q", state.Attributes["rname"])
	}
}

func TestIntegration_DefineQueue_UnsupportedType(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := queueNameForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	err = c.DefineQueue(ctx, mqadmin.QueueSpec{
		Name: name,
		Type: mqadmin.QueueType("model"),
	})
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestIntegration_RunMQSC(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := queueNameForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.DeleteQueue(context.Background(), mqadmin.QueueSpec{Name: name, Type: mqadmin.QueueTypeLocal})
	})

	if err := c.DefineQueue(ctx, mqadmin.QueueSpec{
		Name: name,
		Type: mqadmin.QueueTypeLocal,
	}); err != nil {
		t.Fatalf("DefineQueue: %v", err)
	}

	cmd := fmt.Sprintf("DISPLAY QLOCAL('%s')", name)
	if err := c.RunMQSC(ctx, cmd); err != nil {
		t.Fatalf("RunMQSC: %v", err)
	}
}

func TestIntegration_Factory_ForConnection_Ping(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)

	if err := messagingv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}

	cfg, err := integrationConfig()
	if err != nil {
		t.Fatal(err)
	}

	ns := "mkurator-integration"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: ns},
		Data: map[string][]byte{
			"username":        []byte(cfg.Username),
			"mqAdminPassword": []byte(cfg.Password),
		},
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns, Generation: 1},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: cfg.QueueManager,
			Endpoint:     cfg.Endpoint.String(),
			TLS:          &messagingv1alpha1.TLSConfig{InsecureSkipVerify: true},
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "mq-credentials",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret, conn).Build()
	factory := mqrest.NewClientFactory(cl)

	admin, err := factory.ForConnection(ctx, conn)
	if err != nil {
		t.Fatalf("ForConnection: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		t.Fatalf("Ping via factory: %v", err)
	}
}
