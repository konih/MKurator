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

var _ = Describe("AuthorityRecordReconciler", func() {
	const (
		ns      = "default"
		key     = "app-orders-get-put"
		profile = "APP.ORDERS"
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

		auth := sampleAuthorityRecord(ns, key, "qm1", profile)
		Expect(k8sClient.Create(ctx, auth)).To(Succeed())

		recorder := events.NewFakeRecorder(2)
		rec := &AuthorityRecordReconciler{
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

		updated := &messagingv1alpha1.AuthorityRecord{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionFalse))
		expectRecordedEvent(recorder, corev1.EventTypeNormal, messagingv1alpha1.ReasonProgressing)
	})

	It("applies AUTHREC when the connection is Ready", func() {
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

		auth := sampleAuthorityRecord(ns, key, "qm1", profile)
		Expect(k8sClient.Create(ctx, auth)).To(Succeed())

		desired := mqadmin.AuthoritySpec{
			Profile:     profile,
			ObjectType:  mqadmin.AuthorityObjectTypeQueue,
			Principal:   "app",
			Authorities: []string{"GET", "PUT"},
		}

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().GetAuthority(mock.Anything, desired).Return(nil, mqadmin.ErrNotFound).Once()
		mockAdmin.EXPECT().SetAuthority(mock.Anything, desired).Return(nil).Once()

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		rec := &AuthorityRecordReconciler{
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

		updated := &messagingv1alpha1.AuthorityRecord{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionSynced)).
			To(Equal(metav1.ConditionTrue))
		Expect(updated.Status.MQObjectExists).NotTo(BeNil())
		Expect(*updated.Status.MQObjectExists).To(BeTrue())
		Expect(updated.Status.Message).To(Equal("AuthorityRecord matches spec"))
		Expect(updated.Status.LastSyncTime).NotTo(BeNil())
	})

	It("skips SET when AUTHREC already matches", func() {
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

		auth := sampleAuthorityRecord(ns, key, "qm1", profile)
		auth.Finalizers = []string{messagingv1alpha1.AuthorityRecordFinalizer}
		Expect(k8sClient.Create(ctx, auth)).To(Succeed())

		desired := mqadmin.AuthoritySpec{
			Profile:     profile,
			ObjectType:  mqadmin.AuthorityObjectTypeQueue,
			Principal:   "app",
			Authorities: []string{"GET", "PUT"},
		}

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().GetAuthority(mock.Anything, desired).Return(&mqadmin.AuthorityState{
			Profile:     profile,
			ObjectType:  mqadmin.AuthorityObjectTypeQueue,
			Principal:   "app",
			Authorities: []string{"GET", "PUT"},
		}, nil).Once()

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		rec := &AuthorityRecordReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
		}

		_, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("emits a warning event when set authority fails terminally", func() {
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

		auth := sampleAuthorityRecord(ns, key, "qm1", profile)
		Expect(k8sClient.Create(ctx, auth)).To(Succeed())

		desired := mqadmin.AuthoritySpec{
			Profile:     profile,
			ObjectType:  mqadmin.AuthorityObjectTypeQueue,
			Principal:   "app",
			Authorities: []string{"GET", "PUT"},
		}

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().GetAuthority(mock.Anything, desired).Return(nil, mqadmin.ErrNotFound).Once()
		mockAdmin.EXPECT().SetAuthority(mock.Anything, desired).
			Return(&mqadmin.TerminalError{Reason: "MQSCError", Message: "set authrec failed: AMQ8405E"})

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		recorder := events.NewFakeRecorder(2)
		rec := &AuthorityRecordReconciler{
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

		auth := sampleAuthorityRecord(ns, key, "qm1", profile)
		Expect(k8sClient.Create(ctx, auth)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, auth)).To(Succeed())
		controllerutil.AddFinalizer(auth, messagingv1alpha1.AuthorityRecordFinalizer)
		Expect(k8sClient.Update(ctx, auth)).To(Succeed())

		rec := &AuthorityRecordReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
		}

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Delete(ctx, auth)).To(Succeed())

		result, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))

		updated := &messagingv1alpha1.AuthorityRecord{}
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

		auth := sampleAuthorityRecord(ns, key, "qm1", profile)
		auth.Spec.DeletionPolicy = messagingv1alpha1.DeletionPolicyOrphan
		Expect(k8sClient.Create(ctx, auth)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, auth)).To(Succeed())
		controllerutil.AddFinalizer(auth, messagingv1alpha1.AuthorityRecordFinalizer)
		Expect(k8sClient.Update(ctx, auth)).To(Succeed())

		rec := &AuthorityRecordReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
			Recorder:  testEventsRecorder(),
		}

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Delete(ctx, auth)).To(Succeed())

		result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: key}})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		updated := &messagingv1alpha1.AuthorityRecord{}
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

		auth := sampleAuthorityRecord(ns, key, "qm1", profile)
		Expect(k8sClient.Create(ctx, auth)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, auth)).To(Succeed())
		controllerutil.AddFinalizer(auth, messagingv1alpha1.AuthorityRecordFinalizer)
		Expect(k8sClient.Update(ctx, auth)).To(Succeed())

		rec := &AuthorityRecordReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
			Recorder:  testEventsRecorder(),
		}

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, auth)).To(Succeed())
		if auth.Annotations == nil {
			auth.Annotations = map[string]string{}
		}
		auth.Annotations[messagingv1alpha1.ForceOrphanAnnotation] = "true"
		Expect(k8sClient.Update(ctx, auth)).To(Succeed())
		Expect(k8sClient.Delete(ctx, auth)).To(Succeed())

		result, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		updated := &messagingv1alpha1.AuthorityRecord{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("removes AUTHREC on delete", func() {
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

		auth := sampleAuthorityRecord(ns, key, "qm1", profile)
		Expect(k8sClient.Create(ctx, auth)).To(Succeed())

		desired := mqadmin.AuthoritySpec{
			Profile:     profile,
			ObjectType:  mqadmin.AuthorityObjectTypeQueue,
			Principal:   "app",
			Authorities: []string{"GET", "PUT"},
		}

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().GetAuthority(mock.Anything, desired).Return(nil, mqadmin.ErrNotFound).Once()
		mockAdmin.EXPECT().SetAuthority(mock.Anything, desired).Return(nil).Once()
		mockAdmin.EXPECT().DeleteAuthority(mock.Anything, desired).Return(nil).Once()

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		rec := &AuthorityRecordReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
		}

		_, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Delete(ctx, auth)).To(Succeed())
		_, err = rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			err := k8sClient.Get(
				ctx,
				types.NamespacedName{Namespace: ns, Name: key},
				&messagingv1alpha1.AuthorityRecord{},
			)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}).Should(Succeed())
	})
})

func sampleAuthorityRecord(ns, name, connName, profile string) *messagingv1alpha1.AuthorityRecord {
	return &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: connName},
			Profile:       profile,
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}
}
