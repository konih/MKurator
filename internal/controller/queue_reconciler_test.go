package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	messagingv1alpha1 "github.com/konradheimel/kurator/api/v1alpha1"
	"github.com/konradheimel/kurator/internal/mqadmin"
	mqadmintest "github.com/konradheimel/kurator/test/mocks/mqadmin"
)

var _ = Describe("QueueReconciler", func() {
	const (
		ns  = "default"
		key = "orders"
	)

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cleanupNamespace(context.Background(), ns)
		cancel()
	})

	It("requeues when the connection is not Ready", func() {
		conn := &messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
			Spec: messagingv1alpha1.QueueManagerConnectionSpec{
				QueueManager: testQueueManager,
				Endpoint:     testEndpoint,
				CredentialsSecretRef: messagingv1alpha1.SecretReference{
					Name: testSecretName,
				},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		q := sampleQueue(ns, key, "qm1", testQueueName)
		Expect(k8sClient.Create(ctx, q)).To(Succeed())

		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
		}
		result, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))

		updated := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionFalse))
	})

	It("defines the queue when the connection is Ready", func() {
		conn := readyConnection(ns, "qm1")
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())
		conn.Status = messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:               messagingv1alpha1.ConditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             messagingv1alpha1.ReasonAvailable,
				LastTransitionTime: metav1.Now(),
			}},
		}
		Expect(k8sClient.Status().Update(ctx, conn)).To(Succeed())

		q := sampleQueue(ns, key, "qm1", testQueueName)
		Expect(k8sClient.Create(ctx, q)).To(Succeed())

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().
			GetQueue(mock.Anything, mqadmin.QueueSpec{
				Name: testQueueName,
				Type: mqadmin.QueueTypeLocal,
				Attributes: map[string]string{
					testAttrMaxDepth: testMaxDepth,
				},
			}).
			Return(nil, &mqadmin.NotFoundError{Object: testQueueName})
		mockAdmin.EXPECT().
			DefineQueue(mock.Anything, mqadmin.QueueSpec{
				Name: testQueueName,
				Type: mqadmin.QueueTypeLocal,
				Attributes: map[string]string{
					testAttrMaxDepth: testMaxDepth,
				},
			}).
			Return(nil)

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().
			ForConnection(mock.Anything, mock.Anything).
			Return(mockAdmin, nil)

		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
		}

		// First reconcile adds the finalizer.
		_, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		result, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		updated := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionTrue))
		Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
	})
})

func sampleQueue(ns, name, connName, queueName string) *messagingv1alpha1.Queue {
	return &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: connName},
			QueueName:     queueName,
			Type:          messagingv1alpha1.QueueTypeLocal,
			Attributes: map[string]string{
				testAttrMaxDepth: testMaxDepth,
			},
		},
	}
}

func readyConnection(ns, name string) *messagingv1alpha1.QueueManagerConnection {
	return &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: testQueueManager,
			Endpoint:     testEndpoint,
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: testSecretName,
			},
		},
		Status: messagingv1alpha1.QueueManagerConnectionStatus{
			Conditions: []metav1.Condition{{
				Type:   messagingv1alpha1.ConditionReady,
				Status: metav1.ConditionTrue,
				Reason: messagingv1alpha1.ReasonAvailable,
			}},
		},
	}
}

func conditionStatus(conditions []metav1.Condition, condType string) metav1.ConditionStatus {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Status
		}
	}
	return ""
}
