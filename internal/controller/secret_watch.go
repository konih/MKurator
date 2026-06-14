package controller

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func connectionReferencesSecret(conn *messagingv1alpha1.QueueManagerConnection, secretName string) bool {
	if conn.Spec.CredentialsSecretRef.Name == secretName {
		return true
	}
	if conn.Spec.TLS != nil && conn.Spec.TLS.CASecretRef != nil &&
		conn.Spec.TLS.CASecretRef.Name == secretName {
		return true
	}
	return false
}

func requestsForSecret(
	ctx context.Context,
	c client.Client,
	secret *corev1.Secret,
) []reconcile.Request {
	logger := log.FromContext(ctx)
	connList := &messagingv1alpha1.QueueManagerConnectionList{}
	if err := c.List(ctx, connList, client.InNamespace(secret.Namespace)); err != nil {
		logger.Error(err, "list QueueManagerConnections for secret watch",
			"namespace", secret.Namespace, "secret", secret.Name)
		return nil
	}

	var reqs []reconcile.Request
	for i := range connList.Items {
		conn := &connList.Items[i]
		if connectionReferencesSecret(conn, secret.Name) {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: secret.Namespace, Name: conn.Name},
			})
		}
	}
	return reqs
}

func secretEnqueueMapper(c client.Client) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}
		return requestsForSecret(ctx, c, secret)
	}
}

func secretCredentialsStripped(secret *corev1.Secret) bool {
	return len(secret.Data) == 0 && len(secret.StringData) == 0
}

func secretContentChanged(oldSecret, newSecret *corev1.Secret) bool {
	dataChanged := !reflect.DeepEqual(oldSecret.Data, newSecret.Data) ||
		!reflect.DeepEqual(oldSecret.StringData, newSecret.StringData)
	if dataChanged {
		return true
	}
	// ResourceVersion catches rotations when the informer cache strips Secret data (ADR-0023 / ARCH-05).
	if secretCredentialsStripped(oldSecret) && secretCredentialsStripped(newSecret) &&
		oldSecret.ResourceVersion != "" && newSecret.ResourceVersion != "" &&
		oldSecret.ResourceVersion != newSecret.ResourceVersion {
		return true
	}
	return false
}

func secretWatchPredicates() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.(*corev1.Secret)
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret, okOld := e.ObjectOld.(*corev1.Secret)
			newSecret, okNew := e.ObjectNew.(*corev1.Secret)
			if !okOld || !okNew {
				return false
			}
			return secretContentChanged(oldSecret, newSecret)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			_, ok := e.Object.(*corev1.Secret)
			return ok
		},
	}
}
