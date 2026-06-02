# Roadmap

Phased delivery plan for **Kurator**. Each phase is shippable
on its own and keeps the tree green (build + lint + tests + `verify` pass). See
[ARCHITECTURE.md](ARCHITECTURE.md) for design, [../AGENTS.md](../AGENTS.md) for
conventions, [NON_FUNCTIONAL_REQUIREMENTS.md](NON_FUNCTIONAL_REQUIREMENTS.md) for
quality bars, and [CICD.md](CICD.md) for the pipeline.

## Guiding principles

- Small, atomic, fully-tested increments over big drops.
- Every external interaction sits behind the `MQAdmin` port so it can be mocked.
- The build stays pure Go (`CGO_ENABLED=0`); no native MQ client.
- Generated artifacts are committed and verified fresh (`task verify`).
- NFRs (security, reliability, observability) are built in per phase, not bolted
  on at the end.

## Phase 0 — Foundations (this step)

- [x] `AGENTS.md` with context, conventions, toolchain, and doc map.
- [x] `docs/ARCHITECTURE.md` (runtime concerns, RBAC, error/requeue, security).
- [x] `docs/NON_FUNCTIONAL_REQUIREMENTS.md`, `docs/DEVELOPMENT.md`, `docs/CICD.md`.
- [x] `docs/adr/` with template, index, and initial decisions.
- [x] `SECURITY.md`, `README.md`, `docs/ROADMAP.md`.
- [x] Local platform under `hack/kind-cluster` (kind + Terraform + IBM MQ).

## Phase 1 — Scaffold & toolchain

- [x] Scaffold with **Kubebuilder v4** (manager entrypoint, `PROJECT`, `api/v1alpha1`,
  `internal/controller`, structured logging).
- [x] `Taskfile.yml` + `Taskfile.test.yml` (install, format, lint, manifests,
  generate, **verify**, build, docker:build, cluster:up/down, deploy/undeploy,
  test:run, test:e2e), with Go tools pinned via `go.mod` `tool` directives.
- [x] `.golangci.yaml` (v2), `.mockery.yaml`, `.pre-commit-config.yaml`,
  `.editorconfig`, distroless nonroot `Dockerfile`.
- [x] Manager wiring with leader election, health/readiness probes, and a protected
  metrics endpoint (NFR REL-3/OBS-3).
- [x] GitHub Actions per [CICD.md](CICD.md): `verify` + lint + unit tests +
  `govulncheck` on PRs; pinned action SHAs; Renovate config.

Exit criteria: `task build`, `task lint`, `task verify`, and `task test:run` pass
locally and in CI — **met**.

## Phase 2 — Core API, adapter & tests

- `api/v1alpha1`: `QueueManagerConnection` and `Queue` types + generated
  deepcopy and CRD manifests; basic validation (kubebuilder markers).
- `internal/mqadmin`: the `MQAdmin` port and domain types.
- `internal/adapter/mqrest`: `mqweb` REST client implementing `MQAdmin`
  (define/inspect/delete queue, ping), with `httptest`-based unit tests.
- `internal/controller`: thin reconcilers for both resources — finalizers,
  drift detection, status conditions (`Ready`, `Synced`, `observedGeneration`).
- Tests: mockery mocks of `MQAdmin`, unit tests for reconcilers, envtest for
  API/controller integration. Maintain high coverage on `internal/`.

Exit criteria: applying samples in envtest drives the expected `MQAdmin` calls;
adapter unit tests cover success + error paths.

## Phase 3 — End-to-end & CI hardening

- e2e suite (`test/e2e`, build tag `e2e`) on **kind** against the real IBM MQ
  Queue Manager from `hack/kind-cluster`; assert real MQSC objects for
  create/update/delete and re-apply idempotency (NFR REL-1).
- Wire e2e into CI (kind in GitHub Actions) on a dedicated job.
- Release workflow: multi-arch distroless image publish + Trivy image scan +
  published Kustomize install manifests (NFR OPS-1/OPS-2, SEC-4/SEC-6).
- Optional supply-chain extras (SBOM, signing) deferred per
  [ADR-0005](adr/0005-keep-tooling-lean.md).

Exit criteria: `task test:e2e` green locally and in CI against a live Queue
Manager; release pipeline produces a scanned image and install manifests.

## Phase 4 — User & authority management

- Extend the API toward MQ access control: authority records / channel auth /
  user-style resources (exact CRDs decided when reached).
- Corresponding `MQAdmin` operations, adapter support, and tests at all layers.

## Later / candidate work

- Additional object types (`Topic`, `Channel`, alias/remote queues).
- Optional PCF adapter behind the existing `MQAdmin` port for environments
  without `mqweb`.
- Metrics/dashboards and richer status reporting.
- Documentation site and published install manifests.
