//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/conduit-ops/mkurator/test/utils"
)

// serviceAccountName created for the project
const serviceAccountName = "mkurator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "mkurator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "mkurator-metrics-binding"

var _ = Describe("Manager", Serial, Ordered, Label("smoke"), func() {
	var controllerPodName string

	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			dumpFailureDiagnostics(controllerPodName)
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				By("getting the name of the controller-manager pod")
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				By("validating the pod's status")
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Controller manager pod should be Ready")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should reject invalid Queue at admission", func() {
			waitForControllerAndWebhookReadyCached()

			By("validating that ValidatingWebhookConfiguration is installed")
			cmd := exec.Command("kubectl", "get", "validatingwebhookconfiguration",
				"mkurator-validating-webhook-configuration")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "ValidatingWebhookConfiguration should exist")

			By("applying an alias Queue without targq and missing connectionRef target")
			invalidQueue := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: Queue
metadata:
  name: webhook-e2e-invalid
  namespace: %s
spec:
  connectionRef:
    name: missing-qmc-webhook-e2e
  queueName: APP.INVALID
  type: alias
`, namespace)
			apply := exec.Command("kubectl", "apply", "-f", "-")
			apply.Stdin = strings.NewReader(invalidQueue)
			_, err = utils.Run(apply)
			Expect(err).To(HaveOccurred(), "invalid Queue should be rejected by admission")
			Expect(err.Error()).To(Or(
				ContainSubstring("denied"),
				ContainSubstring("Forbidden"),
				ContainSubstring("Invalid"),
				ContainSubstring("is invalid"),
			))
		})

		It("should reject ChannelAuthRule without a matching Channel at admission", func() {
			waitForControllerAndWebhookReadyCached()

			const carWebhookQMC = "webhook-e2e-car-qmc"
			const carWebhookChannel = "WEBHOOK.MISSING.CHANNEL"

			DeferCleanup(func() {
				kubectlDeleteIgnoreNotFound("channelauthrule", "webhook-e2e-car-invalid", namespace)
				kubectlDeleteIgnoreNotFound("channel", "webhook-e2e-car-channel", namespace)
				kubectlDeleteIgnoreNotFound("queuemanagerconnection", carWebhookQMC, namespace)
				kubectlDeleteIgnoreNotFound("secret", "webhook-e2e-car-creds", namespace)
			})

			Expect(kubectlApply(fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: webhook-e2e-car-creds
  namespace: %s
type: Opaque
stringData:
  username: admin
  mqAdminPassword: placeholder
`, namespace))).To(Succeed())

			Expect(kubectlApply(fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: QueueManagerConnection
metadata:
  name: %s
  namespace: %s
  annotations:
    messaging.mkurator.dev/allow-insecure-tls: "true"
spec:
  queueManager: QM1
  endpoint: https://placeholder.invalid:9443
  tls:
    insecureSkipVerify: true
  credentialsSecretRef:
    name: webhook-e2e-car-creds
`, carWebhookQMC, namespace))).To(Succeed())

			invalidCAR := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
kind: ChannelAuthRule
metadata:
  name: webhook-e2e-car-invalid
  namespace: %s
spec:
  connectionRef:
    name: %s
  channelName: %s
  ruleType: ADDRESSMAP
  address: "*"
`, namespace, carWebhookQMC, carWebhookChannel)
			err := kubectlApply(invalidCAR)
			Expect(err).To(HaveOccurred(), "ChannelAuthRule without a Channel CR should be rejected")
			Expect(err.Error()).To(Or(
				ContainSubstring("denied"),
				ContainSubstring("Forbidden"),
				ContainSubstring("Invalid"),
				ContainSubstring("is invalid"),
				ContainSubstring(carWebhookChannel),
			))
		})

		It("should ensure the metrics endpoint is serving metrics", Label("slow"), func() {
			By("cleaning up webhook CAR fixture leftovers that block /readyz")
			kubectlDeleteIgnoreNotFound("channelauthrule", "webhook-e2e-car-invalid", namespace)
			kubectlDeleteIgnoreNotFound("queuemanagerconnection", "webhook-e2e-car-qmc", namespace)
			kubectlDeleteIgnoreNotFound("secret", "webhook-e2e-car-creds", namespace)

			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			kubectlDeleteClusterIgnoreNotFound("clusterrolebinding", metricsRoleBindingName)
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=mkurator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("ensuring the controller pod is ready")
			verifyControllerPodReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", controllerPodName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Controller pod not ready")
			}
			Eventually(verifyControllerPodReady, 3*time.Minute, time.Second).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image="+metricsCurlImage,
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "%s",
							"command": ["/bin/sh", "-c"],
							"args": [
								"for i in $(seq 1 30); do curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics && exit 0 || sleep 2; done; exit 1"
							],
							"securityContext": {
								"readOnlyRootFilesystem": true,
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccountName": "%s"
					}
				}`, metricsCurlImage, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			verifyMetricsAvailable := func(g Gomega) {
				metricsOutput, err := getMetricsOutput()
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
				g.Expect(metricsOutput).NotTo(BeEmpty())
				g.Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
			}
			Eventually(verifyMetricsAvailable, 5*time.Minute).Should(Succeed())
		})
	})
})

func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	By("creating temporary file to store the token request")
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		By("executing kubectl command to create the token")
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		By("parsing the JSON output to extract the token")
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

func getMetricsOutput() (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	return utils.Run(cmd)
}

type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
