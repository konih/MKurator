#!/usr/bin/env bash
# Run the Ginkgo e2e suite with verbose progress (used by task test:e2e).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/ci/step.sh
source "${ROOT}/hack/ci/step.sh"
# shellcheck source=hack/ci/test-artifacts.sh
source "${ROOT}/hack/ci/test-artifacts.sh"
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

GINKGO_NODES="${KURATOR_E2E_NODES:-3}"
ARTIFACTS_DIR="$(test_artifacts_dir "${ROOT}")"
E2E_JUNIT="${ARTIFACTS_DIR}/e2e-junit.xml"

ci_step "GINKGO E2E — image build, kind load, deploy (platform must already be up; task ci:e2e runs PLATFORM UP first)"

echo "KUBECONFIG=${KUBECONFIG:-<unset>}"
echo "KIND_CLUSTER=${KIND_CLUSTER:-<unset>}"
echo "KURATOR_E2E_DEPLOY=${KURATOR_E2E_DEPLOY:-kustomize}"
echo "KURATOR_E2E_MQ=${KURATOR_E2E_MQ:-<unset>}"
echo "KURATOR_E2E_NODES=${GINKGO_NODES}"
echo "KURATOR_E2E_LABEL_FILTER=${KURATOR_E2E_LABEL_FILTER:-<unset>}"
echo "CERT_MANAGER_INSTALL_SKIP=${CERT_MANAGER_INSTALL_SKIP:-<unset>}"
echo "KURATOR_E2E_VERBOSE_LOGS=${KURATOR_E2E_VERBOSE_LOGS:-0}"
echo "E2E_JUNIT=${E2E_JUNIT}"
echo ""
echo "Note: ginkgo run uses --race (CGO_ENABLED=1). With parallel nodes, use fewer KURATOR_E2E_NODES on small hosts if flaky."
echo ""

export CGO_ENABLED="${CGO_ENABLED:-1}"

GINKGO_FLAGS=(
  --procs="${GINKGO_NODES}"
  --race
  --timeout=120m
  --tags=e2e
  -vv
  --show-node-events
  --junit-report=e2e-junit.xml
  --output-dir="${ARTIFACTS_DIR}"
)
if [[ -n "${KURATOR_E2E_LABEL_FILTER:-}" ]]; then
  GINKGO_FLAGS+=(--label-filter="${KURATOR_E2E_LABEL_FILTER}")
fi
if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
  GINKGO_FLAGS+=(--github-output)
fi

ci_step "GINKGO SUITE — look for [e2e] SPEC START/PASS lines and ==> stage banners"

# Ginkgo v2.29+: --procs must be set via the ginkgo CLI, not go test -ginkgo.procs.
go tool ginkgo run "${GINKGO_FLAGS[@]}" ./test/e2e/...
