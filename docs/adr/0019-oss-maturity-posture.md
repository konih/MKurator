# ADR-0019: OSS maturity posture (docs site, governance, supply chain)

- **Status**: Accepted
- **Date**: 2026-06-06
- **Supersedes (partially)**: [ADR-0005](0005-keep-tooling-lean.md) deferrals listed below

## Context

[ADR-0005](0005-keep-tooling-lean.md) deliberately deferred org-scale machinery
(`GOVERNANCE`, OpenSSF Scorecard, published docs site) while the project was a
solo personal operator. MKurator now targets **external adoption** as open source
(`github.com/conduit-ops/MKurator`). Contributors and adopters need public trust signals,
published documentation, and contributor norms comparable to mature OSS operators —
without copying unrelated inventory features or cert-manager-scale overhead.

An OSS maturity audit (2026-06-06) compared MKurator to sibling project kollect and
identified meta-engineering gaps: community packaging, docs site, SAST/posture CI,
release asset attestations, and layered engineering standards.

## Decision

We **selectively adopt** the following, superseding the matching ADR-0005 deferrals:

| Area | Mechanism |
| --- | --- |
| **Community** | Root `CODE_OF_CONDUCT.md`, `GOVERNANCE.md`, `CONTRIBUTING.md` with DCO sign-off |
| **Published docs** | MkDocs Material + GitHub Pages (`docs.yaml` CI) at `conduit-ops.github.io/MKurator/` |
| **Security posture CI** | CodeQL, OpenSSF Scorecard workflow, RBAC audit job (Polaris + kubeaudit) |
| **Release attestations** | Extend [ADR-0016](0016-release-supply-chain.md): cosign sign-blob on release assets, `actions/attest`, signed Helm OCI chart |
| **Engineering standards** | Split `docs/development/*` (guidelines, coding-standards, testing, tooling-setup) |
| **Assurance docs** | `docs/ASSURANCE-CASE.md`, dated `docs/SECURITY-REVIEW.md`, SCA remediation policy |
| **Architecture lint** | `go-arch-lint` + `depguard` in golangci-lint (fix stale AGENTS.md claims) |

We **keep deferred** from ADR-0005:

- `OWNERS` / klone / Kubernetes review-routing machinery
- KMS-backed cosign / SLSA Level 3 dedicated builders
- Generated CNCF `LICENSES` allowlist
- Multi-module `go.mod` per binary

**SonarCloud** is scaffolded but **disabled** until the repository moves to the
**conduit-ops** GitHub organization.

## Consequences

- ADR-0005 remains Accepted for its core lean-tooling rationale; its deferral list
  is narrowed by this ADR where OSS adoption requires it.
- Maintenance cost increases modestly (docs site CI, Scorecard, standards doc split).
- OpenSSF Best Practices and Scorecard badges become achievable iteratively.
- MQ-specific strengths are preserved: release-gate workflow, Renovate depth, 4-tier
  test pyramid, schema golden tests ([ADR-0011](0011-layered-testing-strategy.md)).

## Alternatives considered

- **Stay lean (ADR-0005 as-is)**: lower overhead but blocks external contributor
  trust and OpenSSF Best Practices. Rejected for current adoption goal.
- **Mirror cert-manager / kollect fully**: disproportionate for a single-maintainer
  MQ operator. Rejected — port meta patterns only.

## References

- [ADR-0016](0016-release-supply-chain.md) — release supply chain baseline
- [ADR-0011](0011-layered-testing-strategy.md) — layered testing
- [CICD.md](../CICD.md) — CI workflow contract
