#!/usr/bin/env bash
# Run IBM MQ integration tests (used by task test:integration).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/ci/suite-lock.sh
source "${ROOT}/hack/ci/suite-lock.sh"

if [[ "${KURATOR_SUITE_LOCK_HELD:-}" != "1" ]]; then
  suite_lock_acquire "${EXCLUSIVE_TEST_LOCK_NAME}"
  trap suite_lock_release EXIT
fi

cd "${ROOT}"
go test -tags=integration -race -v ./test/integration/mq/...
