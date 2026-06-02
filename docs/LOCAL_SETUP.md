# Local setup

Install the tools needed to build, test, and run **Kurator** on your machine.
For day-to-day workflow after setup, see [DEVELOPMENT.md](DEVELOPMENT.md).

Doc index: [README.md](README.md)

## On this page

| | Section |
|---|---------|
| 📋 | [What you need (by tier)](#what-you-need-by-tier) |
| 🔧 | [Go toolchain (pinned in go.mod)](#go-toolchain-pinned-in-gomod) |
| 📦 | [External tools — install by OS](#external-tools--install-by-os) |
| ⚡ | [Quick install (Brewfile + Task)](#quick-install-brewfile--task) |
| ✅ | [Verify your setup](#verify-your-setup) |
| 🖥️ | [Local kind cluster extras](#local-kind-cluster-extras) |
| 🪝 | [Optional quality-of-life tools](#optional-quality-of-life-tools) |
| 🆘 | [Troubleshooting installs](#troubleshooting-installs) |

## What you need (by tier)

Pick the tier that matches what you want to do. Each tier includes everything
above it.

| Tier | Goal | External tools (beyond Go) |
|------|------|----------------------------|
| **A — Inner loop** | `task lint`, `task test:run`, `task build` | **Task** |
| **B — Integration tests** | `task test:integration:local` (Docker MQ, no Kubernetes) | Tier A + **Docker** (or Podman/nerdctl) |
| **C — Full local stack** | `task local:up` (kind + IBM MQ + operator) | Tier B + **kind**, **kubectl**, **Helm**, **Terraform**, **mkcert** |

Go itself is required for every tier. Several dev tools are **not** installed
separately — they ship via `go.mod` `tool` directives (see below).

**Version pins used in CI** (match these when installing manually):

| Tool | Pinned version (CI) | Where defined |
|------|---------------------|---------------|
| Go | **1.26.3** | `go.mod` (Taskfile derives `GOTOOLCHAIN` from it) |
| Task | **3.51.1** | `Taskfile.yml` (`TASK_VERSION`) + CI `arduino/setup-task` |
| kind | **v0.27.0** | `Taskfile.yml` (`KIND_VERSION`) |
| mkcert | **v1.4.4** | `Taskfile.yml` (`MKCERT_VERSION`) |
| Terraform | **1.9.8** | `Taskfile.yml` (`TERRAFORM_VERSION`) |
| git-cliff | **v2.13.1** | `Taskfile.yml` (`GIT_CLIFF_VERSION`) + release workflow |
| Helm | latest from `azure/setup-helm` | `.github/workflows/e2e.yaml` |

Helm and kubectl are not pinned to a specific minor in-repo; use a recent stable
release. Terraform must be **≥ 1.5.0** (`hack/kind-cluster/terraform/versions.tf`).

## Go toolchain (pinned in go.mod)

Install Go **1.26.3** (or enable auto-download — `task` sets `GOTOOLCHAIN` from `go.mod`).

These tools are **already pinned** in `go.mod` and invoked with `go tool …` —
no separate install step:

| Tool | Invoked as | Used for |
|------|------------|----------|
| controller-gen | `go tool controller-gen` | CRDs, RBAC, webhooks (`task manifests`) |
| kustomize | `go tool kustomize` | Kustomize deploy path (`task deploy`) |
| mockery | `go tool mockery` | MQAdmin mocks (`task test:generate`) |
| ginkgo | `go tool ginkgo` | Unit + envtest (`task test:run`) |
| golangci-lint | `go tool golangci-lint` | Lint (`task lint`) |
| goimports | `go tool goimports` | Formatting (`task format`) |
| golines | `go tool golines` | Line wrapping (`task format`) |
| setup-envtest | `go tool setup-envtest` | envtest binaries (first `task test:run`) |
| govulncheck | `go tool govulncheck` | Vulnerability scan (`task vuln:check`) |

After cloning:

```sh
task install    # go mod download + verify
```

The first `task test:run` downloads envtest control-plane binaries (needs network).

## Quick install (Brewfile + Task)

**macOS — one command for Homebrew packages:**

```sh
brew bundle    # reads Brewfile at repo root (Go, Task, kind, kubectl, helm, …)
```

**Any OS — CI-pinned binaries into `bin/`** (kind, mkcert, task, terraform):

```sh
task tools:install
export PATH="$PWD/bin:$PATH"   # or add to your shell profile
```

**Check what is installed** (tier A/B/C via `TOOLS_TIER`):

```sh
task tools:check              # tier C (full stack) by default
TOOLS_TIER=A task tools:check # inner loop only
```

The dev container runs `task tools:install` and `task tools:check` on first
create — see [`.devcontainer/`](../.devcontainer/).

## External tools — install by OS

### macOS (Homebrew)

```sh
brew bundle    # preferred — installs from Brewfile (all tiers + optional QoL tools)
```

Or install manually:

```sh
# Tier A
brew install go go-task/tap/go-task

# Tier B — container runtime (pick one)
brew install --cask docker          # Docker Desktop (recommended)
# or: brew install podman && podman machine init && podman machine start

# Tier C — local kind platform
brew install kind kubectl helm terraform mkcert
```

Pin Task to the CI version if you want exact parity:

```sh
brew install go-task/tap/go-task@3.51
# or upgrade: brew upgrade go-task
```

Install pinned kind/mkcert binaries manually (optional):

```sh
# kind v0.27.0
curl -Lo ./kind "https://kind.sigs.k8s.io/dl/v0.27.0/kind-darwin-$(uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/')"
chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind

# mkcert v1.4.4
curl -Lo ./mkcert "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-darwin-$(uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/')"
chmod +x ./mkcert && sudo mv ./mkcert /usr/local/bin/mkcert
mkcert -install   # trust local CA (may prompt for password)
```

### Linux (Debian / Ubuntu)

```sh
# Go — use https://go.dev/dl/ or your distro if it ships 1.26+
# Example (adjust arch/version):
# curl -fsSL https://go.dev/dl/go1.26.3.linux-amd64.tar.gz | sudo tar -C /usr/local -xz

# Tier A — Task
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin v3.51.1

# Tier B — Docker
# sudo apt install docker.io docker-compose-plugin
# sudo usermod -aG docker "$USER"   # log out/in after

# Tier C
sudo apt install kubectl
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
curl -fsSL https://releases.hashicorp.com/terraform/1.9.8/terraform_1.9.8_linux_amd64.zip -o /tmp/tf.zip
unzip /tmp/tf.zip -d ~/.local/bin && chmod +x ~/.local/bin/terraform

# kind v0.27.0
curl -Lo ~/.local/bin/kind "https://kind.sigs.k8s.io/dl/v0.27.0/kind-linux-amd64"
chmod +x ~/.local/bin/kind

# mkcert v1.4.4
curl -Lo ~/.local/bin/mkcert "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-linux-amd64"
chmod +x ~/.local/bin/mkcert
mkcert -install   # may need sudo for NSS/system trust store
```

Ensure `~/.local/bin` is on your `PATH`.

### Linux (Fedora / RHEL)

```sh
sudo dnf install golang docker kubectl
# Task, kind, mkcert, Terraform, Helm — use install scripts above or distro packages
# where versions are recent enough (Terraform >= 1.5, Go >= 1.26)
```

### Windows

Native Windows is **not** tested for the local kind stack. Use **WSL2** with the
Linux instructions above, or open the repo in the **dev container**
([`.devcontainer/`](../.devcontainer/)).

### Container runtime notes

Scripts under `hack/kind-cluster` auto-detect **docker → nerdctl → podman**.
Override with `KIND_EXPERIMENTAL_PROVIDER` if needed.

- **Docker Desktop** (macOS/Windows) or **docker** on Linux is the smoothest path.
- **Rootless Podman** is not supported for kind in this repo (see `kind-up.sh`).
- Integration tests (`hack/mq-docker`) expect `docker compose`.

## Verify your setup

Run from the repository root after installing your tier:

### Tier A — inner loop

```sh
TOOLS_TIER=A task tools:check
go version          # go1.26.3
task --version      # 3.x (3.51.1 matches CI)
task install
task lint
task test:run
task build
```

### Tier B — integration tests

```sh
docker info
task test:integration:local
```

### Tier C — full local stack

```sh
task tools:check
kind version        # v0.27.x
kubectl version --client
helm version
terraform version   # >= 1.5.0 (1.9.8 in CI)
mkcert -version

task local:up       # first run: image pulls, MQ chart wait (~10–15 min)
task local:info
```

Success looks like: MQ console at `https://mq.localhost:30443/ibmmq/console/`,
operator pods in `kurator-system`, sample CRs with `Synced=True`.

## Local kind cluster extras

Tier C tools are used only by `hack/kind-cluster` (included as `task cluster:*`
from the root Taskfile). What each one does:

| Tool | Role in `task cluster:up` |
|------|---------------------------|
| **kind** | Creates the `kurator` Kubernetes cluster with NodePorts 30080/30443 |
| **Docker** | kind node images; `kind load docker-image` for the operator |
| **kubectl** | Talk to the cluster; MQ CLI helpers (`task mq:cli`) |
| **mkcert** | Wildcard TLS for `*.localhost` (HAProxy ingress, MQ console) |
| **Terraform** | Installs HAProxy ingress, cert-manager, monitoring, IBM MQ (Helm) |
| **Helm** | Used by Terraform providers to deploy charts |

Optional: enable [direnv](https://direnv.net/) — the repo `.envrc` exports:

```sh
export KUBECONFIG=$PWD/hack/kind-cluster/.state/kubeconfig.yaml
export TF_VAR_kubeconfig=$KUBECONFIG
```

Run `direnv allow` once in the repo root.

Platform details: [hack/kind-cluster/README.md](../hack/kind-cluster/README.md).
Workflow after setup: [DEVELOPMENT.md](DEVELOPMENT.md).

## Optional quality-of-life tools

| Tool | Why | Install |
|------|-----|---------|
| **pre-commit** | Runs format, lint, verify, gitleaks before commit | `pip install pre-commit` then `pre-commit install` |
| **gitleaks** | Secret scanning (`task secrets:scan`; also in pre-commit) | `brew install gitleaks` or [releases](https://github.com/gitleaks/gitleaks) |
| **direnv** | Auto-export `KUBECONFIG` for local cluster | `brew install direnv` / `apt install direnv` |
| **git-cliff** | Changelog generation | `task tools:git-cliff` (downloads pinned binary to `bin/`) |

pre-commit hooks call `task format`, `task lint`, `task verify`, and
`task vuln:check` — Tier A must be working first.

## Troubleshooting installs

| Symptom | Fix |
|---------|-----|
| `go: cannot find main module` | Run commands from repository root |
| `task: command not found` | `brew bundle`, `task tools:install`, or add `bin/` to `PATH` |
| `go tool golangci-lint`: slow first run | Normal — builds tool from `go.mod` |
| `kustomize: command not found` | Use `task deploy` (uses `go tool kustomize`), not bare `kustomize` |
| kind fails / wrong runtime | Ensure Docker daemon is running; try `KIND_EXPERIMENTAL_PROVIDER=docker` |
| Browser TLS warning on `*.localhost` | Run `mkcert -install` once, then `task cluster:tls` or re-run `task cluster:up` |
| Port 30080/30443 in use | Stop other kind clusters or services using those ports |
| IBM MQ pod slow | First pull is large; wait up to ~15 min, check `kubectl -n ibm-mq get pods` |
| envtest download fails | Network/firewall; `task test:run` needs to fetch K8s test binaries |

More runtime issues: [DEVELOPMENT.md#troubleshooting](DEVELOPMENT.md#troubleshooting).
