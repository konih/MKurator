# Kurator Helm chart

Installs the Kurator operator (controller Deployment, RBAC, and CRDs).

**Managing MQ queues after install:** see [docs/INSTALL_AND_USE.md](../../docs/INSTALL_AND_USE.md)
and [samples/resources/README.md](samples/resources/README.md) (same content as
[`config/samples/`](../../config/samples/README.md)).

## Prerequisites

- Kubernetes 1.28+
- Helm 3
- An existing IBM MQ queue manager with **mqweb** enabled

## Install

```sh
helm upgrade --install kurator . \
  --namespace kurator-system \
  --create-namespace
```

## Local kind development

With the platform from [`hack/kind-cluster`](../../hack/kind-cluster/README.md):

```sh
task local:up      # recommended: cluster + this chart + sample CRs
# or step by step:
task cluster:up
task deploy:helm
task deploy:samples
```

`deploy:helm` builds the dev image, loads it into the `kurator` kind cluster, and
installs this chart with [`samples/values-kind.yaml`](samples/values-kind.yaml).
`deploy:samples` applies [`samples/resources/`](samples/resources/) (Secret,
`QueueManagerConnection`, `Queue` for `QM1`).

Kustomize install (`task deploy`) remains available for controller-runtime workflows.

## Configuration

| Value | Description | Default |
|-------|-------------|---------|
| `image.repository` | Controller image repository | `kurator-controller-manager` |
| `image.tag` | Image tag | `dev` |
| `leaderElection.enabled` | Pass `--leader-elect` | `true` |
| `metrics.enabled` | Expose HTTPS metrics on :8443 | `true` |
| `logging.level` | `KURATOR_LOG_LEVEL` | `info` |
| `logging.format` | `KURATOR_LOG_FORMAT` | `json` |

## CRDs

CRD manifests live in [`crds/`](crds/). Regenerate from kubebuilder output:

```sh
task manifests
task helm:sync-crds
```

Helm installs CRDs on first install; upgrading CRDs may require a manual `kubectl apply`
when the API changes.

## Publishing

Package the chart for an OCI registry or chart museum:

```sh
task helm:package
# artifact: dist/kurator-0.1.0.tgz
```

Bump `version` in `Chart.yaml` for each release and align `appVersion` with the
controller image tag you publish.
