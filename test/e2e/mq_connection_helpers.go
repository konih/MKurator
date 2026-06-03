//go:build e2e
// +build e2e

package e2e

import (
	"os/exec"
	"time"

	. "github.com/onsi/gomega"

	"github.com/konih/kurator/test/utils"
)

func eventuallyExpectQMCReady(ns string) {
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "queuemanagerconnection", mqConnectionName, "-n", ns,
			"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}")
		out, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(out).To(Equal("True"), "QueueManagerConnection %s should be Ready", mqConnectionName)
	}).WithTimeout(qmcRotationEventuallyTimeout).WithPolling(5 * time.Second).Should(Succeed())
}
