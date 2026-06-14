package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

// MQObject is implemented by workload CRs reconciled against IBM MQ. Method sets live
// on api/v1alpha1 types; the interface lives here so controller-gen does not deepcopy it.
type MQObject interface {
	GetMQConditions() *[]metav1.Condition
	GetMQStatusFields() *messagingv1alpha1.MQObjectStatusFields
	GetStatusObservedGeneration() *int64
	SetStatusObservedGeneration(int64)
	ConnectionRefName() string
}
