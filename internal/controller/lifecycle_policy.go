package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func workloadLifecyclePolicies(obj client.Object) messagingv1alpha1.WorkloadLifecyclePolicies {
	switch o := obj.(type) {
	case *messagingv1alpha1.Queue:
		return o.Spec.WorkloadLifecyclePolicies
	case *messagingv1alpha1.Topic:
		return o.Spec.WorkloadLifecyclePolicies
	case *messagingv1alpha1.Channel:
		return o.Spec.WorkloadLifecyclePolicies
	case *messagingv1alpha1.ChannelAuthRule:
		return o.Spec.WorkloadLifecyclePolicies
	case *messagingv1alpha1.AuthorityRecord:
		return o.Spec.WorkloadLifecyclePolicies
	default:
		return messagingv1alpha1.WorkloadLifecyclePolicies{}
	}
}

func workloadDeletionPolicy(obj client.Object) messagingv1alpha1.DeletionPolicy {
	policy := workloadLifecyclePolicies(obj).DeletionPolicy
	if policy == "" {
		return messagingv1alpha1.DeletionPolicyDelete
	}
	return policy
}

func workloadAdoptionPolicy(obj client.Object) messagingv1alpha1.AdoptionPolicy {
	policy := workloadLifecyclePolicies(obj).AdoptionPolicy
	if policy == "" {
		return messagingv1alpha1.AdoptionPolicyAdopt
	}
	return policy
}

func workloadFirstAdoption(obj client.Object) bool {
	switch o := obj.(type) {
	case *messagingv1alpha1.Queue:
		return o.Status.ObservedGeneration == 0
	case *messagingv1alpha1.Topic:
		return o.Status.ObservedGeneration == 0
	case *messagingv1alpha1.Channel:
		return o.Status.ObservedGeneration == 0
	case *messagingv1alpha1.ChannelAuthRule:
		return o.Status.ObservedGeneration == 0
	case *messagingv1alpha1.AuthorityRecord:
		return o.Status.ObservedGeneration == 0
	default:
		return false
	}
}

func orphanDeletionRequested(obj metav1.Object) bool {
	if forceOrphanRequested(obj) {
		return true
	}
	co, ok := obj.(client.Object)
	if !ok {
		return false
	}
	return workloadDeletionPolicy(co) == messagingv1alpha1.DeletionPolicyOrphan
}
