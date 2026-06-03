//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/konih/kurator/test/utils"
)

// kubectlWaitTimeout is passed to kubectl delete --wait and kubectl wait.
const kubectlWaitTimeout = "2m"

// KubectlWaitDuration matches kubectlWaitTimeout for Gomega Eventually blocks.
const KubectlWaitDuration = 2 * time.Minute

const (
	webhookApplyRetryTimeout  = 2 * time.Minute
	webhookApplyRetryInterval = 2 * time.Second
)

// kubectlApply applies a multi-document manifest from stdin.
func kubectlApply(manifest string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err := utils.Run(cmd)
	return err
}

// isWebhookConnectionRefused reports transient failures reaching the validating webhook.
func isWebhookConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no endpoints available") ||
		strings.Contains(msg, "failed calling webhook")
}

// applyWithWebhookRetry applies a manifest, retrying when the API server cannot reach the webhook.
func applyWithWebhookRetry(manifest string) error {
	deadline := time.Now().Add(webhookApplyRetryTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = kubectlApply(manifest)
		if lastErr == nil {
			return nil
		}
		if !isWebhookConnectionRefused(lastErr) {
			return lastErr
		}
		time.Sleep(webhookApplyRetryInterval)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out after %s waiting for webhook", webhookApplyRetryTimeout)
	}
	return fmt.Errorf("kubectl apply (webhook retries exhausted): %w", lastErr)
}

// kubectlDeleteWait deletes a namespaced resource and blocks until removal or kubectlWaitTimeout.
func kubectlDeleteWait(resource, name, ns string) error {
	cmd := exec.Command("kubectl", "delete", resource, name, "-n", ns,
		"--wait=true", "--timeout="+kubectlWaitTimeout)
	out, err := utils.Run(cmd)
	if err != nil {
		return fmt.Errorf(
			"kubectl delete %s/%s -n %s did not finish within %s: %w; output: %s",
			resource, name, ns, kubectlWaitTimeout, err, strings.TrimSpace(out),
		)
	}
	return nil
}

// kubectlWait runs kubectl wait --for=<condition> with kubectlWaitTimeout.
func kubectlWait(condition, resource, name, ns string) error {
	args := []string{"wait", "--for=" + condition, "--timeout=" + kubectlWaitTimeout, "-n", ns, resource + "/" + name}
	out, err := utils.Run(exec.Command("kubectl", args...))
	if err != nil {
		return fmt.Errorf(
			"kubectl wait %s %s/%s -n %s timed out after %s: %w; output: %s",
			condition, resource, name, ns, kubectlWaitTimeout, err, strings.TrimSpace(out),
		)
	}
	return nil
}

// kubectlDeleteNoWait issues a delete without blocking on finalizers (MQ specs assert via mqweb).
func kubectlDeleteNoWait(resource, name, ns string) error {
	cmd := exec.Command("kubectl", "delete", resource, name, "-n", ns,
		"--ignore-not-found", "--wait=false")
	_, err := utils.Run(cmd)
	return err
}

// kubectlDeleteIgnoreNotFound best-effort deletes without waiting (test cleanup).
func kubectlDeleteIgnoreNotFound(resource, name, ns string) {
	_ = exec.Command("kubectl", "delete", resource, name, "-n", ns, "--ignore-not-found").Run()
}

// kubectlDeleteClusterIgnoreNotFound best-effort deletes a cluster-scoped resource.
func kubectlDeleteClusterIgnoreNotFound(resource, name string) {
	_ = exec.Command("kubectl", "delete", resource, name, "--ignore-not-found").Run()
}
