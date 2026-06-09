package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	messagingv1alpha1 "github.com/konih/mkurator/api/v1alpha1"
)

func TestConnectionReferencesSecret(t *testing.T) {
	t.Parallel()
	conn := &messagingv1alpha1.QueueManagerConnection{
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "creds"},
			TLS: &messagingv1alpha1.TLSConfig{
				CASecretRef: &messagingv1alpha1.SecretReference{Name: "ca"},
			},
		},
	}
	if !connectionReferencesSecret(conn, "creds") {
		t.Fatal("expected creds match")
	}
	if !connectionReferencesSecret(conn, "ca") {
		t.Fatal("expected ca match")
	}
	if connectionReferencesSecret(conn, "other") {
		t.Fatal("expected no match")
	}
}

func TestRequestsForSecret_EnqueuesReferencingConnections(t *testing.T) {
	t.Parallel()
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
		Data:       map[string][]byte{"password": []byte("old")},
	}
	connMatch := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}
	connOther := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm2", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "other"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, connMatch, connOther).Build()
	reqs := requestsForSecret(ctx, cl, secret)
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	if reqs[0].Name != "qm1" || reqs[0].Namespace != ns {
		t.Fatalf("request = %+v", reqs[0])
	}
}

func TestSecretWatchPredicates(t *testing.T) {
	t.Parallel()
	preds := secretWatchPredicates()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s1"}}
	if !preds.Create(event.CreateEvent{Object: secret}) {
		t.Fatal("expected create")
	}
	if !preds.Delete(event.DeleteEvent{Object: secret}) {
		t.Fatal("expected delete")
	}
	old := &corev1.Secret{Data: map[string][]byte{"password": []byte("a")}}
	newDiff := &corev1.Secret{Data: map[string][]byte{"password": []byte("b")}}
	if !preds.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: newDiff}) {
		t.Fatal("expected update on data change")
	}
	if preds.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: old}) {
		t.Fatal("expected no update when data unchanged")
	}
	rvOnlyOld := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{ResourceVersion: "10"},
		Data:       map[string][]byte{"password": []byte("a")},
	}
	rvOnlyNew := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{ResourceVersion: "11"},
		Data:       map[string][]byte{"password": []byte("a")},
	}
	if preds.Update(event.UpdateEvent{ObjectOld: rvOnlyOld, ObjectNew: rvOnlyNew}) {
		t.Fatal("expected no update on resourceVersion-only change when data present")
	}
	rvOld := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "10"}}
	rvNew := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "11"}}
	if !preds.Update(event.UpdateEvent{ObjectOld: rvOld, ObjectNew: rvNew}) {
		t.Fatal("expected update on resourceVersion change when data stripped")
	}
}

func TestSecretEnqueueMapper(t *testing.T) {
	t.Parallel()
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
	}
	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "mq-credentials"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(secret, conn).Build()
	mapper := secretEnqueueMapper(cl)
	reqs := mapper(ctx, secret)
	if len(reqs) != 1 || reqs[0].Name != "qm1" {
		t.Fatalf("reqs = %+v", reqs)
	}
}

func TestSecretContentChanged(t *testing.T) {
	t.Parallel()
	old := &corev1.Secret{Data: map[string][]byte{"password": []byte("a")}}
	newSame := &corev1.Secret{Data: map[string][]byte{"password": []byte("a")}}
	newDiff := &corev1.Secret{Data: map[string][]byte{"password": []byte("b")}}
	if secretContentChanged(old, newSame) {
		t.Fatal("expected unchanged")
	}
	if !secretContentChanged(old, newDiff) {
		t.Fatal("expected changed")
	}
	rvOnlyOld := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
		Data:       map[string][]byte{"password": []byte("a")},
	}
	rvOnlyNew := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"},
		Data:       map[string][]byte{"password": []byte("a")},
	}
	if secretContentChanged(rvOnlyOld, rvOnlyNew) {
		t.Fatal("expected unchanged when credential data is present")
	}
	strippedOld := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}
	strippedNew := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}}
	if !secretContentChanged(strippedOld, strippedNew) {
		t.Fatal("expected changed on resourceVersion when data stripped")
	}
}
