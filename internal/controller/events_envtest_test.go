package controller

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/mqadmin"
	mqadmintest "github.com/konih/kurator/test/mocks/mqadmin"
)

const testEventRecorderName = "kurator-controller-manager"

var (
	testEventBroadcaster      events.EventBroadcaster
	testEventBroadcasterOnce  sync.Once
	testEventBroadcasterReady = make(chan struct{})
)

var _ = Describe("events.k8s.io reconcile events", func() {
	const ns = "default"

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

	It("records Progressing on Queue when the connection is not Ready", func() {
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

		key := "orders-progressing"
		q := sampleQueue(ns, key, "qm1", testQueueName)
		Expect(k8sClient.Create(ctx, q)).To(Succeed())

		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mqadmintest.NewMockFactory(GinkgoT()),
			Recorder:  testEventsRecorder(),
		}
		_, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		eventuallyExpectEventsAPIEvent(
			ctx, ns, "Queue", key, corev1.EventTypeNormal, messagingv1alpha1.ReasonProgressing,
		)
	})

	It("records Available on Queue when reconcile succeeds", func() {
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

		key := "orders-available"
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
		mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

		rec := &QueueReconciler{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			MQFactory: mockFactory,
			Recorder:  testEventsRecorder(),
		}

		_, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		eventuallyExpectEventsAPIEvent(
			ctx, ns, "Queue", key, corev1.EventTypeNormal, messagingv1alpha1.ReasonAvailable,
		)
	})

	It("records Available on Topic when reconcile succeeds", func() {
		const (
			key       = "retail-orders"
			topicName = "RETAIL.ORDERS"
		)

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
			Recorder:  testEventsRecorder(),
		}

		_, err := rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = rec.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: key},
		})
		Expect(err).NotTo(HaveOccurred())

		eventuallyExpectEventsAPIEvent(
			ctx, ns, "Topic", key, corev1.EventTypeNormal, messagingv1alpha1.ReasonAvailable,
		)
	})
})

func testEventsRecorder() events.EventRecorder {
	testEventBroadcasterOnce.Do(func() {
		cs, err := kubernetes.NewForConfig(testEnv.Config)
		Expect(err).NotTo(HaveOccurred())
		testEventBroadcaster = events.NewBroadcaster(&events.EventSinkImpl{Interface: cs.EventsV1()})
		go func() {
			defer GinkgoRecover()
			close(testEventBroadcasterReady)
			Expect(testEventBroadcaster.StartRecordingToSinkWithContext(context.Background())).To(Succeed())
		}()
	})
	<-testEventBroadcasterReady
	return testEventBroadcaster.NewRecorder(k8sClient.Scheme(), testEventRecorderName)
}

func eventuallyExpectEventsAPIEvent(
	ctx context.Context,
	ns, kind, name, eventType, reason string,
) {
	Eventually(func(g Gomega) {
		g.Expect(hasEventsAPIEvent(ctx, ns, kind, name, eventType, reason)).To(BeTrue(),
			"expected %s event with reason %q on %s/%s (events.k8s.io)", eventType, reason, kind, name)
	}).WithTimeout(5 * time.Second).WithPolling(200 * time.Millisecond).Should(Succeed())
}

func hasEventsAPIEvent(ctx context.Context, ns, kind, name, eventType, reason string) bool {
	var list eventsv1.EventList
	if err := k8sClient.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return false
	}
	for _, ev := range list.Items {
		if ev.Regarding.Kind != kind || ev.Regarding.Name != name {
			continue
		}
		if ev.Type != eventType || ev.Reason != reason {
			continue
		}
		gvk := ev.GetObjectKind().GroupVersionKind()
		if gvk.Group != "" && (gvk.Group != "events.k8s.io" || gvk.Version != "v1") {
			continue
		}
		if ev.Action != eventActionReconcile {
			continue
		}
		return true
	}
	return false
}
