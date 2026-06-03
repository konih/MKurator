#!/usr/bin/env bash
# File mutex (flock) for machine-exclusive test suites.
#
# E2e and integration share kind, Docker MQ, kubeconfig, and operator deploy —
# only one may run at a time on a host.
#
# Usage:
#   bash hack/ci/suite-lock.sh <name> <command...>
#   source hack/ci/suite-lock.sh && suite_lock_acquire <name>
#   suite_lock_release   # call from EXIT trap after acquire
set -euo pipefail

if [[ -z "${SUITE_LOCK_ROOT:-}" ]]; then
  SUITE_LOCK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fi
SUITE_LOCK_DIR="${SUITE_LOCK_ROOT}/hack/kind-cluster/.state/locks"

# Shared lock for e2e and integration (kind + Docker MQ conflicts).
EXCLUSIVE_TEST_LOCK_NAME="exclusive-test"

_SUITE_LOCK_PATH=""
_SUITE_LOCK_NAME=""
_SUITE_LOCK_ACTIVE=0

suite_lock_path() {
  printf '%s/%s.lock' "${SUITE_LOCK_DIR}" "$1"
}

_suite_lock_holder_hint() {
  local pid=""
  if [[ -s "${_SUITE_LOCK_PATH}" ]]; then
    pid="$(tr -d '[:space:]' < "${_SUITE_LOCK_PATH}" 2>/dev/null || true)"
  fi
  if [[ -n "${pid}" && "${pid}" =~ ^[0-9]+$ ]]; then
    if kill -0 "${pid}" 2>/dev/null; then
      echo "  holder PID: ${pid} (still running)" >&2
    else
      echo "  stale holder PID in lock file: ${pid} (process exited; remove ${_SUITE_LOCK_PATH} if lock persists)" >&2
    fi
  fi
}

suite_lock_acquire() {
  local name="$1"
  if [[ "${KURATOR_SUITE_LOCK_HELD:-}" == "1" ]]; then
    return 0
  fi
  if [[ "${_SUITE_LOCK_ACTIVE}" -eq 1 ]]; then
    return 0
  fi

  _SUITE_LOCK_NAME="$name"
  _SUITE_LOCK_PATH="$(suite_lock_path "$name")"
  mkdir -p "${SUITE_LOCK_DIR}"

  exec 200>"${_SUITE_LOCK_PATH}"
  if ! flock -n 200; then
    echo "error: cannot acquire suite lock \"${name}\" — another e2e or integration run is active" >&2
    echo "  lock file: ${_SUITE_LOCK_PATH}" >&2
    _suite_lock_holder_hint
    echo "  hint: wait for the other run to finish, or stop it (kill the holder PID)" >&2
    exit 1
  fi

  printf '%s\n' "$$" > "${_SUITE_LOCK_PATH}"
  _SUITE_LOCK_ACTIVE=1
  export KURATOR_SUITE_LOCK_HELD=1
}

suite_lock_release() {
  if [[ "${_SUITE_LOCK_ACTIVE}" -eq 0 ]]; then
    return 0
  fi
  flock -u 200 2>/dev/null || true
  exec 200>&- || true
  _SUITE_LOCK_ACTIVE=0
  unset KURATOR_SUITE_LOCK_HELD
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -lt 2 ]]; then
    echo "usage: $(basename "$0") <lock-name> <command...>" >&2
    exit 2
  fi
  name="$1"
  shift
  suite_lock_acquire "$name"
  trap suite_lock_release EXIT
  "$@"
fi
