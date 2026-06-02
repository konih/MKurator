# Kurator

[![CI](https://github.com/konih/kurator/actions/workflows/ci.yaml/badge.svg)](https://github.com/konih/kurator/actions/workflows/ci.yaml)
[![E2E](https://github.com/konih/kurator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/konih/kurator/actions/workflows/e2e.yaml)
[![License: MIT](https://img.shields.io/github/license/konih/kurator)](https://github.com/konih/kurator/blob/main/LICENSE)
[![codecov](https://codecov.io/gh/konih/kurator/graph/badge.svg)](https://codecov.io/gh/konih/kurator)
[![Go](https://img.shields.io/github/go-mod/go-version/konih/kurator)](https://pkg.go.dev/github.com/konradheimel/kurator)
[![Go Reference](https://pkg.go.dev/badge/github.com/konradheimel/kurator.svg)](https://pkg.go.dev/github.com/konradheimel/kurator)
[![Release](https://img.shields.io/github/v/release/konih/kurator)](https://github.com/konih/kurator/releases)

A Kubernetes operator for declaratively managing **resources on an existing
IBM MQ Queue Manager** — queues today, users/authorities and more later.

> Status: **v0.1.0** — first release; queue reconcile on existing IBM MQ via `mqweb`.
> See the [roadmap](docs/ROADMAP.md) for what is next.

## What it does

- Reconciles custom resources (e.g. `Queue`) into MQSC objects on a running
  Queue Manager.
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
kubectl apply -k config/samples/   # Connection + Queue (create Secret first)
kubectl get qmc,queue -n kurator-system
```

## Local development (contributors)

The repo ships a **kind + Terraform + IBM MQ** platform under
[`hack/kind-cluster`](hack/kind-cluster/README.md). One command brings up the
cluster, Queue Manager, and operator with sample CRs:

```sh
# Prerequisites: Go, Task, Docker, kind, kubectl, Terraform, Helm, mkcert
# Optional: direnv (loads KUBECONFIG from .envrc)

task local:up      # cluster + IBM MQ + operator (Helm) + sample Queue/Connection
task local:info    # URLs, credentials, CR status
task mq:console    # IBM MQ web UI URL (https://mq.localhost:30443/ibmmq/console/)
task mq:cli        # interactive runmqsc on QM1
task local:down    # tear everything down
```

IBM MQ on the kind cluster includes the **web console** and **`runmqsc`** in the
MQ pod. See [docs/IBM_MQ_101.md](docs/IBM_MQ_101.md) to confirm the operator
created `APP.ORDERS` on the queue manager.

**Inner loop** (no cluster — mocks + envtest):

```sh
task install && task lint && task test:run && task build
```

**Incremental** (cluster already running):

```sh
task local:deploy          # rebuild image, helm upgrade, re-apply samples
task deploy:samples        # only sample Secret + CRs
```

| Task | What it does |
|------|----------------|
| `task cluster:up` | kind cluster + ingress + cert-manager + monitoring + IBM MQ |
| `task cluster:info` | MQ/Grafana/Argo CD URLs and passwords |
| `task cluster:down` | Destroy platform and delete kind cluster |
| `task deploy` | Operator via Kustomize (`config/default` + CRDs) |
| `task deploy:helm` | Operator via [Helm chart](charts/kurator/README.md) (recommended on kind) |
| `task deploy:samples` | `mq-credentials` Secret + `QueueManagerConnection` + `Queue` for `QM1` |
| `task test:run` | Unit + envtest (`-race`) |
| `task test:e2e` | E2E on kind (set `KURATOR_E2E_MQ=1` for IBM MQ scenarios) |
| `task ci:e2e` | Same as GitHub Actions e2e job (`cluster:up` + MQ wait + tests) |

After `task local:up`, check reconciliation:

```sh
kubectl get qmc,queue -n kurator-system
kubectl logs -n kurator-system deployment/kurator-controller-manager -f
```

Defaults match the local platform: Queue Manager **`QM1`**, mqweb
**`https://ibm-mq.ibm-mq.svc:9443`**, admin user **`admin`** / **`passw0rd`**
(local dev only). Full detail: [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Documentation

- [docs/INSTALL_AND_USE.md](docs/INSTALL_AND_USE.md) — **install, use, samples, troubleshooting**.
- [config/samples/README.md](config/samples/README.md) — annotated sample manifests.
- [AGENTS.md](AGENTS.md) — context, conventions, toolchain, and doc map.
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) — prerequisites, local platform, deploy, test tiers.
- [hack/kind-cluster/README.md](hack/kind-cluster/README.md) — kind/Terraform/MQ platform only.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — components, runtime, CRDs, reconcile flow, security.
- [docs/NON_FUNCTIONAL_REQUIREMENTS.md](docs/NON_FUNCTIONAL_REQUIREMENTS.md) — quality bars.
- [docs/CICD.md](docs/CICD.md) — CI/CD pipeline design.
- [docs/adr/](docs/adr/) — architecture decision records.
- [docs/ROADMAP.md](docs/ROADMAP.md) — phased delivery plan.
- [charts/kurator/README.md](charts/kurator/README.md) — Helm chart to install the operator.
- [SECURITY.md](SECURITY.md) — security posture and reporting.

## License

MIT — see [LICENSE](LICENSE).
