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

> Status: **Phase 4** — queue, topic, and SVRCONN channel reconcile on existing
> IBM MQ via `mqweb`. See the [roadmap](docs/ROADMAP.md) for what is next.

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
(current badge above). `main` may include fixes not yet in a tag.

## What it does

- Reconciles custom resources (`Queue`, `Topic`, `Channel`) into MQSC objects on
  a running Queue Manager.
- Talks to the Queue Manager through the **IBM MQ Administrative REST API**
  (`mqweb`) over HTTPS — pure Go, no CGO.
- Reports status via conditions and cleans up via finalizers.

It does **not** deploy or operate Queue Manager installations; the Queue
Manager is assumed to already exist and expose `mqweb`.

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

**Canonical reference:** [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) — prerequisites,
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
| 🛠️ **Develop locally** | [Development guide](docs/DEVELOPMENT.md) · [MQ on kind](docs/IBM_MQ_101.md) · [Platform (kind/Terraform/MQ)](hack/kind-cluster/README.md) |
| 🏗️ **Design** | [Architecture](docs/ARCHITECTURE.md) · [Attribute reconciliation](docs/ATTRIBUTE_RECONCILIATION.md) · [ADRs](docs/adr/) |
| 📋 **Project** | [Roadmap](docs/ROADMAP.md) · [CI/CD](docs/CICD.md) · [NFRs](docs/NON_FUNCTIONAL_REQUIREMENTS.md) · [Security](SECURITY.md) |
| 📚 **IBM MQ reference** | [Objects (research)](docs/IBM_MQ_OBJECTS.md) · [REST API](docs/IBM_MQ_REST_API.md) · [Schemas](docs/schemas/README.md) |

Contributors and agents: start with [AGENTS.md](AGENTS.md) (conventions + workflow).

## License

MIT — see [LICENSE](LICENSE).
