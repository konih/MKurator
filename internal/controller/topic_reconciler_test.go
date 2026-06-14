package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
	mqadmintest "github.com/conduit-ops/mkurator/test/mocks/mqadmin"
)

var _ = Describe("TopicReconciler", func() {
	const (
		ns        = "default"
		key       = "retail-orders"
		topicName = "RETAIL.ORDERS"
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

		topic := sampleTopic(ns, key, "qm1", topicName)
		Expect(k8sClient.Create(ctx, topic)).To(Succeed())

		recorder := events.NewFakeRecorder(2)
		rec := &TopicReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
			Recorder:  recorder,
		}
		result, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))

		updated := &messagingv1alpha1.Topic{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionFalse))
		expectRecordedEvent(recorder, corev1.EventTypeNormal, messagingv1alpha1.ReasonProgressing)
	})

	It("defines the topic when the connection is Ready", func() {
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

		topic := sampleTopic(ns, key, "qm1", topicName)
		Expect(k8sClient.Create(ctx, topic)).To(Succeed())

		desired := mqadmin.TopicSpec{
			Name: topicName,
			Attributes: map[string]string{
				"topstr": "retail/orders",
			},
		}

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().GetTopic(mock.Anything, topicName).Return(nil, &mqadmin.NotFoundError{Object: topicName})
		mockAdmin.EXPECT().DefineTopic(mock.Anything, desired).Return(nil)

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		rec := &TopicReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
		}

		_, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		result, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		expectDriftResyncRequeue(result)

		updated := &messagingv1alpha1.Topic{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionTrue))
	})

	It("emits a warning event when define topic fails terminally", func() {
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

		topic := sampleTopic(ns, key, "qm1", topicName)
		Expect(k8sClient.Create(ctx, topic)).To(Succeed())

		desired := mqadmin.TopicSpec{
			Name: topicName,
			Attributes: map[string]string{
				"topstr": "retail/orders",
			},
		}

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().GetTopic(mock.Anything, topicName).Return(nil, &mqadmin.NotFoundError{Object: topicName})
		mockAdmin.EXPECT().DefineTopic(mock.Anything, desired).
			Return(&mqadmin.TerminalError{Reason: "MQSCError", Message: "define failed: AMQ8405E"})

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		recorder := events.NewFakeRecorder(2)
		rec := &TopicReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
			Recorder:  recorder,
		}

		_, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		expectRecordedEvent(recorder, corev1.EventTypeWarning, "MQSCError")
	})

	It("requeues deletion when the connection is missing instead of failing terminally", func() {
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

		topic := sampleTopic(ns, key, "qm1", topicName)
		Expect(k8sClient.Create(ctx, topic)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, topic)).To(Succeed())
		controllerutil.AddFinalizer(topic, messagingv1alpha1.TopicFinalizer)
		Expect(k8sClient.Update(ctx, topic)).To(Succeed())

		rec := &TopicReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
		}

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Delete(ctx, topic)).To(Succeed())

		result, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))

		updated := &messagingv1alpha1.Topic{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(updated.DeletionTimestamp).NotTo(BeZero())
		Expect(conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(messagingv1alpha1.ReasonProgressing))
	})

	It("removes the finalizer on deletionPolicy Orphan without MQ connectivity", func() {
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

		topic := sampleTopic(ns, key, "qm1", topicName)
		topic.Spec.DeletionPolicy = messagingv1alpha1.DeletionPolicyOrphan
		Expect(k8sClient.Create(ctx, topic)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, topic)).To(Succeed())
		controllerutil.AddFinalizer(topic, messagingv1alpha1.TopicFinalizer)
		Expect(k8sClient.Update(ctx, topic)).To(Succeed())

		rec := &TopicReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
			Recorder:  testEventsRecorder(),
		}

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Delete(ctx, topic)).To(Succeed())

		result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: key}})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		updated := &messagingv1alpha1.Topic{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("removes the finalizer on force-orphan without MQ connectivity", func() {
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

		topic := sampleTopic(ns, key, "qm1", topicName)
		Expect(k8sClient.Create(ctx, topic)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, topic)).To(Succeed())
		controllerutil.AddFinalizer(topic, messagingv1alpha1.TopicFinalizer)
		Expect(k8sClient.Update(ctx, topic)).To(Succeed())

		rec := &TopicReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
			Recorder:  testEventsRecorder(),
		}

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, topic)).To(Succeed())
		if topic.Annotations == nil {
			topic.Annotations = map[string]string{}
		}
		topic.Annotations[messagingv1alpha1.ForceOrphanAnnotation] = "true"
		Expect(k8sClient.Update(ctx, topic)).To(Succeed())
		Expect(k8sClient.Delete(ctx, topic)).To(Succeed())

		result, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		updated := &messagingv1alpha1.Topic{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
})

func sampleTopic(ns, name, connName, topicName string) *messagingv1alpha1.Topic {
	return &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: connName},
			TopicName:     topicName,
			Attributes: map[string]string{
				"topstr": "retail/orders",
			},
		},
	}
}
