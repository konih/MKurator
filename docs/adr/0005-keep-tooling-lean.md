# ADR-0005: Keep tooling lean; borrow discipline, not org overhead

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

cert-manager is a CNCF-graduated project and a goldmine of best practices, but it
carries machinery aimed at a large multi-vendor community: `OWNERS`/
`OWNERS_ALIASES`, `GOVERNANCE.md`, `.clomonitor.yml`, klone-synced Makefile
modules, a generated `LICENSES` allowlist, KMS-signed multi-artifact releases,
and a 56-linter golangci config. This is a focused personal project that values
a tight, well-tested codebase over breadth. We want cert-manager's *discipline*
without its *organisational overhead*.

## Decision

We will adopt the high-signal practices and deliberately skip the org-scale
ones.

**Adopt:**

- Strict `golangci-lint` (`default: none`) with the curated linter set in
  [../../AGENTS.md](../../AGENTS.md) — not the full 56-linter list.
- A `generate` / `verify` discipline for all generated artifacts (CRDs, RBAC,
  deepcopy, mocks).
- Layered tests: unit → envtest → kind e2e.
- Short ADRs (this directory) for non-obvious decisions.
- A real, local [SECURITY.md](../../SECURITY.md) and periodic `govulncheck`.
- Pinned tool versions (`go.mod` tool directives) and pinned CI action SHAs.

**Defer / skip (until justified by external consumers):**

- `OWNERS`/`GOVERNANCE`/`.clomonitor` and Kubernetes review-routing machinery.
- klone + Makefile modules + self-upgrade bots.
- Generated `LICENSES` allowlist and CNCF license gating.
- Multi-module `go.mod` per binary.
- OpenSSF Scorecard and SLSA Level 3 dedicated builders.

**Added (release pipeline, 2026):** supply-chain controls on tagged releases —
see [ADR-0016](0016-release-supply-chain.md) (OCI SBOM, SLSA provenance, SPDX file,
cosign keyless, Trivy); not full cert-manager-style KMS signing.

## Consequences

- The repo stays approachable and low-maintenance while remaining
  professionally rigorous on the things that affect code quality and security.
- If the project later gains external consumers, the deferred items have a clear
  home and rationale, and can be added incrementally.

## Alternatives considered

- **Mirror cert-manager fully**: maximal rigor, but disproportionate maintenance
  burden for a single-maintainer project. Rejected.
- **Minimal tooling (KubeOps style)**: low overhead, but sacrifices the
  verify/test/lint discipline that keeps the codebase tight. Rejected.
