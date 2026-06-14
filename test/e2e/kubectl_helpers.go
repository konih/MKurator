//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/conduit-ops/mkurator/test/utils"
)

// kubectlWaitTimeout is passed to kubectl delete --wait and kubectl wait.
const kubectlWaitTimeout = "2m"

// KubectlWaitDuration matches kubectlWaitTimeout for Gomega Eventually blocks.
const KubectlWaitDuration = 2 * time.Minute

const (
	kubectlRequestTimeout = "30s"
	kubectlCommandTimeout = 35 * time.Second

	webhookApplyRetryTimeout  = 8 * time.Minute
	webhookApplyRetryInterval = 2 * time.Second
)

// runKubectl runs kubectl with a bounded client and process timeout so a stuck API cannot hang the suite.
func runKubectl(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), kubectlCommandTimeout)
	defer cancel()
	kubectlArgs := append([]string{"--request-timeout=" + kubectlRequestTimeout}, args...)
	return utils.Run(exec.CommandContext(ctx, "kubectl", kubectlArgs...))
}

// kubectlApply applies a multi-document manifest from stdin.
func kubectlApply(manifest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), kubectlCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "--request-timeout="+kubectlRequestTimeout, "apply", "-f", "-")
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

// isTransientKubectlApplyError reports webhook, CRD discovery, or terminating CRD races.
func isTransientKubectlApplyError(err error) bool {
	return isWebhookConnectionRefused(err) || isCRDDiscoveryNotReady(err)
}

func retryAfterWebhookUnreachable(lastErr error) {
	if isWebhookConnectionRefused(lastErr) {
		invalidateWebhookReadyCache()
		waitForControllerAndWebhookReadyCached()
	}
}

// applyWithWebhookRetry applies a manifest, retrying transient webhook and CRD discovery errors.
func applyWithWebhookRetry(manifest string) error {
	deadline := time.Now().Add(webhookApplyRetryTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = kubectlApply(manifest)
		if lastErr == nil {
			return nil
		}
		if !isTransientKubectlApplyError(lastErr) {
			return lastErr
		}
		retryAfterWebhookUnreachable(lastErr)
		time.Sleep(webhookApplyRetryInterval)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out after %s waiting for webhook", webhookApplyRetryTimeout)
	}
	return fmt.Errorf("kubectl apply (webhook retries exhausted): %w", lastErr)
}

// kubectlDeleteWait deletes a namespaced resource and blocks until removal or kubectlWaitTimeout.
func kubectlDeleteWait(resource, name, ns string) error {
	out, err := runKubectl("delete", resource, name, "-n", ns,
		"--wait=true", "--timeout="+kubectlWaitTimeout)
	if err != nil {
		return fmt.Errorf(
			"kubectl delete %s/%s -n %s did not finish within %s: %w; output: %s",
			resource, name, ns, kubectlWaitTimeout, err, strings.TrimSpace(out),
		)
	}
	return nil
}

// kubectlDeleteWithWebhookRetry deletes with --wait, retrying transient webhook reachability errors.
func kubectlDeleteWithWebhookRetry(resource, name, ns string) error {
	deadline := time.Now().Add(webhookApplyRetryTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = kubectlDeleteWait(resource, name, ns)
		if lastErr == nil {
			return nil
		}
		if !isWebhookConnectionRefused(lastErr) {
			return lastErr
		}
		retryAfterWebhookUnreachable(lastErr)
		time.Sleep(webhookApplyRetryInterval)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out after %s waiting to delete %s/%s", webhookApplyRetryTimeout, resource, name)
	}
	return fmt.Errorf("kubectl delete %s/%s -n %s (webhook retries exhausted): %w", resource, name, ns, lastErr)
}

// kubectlWait runs kubectl wait --for=<condition> with kubectlWaitTimeout.
func kubectlWait(condition, resource, name, ns string) error {
	args := []string{"wait", "--for=" + condition, "--timeout=" + kubectlWaitTimeout, "-n", ns, resource + "/" + name}
	out, err := runKubectl(args...)
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
	_, err := runKubectl("delete", resource, name, "-n", ns,
		"--ignore-not-found", "--wait=false")
	return err
}

// kubectlDeleteIgnoreNotFound best-effort deletes without waiting (test cleanup).
func kubectlDeleteIgnoreNotFound(resource, name, ns string) {
	_ = kubectlDeleteNoWait(resource, name, ns)
}

// kubectlDeleteClusterIgnoreNotFound best-effort deletes a cluster-scoped resource.
func kubectlDeleteClusterIgnoreNotFound(resource, name string) {
	_, _ = runKubectl("delete", resource, name, "--ignore-not-found", "--wait=false")
}

// kubectlStripFinalizers clears finalizers on a resource that still exists (stuck Terminating).
func kubectlStripFinalizers(resource, name, ns string) {
	out, err := runKubectl("get", resource, name, "-n", ns, "-o", "name")
	if err != nil || strings.TrimSpace(out) == "" {
		return
	}
	const patch = `{"metadata":{"finalizers":null}}`
	_, _ = runKubectl("patch", resource, name, "-n", ns, "--type=merge", "-p", patch)
}

// kubectlForceRemoveNamespaced deletes without wait, then strips finalizers if the object remains.
func kubectlForceRemoveNamespaced(resource, name, ns string) {
	_ = kubectlDeleteNoWait(resource, name, ns)
	kubectlStripFinalizers(resource, name, ns)
}

// cleanupMQSpec removes a messaging CR and its QueueManagerConnection without blocking on finalizers.
// Child resources are removed before the connection so the operator can finish MQ teardown when possible.
func cleanupMQSpec(ns, childResource, childName string) {
	if childResource != "" && childName != "" {
		kubectlForceRemoveNamespaced(childResource, childName, ns)
	}
	kubectlForceRemoveNamespaced("queuemanagerconnection", mqConnectionName, ns)
}
