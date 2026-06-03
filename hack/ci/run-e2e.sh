#!/usr/bin/env bash
# Run the Ginkgo e2e suite with verbose progress (used by task test:e2e).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/ci/step.sh
source "${ROOT}/hack/ci/step.sh"
# shellcheck source=hack/ci/suite-lock.sh
source "${ROOT}/hack/ci/suite-lock.sh"

if [[ "${KURATOR_SUITE_LOCK_HELD:-}" != "1" ]]; then
  suite_lock_acquire "${EXCLUSIVE_TEST_LOCK_NAME}"
  trap suite_lock_release EXIT
fi

cd "${ROOT}"

# Prefer the kind cluster kubeconfig when present so kubectl never falls back to
# a stale default context (e.g. localhost:8080) during BeforeSuite cert-manager install.
KIND_KUBECONFIG="${ROOT}/hack/kind-cluster/.state/kubeconfig.yaml"
if [[ -f "${KIND_KUBECONFIG}" ]]; then
  export KUBECONFIG="${KIND_KUBECONFIG}"
fi

ci_step "E2E tests (build image, load kind, deploy operator — output streams below)"

echo "KUBECONFIG=${KUBECONFIG:-<unset>}"
echo "KIND_CLUSTER=${KIND_CLUSTER:-<unset>}"
echo "KURATOR_E2E_MQ=${KURATOR_E2E_MQ:-<unset>}"
echo "CERT_MANAGER_INSTALL_SKIP=${CERT_MANAGER_INSTALL_SKIP:-<unset>}"
echo ""

export CGO_ENABLED="${CGO_ENABLED:-1}"

# -ginkgo.v + -ginkgo.show-node-events: spec output and node events when a spec is stuck
# -count=1: do not skip the suite via cached pass
go test -tags=e2e ./test/e2e/... \
  -race \
  -v \
  -count=1 \
  -timeout=90m \
  -ginkgo.v \
  -ginkgo.show-node-events
