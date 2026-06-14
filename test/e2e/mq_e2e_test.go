//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

const (
	mqConnectionName      = "e2e-qm1"
	mqQueueMaxDepthV1     = "1000"
	mqChannelAuthCRName   = "e2e-dev-app-addressmap"
	mqChannelPrereqCRName = "e2e-dev-app-channel"
	mqAuthorityCRName     = "e2e-app-orders-get-put"
)

func e2eLocalQueueSpec(name string) mqadmin.QueueSpec {
	return mqadmin.QueueSpec{Name: name, Type: mqadmin.QueueTypeLocal}
}

var mqSuiteOnce sync.Once

var _ = Describe("Post-manager IBM MQ integration", Label("mq"), func() {
	BeforeEach(func() {
		if !mqE2EEnabled() {
			Skip("IBM MQ e2e disabled; set KURATOR_E2E_MQ=1 and run task cluster:up")
		}
		mqSuiteOnce.Do(func() {
			e2eStage("MQ SUITE — IBM MQ reconcile scenarios")
			ensureMQE2ENamespaces()
			waitForControllerAndWebhookReadyCached()
		})
		if !webhookReady.Load() {
			waitForControllerAndWebhookReadyCached()
		}
	})

	Context("channel/auth fixtures", Label("smoke"), func() {
		It("confirms MQSC prerequisite channel exists on QM1", func() {
			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				ok, checkErr := channelExists(ctx, client, e2eChannelName)
				if checkErr == nil && !ok {
					g.Expect(applyMQSCFixture(ctx, client, "channel-auth-prereq.mqsc")).To(Succeed())
					ok, checkErr = channelExists(ctx, client, e2eChannelName)
				}
				g.Expect(checkErr).NotTo(HaveOccurred())
				g.Expect(ok).To(BeTrue(), "channel %s should exist after BeforeSuite fixture", e2eChannelName)
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})
	})

	Describe("queues", Label("mq-queue"), func() {
		var (
			ns          string
			prefix      string
			queueObject string
			queueCR     string
		)

		BeforeEach(func() {
			ns = namespaceQueues
			ensureE2ENamespace(ns)
			prefix = mqObjectPrefix()
			queueObject = mqQueueObjectName(prefix)
			queueCR = mqCRName("e2e-orders", prefix)
			ensureMQCredentialsSecret(ns)
			DeferCleanup(func() {
				cleanupMQSpec(ns, "queue", queueCR)
			})
		})

		It("reconciles a Queue CR against the kind IBM MQ queue manager", func() {
			Expect(applyWithWebhookRetry(connectionManifest(ns))).To(Succeed())
			eventuallyExpectQMCReady(ns)

			queueYAML := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: Queue
metadata:
  name: %s
  namespace: %s
spec:
  connectionRef:
    name: %s
  queueName: %s
  type: local
  attributes:
    maxdepth: "%s"
    descr: e2e orders queue
`, queueCR, ns, mqConnectionName, queueObject, mqQueueMaxDepthV1)

			Expect(applyWithWebhookRetry(queueYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				out, err := runKubectl("get", "queue", queueCR, "-n", ns,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(mqSyncedEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())

			eventuallyExpectQueueAvailableEvent(ns, queueCR)

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			state, err := client.GetQueue(ctx, e2eLocalQueueSpec(queueObject))
			Expect(err).NotTo(HaveOccurred())
			Expect(state.Attributes["maxdepth"]).To(Equal(mqQueueMaxDepthV1))

			Expect(kubectlDeleteNoWait("queue", queueCR, ns)).To(Succeed())

			Eventually(func(g Gomega) {
				_, err := client.GetQueue(ctx, e2eLocalQueueSpec(queueObject))
				g.Expect(err).To(HaveOccurred(), "queue %s should be removed from MQ after CR delete", queueObject)
			}).WithTimeout(KubectlWaitDuration).WithPolling(3 * time.Second).Should(Succeed())
		})

		// Queue attribute replace semantics: test/integration/mq (TestIntegration_Queue_UpdateViaReplace).

		It("recovers QueueManagerConnection readiness after secret rotation", Label("slow"), func() {
			ensureE2ENamespace(ns)
			By("creating intentionally invalid MQ credentials")
			Expect(kubectlApply(fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: mq-credentials
  namespace: %s
type: Opaque
stringData:
  username: admin
  mqAdminPassword: wrong-password
`, ns))).To(Succeed())
			Expect(kubectlApply(connectionManifest(ns))).To(Succeed())

			Eventually(func(g Gomega) {
				out, err := runKubectl("get", "queuemanagerconnection", mqConnectionName, "-n", ns,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("False"))
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("rotating secret to the valid credentials and forcing a reconcile")
			Expect(kubectlApply(fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: mq-credentials
  namespace: %s
type: Opaque
stringData:
  username: admin
  mqAdminPassword: %s
`, ns, envOr("KURATOR_E2E_MQ_PASSWORD", "passw0rd")))).To(Succeed())

			invalidateWebhookReadyCache()
			waitForControllerAndWebhookReadyCached()
			Expect(kubectlDeleteWithWebhookRetry("queuemanagerconnection", mqConnectionName, ns)).To(Succeed())
			Expect(kubectlApply(connectionManifest(ns))).To(Succeed())

			Eventually(func(g Gomega) {
				out, runErr := runKubectl("get", "queuemanagerconnection", mqConnectionName, "-n", ns,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}")
				g.Expect(runErr).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(qmcRotationEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())
		})
	})

	Describe("topics", Label("mq-topic"), func() {
		var (
			ns          string
			prefix      string
			topicObject string
			topicCR     string
		)

		BeforeEach(func() {
			ns = namespaceTopics
			ensureE2ENamespace(ns)
			prefix = mqObjectPrefix()
			topicObject = mqTopicObjectName(prefix)
			topicCR = mqCRName("e2e-retail-orders", prefix)
			ensureMQCredentialsSecret(ns)
			DeferCleanup(func() {
				cleanupMQSpec(ns, "topic", topicCR)
			})
		})

		It("reconciles a Topic CR against the kind IBM MQ queue manager", func() {
			invalidateWebhookReadyCache()
			waitForControllerAndWebhookReadyCached()
			Expect(applyWithWebhookRetry(connectionManifest(ns))).To(Succeed())
			eventuallyExpectQMCReady(ns)
			topicYAML := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: Topic
metadata:
  name: %s
  namespace: %s
spec:
  connectionRef:
    name: %s
  topicName: %s
  attributes:
    topstr: e2e/retail/orders
    descr: e2e retail topic
`, topicCR, ns, mqConnectionName, topicObject)
			Expect(applyWithWebhookRetry(topicYAML)).To(Succeed(),
				"Topic apply should succeed once CRDs are Established and the webhook is reachable")

			Eventually(func(g Gomega) {
				out, err := runKubectl("get", "topic", topicCR, "-n", ns,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(mqSyncedEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			state, err := client.GetTopic(ctx, topicObject)
			Expect(err).NotTo(HaveOccurred())
			Expect(state.Attributes["topstr"]).To(Equal("e2e/retail/orders"))

			Expect(kubectlDeleteNoWait("topic", topicCR, ns)).To(Succeed())

			Eventually(func(g Gomega) {
				ok, err := topicExists(ctx, client, topicObject)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ok).To(BeFalse(), "topic %s should be removed from MQ after CR delete", topicObject)
			}).WithTimeout(KubectlWaitDuration).WithPolling(3 * time.Second).Should(Succeed())
		})
	})

	Describe("channels", Label("mq-channel"), func() {
		var (
			ns            string
			prefix        string
			channelObject string
			channelCR     string
		)

		BeforeEach(func() {
			ns = namespaceChannels
			ensureE2ENamespace(ns)
			prefix = mqObjectPrefix()
			channelObject = mqChannelObjectName(prefix)
			channelCR = mqCRName("e2e-orders-app", prefix)
			ensureMQCredentialsSecret(ns)
			DeferCleanup(func() {
				cleanupMQSpec(ns, "channel", channelCR)
			})
		})

		It("reconciles a Channel CR against the kind IBM MQ queue manager", func() {
			Expect(applyWithWebhookRetry(connectionManifest(ns))).To(Succeed())
			eventuallyExpectQMCReady(ns)
			channelYAML := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: Channel
metadata:
  name: %s
  namespace: %s
spec:
  connectionRef:
    name: %s
  channelName: %s
  type: svrconn
  attributes:
    descr: e2e app channel
    trptype: tcp
`, channelCR, ns, mqConnectionName, channelObject)
			Expect(applyWithWebhookRetry(channelYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				out, err := runKubectl("get", "channel", channelCR, "-n", ns,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(mqSyncedEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			ok, err := svrconnChannelExists(ctx, client, channelObject)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(kubectlDeleteNoWait("channel", channelCR, ns)).To(Succeed())

			Eventually(func(g Gomega) {
				ok, err := svrconnChannelExists(ctx, client, channelObject)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ok).To(BeFalse(), "channel %s should be removed from MQ after CR delete", channelObject)
			}).WithTimeout(KubectlWaitDuration).WithPolling(3 * time.Second).Should(Succeed())
		})
	})

})

var _ = Describe("Post-manager IBM MQ auth", Label("mq", "mq-auth-serial"), Serial, Ordered, func() {
	const ns = namespaceAuth

	BeforeEach(func() {
		ensureE2ENamespace(ns)
		ensureMQCredentialsSecret(ns)
		if !webhookReady.Load() {
			waitForControllerAndWebhookReadyCached()
		}
	})

	AfterEach(func() {
		kubectlForceRemoveNamespaced("channelauthrule", mqChannelAuthCRName, ns)
		kubectlForceRemoveNamespaced("authorityrecord", mqAuthorityCRName, ns)
		kubectlForceRemoveNamespaced("channel", mqChannelPrereqCRName, ns)
		kubectlForceRemoveNamespaced("queuemanagerconnection", mqConnectionName, ns)
	})

	It("reconciles a ChannelAuthRule CR against the kind IBM MQ queue manager", func() {
		Expect(kubectlApply(connectionManifest(ns))).To(Succeed())

		channelYAML := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: Channel
metadata:
  name: %s
  namespace: %s
spec:
  connectionRef:
    name: %s
  channelName: %s
  type: svrconn
  attributes:
    trptype: tcp
`, mqChannelPrereqCRName, ns, mqConnectionName, e2eChannelName)
		Expect(applyWithWebhookRetry(channelYAML)).To(Succeed())

		Eventually(func(g Gomega) {
			out, runErr := runKubectl("get", "channel", mqChannelPrereqCRName, "-n", ns,
				"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
			g.Expect(runErr).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("True"), "Channel %s must be Synced before ChannelAuthRule admission", mqChannelPrereqCRName)
		}).WithTimeout(mqSyncedEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())

		carYAML := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: ChannelAuthRule
metadata:
  name: %s
  namespace: %s
spec:
  connectionRef:
    name: %s
  channelName: %s
  ruleType: ADDRESSMAP
  address: "*"
  userSource: CHANNEL
  checkClient: REQUIRED
  description: e2e address map rule
`, mqChannelAuthCRName, ns, mqConnectionName, e2eChannelName)
		Expect(applyWithWebhookRetry(carYAML)).To(Succeed())

		Eventually(func(g Gomega) {
			out, err := runKubectl("get", "channelauthrule", mqChannelAuthCRName, "-n", ns,
				"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("True"))
		}).WithTimeout(mqSyncedEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())

		client, err := newMQClient()
		Expect(err).NotTo(HaveOccurred())
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		carSpec := mqadmin.ChannelAuthSpec{
			ChannelName: e2eChannelName,
			RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
			Address:     "*",
			UserSource:  "CHANNEL",
			CheckClient: "REQUIRED",
			Description: "e2e address map rule",
		}
		ok, err := channelAuthMatches(ctx, client, carSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue(), "CHLAUTH for %s should match ChannelAuthRule spec", e2eChannelName)

		Expect(kubectlDeleteWait("channelauthrule", mqChannelAuthCRName, ns)).To(Succeed(),
			"ChannelAuthRule CR delete should complete within %s", kubectlWaitTimeout)

		carLookup := mqadmin.ChannelAuthSpec{
			ChannelName: e2eChannelName,
			RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
			Address:     "*",
		}
		Eventually(func(g Gomega) {
			ok, err := channelAuthExists(ctx, client, carLookup)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ok).To(BeFalse(), "CHLAUTH for %s should be removed from MQ after CR delete", e2eChannelName)
		}).WithTimeout(KubectlWaitDuration).WithPolling(3 * time.Second).Should(Succeed())
	})

	// BLOCKUSER CHLAUTH edge cases: test/integration/mq (TestIntegration_GetChannelAuth_BlockUser).

	It("reconciles an AuthorityRecord CR against the kind IBM MQ queue manager", func() {
		prefix := mqObjectPrefix()
		queueObject := mqQueueObjectName(prefix)
		queueCR := mqCRName("e2e-orders", prefix)

		DeferCleanup(func() {
			kubectlForceRemoveNamespaced("queue", queueCR, ns)
		})

		Expect(applyWithWebhookRetry(connectionManifest(ns))).To(Succeed())

		queueYAML := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: Queue
metadata:
  name: %s
  namespace: %s
spec:
  connectionRef:
    name: %s
  queueName: %s
  type: local
  attributes:
    maxdepth: "%s"
`, queueCR, ns, mqConnectionName, queueObject, mqQueueMaxDepthV1)
		Expect(applyWithWebhookRetry(queueYAML)).To(Succeed())

		Eventually(func(g Gomega) {
			out, err := runKubectl("get", "queue", queueCR, "-n", ns,
				"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("True"))
		}).WithTimeout(mqSyncedEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())

		authYAML := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: AuthorityRecord
metadata:
  name: %s
  namespace: %s
spec:
  connectionRef:
    name: %s
  profile: %s
  objectType: QUEUE
  principal: app
  authorities:
    - GET
    - PUT
`, mqAuthorityCRName, ns, mqConnectionName, queueObject)
		Expect(applyWithWebhookRetry(authYAML)).To(Succeed())

		Eventually(func(g Gomega) {
			out, err := runKubectl("get", "authorityrecord", mqAuthorityCRName, "-n", ns,
				"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("True"))
		}).WithTimeout(mqSyncedEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())

		client, err := newMQClient()
		Expect(err).NotTo(HaveOccurred())
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		authSpec := mqadmin.AuthoritySpec{
			Profile:     queueObject,
			ObjectType:  mqadmin.AuthorityObjectTypeQueue,
			Principal:   "app",
			Authorities: []string{"GET", "PUT"},
		}
		ok, err := authorityMatches(ctx, client, authSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue(), "AUTHREC for queue %s principal app should match AuthorityRecord spec", queueObject)

		Expect(kubectlDeleteWait("authorityrecord", mqAuthorityCRName, ns)).To(Succeed(),
			"AuthorityRecord CR delete should complete within %s", kubectlWaitTimeout)

		authLookup := mqadmin.AuthoritySpec{
			Profile:    queueObject,
			ObjectType: mqadmin.AuthorityObjectTypeQueue,
			Principal:  "app",
		}
		Eventually(func(g Gomega) {
			pollCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			ok, err := authorityExists(pollCtx, client, authLookup)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ok).To(BeFalse(), "AUTHREC for queue %s principal app should be removed after CR delete", queueObject)
		}).WithTimeout(mqAuthrecCleanupEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())
	})
})

func connectionManifest(ns string) string {
	return fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: QueueManagerConnection
metadata:
  name: %s
  namespace: %s
  annotations:
    messaging.mkurator.dev/allow-insecure-tls: "true"
spec:
  queueManager: %s
  endpoint: https://ibm-mq.ibm-mq.svc:9443
  tls:
    insecureSkipVerify: true
  credentialsSecretRef:
    name: mq-credentials
`, mqConnectionName, ns, envOr("KURATOR_E2E_MQ_QMGR", "QM1"))
}
