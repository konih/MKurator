#!/usr/bin/env bash
# Run self-hosted Renovate in Docker against the local git clone.
# Requires: docker, gh auth login (or RENOVATE_TOKEN in the environment).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

if [[ -z "${RENOVATE_TOKEN:-}" ]]; then
  RENOVATE_TOKEN="$(gh auth token)"
fi

REPO_PATH="${ROOT}/.renovate/repos/github.com/conduit-ops/mkurator"
mkdir -p "$(dirname "${REPO_PATH}")"
if [[ ! -d "${REPO_PATH}/.git" ]]; then
  git clone --local . "${REPO_PATH}"
fi

docker run --rm --user "$(id -u):$(id -g)" \
  -v "${ROOT}/.renovate:/tmp/renovate" \
  -e "RENOVATE_TOKEN=${RENOVATE_TOKEN}" \
  -e RENOVATE_BASE_DIR=/tmp/renovate \
  -e RENOVATE_PLATFORM=github \
  -e RENOVATE_REPOSITORIES=conduit-ops/MKurator \
  -e RENOVATE_ONBOARDING=false \
  -e 'RENOVATE_FORCE={"schedule":null}' \
  -e LOG_LEVEL="${LOG_LEVEL:-info}" \
  renovate/renovate:latest
