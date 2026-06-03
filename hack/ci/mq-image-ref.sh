#!/usr/bin/env bash
# Print the pinned IBM MQ container image (integration + kind platform).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose="${root}/hack/mq-docker/docker-compose.yml"

if [[ ! -f "${compose}" ]]; then
  echo "missing ${compose}" >&2
  exit 1
fi

line="$(grep -E '^[[:space:]]*image:[[:space:]]*icr\.io/ibm-messaging/mq:' "${compose}" | head -1)"
if [[ -z "${line}" ]]; then
  echo "IBM MQ image line not found in ${compose}" >&2
  exit 1
fi

echo "${line#*image:}" | tr -d '[:space:]'
