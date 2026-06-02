# Development

How to set up, build, test, and run **Kurator** locally. For
conventions see [../AGENTS.md](../AGENTS.md); for design see
[ARCHITECTURE.md](ARCHITECTURE.md).

## Quick start

From the repository root (see [Prerequisites](#prerequisites)):

```sh
task local:up       # kind + IBM MQ + operator + sample CRs
task local:info     # URLs, credentials, qmc/queue status
# … hack on code …
task local:deploy   # rebuild image and refresh operator + samples
task local:down     # full teardown
```

Optional: enable [direnv](https://direnv.net/) so `.envrc` exports
`KUBECONFIG=hack/kind-cluster/.state/kubeconfig.yaml` automatically.

Platform-only commands live under `task cluster:*` (see
[hack/kind-cluster/README.md](../hack/kind-cluster/README.md)).

## Prerequisites

| Tool | Why |
|------|-----|
| **Go** (the version in `go.mod`) | Build/test the operator |
| **Task** ([taskfile.dev](https://taskfile.dev)) | Single entry point for all workflows |
| **Docker** (or **nerdctl**/**Podman**) | Container runtime for kind and image builds |
| **kind** | Local Kubernetes cluster |
| **kubectl** | Talk to the cluster |
| **Terraform** | Provision the local platform (ingress, cert-manager, monitoring, IBM MQ) |
| **Helm** | Used by Terraform to install charts |
| **mkcert** | Trusted local TLS for `*.localhost` |

Go-based tools (controller-gen, kustomize, mockery, ginkgo, setup-envtest,
golangci-lint) are pinned via `go.mod` `tool` directives and invoked with
`go tool <name>` — no separate install needed.

Optional: **direnv** to auto-export `KUBECONFIG` for the local cluster.

## The inner loop

Fast feedback without a cluster (mocks + envtest):

```sh
task install      # download/verify modules
task generate     # deepcopy + mocks
task manifests    # CRDs + RBAC
task lint         # golangci-lint v2
task test:run     # unit + envtest (Ginkgo, -race, coverage)
```

`task verify` re-runs codegen into a scratch area and fails if anything is stale
— run it before committing (pre-commit does this automatically).

### Codegen verification (`hack/verify.sh`)

`task verify` runs `hack/verify.sh`, which implements the **generate / verify**
discipline from [CICD.md](CICD.md):

1. Snapshot committed generated artifacts (`config/crd/bases`, `config/rbac`,
   `api/*/zz_generated.deepcopy.go`).
2. Regenerate with `controller-gen`.
3. `diff` snapshot vs working tree and fail on drift.

This catches the common mistake of editing API types or kubebuilder markers
without re-running `task generate && task manifests`. It is unrelated to
`go mod verify` (module checksum verification in `task install`).

### Task vs Makefile

**Task is the canonical entry point** ([ADR-0004](adr/0004-task-as-task-runner.md)):
humans, pre-commit, and CI all run `task <target>`. The root `Makefile` is
**Kubebuilder scaffold** — it ships with `kubebuilder init` and is still used
by the default e2e suite (`make docker-build`, `make deploy`). Prefer `task`
for day-to-day work; ignore `make` unless you are running that scaffold as-is.
A future cleanup can rewire e2e to `task deploy` and trim the Makefile.

Build the manager binary (CGO-free, static):

```sh
task build
```

### Logging

Logging is configured via YAML file, environment variables, or flags (see
[LOGGING.md](LOGGING.md)). Quick local examples:

```sh
# Human-readable logs on stderr (default when not in a pod)
go run ./cmd/main.go --log-format=text --log-level=debug

# JSON to stdout (production-style)
go run ./cmd/main.go --log-format=json --log-level=info

# File-based config
export KURATOR_LOG_CONFIG=config/samples/logging-config.yaml
go run ./cmd/main.go
```

In the cluster, the manager Deployment sets `KURATOR_LOG_FORMAT=json` and
`KURATOR_LOG_LEVEL=info` by default.

## Local platform (kind + IBM MQ)

The `hack/kind-cluster` tree provisions a complete environment: a kind cluster
with **HAProxy Ingress** (NodePorts 30080/30443), **cert-manager**, an optional
**kube-prometheus-stack**, and a real **IBM MQ** Queue Manager exposing `mqweb`
— wired with **Terraform** and trusted TLS from **mkcert**.

> **Canonical reference** for local URLs, credentials, task targets, and test
> tiers is this section and [hack/kind-cluster/README.md](../hack/kind-cluster/README.md).
> [README.md](../README.md) and [INSTALL_AND_USE.md](INSTALL_AND_USE.md) link here
> for quick start only.

Cluster name: `kurator` (override with `CLUSTER_NAME` if you have an existing
`ibm-mq-operator` cluster from before the rename). State (kubeconfig, TLS) is
written to `hack/kind-cluster/.state/`.

### Bring it up

```sh
task cluster:up      # kind + mkcert TLS + terraform (HAProxy ingress, cert-manager, Argo CD, monitoring, IBM MQ)
task cluster:info    # re-print URLs and credentials
```

Equivalent scripts (if you prefer not to use Task):

```sh
cd hack/kind-cluster
./scripts/kind-up.sh && ./scripts/mkcert-gen.sh && ./scripts/terraform-apply.sh
./scripts/info.sh
export KUBECONFIG="$(pwd)/.state/kubeconfig.yaml"
```

`task cluster:up` is idempotent: an existing `kurator` kind cluster is reused.
Override the cluster name with `CLUSTER_NAME=…` (legacy clusters may still be
named `ibm-mq-operator`).

### Endpoints

Printed by `task cluster:info` / `./scripts/info.sh` after bring-up:

| Target | URL |
|--------|-----|
| IBM MQ web console | `https://mq.localhost:30443/ibmmq/console/` |
| IBM MQ admin REST (via ingress) | `https://mq.localhost:30443/ibmmq/rest/v3/admin/qmgr` |
| Argo CD | `https://argocd.localhost:30443/` |
| Grafana | `https://grafana.localhost:30443/` |
| In-cluster mqweb (`QueueManagerConnection.spec.endpoint`) | `https://ibm-mq.ibm-mq.svc:9443` |

Defaults (override via Terraform variables in `hack/kind-cluster/terraform/`):
Queue Manager **`QM1`**, MQ admin **`admin`** / **`passw0rd`**, namespace
**`ibm-mq`**. Sample CRs use a `mq-credentials` Secret in **`kurator-system`**
(see `charts/kurator/samples/resources/`). These are **local-dev defaults
only** — never reuse them anywhere real.

## Deploying a queue manager for Kurator

Kurator requires an **existing** queue manager with **mqweb** enabled. It does not
install or upgrade Queue Managers. Choose one of the options below; then point a
`QueueManagerConnection` at the in-cluster mqweb URL (or a reachable equivalent).

When using a local `references/` clone, keep your own gitignored `docs/REFERENCES.md`
notes (not published in this repository).

### Option A — mq-helm on kind (recommended for local dev)

This is what `hack/kind-cluster` provisions: the upstream
[ibm-messaging/mq-helm](https://github.com/ibm-messaging/mq-helm) chart with
`web.enable: true` and a `mq-credentials` Secret (`mqAdminPassword` / `mqAppPassword`).

```sh
task cluster:up
task local:deploy    # or: task deploy:helm && task deploy:samples
```

The operator pod uses in-cluster DNS (`https://ibm-mq.ibm-mq.svc:9443`). Use
ingress URLs from `task cluster:info` for browser console and manual REST
checks from your laptop.

### Option B — IBM MQ Operator (OpenShift or EKS preview)

Use when the queue manager is already managed by IBM’s operator. Kurator only needs
mqweb credentials and network reachability from the operator namespace.

**OpenShift:** install the IBM Operator Catalog (`icr.io/cpopen/ibm-operator-catalog`)
and the **IBM MQ** operator from OperatorHub. See
[ibm-messaging/mq-gitops-samples](https://github.com/ibm-messaging/mq-gitops-samples/tree/main/queue-manager-basic-deployment) (local clone under `references/`, gitignored).

**Amazon EKS (preview):** Helm chart and CRD in
[ibm-messaging/mq-operator-eks-preview-2025](https://github.com/ibm-messaging/mq-operator-eks-preview-2025); no
controller source is published.

Minimum `QueueManager` fields for mqweb (adapt namespace/license as required):

| Field | Purpose |
|-------|---------|
| `spec.web.enabled: true` | Enables IBM MQ Console and REST APIs on the QM pod |
| `spec.web.console.authentication.provider` / `authorization.provider` | e.g. `manual` for basic registry (see gitops `qmdemo-qm.yaml`) |
| `spec.pki.keys` / `spec.pki.trust` | TLS material for the QM pod (often from cert-manager Secrets) |
| `spec.queueManager.name` | QM name — must match `QueueManagerConnection.spec.queueManager` |
| `spec.queueManager.mqsc` | Optional bootstrap MQSC via ConfigMap (channels, CHLAUTH at install). Kurator reconciles **additional** objects later via CRs. |

On **EKS**, disable OpenShift-only routes in the `QueueManager` spec (see
[Ingress for IBM MQ Console and REST APIs](https://github.com/ibm-messaging/mq-operator-eks-preview-2025/blob/main/configuring_Ingress_and_LoadBalancers/Ingress_for_IBM_MQ_Console_and_REST_APIs.md)):

- `spec.web.route.enabled: false`
- `spec.queueManager.route.enabled: false`
- `spec.queueManager.metrics.serviceMonitor.enabled: false` (unless you run Prometheus Operator)

Create a Kubernetes Secret with mqweb admin credentials. Kurator accepts `username` +
`password` or `mqAdminPassword` (see `internal/adapter/mqrest/factory.go`).

Example `QueueManagerConnection` (same as [samples](../config/samples/)):

```yaml
spec:
  queueManager: QM1   # or your spec.queueManager.name
  endpoint: https://<qm-service>.<namespace>.svc:9443
  tls:
    caSecretRef:
      name: <ca-secret>   # omit insecureSkipVerify in production
  credentialsSecretRef:
    name: mq-credentials
```

### Option C — Other environments

Any queue manager (VM, container, Cloud Pak) works if mqweb is reachable and
admin credentials are in a referenced Secret. Use [IBM_MQ_REST_API.md](IBM_MQ_REST_API.md)
for CSRF, TLS, and MQSC endpoint details.

### Deploy the operator

| Task | Install path | When to use |
|------|----------------|-------------|
| `task local:up` | cluster + Helm + samples | First-time full stack |
| `task local:deploy` | Helm + samples | Cluster/MQ already running |
| `task deploy:helm` | Helm chart only | Operator only |
| `task deploy:samples` | Sample Secret + CRs | After any operator install |
| `task deploy` | Kustomize (`config/default` + CRDs) | controller-runtime / kubebuilder workflow |

**Helm (recommended on kind):**

```sh
task deploy:helm
task deploy:samples
```

**Kustomize:**

```sh
task deploy
kubectl apply -k config/samples    # legacy kubebuilder samples
# or: task deploy:samples          # Helm-aligned samples (preferred)
```

`task deploy` uses `go tool kustomize` (pinned via `go.mod`) — no separate
kustomize binary required.

Both install paths target namespace **`kurator-system`** and expect mqweb at
`https://ibm-mq.ibm-mq.svc:9443` after `task cluster:up`. Chart details:
[charts/kurator/README.md](../charts/kurator/README.md).

### Tear down

```sh
task local:down      # undeploy Helm/samples + delete kind cluster + wipe .state
```

Partial teardown:

```sh
task undeploy:helm   # remove operator and sample CRs; keep kind/MQ
task cluster:cleanup # terraform destroy only (keeps kind cluster)
task cluster:down    # full platform teardown
```

## Test tiers

| Tier | Scope | Needs a cluster? | Command |
|------|-------|------------------|---------|
| **Unit** | Reconciler logic + REST adapter vs mocks / `httptest` | No | `task test:run` |
| **envtest** | Controller + API against a real API server (`setup-envtest`), `MQAdmin` mocked | No (downloads control-plane binaries) | `task test:run` |
| **Integration** | `mqrest` / `mqadmin.Admin` queue CRUD against live mqweb | No (Docker MQ only) | `task test:integration` / `task test:integration:local` |
| **e2e** | Operator in kind against live IBM MQ; asserts real MQSC | Yes (`task cluster:up` or `task ci:e2e`) | `task test:e2e` / `task ci:e2e` |

### IBM MQ integration tests (Docker)

Fast contract tests for queue object operations via mqweb — no Kubernetes or
operator required. Uses `//go:build integration` in [`test/integration/mq/`](../test/integration/mq/).

```sh
task test:integration:local   # docker compose up + wait + tests (first run: image pull)
# or, if the container is already up:
task test:integration
task mq:integration:down
```

Set `KURATOR_INTEGRATION_MQ=1` (done automatically by `task test:integration`).
Without it, integration tests skip so IDEs can run `-tags=integration` safely.

| Variable | Default (Docker) | Purpose |
|----------|------------------|---------|
| `KURATOR_INTEGRATION_MQ` | unset → skip | Enable integration tests |
| `KURATOR_INTEGRATION_MQ_ENDPOINT` | `https://127.0.0.1:9443` | mqweb base URL |
| `KURATOR_INTEGRATION_MQ_QMGR` | `QM1` | Queue manager name |
| `KURATOR_INTEGRATION_MQ_USER` / `_PASSWORD` | `admin` / `passw0rd` | Basic auth |
| `KURATOR_INTEGRATION_MQ_INSECURE_TLS` | `true` | Self-signed container cert |
| `KURATOR_INTEGRATION_MQ_HOST` | empty | Set to `mq.localhost` when reusing kind NodePort |

Reuse the kind platform MQ instead of a second container:

```sh
export KURATOR_INTEGRATION_MQ=1
export KURATOR_INTEGRATION_MQ_ENDPOINT=https://127.0.0.1:30443
export KURATOR_INTEGRATION_MQ_HOST=mq.localhost
task test:integration
```

See [`hack/mq-docker/README.md`](../hack/mq-docker/README.md).

**IBM MQ e2e scenarios** (queue reconcile, channel/auth fixtures) run only when
`KURATOR_E2E_MQ=1` is set and the kind platform with IBM MQ is up. Without that,
the scaffold e2e suite (controller pod, metrics) still runs. MQ-specific tests use
defaults aligned with `hack/kind-cluster` (`QM1`, `admin` / `passw0rd`, endpoint
`https://ibm-mq.ibm-mq.svc:9443`). Override with `KURATOR_E2E_MQ_*` env vars
documented in [`test/e2e/fixtures/README.md`](../test/e2e/fixtures/README.md).

Guidelines:

- Unit + envtest must stay fast and hermetic; mock the `MQAdmin` port, never hit
  a real Queue Manager.
- Integration is gated behind `//go:build integration` and `KURATOR_INTEGRATION_MQ=1`.
- e2e is gated behind a build tag (`//go:build e2e`) so it does not run in the
  default `go test ./...`.
- Keep coverage high on `internal/`; CI reports it.

## Troubleshooting

- **`kustomize: command not found`**: use `task deploy` (invokes `go tool
  kustomize`) or `task deploy:helm`; do not call bare `kustomize` unless installed.
- **kind can't start / wrong runtime**: scripts auto-detect docker → nerdctl →
  podman; override with `KIND_EXPERIMENTAL_PROVIDER`.
- **TLS not trusted in browser**: run `mkcert -install` once, then
  `task cluster:tls` or re-run `task cluster:up`.
- **IBM MQ pod slow to start**: chart wait up to ~15 min; check
  `kubectl -n ibm-mq get pods` and logs.
- **Queue `Synced=False` / MQSC errors**: check
  `kubectl describe queue -n kurator-system` and operator logs; see
  [IBM_MQ_REST_API.md](IBM_MQ_REST_API.md).
- **mqweb 401/403**: confirm `mq-credentials` in `kurator-system` (samples use
  `username` + `mqAdminPassword`; factory also accepts `password`).
- **envtest binaries missing**: `task test:run` downloads them via
  `setup-envtest` on first run (needs network).

## Before you push

1. `task verify` — generated artifacts are fresh.
2. `task lint` — clean.
3. `task test:run` — green.
4. Conventional Commit with a gitmoji (see [../AGENTS.md](../AGENTS.md)).
