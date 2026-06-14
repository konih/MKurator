package health

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func TestEvaluateMQConnectivity(t *testing.T) {
	t.Parallel()

	ready := func(name string) messagingv1alpha1.QueueManagerConnection {
		return messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Status: messagingv1alpha1.QueueManagerConnectionStatus{
				Conditions: []metav1.Condition{{
					Type:   messagingv1alpha1.ConditionReady,
					Status: metav1.ConditionTrue,
					Reason: messagingv1alpha1.ReasonAvailable,
				}},
			},
		}
	}
	unready := func(name string) messagingv1alpha1.QueueManagerConnection {
		return messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Status: messagingv1alpha1.QueueManagerConnectionStatus{
				Conditions: []metav1.Condition{{
					Type:   messagingv1alpha1.ConditionReady,
					Status: metav1.ConditionFalse,
					Reason: messagingv1alpha1.ReasonError,
				}},
			},
		}
	}
	deleting := func(name string) messagingv1alpha1.QueueManagerConnection {
		conn := unready(name)
		now := metav1.Now()
		conn.DeletionTimestamp = &now
		return conn
	}

	tests := []struct {
		name    string
		conns   []messagingv1alpha1.QueueManagerConnection
		wantErr error
	}{
		{name: "no QMCs", conns: nil},
		{name: "only deleting unready", conns: []messagingv1alpha1.QueueManagerConnection{deleting("gone")}},
		{name: "one ready", conns: []messagingv1alpha1.QueueManagerConnection{ready("qm1")}},
		{
			name: "one ready one failing",
			conns: []messagingv1alpha1.QueueManagerConnection{
				unready("bad"),
				ready("good"),
			},
		},
		{
			name:    "all unready",
			conns:   []messagingv1alpha1.QueueManagerConnection{unready("a"), unready("b")},
			wantErr: ErrNoHealthyQMC,
		},
		{
			name:    "no status yet",
			conns:   []messagingv1alpha1.QueueManagerConnection{{ObjectMeta: metav1.ObjectMeta{Name: "new"}}},
			wantErr: ErrNoHealthyQMC,
		},
		{
			name: "deleting ignored unready active blocks",
			conns: []messagingv1alpha1.QueueManagerConnection{
				deleting("old"),
				unready("active"),
			},
			wantErr: ErrNoHealthyQMC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := EvaluateMQConnectivity(tt.conns)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("EvaluateMQConnectivity() err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("EvaluateMQConnectivity() unexpected err: %v", err)
			}
		})
	}
}
