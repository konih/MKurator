package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/mqadmin"
	mqadmintest "github.com/konih/kurator/test/mocks/mqadmin"
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
		Expect(result).To(Equal(ctrl.Result{}))

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
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, &messagingv1alpha1.AuthorityRecord{})
			g.Expect(k8serrors.IsNotFound(err)).To(BeTrue())
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
