#!/usr/bin/env bash
# CI-parity e2e workflow (used by task ci:e2e).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/ci/suite-lock.sh
source "${ROOT}/hack/ci/suite-lock.sh"

suite_lock_acquire "${EXCLUSIVE_TEST_LOCK_NAME}"
export KURATOR_SUITE_LOCK_HELD=1
trap suite_lock_release EXIT

export CERT_MANAGER_INSTALL_SKIP="${CERT_MANAGER_INSTALL_SKIP:-true}"
export KURATOR_E2E_MQ="${KURATOR_E2E_MQ:-1}"
export KIND_CLUSTER="${KIND_CLUSTER:-${CLUSTER_NAME:-kurator}}"
export KUBECONFIG="${KUBECONFIG:-${ROOT}/hack/kind-cluster/.state/kubeconfig.yaml}"

cd "${ROOT}"

bash hack/ci/step.sh "Phase 1/3 — kind cluster + IBM MQ platform (task cluster:up)"
task cluster:up
bash hack/ci/step.sh "Phase 2/3 — wait for mqweb (NodePort / HAProxy)"
bash hack/ci/wait-mqweb.sh
bash hack/ci/step.sh "Phase 3/3 — Ginkgo e2e suite (image build + deploy)"
task test:e2e
