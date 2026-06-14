package webhookv1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

var _ = Describe("CEL validation parity", func() {
	const ns = "cel-parity"

	BeforeEach(func() {
		ctx := context.Background()
		_ = webhookK8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	})

	It("rejects lowercase queueName", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-q1", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "app.orders",
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
	})

	It("rejects alias Queue without targetQueue or targq", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-alias", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "ALIAS.Q",
				Type:          messagingv1alpha1.QueueTypeAlias,
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("targetQueue"))
	})

	It("rejects remote Queue without xmitQueue or xmitq", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-remote-xmit", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef:      messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:          "REMOTE.Q",
				Type:               messagingv1alpha1.QueueTypeRemote,
				RemoteQueueManager: "QM2",
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("xmitQueue"))
	})

	It("rejects remote Queue without remoteQueueManager or rqmname", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-remote-rqm", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "REMOTE.Q",
				Type:          messagingv1alpha1.QueueTypeRemote,
				XmitQueue:     "SYSTEM.DEFAULT.XMIT.QUEUE",
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("remoteQueueManager"))
	})

	It("rejects remote Queue with both xmitQueue and attributes.xmitq", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-remote-xmit-both", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef:      messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:          "REMOTE.Q",
				Type:               messagingv1alpha1.QueueTypeRemote,
				XmitQueue:          "SYSTEM.DEFAULT.XMIT.QUEUE",
				RemoteQueueManager: "QM2",
				Attributes:         map[string]string{"xmitq": "SYSTEM.DEFAULT.XMIT.QUEUE"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("xmitQueue"))
	})

	It("rejects remote Queue with both remoteQueueManager and attributes.rqmname", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-remote-rqm-both", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef:      messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:          "REMOTE.Q",
				Type:               messagingv1alpha1.QueueTypeRemote,
				XmitQueue:          "SYSTEM.DEFAULT.XMIT.QUEUE",
				RemoteQueueManager: "QM2",
				Attributes:         map[string]string{"rqmname": "QM2"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("remoteQueueManager"))
	})

	It("rejects alias Queue with both targetQueue and attributes.targq", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-alias-both", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "ALIAS.Q",
				Type:          messagingv1alpha1.QueueTypeAlias,
				TargetQueue:   "APP.ORDERS",
				Attributes:    map[string]string{"targq": "APP.ORDERS"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("targetQueue"))
	})

	It("rejects Topic with both topicString and attributes.topstr", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Topic{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-topstr", Namespace: ns},
			Spec: messagingv1alpha1.TopicSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				TopicName:     "RETAIL.ORDERS",
				TopicString:   "retail/orders",
				Attributes:    map[string]string{"topstr": "retail/orders"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("topicString"))
	})

	It("rejects Topic with both description and attributes.descr", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Topic{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-topic-descr", Namespace: ns},
			Spec: messagingv1alpha1.TopicSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				TopicName:     "RETAIL.ORDERS",
				Description:   "Retail orders topic",
				Attributes:    map[string]string{"descr": "Retail orders topic"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("description"))
	})

	It("rejects Channel with both description and attributes.descr", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Channel{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-channel-descr", Namespace: ns},
			Spec: messagingv1alpha1.ChannelSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				Description:   "Application server-connection channel",
				Attributes:    map[string]string{"descr": "Application server-connection channel"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("description"))
	})

	It("rejects Channel with both maxMsgLength and attributes.maxmsgl", func() {
		ctx := context.Background()
		maxMsgLength := int32(4194304)
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Channel{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-channel-maxmsgl", Namespace: ns},
			Spec: messagingv1alpha1.ChannelSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				MaxMsgLength:  &maxMsgLength,
				Attributes:    map[string]string{"maxmsgl": "4194304"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("maxMsgLength"))
	})

	It("rejects Queue with both maxDepth and attributes.maxdepth", func() {
		ctx := context.Background()
		depth := int32(5000)
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-maxdepth", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "APP.ORDERS",
				MaxDepth:      &depth,
				Attributes:    map[string]string{"maxdepth": "5000"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("maxDepth"))
	})

	It("rejects Queue with both description and attributes.descr", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-descr", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "APP.ORDERS",
				Description:   "Orders queue",
				Attributes:    map[string]string{"descr": "Orders queue"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("description"))
	})

	It("rejects Queue with both defPersistence and attributes.defpsist", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-defpsist", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef:  messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:      "APP.ORDERS",
				DefPersistence: messagingv1alpha1.QueueDefaultPersistenceYes,
				Attributes:     map[string]string{"defpsist": "yes"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("defPersistence"))
	})

	It("rejects Queue with both get and attributes.get", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-get", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "APP.ORDERS",
				Get:           messagingv1alpha1.QueueAccessEnabledEnabled,
				Attributes:    map[string]string{"get": "enabled"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("get"))
	})

	It("rejects Queue with both put and attributes.put", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-put", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "APP.ORDERS",
				Put:           messagingv1alpha1.QueueAccessEnabledDisabled,
				Attributes:    map[string]string{"put": "disabled"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("put"))
	})

	It("rejects ChannelAuthRule ADDRESSMAP without address", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.ChannelAuthRule{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-car", Namespace: ns},
			Spec: messagingv1alpha1.ChannelAuthRuleSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("ADDRESSMAP"))
	})

	It("rejects AuthorityRecord with both principal and group", func() {
		ctx := context.Background()
		err := webhookK8sClient.Create(ctx, &messagingv1alpha1.AuthorityRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-auth1", Namespace: ns},
			Spec: messagingv1alpha1.AuthorityRecordSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				Profile:       "APP.ORDERS",
				ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
				Principal:     "app",
				Group:         "apps",
				Authorities:   []string{"GET"},
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("principal or group"))
	})
})
