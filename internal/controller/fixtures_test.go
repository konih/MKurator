package controller

import (
	"context"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

const (
	testQueueManager = "QM1"
	testEndpoint     = "https://mq.example:9443"
	testSecretName   = "mq-credentials"
	testQueueName    = "APP.ORDERS"
	testMaxDepth     = "5000"
	testAttrMaxDepth = "maxdepth"
)

func expectDriftResyncRequeue(result ctrl.Result) {
	Expect(result.RequeueAfter).To(BeNumerically(">=", DriftResyncLower()))
	Expect(result.RequeueAfter).To(BeNumerically("<=", DriftResyncUpper()))
}

func withFixedDriftResyncInterval(d time.Duration) func() {
	prevLower, prevUpper := DriftResyncLower(), DriftResyncUpper()
	SetDriftResyncInterval(d, d)
	return func() {
		SetDriftResyncInterval(prevLower, prevUpper)
	}
}

func cleanupNamespace(ctx context.Context, ns string) {
	deleteAllOf(ctx, &messagingv1alpha1.QueueList{}, ns)
	deleteAllOf(ctx, &messagingv1alpha1.TopicList{}, ns)
	deleteAllOf(ctx, &messagingv1alpha1.ChannelList{}, ns)
	deleteAllOf(ctx, &messagingv1alpha1.ChannelAuthRuleList{}, ns)
	deleteAllOf(ctx, &messagingv1alpha1.AuthorityRecordList{}, ns)
	deleteAllOf(ctx, &messagingv1alpha1.QueueManagerConnectionList{}, ns)
	deleteAllOf(ctx, &corev1.SecretList{}, ns)
	deleteAllOf(ctx, &eventsv1.EventList{}, ns)
}

func deleteAllOf(ctx context.Context, list client.ObjectList, ns string) {
	Expect(k8sClient.List(ctx, list, client.InNamespace(ns))).To(Succeed())
	switch items := list.(type) {
	case *messagingv1alpha1.QueueList:
		for i := range items.Items {
			obj := &items.Items[i]
			obj.Finalizers = nil
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		}
	case *messagingv1alpha1.TopicList:
		for i := range items.Items {
			obj := &items.Items[i]
			obj.Finalizers = nil
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		}
	case *messagingv1alpha1.ChannelList:
		for i := range items.Items {
			obj := &items.Items[i]
			obj.Finalizers = nil
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		}
	case *messagingv1alpha1.ChannelAuthRuleList:
		for i := range items.Items {
			obj := &items.Items[i]
			obj.Finalizers = nil
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		}
	case *messagingv1alpha1.AuthorityRecordList:
		for i := range items.Items {
			obj := &items.Items[i]
			obj.Finalizers = nil
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		}
	case *messagingv1alpha1.QueueManagerConnectionList:
		for i := range items.Items {
			obj := &items.Items[i]
			obj.Finalizers = nil
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		}
	case *corev1.SecretList:
		for i := range items.Items {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &items.Items[i]))).To(Succeed())
		}
	case *eventsv1.EventList:
		for i := range items.Items {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &items.Items[i]))).To(Succeed())
		}
	}
}
