# Development

How to set up, build, test, and run **Kurator** locally. For commit format and
contributor guidelines see [CONTRIBUTING.md](CONTRIBUTING.md); for Go style and
agent workflow see [../AGENTS.md](../AGENTS.md); for design see
[ARCHITECTURE.md](ARCHITECTURE.md).

The Git repository is [konih/kurator](https://github.com/konih/kurator); your
local clone directory may differ (for example `IBM-Message-Queue-Operator`).
See [ADR-0006](adr/0006-project-name-kurator.md).

Doc index: [README.md](README.md)

**Codegen and test matrix:** [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) (CRD
regeneration, reconciler/MQAdmin mocks, which tier to run). **Module layout:**
[GO_MODULE.md](GO_MODULE.md). **Runtime wiring:** [OPERATOR_RUNTIME.md](OPERATOR_RUNTIME.md).

## On this page

| | Section |
|---|---------|
| 🚀 | [Quick start](#quick-start) (includes [task reference](#task-reference)) |
| 📋 | [Prerequisites](#prerequisites) |
| 🔄 | [The inner loop](#the-inner-loop) |
| 🖥️ | [Local platform (kind + IBM MQ)](#local-platform-kind--ibm-mq) |
| 📦 | [Deploying a queue manager](#deploying-a-queue-manager-for-kurator) |
| 🧪 | [Test tiers](#test-tiers) |
| 📐 | [Developer guide](DEVELOPER_GUIDE.md) — regenerate / test checklist |
| 🆘 | [Troubleshooting](#troubleshooting) |
| ✉️ | [Commits and changelog](CONTRIBUTING.md) |
| ✅ | [Before you push](#before-you-push) |

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

### Task reference

| Task | What it does |
|------|----------------|
| `task tools:check` | Verify dev tools by tier A/B/C (`TOOLS_TIER` env) |
| `task tools:install` | Download CI-pinned kind/mkcert/task/terraform into `bin/` |
| `task local:up` | kind cluster + IBM MQ + operator (Helm) + sample CRs |
| `task local:info` | URLs, credentials, CR status |
| `task local:deploy` | Rebuild image, helm upgrade, re-apply samples (cluster already up) |
| `task local:down` | Undeploy samples/operator and delete kind cluster |
| `task cluster:up` | kind + ingress + cert-manager + monitoring + IBM MQ |
| `task cluster:info` | MQ/Grafana/Argo CD URLs and passwords |
| `task cluster:down` | Destroy platform and delete kind cluster |
| `task deploy` | Operator via Kustomize (`config/default` + CRDs) |
| `task deploy:helm` | Operator via [Helm chart](../charts/kurator/README.md) (recommended on kind) |
| `task deploy:samples` | Sample Secret + `QueueManagerConnection` + `Queue` + `Topic` + `Channel` |
| `task mq:console` | IBM MQ web UI URL (`https://mq.localhost:30443/ibmmq/console/`) |
| `task mq:cli` | Interactive `runmqsc` on QM1 |
| `task mq:runmqsc` | One-shot `runmqsc` (pass MQSC as args) |
| `task test:run` | Unit + envtest (`-race`) |
| `task test:schema` | CRD OpenAPI fragment contract (also in `task verify`; `make test-schema`) |
| `task test:schema:update` | Rewrite `test/schema/golden/` (`make test-schema-update`) |
| `task test:integration` | MQ integration tests vs Docker mqweb (`KURATOR_INTEGRATION_MQ=1`) |
| `task test:integration:local` | Docker MQ up + wait + integration tests |
| `task ci:integration` | Same as GitHub Actions integration job |
| `task test:e2e` | E2E on kind (set `KURATOR_E2E_MQ=1` for IBM MQ scenarios) |
| `task test:e2e:helm` | Same suite with Helm operator deploy (`KURATOR_E2E_DEPLOY=helm`; CI job `e2e (helm)` on `main` / `workflow_dispatch`) |
| `task ci:e2e` | Same as GitHub Actions e2e job (`cluster:up` + MQ wait + tests) |
| `task changelog` | Preview unreleased changelog (`git-cliff`; see [CICD.md](CICD.md)) |
| `task changelog:write` | Regenerate `CHANGELOG.md` before tagging a release |

After `task local:up`, check reconciliation:

```sh
kubectl get qmc,mq,tp,chl -n kurator-system
kubectl logs -n kurator-system deployment/kurator-controller-manager -f
```

Confirm on MQ (`task mq:runmqsc`):

```sh
task mq:runmqsc -- "DISPLAY QLOCAL('APP.ORDERS') MAXDEPTH"
task mq:runmqsc -- "DISPLAY TOPIC('RETAIL.ORDERS') TOPSTR"
task mq:runmqsc -- "DISPLAY CHANNEL('ORDERS.APP') CHLTYPE(SVRCONN)"
```

## Prerequisites

**Install guide:** [LOCAL_SETUP.md](LOCAL_SETUP.md) — tiered tool list, OS-specific
install commands, CI version pins, and verification steps.

Summary:

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
**Kubebuilder scaffold** — kept for framework compatibility; overlapping targets
delegate to Task so both produce the same artifacts.

| `make` target | `task` equivalent | Notes |
|---------------|-------------------|-------|
| `manifests` | `task manifests` | CRDs, RBAC, webhooks |
| `generate` | `task generate` | deepcopy + mocks |
| `docker-build` | `task docker:build` | set `IMG` → `DOCKER_IMAGE` |
| `install` | `task install:crds` | server-side CRD apply |
| `deploy` | `task deploy:operator` | Kustomize apply only (no image build) |
| `undeploy` | `task undeploy` | removes operator manifests |

Prefer `task` for day-to-day work. Use `make` only when following Kubebuilder
scaffold targets (`make test-e2e`, `make run`, …) or tooling that expects Make.

**E2e deploy:** process 1 in `SynchronizedBeforeSuite` builds and loads images once
(`task docker:build`), then installs CRDs and the operator via `task install:crds` +
`task deploy:operator` (Kustomize, default) or `task deploy:helm:operator` when
`KURATOR_E2E_DEPLOY=helm` — no second image build during deploy
(`test/e2e/deploy_helpers.go`). `AfterSuite` undeploys and removes test namespaces.

**E2e Helm admission path (deferred-e2e-helm):** validating webhook negative apply
specs in `test/e2e/e2e_test.go` exercise the same resource names whether the operator
is installed with Kustomize or Helm (`kurator-validating-webhook-configuration`,
`kurator-serving-cert`, `kurator-webhook-service`). Helm chart templates now render
those resources (`charts/kurator/templates/webhook-*.yaml`,
`validating-webhook-configuration.yaml`); `task helm:lint` runs
`hack/helm-verify-admission.sh` to keep them aligned with `config/webhook/manifests.yaml`.

| Deploy mode | Env | Install | Teardown | Task |
|-------------|-----|---------|----------|------|
| Kustomize (default) | unset or `KURATOR_E2E_DEPLOY=kustomize` | `task deploy` | `task undeploy:operator` | `task test:e2e` |
| Helm | `KURATOR_E2E_DEPLOY=helm` | `task deploy:helm` | `task undeploy:helm` | `task test:e2e:helm` |

**Helm e2e in CI:** [`.github/workflows/e2e.yaml`](../.github/workflows/e2e.yaml) runs
`e2e (helm)` on `workflow_dispatch` and weekly cron (not on every PR). PRs run
`e2e (kustomize)` only. Local Helm path: `KURATOR_E2E_MQ=1 task test:e2e:helm` after
`task cluster:up`. Full kustomize + Helm on one cluster: `KURATOR_CI_E2E_BOTH=1 task ci:e2e`.

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

When using a local `references/` clone, copy
[docs/REFERENCES.md.example](REFERENCES.md.example) to gitignored `docs/REFERENCES.md`
and edit as needed.

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

Design rationale: [adr/0011-layered-testing-strategy.md](adr/0011-layered-testing-strategy.md).
Per-change checklist: [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md).

### What CI proves

Same tiers as [README.md#what-ci-proves](../README.md#what-ci-proves) (summary table).
Commands and env vars below.

| Tier | Scope | Needs a cluster? | Command |
|------|-------|------------------|---------|
| **Unit** | Reconciler logic + REST adapter vs mocks / `httptest` | No | `task test:run` |
| **envtest** | Controller + API against a real API server (`setup-envtest`), `MQAdmin` mocked | No (downloads control-plane binaries) | `task test:run` |
| **Integration** | `mqrest` / `mqadmin.Admin` queue, topic, channel, CHLAUTH, AUTHREC against live mqweb | No (Docker MQ only) | `task test:integration` / `task test:integration:local` |
| **e2e** | Operator in kind against live IBM MQ; asserts real MQSC | Yes (`task cluster:up` or `task ci:e2e`) | `task test:e2e` / `task ci:e2e` |

### IBM MQ integration tests (Docker)

Fast contract tests for queue, topic, channel, CHLAUTH, and AUTHREC operations
via mqweb — no Kubernetes or operator required. Covers CRUD, replace-on-update,
delete (including idempotent delete), and not-found paths for auth as well as
queue/topic/channel. **Alias, remote queues, replace-on-update, and CHLAUTH/AUTHREC
edge cases** live here (ADR-0011); kind e2e keeps one happy-path reconcile + delete
per CR kind plus admission smoke. Uses `//go:build integration` in
[`test/integration/mq/`](../test/integration/mq/).

**Machine lock:** e2e and integration share Docker MQ, kind, kubeconfig, and
operator deploy on one host — only one suite may run at a time. Entry points
acquire a file lock (`flock`) at
`hack/kind-cluster/.state/locks/exclusive-test.lock`. A second concurrent run
exits immediately with the lock path and holder PID; wait for the other run or
stop that process.

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

**IBM MQ e2e scenarios** (Queue, Topic, Channel, ChannelAuthRule, AuthorityRecord
reconcile and delete; channel/auth fixtures) run only when `KURATOR_E2E_MQ=1` is
set and the kind platform with IBM MQ is up. Without that, the scaffold e2e suite
(controller pod, metrics) still runs. MQ-specific tests use defaults aligned with
`hack/kind-cluster` (`QM1`, `admin` / `passw0rd`, endpoint
`https://ibm-mq.ibm-mq.svc:9443`). Override with `KURATOR_E2E_MQ_*` env vars
documented in [`test/e2e/fixtures/README.md`](../test/e2e/fixtures/README.md).
The same **machine lock** as integration applies (`exclusive-test.lock`); do not
run `task ci:e2e`, `task test:e2e`, and integration tasks in parallel on one host.
Phase 5 and test-tier follow-ups: [ROADMAP.md](ROADMAP.md#phase-5--user--authority-management).

### Reading e2e output locally

`task test:e2e` runs `hack/ci/run-e2e.sh`, which prints **stage banners** in the
same style as other CI tasks (`==> <timestamp> …`). Full platform bring-up uses
`task ci:e2e`, which adds **PLATFORM UP** steps before the Ginkgo suite.

| Marker | Meaning |
|--------|---------|
| `==> … PLATFORM UP` | kind + IBM MQ (`ci:e2e` only) |
| `==> … GINKGO E2E` | Suite runner starting (`test:e2e`) |
| `==> … PLATFORM PREP` | BeforeSuite: image build/load, cert-manager |
| `==> … DEPLOY OPERATOR` | Operator install (`task deploy` or Helm) |
| `==> … MQ SUITE` | IBM MQ scenarios (`KURATOR_E2E_MQ=1`) |
| `[e2e] SPEC START` / `SPEC PASS` / `SPEC FAIL` | Per-spec boundaries |
| `[e2e] …` | Progress lines from `e2eBy()` inside specs |

Ginkgo runs with `-ginkgo.vv`, `-ginkgo.show-node-events`, and `-ginkgo.procs` (default
**3** via `KURATOR_E2E_NODES`). MQ specs use per-family namespaces (`kurator-e2e-queues`,
`kurator-e2e-topics`, `kurator-e2e-channels`, `kurator-e2e-auth`) and unique MQ object
prefixes per process (`E2E.N1.…`). CHLAUTH specs run in a **serial** `mq-auth-serial` lane.
PR CI sets `KURATOR_E2E_LABEL_FILTER='(smoke || mq) && !slow'` (manager smoke + MQ
paths; skips metrics and QMC rotation). Override
locally, e.g. `KURATOR_E2E_NODES=1 KURATOR_E2E_LABEL_FILTER= task test:e2e` for a full
serial run. `-race` stays enabled (`CGO_ENABLED=1`); reduce nodes on small hosts if flaky.
In GitHub Actions, `-ginkgo.github-output` is added for workflow annotations.

On spec failure, diagnostics go to **GinkgoWriter** (visible with `-v`). Controller
logs are **truncated** (`kubectl logs --tail=40`) unless
`KURATOR_E2E_VERBOSE_LOGS=1` is set for the full structured JSON stream.

```sh
KURATOR_E2E_MQ=1 task test:e2e                              # suite only (cluster already up)
KURATOR_E2E_NODES=3 KURATOR_E2E_MQ=1 task test:e2e        # parallel MQ lanes (default nodes)
KURATOR_E2E_NODES=1 KURATOR_E2E_MQ=1 task test:e2e        # fully serial Ginkgo
KURATOR_E2E_VERBOSE_LOGS=1 KURATOR_E2E_MQ=1 task test:e2e   # full controller logs on failure
task ci:e2e                                                 # PLATFORM UP + MQ wait + suite
KURATOR_CI_E2E_BOTH=1 task ci:e2e                           # same cluster: kustomize then Helm
```

Guidelines:

- Unit + envtest must stay fast and hermetic; mock the `MQAdmin` port, never hit
  a real Queue Manager.
- Integration is gated behind `//go:build integration` and `KURATOR_INTEGRATION_MQ=1`.
- e2e is gated behind a build tag (`//go:build e2e`) so it does not run in the
  default `go test ./...`.
- Keep coverage high on `internal/`; CI reports it.

## Operator tuning

**Concurrency (NFR PERF-3):** each reconciler uses `MaxConcurrentReconciles` workers.
Increase when reconciling many CRs against one mqweb endpoint (watch mqweb load):

```sh
# Helm / Kustomize manager Deployment args:
--max-concurrent-reconciles=4

# Or environment (used as default before the flag is parsed):
KURATOR_MAX_CONCURRENT_RECONCILES=4
```

Default is **1** (controller-runtime default). Values below 1 are clamped to 1.

**Health probes:** the manager serves `:8081` with `/healthz` (liveness, always ok) and
`/readyz` (readiness). Readiness reflects aggregated `QueueManagerConnection` status: no
QMCs → ready; at least one `Ready=True` → ready; otherwise not ready (e.g. all pings
failing). See [ARCHITECTURE.md](ARCHITECTURE.md#operator-runtime-concerns) and
`internal/health/mq_connectivity.go`.

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
4. [Conventional Commit with gitmoji](CONTRIBUTING.md#commit-message-format).
