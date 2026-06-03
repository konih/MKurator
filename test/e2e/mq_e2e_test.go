//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/konih/kurator/internal/mqadmin"
	"github.com/konih/kurator/test/utils"
)

const (
	mqConnectionName      = "e2e-qm1"
	mqQueueCRName         = "e2e-orders"
	mqQueueObject         = "E2E.APP.ORDERS"
	mqQueueMaxDepthV1     = "1000"
	mqQueueMaxDepthV2     = "2000"
	mqTopicCRName         = "e2e-retail-orders"
	mqTopicObject         = "E2E.RETAIL.ORDERS"
	mqChannelCRName       = "e2e-orders-app"
	mqChannelObject       = "E2E.ORDERS.APP"
	mqChannelAuthCRName   = "e2e-dev-app-addressmap"
	mqChannelPrereqCRName = "e2e-dev-app-channel"
	mqAuthorityCRName     = "e2e-app-orders-get-put"
)

func e2eLocalQueueSpec() mqadmin.QueueSpec {
	return mqadmin.QueueSpec{Name: mqQueueObject, Type: mqadmin.QueueTypeLocal}
}

var _ = Describe("Post-manager IBM MQ integration", Serial, Label("mq"), func() {
	BeforeEach(func() {
		if !mqE2EEnabled() {
			Skip("IBM MQ e2e disabled; set KURATOR_E2E_MQ=1 and run task cluster:up")
		}
	})

	Context("channel/auth fixtures", func() {
		It("applies gitops-derived MQSC prerequisites via mqweb", func() {
			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			Eventually(func(g Gomega) {
				g.Expect(applyMQSCFixture(ctx, client, "channel-auth-prereq.mqsc")).To(Succeed())
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			Eventually(func(g Gomega) {
				ok, err := channelExists(ctx, client, e2eChannelName)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ok).To(BeTrue(), "channel %s should exist after fixture", e2eChannelName)
			}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
		})
	})

	Context("Queue reconciliation", Ordered, func() {
		BeforeAll(func() {
			if !mqE2EEnabled() {
				return
			}
			ensureOperatorForMQE2E()
		})

		BeforeEach(func() {
			waitForControllerAndWebhookReady()

			By("creating mq-credentials secret for QueueManagerConnection")
			cmd := exec.Command("kubectl", "create", "secret", "generic", "mq-credentials",
				"-n", namespace,
				"--from-literal=username=admin",
				fmt.Sprintf("--from-literal=mqAdminPassword=%s", envOr("KURATOR_E2E_MQ_PASSWORD", "passw0rd")),
				"--dry-run=client", "-o", "yaml",
			)
			manifest, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			apply := exec.Command("kubectl", "apply", "-f", "-")
			apply.Stdin = strings.NewReader(manifest)
			_, err = utils.Run(apply)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			kubectlDeleteIgnoreNotFound("queue", mqQueueCRName, namespace)
			kubectlDeleteIgnoreNotFound("topic", mqTopicCRName, namespace)
			kubectlDeleteIgnoreNotFound("channel", mqChannelCRName, namespace)
			kubectlDeleteIgnoreNotFound("queuemanagerconnection", mqConnectionName, namespace)
		})

		It("reconciles a Queue CR against the kind IBM MQ queue manager", func() {
			Expect(kubectlApply(connectionManifest())).To(Succeed())

			queueYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
`, mqQueueCRName, namespace, mqConnectionName, mqQueueObject, mqQueueMaxDepthV1)

			Expect(kubectlApply(queueYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "queue", mqQueueCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			eventuallyExpectQueueAvailableEvent(namespace, mqQueueCRName)

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			state, err := client.GetQueue(ctx, e2eLocalQueueSpec())
			Expect(err).NotTo(HaveOccurred())
			Expect(state.Attributes["maxdepth"]).To(Equal(mqQueueMaxDepthV1))

			By("deleting the Queue CR and expecting MQ object removal")
			Expect(kubectlDeleteWait("queue", mqQueueCRName, namespace)).To(Succeed(),
				"Queue CR delete should complete within %s", kubectlWaitTimeout)

			Eventually(func(g Gomega) {
				_, err := client.GetQueue(ctx, e2eLocalQueueSpec())
				g.Expect(err).To(HaveOccurred(), "queue %s should be removed from MQ after CR delete", mqQueueObject)
			}).WithTimeout(KubectlWaitDuration).WithPolling(3 * time.Second).Should(Succeed())
		})

		It("reconciles Queue attribute updates after spec changes", func() {
			Expect(kubectlApply(connectionManifest())).To(Succeed())

			queueYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
    descr: e2e orders queue v1
`, mqQueueCRName, namespace, mqConnectionName, mqQueueObject, mqQueueMaxDepthV1)
			Expect(kubectlApply(queueYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "queue", mqQueueCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			queueYAML = fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
    descr: e2e orders queue v2
`, mqQueueCRName, namespace, mqConnectionName, mqQueueObject, mqQueueMaxDepthV2)
			Expect(kubectlApply(queueYAML)).To(Succeed())

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			Eventually(func(g Gomega) {
				state, getErr := client.GetQueue(ctx, e2eLocalQueueSpec())
				g.Expect(getErr).NotTo(HaveOccurred())
				g.Expect(state.Attributes["maxdepth"]).To(Equal(mqQueueMaxDepthV2))
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})

		It("recovers QueueManagerConnection readiness after secret rotation", func() {
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
`, namespace))).To(Succeed())
			Expect(kubectlApply(connectionManifest())).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "queuemanagerconnection", mqConnectionName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}")
				out, err := utils.Run(cmd)
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
`, namespace, envOr("KURATOR_E2E_MQ_PASSWORD", "passw0rd")))).To(Succeed())

			cmd := exec.Command("kubectl", "annotate", "queuemanagerconnection", mqConnectionName, "-n", namespace,
				fmt.Sprintf("e2e-refresh-ts=%d", time.Now().UnixNano()), "--overwrite")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				check := exec.Command("kubectl", "get", "queuemanagerconnection", mqConnectionName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}")
				out, runErr := utils.Run(check)
				g.Expect(runErr).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		})

		It("reconciles a Topic CR against the kind IBM MQ queue manager", func() {
			Expect(kubectlApply(connectionManifest())).To(Succeed())
			topicYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
`, mqTopicCRName, namespace, mqConnectionName, mqTopicObject)
			Expect(kubectlApply(topicYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "topic", mqTopicCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			state, err := client.GetTopic(ctx, mqTopicObject)
			Expect(err).NotTo(HaveOccurred())
			Expect(state.Attributes["topstr"]).To(Equal("e2e/retail/orders"))

			Expect(kubectlDeleteWait("topic", mqTopicCRName, namespace)).To(Succeed(),
				"Topic CR delete should complete within %s", kubectlWaitTimeout)

			Eventually(func(g Gomega) {
				ok, err := topicExists(ctx, client, mqTopicObject)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ok).To(BeFalse(), "topic %s should be removed from MQ after CR delete", mqTopicObject)
			}).WithTimeout(KubectlWaitDuration).WithPolling(3 * time.Second).Should(Succeed())
		})

		It("reconciles a Channel CR against the kind IBM MQ queue manager", func() {
			Expect(kubectlApply(connectionManifest())).To(Succeed())
			channelYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
`, mqChannelCRName, namespace, mqConnectionName, mqChannelObject)
			Expect(kubectlApply(channelYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "channel", mqChannelCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			ok, err := svrconnChannelExists(ctx, client, mqChannelObject)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(kubectlDeleteWait("channel", mqChannelCRName, namespace)).To(Succeed(),
				"Channel CR delete should complete within %s", kubectlWaitTimeout)

			Eventually(func(g Gomega) {
				ok, err := svrconnChannelExists(ctx, client, mqChannelObject)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ok).To(BeFalse(), "channel %s should be removed from MQ after CR delete", mqChannelObject)
			}).WithTimeout(KubectlWaitDuration).WithPolling(3 * time.Second).Should(Succeed())
		})
	})

	Context("Auth reconciliation", Ordered, func() {
		BeforeAll(func() {
			if !mqE2EEnabled() {
				return
			}
			ensureOperatorForMQE2E()
		})

		BeforeEach(func() {
			if !mqE2EEnabled() {
				return
			}
			waitForControllerAndWebhookReady()

			cmd := exec.Command("kubectl", "create", "secret", "generic", "mq-credentials",
				"-n", namespace,
				"--from-literal=username=admin",
				fmt.Sprintf("--from-literal=mqAdminPassword=%s", envOr("KURATOR_E2E_MQ_PASSWORD", "passw0rd")),
				"--dry-run=client", "-o", "yaml",
			)
			manifest, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			apply := exec.Command("kubectl", "apply", "-f", "-")
			apply.Stdin = strings.NewReader(manifest)
			_, err = utils.Run(apply)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			kubectlDeleteIgnoreNotFound("channelauthrule", mqChannelAuthCRName, namespace)
			kubectlDeleteIgnoreNotFound("authorityrecord", mqAuthorityCRName, namespace)
			kubectlDeleteIgnoreNotFound("channel", mqChannelPrereqCRName, namespace)
			kubectlDeleteIgnoreNotFound("queue", mqQueueCRName, namespace)
			kubectlDeleteIgnoreNotFound("queuemanagerconnection", mqConnectionName, namespace)
		})

		It("reconciles a ChannelAuthRule CR against the kind IBM MQ queue manager", func() {
			Expect(kubectlApply(connectionManifest())).To(Succeed())

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			Expect(applyMQSCFixture(ctx, client, "channel-auth-prereq.mqsc")).To(Succeed())

			channelYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
`, mqChannelPrereqCRName, namespace, mqConnectionName, e2eChannelName)
			Expect(kubectlApply(channelYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "channel", mqChannelPrereqCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, runErr := utils.Run(cmd)
				g.Expect(runErr).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"), "Channel %s must be Synced before ChannelAuthRule admission", mqChannelPrereqCRName)
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			carYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
`, mqChannelAuthCRName, namespace, mqConnectionName, e2eChannelName)
			Expect(kubectlApply(carYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "channelauthrule", mqChannelAuthCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

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

			Expect(kubectlDeleteWait("channelauthrule", mqChannelAuthCRName, namespace)).To(Succeed(),
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

		It("reconciles an AuthorityRecord CR against the kind IBM MQ queue manager", func() {
			Expect(kubectlApply(connectionManifest())).To(Succeed())

			queueYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
`, mqQueueCRName, namespace, mqConnectionName, mqQueueObject, mqQueueMaxDepthV1)
			Expect(kubectlApply(queueYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "queue", mqQueueCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			authYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
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
`, mqAuthorityCRName, namespace, mqConnectionName, mqQueueObject)
			Expect(kubectlApply(authYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "authorityrecord", mqAuthorityCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			authSpec := mqadmin.AuthoritySpec{
				Profile:     mqQueueObject,
				ObjectType:  mqadmin.AuthorityObjectTypeQueue,
				Principal:   "app",
				Authorities: []string{"GET", "PUT"},
			}
			ok, err := authorityMatches(ctx, client, authSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue(), "AUTHREC for queue %s principal app should match AuthorityRecord spec", mqQueueObject)

			Expect(kubectlDeleteWait("authorityrecord", mqAuthorityCRName, namespace)).To(Succeed(),
				"AuthorityRecord CR delete should complete within %s", kubectlWaitTimeout)

			authLookup := mqadmin.AuthoritySpec{
				Profile:    mqQueueObject,
				ObjectType: mqadmin.AuthorityObjectTypeQueue,
				Principal:  "app",
			}
			Eventually(func(g Gomega) {
				ok, err := authorityExists(ctx, client, authLookup)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ok).To(BeFalse(), "AUTHREC for queue %s principal app should be removed after CR delete", mqQueueObject)
			}).WithTimeout(KubectlWaitDuration).WithPolling(3 * time.Second).Should(Succeed())
		})
	})
})

func connectionManifest() string {
	return fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
kind: QueueManagerConnection
metadata:
  name: %s
  namespace: %s
  annotations:
    messaging.kurator.dev/allow-insecure-tls: "true"
spec:
  queueManager: %s
  endpoint: https://ibm-mq.ibm-mq.svc:9443
  tls:
    insecureSkipVerify: true
  credentialsSecretRef:
    name: mq-credentials
`, mqConnectionName, namespace, envOr("KURATOR_E2E_MQ_QMGR", "QM1"))
}
