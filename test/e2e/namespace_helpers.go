//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/konih/kurator/test/utils"
)

const (
	namespaceQueues   = "kurator-e2e-queues"
	namespaceTopics   = "kurator-e2e-topics"
	namespaceChannels = "kurator-e2e-channels"
	namespaceAuth     = "kurator-e2e-auth"
)

var mqE2ENamespaces = []string{
	namespaceQueues,
	namespaceTopics,
	namespaceChannels,
	namespaceAuth,
}

// mqObjectPrefix returns a unique MQ object name segment per parallel Ginkgo process.
func mqObjectPrefix() string {
	return fmt.Sprintf("N%d", GinkgoParallelProcess())
}

func mqQueueObjectName(prefix string) string {
	return fmt.Sprintf("E2E.%s.APP.ORDERS", prefix)
}

func mqTopicObjectName(prefix string) string {
	return fmt.Sprintf("E2E.%s.RETAIL.ORDERS", prefix)
}

func mqChannelObjectName(prefix string) string {
	return fmt.Sprintf("E2E.%s.ORDERS.APP", prefix)
}

func mqCRName(base, prefix string) string {
	return fmt.Sprintf("%s-%s", base, strings.ToLower(prefix))
}

func ensureMQE2ENamespaces() {
	for _, ns := range mqE2ENamespaces {
		ensureE2ENamespace(ns)
	}
}

func ensureE2ENamespace(name string) {
	By("creating namespace " + name)
	cmd := exec.Command("kubectl", "create", "ns", name, "--dry-run=client", "-o", "yaml")
	manifest, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to render namespace manifest for %s", name)
	Expect(kubectlApply(manifest)).To(Succeed())

	cmd = exec.Command("kubectl", "label", "--overwrite", "ns", name,
		"pod-security.kubernetes.io/enforce=restricted")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to label namespace %s with restricted policy", name)
}

func ensureMQCredentialsSecret(ns string) {
	cmd := exec.Command("kubectl", "create", "secret", "generic", "mq-credentials",
		"-n", ns,
		"--from-literal=username=admin",
		fmt.Sprintf("--from-literal=mqAdminPassword=%s", envOr("KURATOR_E2E_MQ_PASSWORD", "passw0rd")),
		"--dry-run=client", "-o", "yaml",
	)
	manifest, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
	cmd = exec.Command("kubectl", "apply", "--server-side", "--force-conflicts", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to apply mq-credentials in %s", ns)
}

func cleanupE2EResources() {
	undeployOperatorForE2E()

	namespaces := append([]string{namespace}, mqE2ENamespaces...)
	for _, ns := range namespaces {
		cmd := exec.Command("kubectl", "delete", "ns", ns, "--ignore-not-found", "--wait=false")
		_, _ = utils.Run(cmd)
	}
}
