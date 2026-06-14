//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/conduit-ops/mkurator/test/utils"
)

var webhookReady atomic.Bool

// mkuratorE2ECRDs lists CRDs installed by task install:crds / helm:sync-crds.
var mkuratorE2ECRDs = []string{
	"queuemanagerconnections.messaging.mkurator.dev",
	"queues.messaging.mkurator.dev",
	"topics.messaging.mkurator.dev",
	"channels.messaging.mkurator.dev",
	"channelauthrules.messaging.mkurator.dev",
	"authorityrecords.messaging.mkurator.dev",
}

func invalidateWebhookReadyCache() {
	webhookReady.Store(false)
}

// waitForControllerAndWebhookReadyCached waits once per process unless a prior spec failed.
func waitForControllerAndWebhookReadyCached() {
	if webhookReady.Load() {
		return
	}
	waitForControllerAndWebhookReady()
	webhookReady.Store(true)
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
// KURATOR_E2E_DEPLOY=helm. Images must already be built and loaded (BeforeSuite).
func deployOperatorForE2E() {
	e2eStage("DEPLOY OPERATOR — install controller (" + e2eDeployMode() + ")")
	switch e2eDeployMode() {
	case "helm":
		deployOperatorForE2EHelm()
	default:
		deployOperatorForE2EKustomize()
	}
}

// ensureManagerNamespaceAndDeploy creates mkurator-system (kustomize) and installs the operator once.
func ensureManagerNamespaceAndDeploy() {
	switch e2eDeployMode() {
	case "helm":
		deployOperatorForE2EHelm()
	default:
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace, "--dry-run=client", "-o", "yaml")
		manifest, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to render namespace manifest")
		Expect(kubectlApply(manifest)).To(Succeed())

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		deployOperatorForE2EKustomize()
	}
}

// undeployOperatorForE2E removes the operator install matching the deploy mode used in the suite.
// cleanupE2EResources calls deleteAllE2ECustomResources first; this repeats the CR pass so a
// direct undeploy path still drops instances before CRD delete (operator may already be gone).
func undeployOperatorForE2E() {
	deleteAllE2ECustomResources()
	switch e2eDeployMode() {
	case "helm":
		By("uninstalling the controller-manager Helm release")
		cmd := exec.Command("task", "undeploy:helm")
		cmd.Env = taskEnv()
		_, _ = utils.Run(cmd)
		undeployMKuratorCRDsNoWait()
	default:
		undeployKustomizeOperatorNoWait()
		undeployMKuratorCRDsNoWait()
	}
}

func taskEnv() []string {
	env := append(os.Environ(), fmt.Sprintf("DOCKER_IMAGE=%s", managerImage))
	if e2eDeployMode() == "helm" {
		env = append(env, "KURATOR_E2E_DEPLOY=helm")
	}
	kc := os.Getenv("KUBECONFIG")
	if kc == "" {
		if projectDir, err := utils.GetProjectDir(); err == nil {
			kc = filepath.Join(projectDir, "hack", "kind-cluster", ".state", "kubeconfig.yaml")
		}
	}
	if kc != "" {
		env = append(env, "KUBECONFIG="+kc)
	}
	return env
}

// waitForMKuratorCRDsNotTerminating blocks until MKurator CRDs from a prior e2e
// AfterSuite (--wait=false delete) are gone or re-Established. Stuck terminating CRDs
// are force-cleared so a back-to-back local e2e run can proceed.
func waitForMKuratorCRDsNotTerminating() {
	By("waiting for MKurator CRDs to finish terminating from a prior e2e run")
	Eventually(func(g Gomega) {
		for _, crd := range mkuratorE2ECRDs {
			cmd := exec.Command("kubectl", "get", "crd", crd,
				"-o", "jsonpath={.metadata.deletionTimestamp}")
			out, runErr := utils.Run(cmd)
			if runErr != nil {
				continue
			}
			if ts := strings.TrimSpace(out); ts != "" {
				_, _ = runKubectl("patch", "crd", crd, "--type=merge",
					"-p", `{"metadata":{"finalizers":null}}`)
				g.Expect(ts).To(BeEmpty(), "CRD %s should not be terminating", crd)
			}
		}
	}).WithTimeout(10 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
}

// waitForMKuratorCRDsEstablished blocks until all MKurator CRDs are Established, not
// terminating, and messaging.mkurator.dev kinds are visible in API discovery. Call on
// every Ginkgo process after SynchronizedBeforeSuite process 1 deploys CRDs.
func waitForMKuratorCRDsEstablished() {
	By("waiting for MKurator CRDs to be Established and discoverable")
	Eventually(func(g Gomega) {
		for _, crd := range mkuratorE2ECRDs {
			cmd := exec.Command("kubectl", "get", "crd", crd,
				"-o", "jsonpath={.status.conditions[?(@.type=='Established')].status}{\"\\t\"}{.metadata.deletionTimestamp}")
			out, runErr := utils.Run(cmd)
			g.Expect(runErr).NotTo(HaveOccurred(), "CRD %s should exist", crd)
			fields := strings.Split(strings.TrimSpace(out), "\t")
			g.Expect(fields).NotTo(BeEmpty())
			g.Expect(fields[0]).To(Equal("True"), "CRD %s should be Established", crd)
			if len(fields) > 1 {
				g.Expect(fields[1]).To(BeEmpty(), "CRD %s should not be terminating", crd)
			}
		}
		cmd := exec.Command("kubectl", "api-resources", "--api-group=messaging.mkurator.dev",
			"-o", "name")
		out, runErr := utils.Run(cmd)
		g.Expect(runErr).NotTo(HaveOccurred(), "messaging.mkurator.dev API group should be discoverable")
		for _, kind := range []string{
			"queuemanagerconnections", "queues", "topics", "channels",
			"channelauthrules", "authorityrecords",
		} {
			g.Expect(out).To(ContainSubstring(kind),
				"%s kind should appear in API discovery", kind)
		}
	}).WithTimeout(5 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
}

// deployOperatorForE2EKustomize applies CRDs and operator manifests without rebuilding the image.
func deployOperatorForE2EKustomize() {
	waitForMKuratorCRDsNotTerminating()

	By("installing CRDs (task install:crds)")
	cmd := exec.Command("task", "install:crds")
	cmd.Env = taskEnv()
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

	By("removing stale controller-manager deployment before apply")
	cmd = exec.Command("kubectl", "delete", "deployment", "mkurator-controller-manager", "-n", namespace,
		"--ignore-not-found", "--wait=true", "--timeout=120s")
	_, _ = utils.Run(cmd)

	By("deploying the controller-manager (task deploy:operator)")
	cmd = exec.Command("task", "deploy:operator")
	cmd.Env = taskEnv()
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

	waitForMKuratorCRDsEstablished()

	waitForControllerAndWebhookReady()
	webhookReady.Store(true)
}

// deployOperatorForE2EHelm installs CRDs and the operator via Helm without rebuilding the image.
func deployOperatorForE2EHelm() {
	waitForMKuratorCRDsNotTerminating()

	By("removing stale mkurator-system namespace before Helm install")
	cmd := exec.Command("kubectl", "delete", "ns", namespace, "--ignore-not-found", "--wait=true", "--timeout=120s")
	_, _ = utils.Run(cmd)

	By("syncing Helm CRDs (task helm:sync-crds)")
	cmd = exec.Command("task", "helm:sync-crds")
	cmd.Env = taskEnv()
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to sync Helm CRDs")

	By("deploying the controller-manager (task deploy:helm:operator)")
	cmd = exec.Command("task", "deploy:helm:operator")
	cmd.Env = taskEnv()
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager via Helm")

	By("labeling the namespace to enforce the restricted security policy")
	cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
		"pod-security.kubernetes.io/enforce=restricted")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

	waitForMKuratorCRDsEstablished()

	waitForControllerAndWebhookReady()
	webhookReady.Store(true)
}

// applyChannelAuthPrereqFixtureOnce applies channel-auth-prereq.mqsc when MQ e2e is enabled.
func applyChannelAuthPrereqFixtureOnce() {
	if !mqE2EEnabled() {
		return
	}
	client, err := newMQClient()
	Expect(err).NotTo(HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	Eventually(func(g Gomega) {
		g.Expect(applyMQSCFixture(ctx, client, "channel-auth-prereq.mqsc")).To(Succeed())
	}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
}

// waitForControllerAndWebhookReady blocks until cert-manager has issued the webhook
// TLS secret, the controller-manager is rolled out, and the webhook Service has endpoints.
func waitForControllerAndWebhookReady() {
	By("ensuring manager namespace exists before webhook readiness checks")
	Eventually(func(g Gomega) {
		_, err := runKubectl("get", "ns", namespace)
		g.Expect(err).NotTo(HaveOccurred(), "manager namespace %s should exist", namespace)
	}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "certificate", "mkurator-serving-cert", "-n", namespace,
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

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "rollout", "status", "deployment/mkurator-controller-manager",
			"-n", namespace, "--timeout=2m")
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred(), "controller-manager rollout should complete")
	}).WithTimeout(8 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "pods", "-n", namespace,
			"-l", "control-plane=controller-manager",
			"-o", "jsonpath={.items[0].status.conditions[?(@.type=='Ready')].status}")
		out, runErr := utils.Run(cmd)
		g.Expect(runErr).NotTo(HaveOccurred())
		g.Expect(out).To(Equal("True"), "controller-manager should be Ready")
	}).WithTimeout(5 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "endpoints", "mkurator-webhook-service", "-n", namespace,
			"-o", "jsonpath={.subsets[0].addresses[0].ip}")
		out, runErr := utils.Run(cmd)
		g.Expect(runErr).NotTo(HaveOccurred())
		g.Expect(out).NotTo(BeEmpty(), "webhook service should have endpoints")
	}).WithTimeout(5 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "validatingwebhookconfiguration",
			"mkurator-validating-webhook-configuration")
		_, runErr := utils.Run(cmd)
		g.Expect(runErr).NotTo(HaveOccurred(), "ValidatingWebhookConfiguration should exist")
	}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		g.Expect(webhookAdmissionResponds()).To(Succeed(), "validating webhook should accept traffic")
	}).WithTimeout(3 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "auth", "can-i", "create", "events.events.k8s.io",
			"-n", namespace,
			"--as", fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccountName))
		out, runErr := utils.Run(cmd)
		g.Expect(runErr).NotTo(HaveOccurred())
		g.Expect(strings.TrimSpace(out)).To(Equal("yes"), "controller SA should create events.k8s.io")
	}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
}

// webhookAdmissionResponds checks the validating webhook is reachable by dry-running an invalid Queue.
func webhookAdmissionResponds() error {
	invalidQueue := fmt.Sprintf(`apiVersion: messaging.mkurator.dev/v1alpha1
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
	if isCRDDiscoveryNotReady(err) {
		return err
	}
	return nil
}

// isCRDDiscoveryNotReady reports API errors while CRDs are still registering or terminating.
func isCRDDiscoveryNotReady(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no matches for kind") ||
		strings.Contains(msg, "could not find the requested resource") ||
		strings.Contains(msg, "custom resource definition is terminating")
}
