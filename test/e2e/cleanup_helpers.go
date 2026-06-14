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
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/conduit-ops/mkurator/test/utils"
)

const e2eCleanupTimeout = 5 * time.Minute

// mkuratorMQResources are namespaced messaging.mkurator.dev kinds (kubectl resource names).
var mkuratorMQResources = []string{
	"queuemanagerconnection",
	"queue",
	"topic",
	"channel",
	"channelauthrule",
	"authorityrecord",
}

func e2eCleanupNamespaces() []string {
	return append([]string{namespace}, mqE2ENamespaces...)
}

func skipCRDUndeploy() bool {
	return os.Getenv("KURATOR_E2E_SKIP_CRD_UNDEPLOY") == "1"
}

// cleanupE2EResources tears down MQ CRs, the operator, CRDs, and e2e namespaces.
// It is bounded by e2eCleanupTimeout so AfterSuite cannot hang indefinitely.
func cleanupE2EResources() {
	By(fmt.Sprintf("cleaning up e2e resources (max %s)", e2eCleanupTimeout))
	done := make(chan struct{})
	go func() {
		defer close(done)
		cleanupE2EResourcesOnce()
	}()
	select {
	case <-done:
	case <-time.After(e2eCleanupTimeout):
		_, _ = fmt.Fprintf(GinkgoWriter,
			"e2e cleanup timed out after %s (kill the test process if kubectl is still stuck)\n",
			e2eCleanupTimeout,
		)
	}
}

func cleanupE2EResourcesOnce() {
	namespaces := e2eCleanupNamespaces()
	undeployOperatorForE2E()
	deleteE2ENamespacesNoWait(namespaces)
}

// deleteAllE2ECustomResources removes messaging.mkurator.dev CRs from mkurator-e2e-* and
// mkurator-system without waiting on operator finalizers, then strips stuck finalizers.
// Run before CRD delete so kubectl does not block on InstanceDeletionInProgress.
func deleteAllE2ECustomResources() {
	namespaces := e2eCleanupNamespaces()
	deleteAllMKuratorCRsNoWait(namespaces)
	stripRemainingFinalizers(namespaces)
}

func deleteAllMKuratorCRsNoWait(namespaces []string) {
	By("deleting MKurator custom resources in e2e namespaces (no wait)")
	for _, ns := range namespaces {
		for _, res := range mkuratorMQResources {
			_, _ = runKubectl("delete", res, "--all", "-n", ns,
				"--ignore-not-found", "--wait=false")
		}
	}
}

// stripRemainingFinalizers removes operator finalizers when the controller is already gone.
func stripRemainingFinalizers(namespaces []string) {
	const patch = `{"metadata":{"finalizers":null}}`
	for _, ns := range namespaces {
		for _, res := range mkuratorMQResources {
			out, err := runKubectl("get", res, "-n", ns,
				"-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}")
			if err != nil {
				continue
			}
			for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
				if name == "" {
					continue
				}
				_, _ = runKubectl("patch", res, name, "-n", ns, "--type=merge", "-p", patch)
			}
		}
	}
}

func deleteE2ENamespacesNoWait(namespaces []string) {
	By("deleting e2e namespaces (no wait)")
	for _, ns := range namespaces {
		_, _ = runKubectl("delete", "ns", ns, "--ignore-not-found", "--wait=false")
	}
}

func undeployKustomizeOperatorNoWait() {
	By("removing controller-manager manifests (no wait)")
	ctx, cancel := context.WithTimeout(context.Background(), kubectlCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "--request-timeout="+kubectlRequestTimeout,
		"delete", "--ignore-not-found", "-k", "config/default", "--wait=false")
	cmd.Env = taskEnv()
	_, _ = utils.Run(cmd)
}

func undeployMKuratorCRDsNoWait() {
	if skipCRDUndeploy() {
		_, _ = fmt.Fprintf(GinkgoWriter,
			"Skipping MKurator CRD delete (KURATOR_E2E_SKIP_CRD_UNDEPLOY=1)\n")
		return
	}
	By("removing MKurator CRDs (no wait)")
	projectDir, err := utils.GetProjectDir()
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "undeploy CRDs: get project dir: %v\n", err)
		return
	}
	crdDir := filepath.Join(projectDir, "config", "crd", "bases")
	ctx, cancel := context.WithTimeout(context.Background(), kubectlCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "--request-timeout="+kubectlRequestTimeout,
		"delete", "--ignore-not-found", "-f", crdDir, "--wait=false")
	cmd.Env = taskEnv()
	_, _ = utils.Run(cmd)
}
