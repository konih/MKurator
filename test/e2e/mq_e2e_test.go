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

	"github.com/konradheimel/kurator/internal/mqadmin"
	"github.com/konradheimel/kurator/test/utils"
)

const (
	mqConnectionName = "e2e-qm1"
	mqQueueCRName    = "e2e-orders"
	mqQueueObject    = "E2E.APP.ORDERS"
)

func e2eLocalQueueSpec() mqadmin.QueueSpec {
	return mqadmin.QueueSpec{Name: mqQueueObject, Type: mqadmin.QueueTypeLocal}
}


var _ = Describe("IBM MQ integration", Serial, Label("mq"), func() {
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

			Expect(applyMQSCFixture(ctx, client, "channel-auth-prereq.mqsc")).To(Succeed())

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
			_ = exec.Command("kubectl", "delete", "queue", mqQueueCRName, "-n", namespace, "--ignore-not-found").Run()
			_ = exec.Command("kubectl", "delete", "queuemanagerconnection", mqConnectionName, "-n", namespace, "--ignore-not-found").Run()
		})

		It("reconciles a Queue CR against the kind IBM MQ queue manager", func() {
			connectionYAML := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
kind: QueueManagerConnection
metadata:
  name: %s
  namespace: %s
spec:
  queueManager: %s
  endpoint: https://ibm-mq.ibm-mq.svc:9443
  tls:
    insecureSkipVerify: true
  credentialsSecretRef:
    name: mq-credentials
`, mqConnectionName, namespace, envOr("KURATOR_E2E_MQ_QMGR", "QM1"))

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
    maxdepth: "1000"
    descr: e2e orders queue
`, mqQueueCRName, namespace, mqConnectionName, mqQueueObject)

			Expect(kubectlApply(connectionYAML)).To(Succeed())
			Expect(kubectlApply(queueYAML)).To(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "queue", mqQueueCRName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Synced\")].status}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("True"))
			}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			client, err := newMQClient()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			state, err := client.GetQueue(ctx, e2eLocalQueueSpec())
			Expect(err).NotTo(HaveOccurred())
			Expect(state.Attributes["maxdepth"]).To(Equal("1000"))

			By("deleting the Queue CR and expecting MQ object removal")
			cmd := exec.Command("kubectl", "delete", "queue", mqQueueCRName, "-n", namespace, "--wait=true")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				_, err := client.GetQueue(ctx, e2eLocalQueueSpec())
				g.Expect(err).To(HaveOccurred())
			}).WithTimeout(2 * time.Minute).WithPolling(3 * time.Second).Should(Succeed())
		})
	})
})

func kubectlApply(manifest string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err := utils.Run(cmd)
	return err
}
