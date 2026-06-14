# Quick start

This page is a **fast path** into MKurator. Full install details, prerequisites, and
troubleshooting live in [INSTALL_AND_USE.md](INSTALL_AND_USE.md).

## Prerequisites

- A Kubernetes cluster (1.28+ recommended)
- An **existing IBM MQ queue manager** with **mqweb** (Administrative REST API) enabled
- Network path from the cluster to mqweb (HTTPS, TLS verification on by default)
- A Kubernetes `Secret` holding MQ credentials (referenced by `QueueManagerConnection`)

## Install the operator

Pick one:

| Method | Doc |
| --- | --- |
| Helm (recommended) | [INSTALL_AND_USE.md — Install the operator](INSTALL_AND_USE.md#install-the-operator) |
| Kustomize / release manifests | [INSTALL_AND_USE.md — Install the operator](INSTALL_AND_USE.md#install-the-operator) |

Release artifacts: [GitHub Releases](https://github.com/conduit-ops/MKurator/releases) (`install.yaml`,
`install-crds.yaml`, Helm chart on GHCR).

## Connect and create a queue

1. Create a `Secret` with MQ credentials (see [sample Secret](../charts/mkurator/samples/resources/mq-credentials-secret.yaml) or [Credentials secret](../config/samples/README.md#credentials-secret)).
2. Apply a `QueueManagerConnection` pointing at your mqweb endpoint.
3. Wait for the connection `Ready` condition.
4. Apply a `Queue` CR — the operator runs `DEFINE QLOCAL` (or alias/remote) via mqweb.

Step-by-step YAML and verification commands:
[INSTALL_AND_USE.md — Quick start: one queue](INSTALL_AND_USE.md#quick-start-one-queue-on-your-queue-manager).

## Verify

```bash
kubectl get qmc,queue,topic,channel -A
kubectl describe queue <name> -n <namespace>
```

MQ-side checks: [IBM_MQ_101.md](IBM_MQ_101.md) (`runmqsc`, MQ console).

## Next steps

| Topic | Link |
| --- | --- |
| All CR kinds | [crds/README.md](crds/README.md) |
| Sample manifests | [config/samples/README.md](../config/samples/README.md) |
| Upgrade | [UPGRADE.md](UPGRADE.md) |
| Local dev platform | [DEVELOPMENT.md](DEVELOPMENT.md) |
| Examples | [examples/README.md](examples/README.md) |
