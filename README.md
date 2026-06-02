<p align="center">
  <img src="docs/images/kurator-logo.png" alt="Kurator logo" width="200">
</p>

# Kurator

[![CI](https://github.com/konih/kurator/actions/workflows/ci.yaml/badge.svg)](https://github.com/konih/kurator/actions/workflows/ci.yaml)
[![E2E](https://github.com/konih/kurator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/konih/kurator/actions/workflows/e2e.yaml)
[![License: MIT](https://img.shields.io/github/license/konih/kurator)](https://github.com/konih/kurator/blob/main/LICENSE)
[![codecov](https://codecov.io/gh/konih/kurator/graph/badge.svg)](https://codecov.io/gh/konih/kurator)
[![Go](https://img.shields.io/github/go-mod/go-version/konih/kurator)](https://pkg.go.dev/github.com/konih/kurator)
[![Go Reference](https://pkg.go.dev/badge/github.com/konih/kurator.svg)](https://pkg.go.dev/github.com/konih/kurator)
[![Go Report Card](https://goreportcard.com/badge/github.com/konih/kurator)](https://goreportcard.com/report/github.com/konih/kurator)
[![Release](https://img.shields.io/github/v/release/konih/kurator)](https://github.com/konih/kurator/releases)

A Kubernetes operator for declaratively managing **resources on an existing
IBM MQ Queue Manager** — queues, topics, SVRCONN channels; users/authorities and
more later.

> Status: **Phase 4 + 4b complete** — queue, topic, SVRCONN channel, and
> validating admission webhooks on existing IBM MQ via `mqweb`. **Phase 5**
> (auth) is next. See the [roadmap](docs/ROADMAP.md).

## What ships in v1alpha1 (today)

| Custom resource | MQ objects | Notes |
|-----------------|------------|-------|
| `QueueManagerConnection` | (connectivity) | Ping + credentials from a referenced `Secret` |
| `Queue` | `QLOCAL`, `QALIAS`, `QREMOTE` | `spec.type`: `local` (default), `alias`, `remote` |
| `Topic` | `TOPIC` | Drift-checked attributes per [ATTRIBUTE_RECONCILIATION.md](docs/ATTRIBUTE_RECONCILIATION.md) |
| `Channel` | `CHANNEL` … `CHLTYPE(SVRCONN)` | Other channel types planned later |

**Not shipped yet:** `SET CHLAUTH`, `SET AUTHREC`, and related access-control
resources (Phase 5 — see [PHASE5_AUTH_SKETCH.md](docs/PHASE5_AUTH_SKETCH.md)).

**Repository:** [github.com/konih/kurator](https://github.com/konih/kurator) — Go module
[`github.com/konih/kurator`](https://pkg.go.dev/github.com/konih/kurator), images
`ghcr.io/konih/kurator` ([ADR-0006](docs/adr/0006-project-name-kurator.md)).

### What CI proves

| Tier | Scope |
|------|-------|
| Unit + envtest | Reconcilers and adapter (mocked MQ); validating admission (envtest); Queue, Topic, Channel, and QMC |
| Docker integration | Queue (local/alias/remote), Topic, Channel against live mqweb |
| kind e2e (`KURATOR_E2E_MQ=1`) | Queue, Topic, and Channel CR reconcile + delete on live `QM1` |

Details and commands: [DEVELOPMENT.md#test-tiers](docs/DEVELOPMENT.md#test-tiers).

Latest tagged release: [GitHub Releases](https://github.com/konih/kurator/releases)
(current badge above). `main` may include fixes not yet in a tag. See
[CHANGELOG.md](CHANGELOG.md) for version history (generated from Conventional Commits).

## What it does

- Reconciles custom resources (`Queue`, `Topic`, `Channel`) into MQSC objects on
  a running Queue Manager.
- Talks to the Queue Manager through the **IBM MQ Administrative REST API**
  (`mqweb`) over HTTPS — pure Go, no CGO.
- Reports status via conditions and cleans up via finalizers.

It does **not** deploy or operate Queue Manager installations; the Queue
Manager is assumed to already exist and expose `mqweb`.

## Repository structure

[Kubebuilder v4](https://book.kubebuilder.io/) layout — thin reconcilers, an
[`MQAdmin`](internal/mqadmin) port, and an [`mqweb`](internal/adapter/mqrest)
adapter. Full design: [ARCHITECTURE.md](docs/ARCHITECTURE.md) · extended map:
[AGENTS.md](AGENTS.md#repository-layout).

```text
kurator/
├── 📦 api/v1alpha1/                 CRD types + deepcopy (QMC, Queue, Topic, Channel)
├── 🚀 cmd/                          Manager entrypoint (controller-runtime)
├── 🧠 internal/
│   ├── controller/                  Reconcilers (thin) + unit/envtest suites
│   ├── validation/                  Admission validation rules (pure functions)
│   ├── webhook/v1alpha1/            Validating webhook handlers
│   ├── mqadmin/                     MQAdmin port — interface + domain errors
│   ├── adapter/mqrest/              mqweb REST client (sole adapter today)
│   ├── logging/                     Structured logging helpers
│   └── metrics/                     Prometheus metrics
├── ⚙️  config/                       Kustomize — CRDs, RBAC, manager, webhook, samples
├── ⎈  charts/kurator/                Publishable Helm chart + sample CRs
├── 🧪 test/
│   ├── integration/                 Docker MQ tests (build tag `integration`)
│   ├── e2e/                         kind + live QM1 (build tag `e2e`)
│   └── mocks/                       mockery-generated MQAdmin mocks
├── 🔧 hack/
│   ├── kind-cluster/                Local platform: kind + Terraform + IBM MQ Helm
│   ├── mq-docker/                   Standalone IBM MQ container for integration CI
│   └── *.sh                         verify, release assets, tool install helpers
├── 📚 docs/                         Guides, ADRs, MQ research (see docs/README.md)
├── Taskfile.yml                     Primary task runner (`task local:up`, …)
└── AGENTS.md                        Go conventions + agent entry point
```

## Install and use

**Start here:** [docs/INSTALL_AND_USE.md](docs/INSTALL_AND_USE.md) — install the
operator (Release manifests, Helm), connect to your queue manager, manage queues,
troubleshooting, and uninstall.

Sample YAML with annotations:
[config/samples/README.md](config/samples/README.md).

```sh
# After install — apply samples (see config/samples/README.md)
kubectl apply -k config/samples/   # Connection + Queue + Topic + Channel (Secret first)
kubectl get qmc,mq,tp,chl -n kurator-system
```

## Local development (contributors)

**Tool install:** [docs/LOCAL_SETUP.md](docs/LOCAL_SETUP.md) — Go, Task, Docker, kind,
Terraform, and verification by tier.

**Canonical reference:** [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) — inner loop,
local platform, task reference, test tiers, URLs, and credentials.

```sh
task local:up      # cluster + IBM MQ + operator (Helm) + sample CRs
task local:info    # URLs, credentials, CR status
task local:down    # tear everything down
```

Verify reconciliation with [docs/IBM_MQ_101.md](docs/IBM_MQ_101.md) (`runmqsc`, MQ console).

## Documentation

Full index with paths by role: **[docs/README.md](docs/README.md)**.

| | Doc |
|---|-----|
| 🎯 **Use Kurator** | [Install and use](docs/INSTALL_AND_USE.md) · [Sample YAML](config/samples/README.md) · [Helm chart](charts/kurator/README.md) |
| 🛠️ **Develop locally** | [Development guide](docs/DEVELOPMENT.md) · [Contributing](docs/CONTRIBUTING.md) · [MQ on kind](docs/IBM_MQ_101.md) · [Platform (kind/Terraform/MQ)](hack/kind-cluster/README.md) |
| 🏗️ **Design** | [Architecture](docs/ARCHITECTURE.md) · [Attribute reconciliation](docs/ATTRIBUTE_RECONCILIATION.md) · [ADRs](docs/adr/) |
| 📋 **Project** | [Roadmap](docs/ROADMAP.md) · [CI/CD](docs/CICD.md) · [Release guide](docs/RELEASE.md) · [NFRs](docs/NON_FUNCTIONAL_REQUIREMENTS.md) · [Security](SECURITY.md) |
| 📚 **IBM MQ reference** | [Objects (research)](docs/IBM_MQ_OBJECTS.md) · [REST API](docs/IBM_MQ_REST_API.md) · [Schemas](docs/schemas/README.md) |

Contributors: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) (guidelines, commits,
gitmoji) · [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) (local workflow) ·
[AGENTS.md](AGENTS.md) (Go conventions and agent entry point).

## License

MIT — see [LICENSE](LICENSE).
