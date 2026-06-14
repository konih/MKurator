# ADR-0018: Rename project Kurator → MKurator

- **Status**: Accepted
- **Date**: 2026-06-03
- **Supersedes**: [ADR-0006](0006-project-name-kurator.md) (identity only; MQ scope unchanged)

## Context

The operator shipped as **Kurator** ([ADR-0006](0006-project-name-kurator.md)) with
module `github.com/conduit-ops/mkurator`, API group `messaging.kurator.dev`, and Helm chart
`charts/kurator`. The maintainer renamed the GitHub repository to `mkurator` and
wanted the product name **MKurator** (emphasising MQ curation) reflected consistently
in module paths, CRDs, namespaces, images, and docs.

## Decision

- **Display name**: **MKurator**
- **Slug / identifiers**: `mkurator` (namespaces, images, kind cluster, chart dir)
- **Go module**: `github.com/conduit-ops/mkurator`
- **GitHub / GHCR**: `github.com/conduit-ops/MKurator`, `ghcr.io/conduit-ops/mkurator`
- **API group / domain**: `messaging.mkurator.dev`, version `v1alpha1` (breaking)
- **System namespace**: `mkurator-system` (was `kurator-system`)
- **E2E namespaces**: `mkurator-e2e-*` (was `kurator-e2e-*`)
- **Local kind cluster** default: `mkurator` (`CLUSTER_NAME` overrides)
- **Helm chart path**: `charts/mkurator`
- **Leader election ID** and webhook names use `mkurator` / `mkurator.dev` prefixes

## Consequences

- **Breaking**: API group change requires uninstalling old CRDs (`messaging.kurator.dev`)
  and reinstalling `messaging.mkurator.dev` CRDs; existing CRs must be migrated or
  recreated.
- All kubebuilder markers, generated CRDs, RBAC, webhooks, and samples regenerated.
- CI image references and `cliff.toml` repo updated to `conduit-ops/MKurator`.
- Local kind clusters named `kurator` continue to work via `CLUSTER_NAME=kurator` until
  recreated; new defaults use `mkurator`.
- Workspace directory may remain `IBM-Message-Queue-Operator`; module path is authoritative.
- **Environment variable prefix** `KURATOR_` (e.g. `KURATOR_LOG_LEVEL`, `KURATOR_E2E_MQ`,
  `KURATOR_INTEGRATION_MQ`) was **deliberately retained** for compatibility with existing
  scripts, CI, and developer muscle memory; only product identifiers (module, API group,
  namespaces, images) use `mkurator`.

## Alternatives considered

- **Keep API group `messaging.kurator.dev`** — avoids CRD migration but contradicts
  the rename; rejected for consistency.
- **Rename only repo, not API group** — rejected; full rename reduces long-term confusion.
