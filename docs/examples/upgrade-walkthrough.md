# Upgrade walkthrough

Upgrade MKurator operator, CRDs, and webhooks without losing MQ object state.

Full runbook: [UPGRADE.md](../UPGRADE.md). This page summarizes the happy path.

## Before you upgrade

1. Read release notes for the target version on [GitHub Releases](https://github.com/conduit-ops/MKurator/releases).
2. Back up custom resources: `kubectl get queue,topic,channel,qmc,car,auth -A -o yaml > mkurator-crs.yaml`
3. Note your current chart/app version: `helm list -A` or deployment image tag.

## Helm upgrade

```bash
helm upgrade mkurator oci://ghcr.io/conduit-ops/mkurator --version <X.Y.Z> -n mkurator-system
```

Or upgrade from a release tarball:

```bash
helm upgrade mkurator dist/mkurator-<version>.tgz -n mkurator-system
```

## CRD and webhook changes

When CRD schemas change between releases:

1. Apply updated CRDs from the release (`install-crds.yaml` or chart CRDs).
2. Wait for the operator deployment to roll out.
3. Confirm validating webhook pods are ready (cert-manager or bundled certs).

Details: [UPGRADE.md — CRDs and webhooks](../UPGRADE.md).

## Post-upgrade checks

```bash
kubectl get deploy -n mkurator-system
kubectl get qmc,queue -A
kubectl logs deploy/mkurator-controller-manager -n mkurator-system --tail=50
```

Existing MQ objects remain on the queue manager; MKurator reconciles desired state after upgrade.

## Rollback

Helm rollback or re-install the previous release image/chart. See [UPGRADE.md](../UPGRADE.md)
for webhook cert and CRD compatibility notes.
