//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/conduit-ops/mkurator/test/utils"
)

const (
	namespace         = "mkurator-system"
	namespaceQueues   = "mkurator-e2e-queues"
	namespaceTopics   = "mkurator-e2e-topics"
	namespaceChannels = "mkurator-e2e-channels"
	namespaceAuth     = "mkurator-e2e-auth"
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
	manifest, err := runKubectl("create", "ns", name, "--dry-run=client", "-o", "yaml")
	Expect(err).NotTo(HaveOccurred(), "Failed to render namespace manifest for %s", name)
	Expect(kubectlApply(manifest)).To(Succeed())

	_, err = runKubectl("label", "--overwrite", "ns", name,
		"pod-security.kubernetes.io/enforce=restricted")
	Expect(err).NotTo(HaveOccurred(), "Failed to label namespace %s with restricted policy", name)
}

func ensureMQCredentialsSecret(ns string) {
	manifest, err := runKubectl("create", "secret", "generic", "mq-credentials",
		"-n", ns,
		"--from-literal=username=admin",
		fmt.Sprintf("--from-literal=mqAdminPassword=%s", envOr("KURATOR_E2E_MQ_PASSWORD", "passw0rd")),
		"--dry-run=client", "-o", "yaml",
	)
	Expect(err).NotTo(HaveOccurred())
	ctx, cancel := context.WithTimeout(context.Background(), kubectlCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "--request-timeout="+kubectlRequestTimeout,
		"apply", "--server-side", "--force-conflicts", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to apply mq-credentials in %s", ns)
}
