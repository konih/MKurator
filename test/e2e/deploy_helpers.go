//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/konih/kurator/test/utils"
)

// ensureOperatorForMQE2E (re)creates the operator install needed by Queue reconciliation tests.
// The Manager suite tears down the namespace in AfterAll; MQ specs run afterward.
func ensureOperatorForMQE2E() {
	By("creating manager namespace for MQ e2e")
	Expect(kubectlApply(fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
`, namespace))).To(Succeed())

	By("labeling the namespace to enforce the restricted security policy")
	cmd := exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
		"pod-security.kubernetes.io/enforce=restricted")
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

	deployOperatorForE2E()
}

// e2eDeployMode returns how the e2e suite installs the operator: "kustomize" (default) or "helm".
func e2eDeployMode() string {
	switch os.Getenv("KURATOR_E2E_DEPLOY") {
	case "helm":
		return "helm"
	default:
		return "kustomize"
	}
}

// deployOperatorForE2E installs the operator using Kustomize (default) or Helm when
// KURATOR_E2E_DEPLOY=helm.
func deployOperatorForE2E() {
	switch e2eDeployMode() {
	case "helm":
		deployOperatorForE2EHelm()
	default:
		deployOperatorForE2EKustomize()
	}
}

// undeployOperatorForE2E removes the operator install matching the deploy mode used in the suite.
func undeployOperatorForE2E() {
	switch e2eDeployMode() {
	case "helm":
		By("uninstalling the controller-manager Helm release")
		cmd := exec.Command("task", "undeploy:helm")
		_, _ = utils.Run(cmd)
	default:
		By("undeploying the controller-manager and CRDs")
		cmd := exec.Command("task", "undeploy:operator")
		_, _ = utils.Run(cmd)
	}
}

// deployOperatorForE2EKustomize applies CRDs and the full Kustomize stack via task deploy
// (docker:build + kind load + install:crds + deploy:operator).
func deployOperatorForE2EKustomize() {
	By("deploying the controller-manager (task deploy)")
	cmd := exec.Command("task", "deploy")
	cmd.Env = append(os.Environ(), fmt.Sprintf("DOCKER_IMAGE=%s", managerImage))
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

	Eventually(func(g Gomega) {
		check := exec.Command("kubectl", "get", "crd", "queuemanagerconnections.messaging.kurator.dev")
		_, runErr := utils.Run(check)
		g.Expect(runErr).NotTo(HaveOccurred(), "QueueManagerConnection CRD should be registered")
	}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	waitForControllerAndWebhookReady()
}

// deployOperatorForE2EHelm installs CRDs and the operator via task deploy:helm.
func deployOperatorForE2EHelm() {
	By("deploying the controller-manager (task deploy:helm)")
	cmd := exec.Command("task", "deploy:helm")
	cmd.Env = append(os.Environ(), fmt.Sprintf("DOCKER_IMAGE=%s", managerImage))
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager via Helm")

	Eventually(func(g Gomega) {
		check := exec.Command("kubectl", "get", "crd", "queuemanagerconnections.messaging.kurator.dev")
		_, runErr := utils.Run(check)
		g.Expect(runErr).NotTo(HaveOccurred(), "QueueManagerConnection CRD should be registered")
	}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	waitForControllerAndWebhookReady()
}

// waitForControllerAndWebhookReady blocks until cert-manager has issued the webhook
// TLS secret, the controller-manager is rolled out, and the webhook Service has endpoints.
func waitForControllerAndWebhookReady() {
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "certificate", "kurator-serving-cert", "-n", namespace,
			"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		out, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred(), "serving Certificate should exist")
		g.Expect(out).To(Equal("True"), "serving Certificate should be Ready")
	}).WithTimeout(5 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "secret", "webhook-server-cert", "-n", namespace)
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred(), "webhook-server-cert should exist")
	}).WithTimeout(3 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	cmd := exec.Command("kubectl", "rollout", "status", "deployment/kurator-controller-manager",
		"-n", namespace, "--timeout=5m")
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "controller-manager rollout should complete")

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "pods", "-n", namespace,
			"-l", "control-plane=controller-manager",
			"-o", "jsonpath={.items[0].status.conditions[?(@.type=='Ready')].status}")
		out, runErr := utils.Run(cmd)
		g.Expect(runErr).NotTo(HaveOccurred())
		g.Expect(out).To(Equal("True"), "controller-manager should be Ready")
	}).WithTimeout(5 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "endpoints", "kurator-webhook-service", "-n", namespace,
			"-o", "jsonpath={.subsets[0].addresses[0].ip}")
		out, runErr := utils.Run(cmd)
		g.Expect(runErr).NotTo(HaveOccurred())
		g.Expect(out).NotTo(BeEmpty(), "webhook service should have endpoints")
	}).WithTimeout(5 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "validatingwebhookconfiguration",
			"kurator-validating-webhook-configuration")
		_, runErr := utils.Run(cmd)
		g.Expect(runErr).NotTo(HaveOccurred(), "ValidatingWebhookConfiguration should exist")
	}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		g.Expect(webhookAdmissionResponds()).To(Succeed(), "validating webhook should accept traffic")
	}).WithTimeout(3 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
}

// webhookAdmissionResponds checks the validating webhook is reachable by dry-running an invalid Queue.
func webhookAdmissionResponds() error {
	invalidQueue := fmt.Sprintf(`apiVersion: messaging.kurator.dev/v1alpha1
kind: Queue
metadata:
  name: webhook-readiness-probe
  namespace: %s
spec:
  connectionRef:
    name: missing-qmc-webhook-readiness
  queueName: APP.INVALID
  type: alias
`, namespace)
	apply := exec.Command("kubectl", "apply", "--dry-run=server", "-f", "-")
	apply.Stdin = strings.NewReader(invalidQueue)
	_, err := utils.Run(apply)
	if err == nil {
		return fmt.Errorf("invalid Queue dry-run should be rejected by admission")
	}
	if isWebhookConnectionRefused(err) {
		return err
	}
	return nil
}
