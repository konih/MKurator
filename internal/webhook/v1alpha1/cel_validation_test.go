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

	It("rejects alias Queue without targq", func() {
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
		Expect(err.Error()).To(ContainSubstring("targq"))
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
