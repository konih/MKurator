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
	ctrl "sigs.k8s.io/controller-runtime"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
	mqadmintest "github.com/conduit-ops/mkurator/test/mocks/mqadmin"
)

var _ = Describe("QueueManagerConnectionReconciler", func() {
	const (
		ns  = "default"
		key = "qm1"
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

	It("sets Ready after a successful ping", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: ns},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret"),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		conn := &messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: key, Namespace: ns},
			Spec: messagingv1alpha1.QueueManagerConnectionSpec{
				QueueManager: testQueueManager,
				Endpoint:     testEndpoint,
				CredentialsSecretRef: messagingv1alpha1.SecretReference{
					Name: testSecretName,
				},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().Ping(mock.Anything).Return(nil)

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		rec := &QueueManagerConnectionReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
		}

		_, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &messagingv1alpha1.QueueManagerConnection{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(metav1.ConditionTrue))
		Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
	})

	It("sets Error when ping fails with a terminal error", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: ns},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret"),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		conn := &messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: key, Namespace: ns},
			Spec: messagingv1alpha1.QueueManagerConnectionSpec{
				QueueManager: testQueueManager,
				Endpoint:     testEndpoint,
				CredentialsSecretRef: messagingv1alpha1.SecretReference{
					Name: testSecretName,
				},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().Ping(mock.Anything).Return(&mqadmin.TerminalError{Message: "unauthorized"})

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		rec := &QueueManagerConnectionReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
		}

		_, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &messagingv1alpha1.QueueManagerConnection{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)).To(Succeed())
		Expect(conditionStatus(updated.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(metav1.ConditionFalse))
		Expect(conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(messagingv1alpha1.ReasonError))
	})

	It("removes the finalizer when the connection is deleted", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: ns},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret"),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		conn := &messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: key, Namespace: ns},
			Spec: messagingv1alpha1.QueueManagerConnectionSpec{
				QueueManager: testQueueManager,
				Endpoint:     testEndpoint,
				CredentialsSecretRef: messagingv1alpha1.SecretReference{
					Name: testSecretName,
				},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		rec := &QueueManagerConnectionReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
		}

		_, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())

		mockFactory.EXPECT().ReleaseConnection(mock.Anything, mock.Anything).Return(nil).Once()

		_, err = rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &messagingv1alpha1.QueueManagerConnection{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "finalizer removed and object deleted from API")
	})

	It("removes the finalizer when deleted after the credentials Secret is gone (T2)", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: ns},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret"),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		conn := &messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: key, Namespace: ns},
			Spec: messagingv1alpha1.QueueManagerConnectionSpec{
				QueueManager: testQueueManager,
				Endpoint:     testEndpoint,
				CredentialsSecretRef: messagingv1alpha1.SecretReference{
					Name: testSecretName,
				},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().Ping(mock.Anything).Return(nil).Maybe()

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil).Maybe()

		rec := &QueueManagerConnectionReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
		}

		_, err := rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())

		rec.MQFactory = mqrest.NewClientFactory(k8sClient)
		_, err = rec.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &messagingv1alpha1.QueueManagerConnection{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, updated)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "finalizer removed despite missing Secret")
	})

	It("emits exactly one Available event across two successful reconciles (T4)", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: ns},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret"),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		conn := &messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: key, Namespace: ns},
			Spec: messagingv1alpha1.QueueManagerConnectionSpec{
				QueueManager: testQueueManager,
				Endpoint:     testEndpoint,
				CredentialsSecretRef: messagingv1alpha1.SecretReference{
					Name: testSecretName,
				},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		mockAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		mockAdmin.EXPECT().Ping(mock.Anything).Return(nil).Times(2)

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil).Times(2)

		rec := &QueueManagerConnectionReconciler{
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

		afterReady := &messagingv1alpha1.QueueManagerConnection{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, afterReady)).To(Succeed())
		Expect(conditionStatus(afterReady.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(metav1.ConditionTrue))

		_, err = rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		afterSecond := &messagingv1alpha1.QueueManagerConnection{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, afterSecond)).To(Succeed())
		Expect(conditionStatus(afterSecond.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(metav1.ConditionTrue))

		Eventually(func(g Gomega) {
			g.Expect(countEventsAPIEvents(ctx, ns, "QueueManagerConnection", key,
				corev1.EventTypeNormal, messagingv1alpha1.ReasonAvailable)).To(Equal(1))
		}).WithTimeout(5 * time.Second).WithPolling(200 * time.Millisecond).Should(Succeed())
	})

	It("becomes Ready after credentials Secret is fixed without editing QMC (T5)", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: ns},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("bad"),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		conn := &messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: key, Namespace: ns},
			Spec: messagingv1alpha1.QueueManagerConnectionSpec{
				QueueManager: testQueueManager,
				Endpoint:     testEndpoint,
				CredentialsSecretRef: messagingv1alpha1.SecretReference{
					Name: testSecretName,
				},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		failAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		failAdmin.EXPECT().Ping(mock.Anything).Return(&mqadmin.TerminalError{
			Reason:  "Unauthorized",
			Message: "unauthorized",
		}).Once()

		failFactory := mqadmintest.NewMockFactory(GinkgoT())
		failFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(failAdmin, nil).Once()

		rec := &QueueManagerConnectionReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: failFactory,
		}
		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: key}}

		_, err := rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		failed := &messagingv1alpha1.QueueManagerConnection{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, failed)).To(Succeed())
		Expect(conditionStatus(failed.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(metav1.ConditionFalse))
		Expect(conditionReason(failed.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(messagingv1alpha1.ReasonError))
		origGen := failed.Generation

		updatedSecret := &corev1.Secret{}
		Expect(
			k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: testSecretName}, updatedSecret),
		).To(Succeed())
		updatedSecret.Data["password"] = []byte("good")
		Expect(k8sClient.Update(ctx, updatedSecret)).To(Succeed())

		reqs := requestsForSecret(ctx, k8sClient, updatedSecret)
		Expect(reqs).To(ContainElement(req))

		okAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		okAdmin.EXPECT().Ping(mock.Anything).Return(nil).Times(2)
		okFactory := mqadmintest.NewMockFactory(GinkgoT())
		okFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(okAdmin, nil).Times(2)
		rec.MQFactory = okFactory

		_, err = rec.Reconcile(ctx, reqs[0])
		Expect(err).NotTo(HaveOccurred())
		_, err = rec.Reconcile(ctx, reqs[0])
		Expect(err).NotTo(HaveOccurred())

		recovered := &messagingv1alpha1.QueueManagerConnection{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, recovered)).To(Succeed())
		Expect(recovered.Generation).To(Equal(origGen))
		Expect(conditionStatus(recovered.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(metav1.ConditionTrue))
	})

	It("preserves Ready on transient ping while already Ready (T6)", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: ns},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret"),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		conn := &messagingv1alpha1.QueueManagerConnection{
			ObjectMeta: metav1.ObjectMeta{Name: key, Namespace: ns},
			Spec: messagingv1alpha1.QueueManagerConnectionSpec{
				QueueManager: testQueueManager,
				Endpoint:     testEndpoint,
				CredentialsSecretRef: messagingv1alpha1.SecretReference{
					Name: testSecretName,
				},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		okAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		okAdmin.EXPECT().Ping(mock.Anything).Return(nil).Once()

		okFactory := mqadmintest.NewMockFactory(GinkgoT())
		okFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(okAdmin, nil).Once()

		rec := &QueueManagerConnectionReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: okFactory,
		}
		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: key}}

		_, err := rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		ready := &messagingv1alpha1.QueueManagerConnection{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, ready)).To(Succeed())
		Expect(conditionStatus(ready.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(metav1.ConditionTrue))

		transientAdmin := mqadmintest.NewMockAdmin(GinkgoT())
		transientAdmin.EXPECT().Ping(mock.Anything).Return(&mqadmin.TransientError{Message: "timeout"}).Once()
		transientFactory := mqadmintest.NewMockFactory(GinkgoT())
		transientFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(transientAdmin, nil).Once()
		rec.MQFactory = transientFactory

		result, err := rec.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(TransientRequeueInterval()))

		stillReady := &messagingv1alpha1.QueueManagerConnection{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: key}, stillReady)).To(Succeed())
		Expect(conditionStatus(stillReady.Status.Conditions, messagingv1alpha1.ConditionReady)).
			To(Equal(metav1.ConditionTrue))
	})
})
