#!/usr/bin/env bash
# kind + TLS + Terraform IBM MQ; expects MQ image on host (see mq-docker-image action).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${root}"

task cluster:kind:up
bash hack/ci/kind-load-mq-image.sh
task cluster:tls
task cluster:apply
