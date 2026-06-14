package controller

import (
	"context"
	"time"

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

		recorder := events.NewFakeRecorder(2)
		rec := &QueueReconciler{
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

		updated := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionFalse))
		expectRecordedEvent(recorder, corev1.EventTypeNormal, messagingv1alpha1.ReasonProgressing)
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

		recorder := events.NewFakeRecorder(2)
		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
			Recorder:  recorder,
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
		expectDriftResyncRequeue(result)

		updated := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionTrue))
		Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
		expectRecordedEvent(recorder, corev1.EventTypeNormal, messagingv1alpha1.ReasonAvailable)
	})

	It("schedules periodic resync and re-detects MQ drift after successful sync (T3)", func() {
		reset := withFixedDriftResyncInterval(5 * time.Minute)
		defer reset()

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

		queueSpec := mqadmin.QueueSpec{
			Name: testQueueName,
			Type: mqadmin.QueueTypeLocal,
			Attributes: map[string]string{
				testAttrMaxDepth: testMaxDepth,
			},
		}

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().
			GetQueue(mock.Anything, queueSpec).
			Return(&mqadmin.QueueState{
				Attributes: map[string]string{testAttrMaxDepth: testMaxDepth},
			}, nil).
			Once()
		mockAdmin.EXPECT().
			GetQueue(mock.Anything, queueSpec).
			Return(&mqadmin.QueueState{
				Attributes: map[string]string{testAttrMaxDepth: "1000"},
			}, nil).
			Once()
		mockAdmin.EXPECT().
			DefineQueue(mock.Anything, queueSpec).
			Return(nil).
			Once()

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().
			ForConnection(mock.Anything, mock.Anything).
			Return(mockAdmin, nil)

		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
			Recorder:  testEventsRecorder(),
		}

		_, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		result, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(5 * time.Minute))

		updated := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionTrue))

		result, err = rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		expectDriftResyncRequeue(result)

		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionTrue))
	})

	It("emits a warning event when define queue fails terminally", func() {
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
			GetQueue(mock.Anything, mock.Anything).
			Return(nil, &mqadmin.NotFoundError{Object: testQueueName})
		mockAdmin.EXPECT().
			DefineQueue(mock.Anything, mock.Anything).
			Return(&mqadmin.TerminalError{Reason: "MQSCError", Message: "define failed: AMQ8405E"})

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().
			ForConnection(mock.Anything, mock.Anything).
			Return(mockAdmin, nil)

		recorder := events.NewFakeRecorder(2)
		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
			Recorder:  recorder,
		}

		_, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		expectRecordedEvent(recorder, corev1.EventTypeWarning, "MQSCError")
	})

	It("requeues deletion when the connection is missing instead of failing terminally (T1)", func() {
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
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, q)).To(Succeed())
		controllerutil.AddFinalizer(q, messagingv1alpha1.QueueFinalizer)
		Expect(k8sClient.Update(ctx, q)).To(Succeed())

		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
		}

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Delete(ctx, q)).To(Succeed())

		result, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))

		updated := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(updated.DeletionTimestamp).NotTo(BeZero())
		Expect(conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(messagingv1alpha1.ReasonProgressing))
	})

	It("removes the finalizer on deletionPolicy Orphan without MQ connectivity (T1)", func() {
		conn := readyConnection(ns, "qm1")
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())
		conn.Status = messagingv1alpha1.QueueManagerConnectionStatus{Conditions: []metav1.Condition{{
			Type:               messagingv1alpha1.ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             messagingv1alpha1.ReasonAvailable,
			LastTransitionTime: metav1.Now(),
		}}}
		Expect(k8sClient.Status().Update(ctx, conn)).To(Succeed())
		q := sampleQueue(ns, key, "qm1", testQueueName)
		q.Spec.DeletionPolicy = messagingv1alpha1.DeletionPolicyOrphan
		Expect(k8sClient.Create(ctx, q)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, q)).To(Succeed())
		controllerutil.AddFinalizer(q, messagingv1alpha1.QueueFinalizer)
		Expect(k8sClient.Update(ctx, q)).To(Succeed())
		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
			Recorder:  testEventsRecorder(),
		}
		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Delete(ctx, q)).To(Succeed())
		result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: key}})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))
		updated := &messagingv1alpha1.Queue{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("removes the finalizer on force-orphan without MQ connectivity (T1)", func() {
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
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, q)).To(Succeed())
		controllerutil.AddFinalizer(q, messagingv1alpha1.QueueFinalizer)
		Expect(k8sClient.Update(ctx, q)).To(Succeed())

		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
			Recorder:  testEventsRecorder(),
		}

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, q)).To(Succeed())
		if q.Annotations == nil {
			q.Annotations = map[string]string{}
		}
		q.Annotations[messagingv1alpha1.ForceOrphanAnnotation] = "true"
		Expect(k8sClient.Update(ctx, q)).To(Succeed())
		Expect(k8sClient.Delete(ctx, q)).To(Succeed())

		result, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		updated := &messagingv1alpha1.Queue{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("skips MQ ops and sets Suspended when spec.suspend is true", func() {
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
		q.Spec.Suspend = true
		Expect(k8sClient.Create(ctx, q)).To(Succeed())

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
			Recorder:  testEventsRecorder(),
		}

		result, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeZero())

		updated := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionFalse))
		Expect(conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(messagingv1alpha1.ReasonSuspended))

		result, err = rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeZero())
	})

	It("reconciles when reconcile-requested-at annotation changes", func() {
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
			GetQueue(mock.Anything, mock.Anything).
			Return(nil, &mqadmin.NotFoundError{Object: testQueueName})
		mockAdmin.EXPECT().
			DefineQueue(mock.Anything, mock.Anything).
			Return(nil)

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().
			ForConnection(mock.Anything, mock.Anything).
			Return(mockAdmin, nil).
			Times(3)

		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
			Recorder:  testEventsRecorder(),
		}

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: key}}
		_, err := rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		updated := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		if updated.Annotations == nil {
			updated.Annotations = map[string]string{}
		}
		updated.Annotations[messagingv1alpha1.ReconcileRequestedAtAnnotation] = time.Now().UTC().Format(time.RFC3339)
		Expect(k8sClient.Update(ctx, updated)).To(Succeed())

		_, err = rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionTrue))
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

func conditionReason(conditions []metav1.Condition, condType string) string {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Reason
		}
	}
	return ""
}

func expectRecordedEvent(recorder *events.FakeRecorder, eventType, reason string) {
	select {
	case ev := <-recorder.Events:
		Expect(ev).To(ContainSubstring(eventType))
		Expect(ev).To(ContainSubstring(reason))
	case <-time.After(time.Second):
		Fail("expected kubernetes event")
	}
}
