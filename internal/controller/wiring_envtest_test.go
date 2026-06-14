package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	mqadmintest "github.com/conduit-ops/mkurator/test/mocks/mqadmin"
)

var _ = Describe("Controller wiring", func() {
	It("registers all reconcilers with the manager", func() {
		cfg := testEnv.Config
		Expect(cfg).NotTo(BeNil())

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:  scheme.Scheme,
			Metrics: metricsserver.Options{BindAddress: "0"},
		})
		Expect(err).NotTo(HaveOccurred())

		mockFactory := mqadmintest.NewMockFactory(GinkgoT())

		Expect((&QueueManagerConnectionReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			MQFactory: mockFactory,
		}).SetupWithManager(mgr)).To(Succeed())

		Expect((&QueueReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			MQFactory: mockFactory,
		}).SetupWithManager(mgr)).To(Succeed())

		Expect((&TopicReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			MQFactory: mockFactory,
		}).SetupWithManager(mgr)).To(Succeed())

		Expect((&ChannelReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			MQFactory: mockFactory,
		}).SetupWithManager(mgr)).To(Succeed())

		Expect((&ChannelAuthRuleReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			MQFactory: mockFactory,
		}).SetupWithManager(mgr)).To(Succeed())

		Expect((&AuthorityRecordReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			MQFactory: mockFactory,
		}).SetupWithManager(mgr)).To(Succeed())
	})
})
