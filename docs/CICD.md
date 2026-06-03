# CI/CD

This document describes the continuous integration and delivery design for
**Kurator**. The guiding principle: **the same checks run locally
(via Task and pre-commit) and in CI**, so "green locally" means "green in CI".

CI runs on **GitHub Actions** per the workflows under `.github/workflows/`
(`ci.yaml`, `integration.yaml`, `e2e.yaml`, `release.yaml`, `renovate.yaml`).
This doc is the contract they implement. See [ROADMAP.md](ROADMAP.md) for
delivery context.

## Principles

- **Parity**: every CI step maps to a `task` target. No bespoke CI-only logic.
- **Fail fast, fail loud**: lint, codegen drift, test failures, and vuln
  findings all block merge.
- **Reproducible**: tools pinned via `go.mod` `tool` directives; GitHub Actions
  pinned to commit SHAs; `go.sum` committed.
- **Least privilege**: workflows request only the permissions they need;
  registry/release credentials are scoped and only used on protected refs.

## Pipeline overview

```mermaid
flowchart LR
  pr["Pull request / push to main"] --> gitleaks
  pr --> verify
  pr --> lint
  pr --> test
  pr --> build
  pr --> docker["docker-build"]
  pr --> helmlint["helm-lint"]
  lint["lint (format:check + golangci-lint)"]
  test["test (unit + envtest, govulncheck, coverage)"]
  pr --> integration["integration (Docker IBM MQ)"]
  pr --> e2e["e2e (kind + IBM MQ)"]
  tag["Tag v*"] --> release["release: image + manifests"]
  release --> scan["image scan (Trivy)"]
```

All `ci.yaml` jobs run **in parallel** on each PR and `main` push; the diagram lists
what each job runs, not execution order.

## Triggers

| Event | Runs |
|-------|------|
| PR / push to `main` | `ci.yaml`: gitleaks, verify, lint, test, build, docker-build, helm-lint (seven parallel jobs) |
| PR / push to `main` (non-docs paths) | `integration.yaml`: Docker IBM MQ integration tests |
| PR / push to `main` (non-docs paths) | `e2e.yaml`: kind + IBM MQ e2e |
| Tag `v*` | `release.yaml`: build + push image, publish install manifests, Trivy scan |
| Schedule (weekly, self-hosted) | `renovate.yaml`: dependency update PRs |

**Path filters:** `integration.yaml` and `e2e.yaml` skip when a push or PR
changes only markdown (`**.md`), `docs/**`, or `charts/**/README.md`. The main
`ci.yaml` workflow runs on every PR and `main` push (no path filters).

### Concurrency

`integration.yaml` and `e2e.yaml` each define a workflow-scoped concurrency
group (`integration-…` / `e2e-…` plus `github.ref`). They do **not** share a
group with `ci.yaml` or each other.

| Workflow | `cancel-in-progress` | Effect |
|----------|----------------------|--------|
| `e2e`, `integration` on **PR** | `true` | A new push cancels the in-flight run for that PR ref (saves runner time). |
| `e2e`, `integration` on **`main`** | `false` | Rapid pushes do not cancel a run already on the cluster; newer runs **queue** until the group is free, so each finished run keeps a visible result. |

`ci.yaml` has no concurrency block (jobs always run in parallel per trigger).

## Jobs

### `gitleaks`
Secret scan on PRs and `main` pushes (`gitleaks/gitleaks-action` with full git
history).

### `verify`
Regenerates CRDs, RBAC, deepcopy, and **mockery mocks** and fails on any diff
(`task verify` → `hack/verify.sh`). Guarantees committed generated artifacts
never drift.

### `lint`
Runs in order within the job (same runner, no extra wall-clock job):

1. `task format:check` — fails when `gofmt`, `goimports`, or `golines` would change
   any file. Locally, `task format` auto-fixes; pre-commit runs the same formatters.
2. `task lint` — `golangci-lint run ./...`.

### `test`
Runs in order within the job:

1. `task test:run` — Ginkgo unit + envtest with the race detector and a coverage
   profile (`coverage.out`). envtest control-plane binaries come from
   `setup-envtest` (pinned K8s API version in `Taskfile.test.yml`).
2. `task vuln:check` (`govulncheck ./...`) after tests pass. There is no separate
   scheduled govulncheck workflow (Renovate runs weekly).

CI then uploads `coverage.out` as a workflow artifact, prints a **job summary**
(`go tool cover -func`), and publishes to [Codecov](https://codecov.io/gh/konih/kurator)
(`codecov.yml`) via `codecov/codecov-action` using the repository secret
`CODECOV_TOKEN`. A regression is investigated, not ignored.

### `build`
`task build` — static `CGO_ENABLED=0` manager binary.

### `docker-build`
`task docker:build` — builds the controller-manager container image locally on
the runner (`Dockerfile`; same Go toolchain and build flags as release). Verifies
the image builds on every PR and `main` push; **no registry push** (push, scan,
and signing run only in `release.yaml` on tags).

### `helm-lint`
`task helm:lint` — `helm lint ./charts/kurator` on the publishable Helm chart,
then [`hack/helm-verify-admission.sh`](../hack/helm-verify-admission.sh) and
[`hack/helm-verify-rbac.sh`](../hack/helm-verify-rbac.sh) to assert rendered
webhook and manager ClusterRole templates stay aligned with
`config/webhook/manifests.yaml` and `config/rbac/role.yaml`.
Runs in parallel with other `ci.yaml` jobs; no cluster or MQ required.

### `integration`
Dedicated workflow [`.github/workflows/integration.yaml`](../.github/workflows/integration.yaml):
`task mq:integration:up` → `task mq:integration:wait` → `task test:integration`
→ `task mq:integration:down` (always). Exercises `mqadmin.Admin` queue, topic,
channel, **CHLAUTH**, and **AUTHREC** operations against live mqweb without kind.
Local equivalent: `task test:integration:local` or `task ci:integration`.

### `e2e`
Dedicated workflow [`.github/workflows/e2e.yaml`](../.github/workflows/e2e.yaml):
`task tools:install` → `task cluster:up` (kind + Terraform + IBM MQ) →
`hack/ci/wait-mqweb.sh` → `task test:e2e` with `KURATOR_E2E_MQ=1` and
`CERT_MANAGER_INSTALL_SKIP=true` (cert-manager is already installed by
Terraform) → `task cluster:down` (always). Local equivalent: `task ci:e2e`.

### `release` (tags only)
Builds and pushes the multi-arch controller image to GHCR with **OCI SBOM** and
**SLSA provenance** attestations, scans with Trivy, **cosign-signs** the image
digest (keyless OIDC), generates an SPDX SBOM (`dist/sbom.spdx.json`), packages
release assets via [`hack/release-assets.sh`](../hack/release-assets.sh)
(Kustomize manifests, Helm `.tgz`, checksums), **pushes the Helm chart to GHCR
OCI** (`helm push` → `oci://ghcr.io/<owner>/kurator:<version>`; reuses the
existing GHCR login — no extra token step), then publishes the same install
artifacts on the GitHub Release. Runs only on `v*.*.*` tags (or
`workflow_dispatch` for testing).

**Changelog:** [git-cliff](https://git-cliff.org/) (`cliff.toml`) generates the
release-notes section from Conventional Commits since the previous tag
(`orhun/git-cliff-action`, pinned to the same version as `task tools:git-cliff`).
Install instructions are appended from [`.github/release-notes-install.md`](../.github/release-notes-install.md)
via [`hack/assemble-release-notes.sh`](../hack/assemble-release-notes.sh). Checkout
uses `fetch-depth: 0` so tag ranges resolve correctly.

Maintainer steps: [RELEASE.md](RELEASE.md). Before tagging: `task changelog` (preview),
bump `charts/kurator/Chart.yaml`, `task changelog:write`, commit, then confirm
**CI, integration, and e2e are green on the exact commit SHA** you will tag
(release is a supply-chain gate — do not tag ahead of a red pipeline).
`git tag vX.Y.Z && git push origin vX.Y.Z`. Rationale: [ADR-0008](adr/0008-changelog-git-cliff.md).
Supply chain: [ADR-0016](adr/0016-release-supply-chain.md).

### image scan
**Trivy** scans the built image for OS/dependency vulnerabilities on release;
documented false positives live in `.trivyignore` with a rationale comment.
Critical/high findings fail the job.

## Caching

Go-heavy jobs in `ci.yaml` and the `integration` workflow restore and save
`actions/cache` entries keyed on `go.sum`:

| Cache | Path | Jobs |
|-------|------|------|
| Go modules + build cache | `~/go/pkg/mod`, `~/.cache/go-build` | verify, lint, test, build, docker-build, integration |
| envtest binaries | `~/.local/share/kubebuilder-envtest` | test only |

The envtest cache key includes the pinned K8s version (`1.35.x`, from
`Taskfile.test.yml`) so a version bump invalidates stale binaries. Docker layer
caching is not configured (integration/e2e pull IBM MQ images on each run).

## Security & supply chain

| Control | Mechanism |
|---------|-----------|
| Secret scan | gitleaks (pre-commit + CI) |
| Dependency vulns | `govulncheck` on PR / `main` push (in `test` job) |
| Image vulns | Trivy scan on release image |
| Dependency freshness | **Renovate** weekly workflow (`renovate.yaml`) |
| Pinned actions | GitHub Actions referenced by commit SHA |
| Node 24 runtime | Third-party actions bumped to Node 24 releases where available; `arduino/setup-task@v2.0.0` (no Node 24 tag yet) uses workflow `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true` in `ci.yaml`, `integration.yaml`, and `e2e.yaml` |
| Minimal permissions | `permissions:` block per workflow; default read-only |
| Reproducible build | CGO-free, pinned toolchain, committed `go.sum` |
| Nonroot runtime | distroless nonroot base, read-only FS, dropped caps |
| Release SBOM | BuildKit attestation on push + SPDX file on GitHub Release |
| Image signing | cosign keyless (`sigstore/cosign-installer`) on image digest |
| SLSA provenance | `provenance: mode=max` on `docker/build-push-action` |
| Helm chart (OCI) | `helm push` to `oci://ghcr.io/<owner>/kurator` on tag (GHCR package) |

Further supply-chain hardening (OpenSSF Scorecard, SLSA Level 3 builders) remains
optional; see [ADR-0005](adr/0005-keep-tooling-lean.md).

### Renovate (dependency freshness)

Weekly dependency update PRs are driven by
[`.github/workflows/renovate.yaml`](../.github/workflows/renovate.yaml) using
[`renovatebot/github-action`](https://github.com/renovatebot/github-action).

Configuration is split on purpose:

| File | Role |
|------|------|
| [`.github/renovate-config.json`](../.github/renovate-config.json) | **Global** (self-hosted) config passed to the action: target repo via `RENOVATE_REPOSITORIES`, onboarding disabled |
| [`renovate.json`](../renovate.json) | **Repository** config: schedules, grouping, custom managers, package rules |

**Maintainer setup:** add a repository secret `RENOVATE_TOKEN` — a classic
[Personal Access Token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
with:

- **Private repo:** `repo` + `workflow` scopes
- **Public repo:** `public_repo` + `workflow` scopes

The workflow falls back to `github.token` when `RENOVATE_TOKEN` is unset, but
that token is **not sufficient** for the `github-actions` manager: updating
pinned action SHAs in workflow files requires the `workflow` scope, which the
default `GITHUB_TOKEN` does not grant to third-party actions. Without
`RENOVATE_TOKEN`, Go/module/Docker bumps may still open PRs, but GitHub Actions
pin updates will fail or be skipped.

The job sets `RENOVATE_REPOSITORIES: ${{ github.repository }}` so Renovate knows
which repo to scan (global-only; do not put `autodiscover` in `renovate.json`).
Workflow file updates rely on the PAT `workflow` scope on `RENOVATE_TOKEN`, not
on `GITHUB_TOKEN` job permissions.
`LOG_LEVEL=debug` is enabled only on manual `workflow_dispatch` runs.

## Branch protection

The default branch requires CI jobs to pass before merge. Exact required checks
depend on GitHub branch protection settings; `e2e` and `integration` run on every
non-docs PR today. No direct pushes to the default branch.

If branch protection still lists removed job names (`format`, `govulncheck`), update
required checks to `lint` and `test` — those jobs now run the same `task` targets.

## Local equivalents

| CI job | Local command |
|--------|---------------|
| gitleaks | `task secrets:scan` |
| verify | `task verify` |
| lint | `task format:check` then `task lint` |
| test | `task test:run` then `task vuln:check` |
| build | `task build` |
| docker-build | `task docker:build` |
| helm-lint | `task helm:lint` |
| integration | `task ci:integration` (or `task test:integration:local`) |
| e2e | `task ci:e2e` (or `task cluster:up && KURATOR_E2E_MQ=1 task test:e2e`) |
| release changelog | `task changelog` / `task changelog:write` |

pre-commit runs `gofmt`/`goimports`, `golangci-lint`, and `task verify` so most
CI failures are caught before pushing.
