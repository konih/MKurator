#!/usr/bin/env bash
# Docker MQ up + wait + integration tests (used by task test:integration:local).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/ci/suite-lock.sh
source "${ROOT}/hack/ci/suite-lock.sh"

suite_lock_acquire "${EXCLUSIVE_TEST_LOCK_NAME}"
export KURATOR_SUITE_LOCK_HELD=1
trap suite_lock_release EXIT

cd "${ROOT}"
task mq:integration:up
task mq:integration:wait
task test:integration
