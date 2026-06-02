# ADR-0016: Release supply chain (image, SBOM, signing, scan)

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

Tagged releases publish a container image and install manifests consumed by users.
Supply-chain expectations for a Kubernetes operator include vulnerability scanning,
artifact attestations, and verifiable image signatures — without adopting
cert-manager-scale KMS multi-artifact signing ([ADR-0005](0005-keep-tooling-lean.md)).

## Decision

On each **`v*.*.*` tag** (and `workflow_dispatch` for rebuild tests), the
[`.github/workflows/release.yaml`](../../.github/workflows/release.yaml) job will:

| Control | Mechanism |
|---------|-----------|
| **Image** | Multi-arch (`linux/amd64`, `linux/arm64`) build; **distroless nonroot** base; pushed to GHCR |
| **Vuln scan** | Trivy on the release image digest; CRITICAL/HIGH unfixed fail the job (`.trivyignore` for documented exceptions) |
| **OCI attestations** | BuildKit `sbom: true` and `provenance: mode=max` on push |
| **SPDX SBOM file** | `anchore/sbom-action` → `dist/sbom.spdx.json` attached to GitHub Release |
| **Signing** | **cosign keyless** (OIDC via GitHub Actions) on the image digest |
| **Install artifacts** | `hack/release-assets.sh` — Kustomize manifest, CRD bundle, Helm `.tgz`, checksums |
| **Helm OCI** | Chart pushed to GHCR OCI registry |
| **Release notes** | git-cliff ([ADR-0008](0008-changelog-git-cliff.md)) + install template |

Permissions are scoped: `contents: write`, `packages: write`, `id-token: write`
for signing only on the release job.

We do **not** use KMS-backed cosign, SLSA Level 3 dedicated builders, or generated
`LICENSES` allowlists in this phase.

## Consequences

- Users can verify images with cosign and find SBOM/provenance in GHCR.
- Release failures on Trivy or build block publishing — investigate or justify in
  `.trivyignore` with comments.
- Changelog and version bump remain **manual** before tag (chart version, `CHANGELOG.md`).
- Forks must configure GHCR/repo secrets and permissions to reproduce releases.

## Alternatives considered

- **Unsigned images**: fails NFR SEC-6 posture. Rejected.
- **KMS / hardware signing**: operational overhead disproportionate for solo
  maintainer. Deferred in ADR-0005.
- **SBOM only on GitHub Release, not OCI**: we do both OCI attestation and SPDX file.

## References

- [CICD.md](../CICD.md) — release job
- [NON_FUNCTIONAL_REQUIREMENTS.md](../NON_FUNCTIONAL_REQUIREMENTS.md) — SEC-6, OPS
