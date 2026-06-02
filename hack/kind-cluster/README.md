# Local kind dev cluster

A one-command local environment for developing and testing the Kurator
operator. It provisions a [kind](https://kind.sigs.k8s.io/) cluster and,
via Terraform + Helm, installs:

- **HAProxy Ingress** (NodePort 30080/30443, mapped to the host).
- **cert-manager** (for future operator webhook certificates).
- **Argo CD** (optional; `ENABLE_ARGOCD=true` on apply — GitOps UI, password in `.state/argocd.env`).
- **kube-prometheus-stack** (Prometheus + **Grafana**).
- **IBM MQ** queue manager (`QM1`) from the upstream Helm repo
  ([`helm repo add ibm-messaging-mq https://ibm-messaging.github.io/mq-helm`](https://ibm-messaging.github.io/mq-helm)),
  chart `ibm-mq`, with mqweb exposed through a Terraform-managed HAProxy Ingress
  (the upstream chart's Ingress targets nginx and is not used).

TLS uses a [mkcert](https://github.com/FiloSottile/mkcert) wildcard certificate
for `*.localhost`.

## Prerequisites

Install instructions (versions, macOS/Linux commands, verification):
[docs/LOCAL_SETUP.md](../../docs/LOCAL_SETUP.md) (Tier C).

- A container runtime: Docker (recommended), nerdctl, or Podman
- [`kind`](https://kind.sigs.k8s.io/), [`kubectl`](https://kubernetes.io/docs/tasks/tools/),
  [`helm`](https://helm.sh/), [`terraform`](https://developer.hashicorp.com/terraform),
  [`mkcert`](https://github.com/FiloSottile/mkcert), and [`task`](https://taskfile.dev)

## Usage

From the repository root:

```sh
task cluster:up       # kind + TLS + Terraform; prints URLs
task cluster:info     # re-print access URLs
task cluster:cleanup  # terraform destroy (keeps kind cluster)
task cluster:down     # destroy Terraform + delete kind + wipe .state
```

`task cluster:up` is idempotent: an existing `kurator` cluster is reused; other
kind clusters blocking NodePorts 30080/30443 are removed automatically.

## Access (after `task cluster:up`)

| What | URL | Credentials |
|------|-----|---------------|
| Argo CD (optional) | https://argocd.localhost:30443/ | `admin` — `ENABLE_ARGOCD=true task cluster:apply`; password in `.state/argocd.env` |
| IBM MQ web console (UI) | https://mq.localhost:30443/ibmmq/console/ | `admin` / `passw0rd` |
| IBM MQ admin REST | https://mq.localhost:30443/ibmmq/rest/v3/admin/qmgr | `admin` / `passw0rd` |
| MQSC CLI | `task mq:cli` or `task mq:runmqsc -- "DISPLAY QLOCAL(*)"` | (in-cluster `runmqsc`) |
| Grafana | https://grafana.localhost:30443/ | `admin` / `admin` |

In-cluster: `https://ibm-mq.ibm-mq.svc:9443` (`QueueManagerConnection.endpoint`).

## Kurator operator on this cluster

From the repository root:

```sh
task local:up        # cluster + MQ + operator + sample CRs (one shot)
# or, if the cluster is already up:
task local:deploy    # operator + samples only
task local:info      # URLs + qmc/queue status
```

See [docs/DEVELOPMENT.md](../../docs/DEVELOPMENT.md),
[docs/IBM_MQ_101.md](../../docs/IBM_MQ_101.md) (console, CLI, operator checks), and
[charts/kurator/README.md](../../charts/kurator/README.md).

## Notes

- Cluster name defaults to `kurator` (`CLUSTER_NAME` env var overrides).
- State lives under `hack/kind-cluster/.state/` (git-ignored).
- IBM MQ uses the Advanced for Developers license (`license: accept`) for local dev only.
