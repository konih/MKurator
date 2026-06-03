#!/usr/bin/env bash
# Load the pinned IBM MQ image into a kind cluster (host image must exist).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
image="$("${root}/hack/ci/mq-image-ref.sh")"
cluster="${KIND_CLUSTER:-${CLUSTER_NAME:-kurator}}"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is required on PATH" >&2
  exit 1
fi

if ! docker image inspect "${image}" >/dev/null 2>&1; then
  echo "IBM MQ image not present on host: ${image}" >&2
  exit 1
fi

kind load docker-image "${image}" --name "${cluster}"
echo "Loaded ${image} into kind cluster ${cluster}"
