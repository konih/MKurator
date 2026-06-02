#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

echo "===================================="
echo "Kurator DevContainer setup"
echo "===================================="

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR: post-install must run as root in the devcontainer" >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "WARNING: unsupported architecture $(uname -m), defaulting to amd64"
    ARCH="amd64"
    ;;
esac

echo ""
echo "Installing kubectl and helm..."
if ! command -v kubectl >/dev/null 2>&1; then
  KUBECTL_VERSION="$(curl -fsSL https://dl.k8s.io/release/stable.txt)"
  curl -fsSL -o /usr/local/bin/kubectl \
    "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl"
  chmod +x /usr/local/bin/kubectl
fi

if ! command -v helm >/dev/null 2>&1; then
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
fi

echo ""
echo "Installing CI-pinned platform tools into bin/ (via Taskfile.yml)..."
mkdir -p "${ROOT}/bin"

if ! command -v task >/dev/null 2>&1; then
  TASK_VERSION="$(sed -n 's/^  TASK_VERSION: "\(.*\)"/\1/p' Taskfile.yml | head -1)"
  bash hack/install-external-tool.sh task "v${TASK_VERSION}" "${ROOT}/bin/task"
fi

export PATH="${ROOT}/bin:${PATH}"
task tools:install
chown -R vscode:vscode "${ROOT}/bin" 2>/dev/null || true

echo ""
echo "Waiting for Docker..."
for i in $(seq 1 60); do
  if docker info >/dev/null 2>&1; then
    echo "Docker is ready"
    break
  fi
  if [ "${i}" -eq 60 ]; then
    echo "WARNING: Docker not ready after 60s (kind/local:up may fail until it is)"
  fi
  sleep 1
done

if docker info >/dev/null 2>&1 && ! docker network inspect kind >/dev/null 2>&1; then
  docker network create kind >/dev/null 2>&1 || true
fi

echo ""
echo "Go modules and tool check..."
task install
task tools:check

echo ""
echo "===================================="
echo "DevContainer ready"
echo "===================================="
echo "  task lint / task test:run     — inner loop (tier A)"
echo "  task test:integration:local — Docker MQ (tier B)"
echo "  task local:up                 — kind + IBM MQ + operator (tier C)"
echo "  docs/LOCAL_SETUP.md           — full tool reference"
