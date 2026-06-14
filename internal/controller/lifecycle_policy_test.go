package controller

import (
	"testing"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

func TestOrphanDeletionRequested(t *testing.T) {
	t.Parallel()
	q := &messagingv1alpha1.Queue{}
	if orphanDeletionRequested(q) {
		t.Fatal("expected false")
	}
	q.Annotations = map[string]string{messagingv1alpha1.ForceOrphanAnnotation: "true"}
	if !orphanDeletionRequested(q) {
		t.Fatal("expected force-orphan")
	}
	q.Annotations = nil
	q.Spec.DeletionPolicy = messagingv1alpha1.DeletionPolicyOrphan
	if !orphanDeletionRequested(q) {
		t.Fatal("expected Orphan policy")
	}
}
