// Package health implements operator probe checks beyond controller-runtime defaults.
package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

// ErrNoHealthyQMC is returned when at least one non-deleting QueueManagerConnection
// exists but none reports Ready=True (successful mqweb ping).
var ErrNoHealthyQMC = errors.New("no QueueManagerConnection reports Ready=True")

// EvaluateMQConnectivity decides whether the operator should be considered ready
// to reconcile MQ-backed resources from connection status alone.
//
//   - No non-deleting QueueManagerConnections → ready (MQ not configured yet).
//   - At least one non-deleting QMC with Ready=True → ready.
//   - Otherwise → not ready (all connections down or still progressing).
func EvaluateMQConnectivity(conns []messagingv1alpha1.QueueManagerConnection) error {
	active := 0
	for i := range conns {
		conn := &conns[i]
		if !conn.DeletionTimestamp.IsZero() {
			continue
		}
		active++
		if qmcReady(conn) {
			return nil
		}
	}
	if active == 0 {
		return nil
	}
	return ErrNoHealthyQMC
}

func qmcReady(conn *messagingv1alpha1.QueueManagerConnection) bool {
	for _, c := range conn.Status.Conditions {
		if c.Type == messagingv1alpha1.ConditionReady && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// MQConnectivityChecker lists QueueManagerConnections and applies EvaluateMQConnectivity.
type MQConnectivityChecker struct {
	Client client.Reader
}

// Check implements healthz.Checker.
func (c *MQConnectivityChecker) Check(_ *http.Request) error {
	var list messagingv1alpha1.QueueManagerConnectionList
	if err := c.Client.List(context.Background(), &list); err != nil {
		return fmt.Errorf("list QueueManagerConnections: %w", err)
	}
	return EvaluateMQConnectivity(list.Items)
}

// NewMQConnectivityChecker returns a readyz checker backed by the API reader (cache).
func NewMQConnectivityChecker(reader client.Reader) healthz.Checker {
	return (&MQConnectivityChecker{Client: reader}).Check
}
