//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/conduit-ops/mkurator/test/utils"
)

func e2eVerboseLogs() bool {
	return os.Getenv("KURATOR_E2E_VERBOSE_LOGS") == "1"
}

// e2eStage prints a ci_step-style banner to stdout (visible outside GinkgoWriter).
func e2eStage(stage string) {
	_, _ = fmt.Fprintf(os.Stdout, "\n==> %s %s\n\n", time.Now().Format(time.RFC3339), stage)
}

// e2eBy mirrors Ginkgo By() and echoes a short progress line to stdout.
func e2eBy(msg string) {
	_, _ = fmt.Fprintf(os.Stdout, "[e2e] %s\n", msg)
	By(msg)
}

// e2eSpecLine prints spec start/end markers when ReportBeforeEach/AfterEach fire.
func e2eSpecLine(state, fullText string) {
	_, _ = fmt.Fprintf(os.Stdout, "[e2e] %s %s\n", state, fullText)
}

// dumpFailureDiagnostics collects kubectl context on spec failure without full JSON logs by default.
func dumpFailureDiagnostics(controllerPodName string) {
	e2eBy("collecting failure diagnostics")

	if controllerPodName != "" {
		args := []string{"logs", controllerPodName, "-n", namespace}
		if !e2eVerboseLogs() {
			args = append(args, "--tail=40")
		}
		cmd := exec.Command("kubectl", args...)
		controllerLogs, err := utils.Run(cmd)
		if err == nil {
			hint := "last 40 lines"
			if e2eVerboseLogs() {
				hint = "full log"
			}
			_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs (%s; KURATOR_E2E_VERBOSE_LOGS=1 for full dump):\n%s",
				hint, controllerLogs)
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get controller logs: %s\n", err)
		}
	}

	cmd := exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
	eventsOutput, err := utils.Run(cmd)
	if err == nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s\n", err)
	}

	cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace, "--tail=20")
	metricsOutput, err := utils.Run(cmd)
	if err == nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "Metrics curl logs (last 20 lines):\n%s", metricsOutput)
	} else if e2eVerboseLogs() {
		_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s\n", err)
	}

	if controllerPodName != "" {
		cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
		podDescription, err := utils.Run(cmd)
		if err == nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Pod description:\n%s", podDescription)
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "Failed to describe controller pod: %s\n", err)
		}
	}
}
