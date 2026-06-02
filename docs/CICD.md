# CI/CD

This document describes the continuous integration and delivery design for the
**Kurator**. The guiding principle: **the same checks run locally
(via Task and pre-commit) and in CI**, so "green locally" means "green in CI".

CI runs on **GitHub Actions**. Workflows land with the Phase 1 scaffold
([ROADMAP.md](ROADMAP.md)); this doc is the contract they implement.

## Principles

- **Parity**: every CI step maps to a `task` target. No bespoke CI-only logic.
- **Fail fast, fail loud**: lint, codegen drift, test failures, and vuln
  findings all block merge.
- **Reproducible**: tools pinned via `go.mod` `tool` directives; GitHub Actions
  pinned to commit SHAs; `go.sum` committed.
- **Cheap things often, expensive things deliberately**: unit/envtest on every
  PR; e2e and image scans on PRs touching relevant paths or on a schedule.
- **Least privilege**: workflows request only the permissions they need;
  registry/release credentials are scoped and only used on protected refs.

## Pipeline overview

```mermaid
flowchart LR
  pr["Pull request"] --> verify
  verify["verify (codegen/manifests fresh)"] --> lint
  lint --> test["test (unit + envtest, -race, coverage)"]
  test --> build["build (CGO-free static)"]
  build --> vuln["govulncheck"]
  pr --> e2e["e2e (kind + IBM MQ)"]
  tag["Tag v*"] --> release["release: image + manifests"]
  release --> scan["image scan (Trivy)"]
```

## Triggers

| Event | Runs |
|-------|------|
| PR / push to default branch | `ci.yaml`: verify, lint, test, build, govulncheck; `integration.yaml`: Docker IBM MQ; `e2e.yaml`: kind + IBM MQ e2e |
| Tag `v*` | `release` (build + push image, publish install manifests) + image scan |
| Schedule (e.g. weekly) | `govulncheck`, image scan, dependency bot |

## Jobs

### `verify`
Regenerates CRDs, RBAC, deepcopy, and mocks and fails on any diff
(`task verify`). Guarantees committed generated artifacts never drift.

### `lint`
`task lint` — golangci-lint v2 (`default: none`, curated linter set per
[AGENTS.md](../AGENTS.md)) plus `gofmt`/`goimports`/`golines`. Fails on any
finding or formatting diff.

### `test`
`task test:run` — Ginkgo unit + envtest with the race detector and a coverage
profile (`coverage.out`). envtest control-plane binaries come from
`setup-envtest` (pinned K8s API version). CI uploads `coverage.out` as a workflow
artifact, prints a **job summary** (`go tool cover -func`), and publishes to
[Codecov](https://codecov.io/gh/konih/kurator) (`codecov.yml`) via
`codecov/codecov-action` using the repository secret `CODECOV_TOKEN`. A
regression is investigated, not ignored.

### `build`
`task build` — static `CGO_ENABLED=0` binary; later `task docker:build` for a
multi-arch (`amd64`/`arm64`) **distroless nonroot** image. On PRs the image is
built but not pushed.

### `govulncheck`
`govulncheck ./...` against code and dependencies. Runs on PRs and on a
schedule so newly disclosed CVEs surface even without code changes.

### `integration`
Dedicated workflow [`.github/workflows/integration.yaml`](../.github/workflows/integration.yaml)
on PRs and `main`: `task ci:integration` (Docker Compose IBM MQ, mqweb wait,
`task test:integration`, teardown). Exercises `mqadmin.Admin` queue operations
against live mqweb without kind. Local equivalent: `task test:integration:local`
or `task ci:integration`.

### `e2e`
Dedicated workflow [`.github/workflows/e2e.yaml`](../.github/workflows/e2e.yaml)
on PRs and `main`: `task cluster:up` (kind + Terraform + IBM MQ), `hack/ci/wait-mqweb.sh`,
then `task test:e2e` with `KURATOR_E2E_MQ=1` and `CERT_MANAGER_INSTALL_SKIP=true`
(cert-manager is already installed by Terraform). Tears down with `task cluster:down`
on completion. Local equivalent: `task ci:e2e`.

### `release` (tags only)
Builds and pushes the multi-arch controller image to GHCR with **OCI SBOM** and
**SLSA provenance** attestations, scans with Trivy, **cosign-signs** the image
digest (keyless OIDC), generates an SPDX SBOM (`dist/sbom.spdx.json`), then
publishes Kustomize/Helm install manifests on the GitHub Release. Runs only on
`v*.*.*` tags (or `workflow_dispatch` for testing).

### image scan
**Trivy** scans the built image for OS/dependency vulnerabilities; documented
false positives live in `.trivyignore` with a rationale comment. Critical/high
findings fail the job.

## Caching

- Go build and module cache keyed on `go.sum`.
- golangci-lint cache keyed on config + Go version.
- setup-envtest assets cached by K8s version.
- Docker layer cache for image builds.

## Security & supply chain

| Control | Mechanism |
|---------|-----------|
| Dependency vulns | `govulncheck` (PR + schedule) |
| Image vulns | Trivy scan on release image |
| Dependency freshness | **Renovate** (or Dependabot) PRs for Go modules, Actions, Dockerfile, Terraform |
| Pinned actions | GitHub Actions referenced by commit SHA |
| Minimal permissions | `permissions:` block per workflow; default read-only |
| Reproducible build | CGO-free, pinned toolchain, committed `go.sum` |
| Nonroot runtime | distroless nonroot base, read-only FS, dropped caps |
| Release SBOM | BuildKit attestation on push + SPDX file on GitHub Release |
| Image signing | cosign keyless (`sigstore/cosign-installer`) on image digest |
| SLSA provenance | `provenance: mode=max` on `docker/build-push-action` |

Further supply-chain hardening (OpenSSF Scorecard, SLSA Level 3 builders) remains
optional; see [ADR-0005](adr/0005-keep-tooling-lean.md).

## Branch protection

The default branch requires: `verify`, `lint`, `test`, `build`, and
`govulncheck` to pass before merge. `e2e` is required when it runs. No direct
pushes to the default branch.

## Local equivalents

| CI job | Local command |
|--------|---------------|
| verify | `task verify` |
| lint | `task lint` |
| test | `task test:run` |
| build | `task build` |
| govulncheck | `govulncheck ./...` |
| e2e | `task ci:e2e` (or `task cluster:up && KURATOR_E2E_MQ=1 task test:e2e`) |

pre-commit runs `gofmt`/`goimports`, `golangci-lint`, and `task verify` so most
CI failures are caught before pushing.
