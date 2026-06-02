# Roadmap

Phased delivery plan for **Kurator**. Each phase is shippable
on its own and keeps the tree green (build + lint + tests + `verify` pass). See
[ARCHITECTURE.md](ARCHITECTURE.md) for design, [DEVELOPMENT.md](DEVELOPMENT.md) for
conventions, [NON_FUNCTIONAL_REQUIREMENTS.md](NON_FUNCTIONAL_REQUIREMENTS.md) for
quality bars, and [CICD.md](CICD.md) for the pipeline.

## Guiding principles

- Small, atomic, fully-tested increments over big drops.
- Every external interaction sits behind the `MQAdmin` port so it can be mocked.
- The build stays pure Go (`CGO_ENABLED=0`); no native MQ client.
- Generated artifacts are committed and verified fresh (`task verify`).
- NFRs (security, reliability, observability) are built in per phase, not bolted
  on at the end.

## Phase 0 — Foundations

- [x] Contributor conventions and toolchain documented (`docs/DEVELOPMENT.md`, doc map in README).
- [x] `docs/ARCHITECTURE.md` (runtime concerns, RBAC, error/requeue, security).
- [x] `docs/NON_FUNCTIONAL_REQUIREMENTS.md`, `docs/DEVELOPMENT.md`, `docs/CICD.md`.
- [x] `docs/adr/` with template, index, and initial decisions.
- [x] `SECURITY.md`, `README.md`, `docs/ROADMAP.md`.
- [x] Local platform under `hack/kind-cluster` (kind + Terraform + IBM MQ).

Exit criteria: design and local MQ platform documented and runnable — **met**.

## Phase 1 — Scaffold & toolchain

- [x] Scaffold with **Kubebuilder v4** (manager entrypoint, `PROJECT`, `api/v1alpha1`
  group shell, `internal/controller` package, structured logging).
- [x] `Taskfile.yml` + `Taskfile.test.yml` (install, format, lint, manifests,
  generate, **verify**, build, docker:build, `cluster:*`, deploy/undeploy,
  test:run, test:e2e), with Go tools pinned via `go.mod` `tool` directives.
- [x] `.golangci.yaml` (v2), `.mockery.yaml`, `.pre-commit-config.yaml`,
  `.editorconfig`, distroless nonroot `Dockerfile`.
- [x] Manager wiring with leader election, health/readiness probes, and a protected
  metrics endpoint (NFR REL-3/OBS-3).
- [x] GitHub Actions per [CICD.md](CICD.md): `verify` + lint + unit tests +
  `govulncheck` + **gitleaks** on PRs; pinned action SHAs.
- [x] **Renovate** — `renovate.json` (Go, Actions, Docker, Terraform, pre-commit)
  and a weekly self-hosted `.github/workflows/renovate.yaml` (no Mend app required).

**Also delivered in Phase 1:**

- [x] [ADR-0006](adr/0006-project-name-kurator.md) — project name, module path,
  API group `messaging.kurator.dev`.
- [x] [ADR-0007](adr/0007-structured-logging-logr-slog.md) + [LOGGING.md](LOGGING.md)
  — `logr`/`slog`, redaction handler, manager flags/env.
- [x] `hack/verify.sh` (codegen drift check) and `hack/goformat.sh`.
- [x] **Secret scanning** — gitleaks in pre-commit (`.pre-commit-config.yaml`),
  CI (`ci.yaml`), `task secrets:scan`, and `.gitleaks.toml` allowlists for local
  dev artifacts (`references/`, kind cluster state, terraform state).
- [x] `.envrc` — `KUBECONFIG` / `TF_VAR_kubeconfig` for the local kind cluster.

Exit criteria: `task build`, `task lint`, `task verify`, and `task test:run` pass
locally and in CI — **met**.

## Phase 2 — Core API, adapter & tests

- [x] `api/v1alpha1`: `QueueManagerConnection` and `Queue` types + generated
  deepcopy and CRD manifests; kubebuilder validation and print columns.
- [x] `internal/mqadmin`: `Admin` + `Factory` ports, domain types, sentinel errors.
- [x] `internal/adapter/mqrest`: `mqweb` client (`Ping`, `GetQueue`, `DefineQueue`,
  `DeleteQueue`) + `ClientFactory` (Secret/credential resolution, TLS, client cache).
- [x] `internal/adapter/mqrest`: `httptest` unit tests (success, not-found, ping).
- [x] `internal/controller`: thin reconcilers — finalizers, drift detection, status
  conditions (`Ready`, `Synced`, `observedGeneration`); Queue waits for connection
  `Ready` before calling MQ.
- [x] Tests: mockery mocks (`test/mocks/mqadmin`), `needsUpdate` unit test,
  Ginkgo envtest suite (controller + API server, mocked `Admin`).
- [x] RBAC from reconciler markers (`controller-gen` paths include
  `internal/controller/...`).
- [x] `config/samples/` (Kustomize) and `config/crd/bases/`.

**Also delivered in Phase 2:**

- [x] [docs/schemas/](schemas/) — `mqsc-runcommand.schema.json` + README; optional
  `scripts/fetch-mqweb-swagger.sh` for full mqweb Swagger export.
- [x] `task deploy` — `go tool kustomize`, explicit CRD apply (no global kustomize).
- [x] **`charts/kurator/`** — publishable Helm chart, `helm:sync-crds`, kind
  `values-kind.yaml`, sample Secret + CRs under `charts/kurator/samples/resources/`.
- [x] Local workflow tasks: `deploy:helm`, `deploy:samples`, `local:up` /
  `local:deploy` / `local:info` / `local:down`.
- [x] README + [DEVELOPMENT.md](DEVELOPMENT.md) — full local setup documented;
  `hack/kind-cluster/README.md` cross-linked.
- [x] Optional local `docs/REFERENCES.md` + `references/` clones (both gitignored)
  for vendored IBM MQ sample trees — not published in the repository.
- [x] `setup-envtest` wired in `Taskfile.test.yml` (`KUBEBUILDER_ASSETS`).
- [x] Manual validation on kind: `QueueManagerConnection` reaches **Ready** against
  live `QM1`; operator reaches mqweb in-cluster.

**Remaining before Phase 2 is fully closed:**

- [x] Fix **DISPLAY QLOCAL** on live MQ — drop `maxmsglen` from display parameters
  (mqweb 9.4 returns `MQWB0120E`); coerce numeric DEFINE attrs; **Queue** reaches
  **Synced=True** on `task local:up`.
- [x] Raise `internal/` coverage to **≥85%** (enforced in `task test:run`).

Exit criteria: envtest + adapter tests + live queue on kind — **met**.

## Phase 3 — E2E, CI & release

- [x] e2e scaffold (`test/e2e`, build tag `e2e`) — controller pod, metrics, suite
  wiring on kind.
- [x] MQ e2e scenarios (`test/e2e/mq_e2e_test.go`, `mq_helpers.go`) gated by
  `KURATOR_E2E_MQ=1` — QueueManagerConnection + Queue CR apply, MQSC fixture
  apply via mqweb; assert real MQSC objects for create/update/delete and re-apply
  idempotency (NFR REL-1) once DEFINE QLOCAL is fixed (Phase 2 blocker).
- [x] `test/e2e/fixtures/` — MQSC bootstrap for channels/auth (from
  mq-gitops-samples); see [PHASE5_AUTH_SKETCH.md](PHASE5_AUTH_SKETCH.md).
- [x] Wire e2e into CI (`.github/workflows/e2e.yaml`: kind + IBM MQ + `task test:e2e`;
  `task ci:e2e` for local parity).
- [x] Release workflow (`.github/workflows/release.yaml`): multi-arch distroless
  image publish to GHCR + Trivy image scan + published Kustomize/Helm install
  manifests (`hack/release-assets.sh`, `charts/kurator/samples/values-release.yaml`,
  `.trivyignore`) on `v*.*.*` tags (NFR OPS-1/OPS-2, SEC-4/SEC-6).
- [x] Release supply chain — OCI SBOM + SLSA provenance (`docker/build-push-action`),
  SPDX SBOM on GitHub Releases (`anchore/sbom-action`), cosign keyless signing
  (`sigstore/cosign-installer`).

**Also delivered in Phase 3:**

- [x] Helm install path for local and release publish (`charts/kurator`, `task helm:*`).
- [x] `RunMQSC` helper on `mqrest` client (`runCommand` plaintext) + unit test —
  groundwork for e2e fixtures and Phase 5 MQSC.

Exit criteria: `task test:e2e` green locally and in CI against a live Queue
Manager; release pipeline produces a scanned, signed image with SBOM and install
manifests — **met** (e2e in CI via `e2e.yaml`; signing/SBOM on release tags).

## Phase 4 — Additional MQ objects (Topic, Channel)

Extend declarative management beyond local queues to other common MQSC object types
before access-control work.

- [x] `Topic` CRD — DEFINE/DISPLAY/DELETE topic (`DEFINE TOPIC`, drift detection,
  finalizers); map attributes per [IBM_MQ_OBJECTS.md](IBM_MQ_OBJECTS.md).
- [x] `Channel` CRD — `CHLTYPE(SVRCONN)` in v1alpha1 (other channel types later).
- [x] Extend `MQAdmin` port and `mqrest` adapter for topic/channel operations;
  table-driven adapter tests with `httptest`.
- [x] Thin reconcilers, RBAC, samples under `config/samples/` and
  `charts/kurator/samples/resources/`.
- [x] Unit + envtest coverage; [x] e2e scenarios on kind against live `QM1`
  (Queue, Topic, Channel — see [`test/e2e/mq_e2e_test.go`](../test/e2e/mq_e2e_test.go)).
- [x] [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md) — DEFINE vs DISPLAY
  drift matrix per object; user tables in [INSTALL_AND_USE.md](INSTALL_AND_USE.md).
- [x] `Queue.spec.type` OpenAPI aligned with reconcilers (`local`, `alias`, `remote`).
- [x] Drift detection: case-insensitive `pub`/`sub`/policies; channel `maxinst` /
  `maxinstc`; topic `pubscope`/`subscope` where mqweb DISPLAY allows.
- [x] **Alias** and **remote** queue types (`QALIAS`, `QREMOTE`) with drift detection.
- [ ] Optional: extend queue DISPLAY list (`share`, `defopts`, …) after mqweb 9.4
  compatibility testing; TLS channel attrs (`sslciph`, `sslcauth`) for drift.

Exit criteria: at least **Topic** and one **Channel** kind reconcile end-to-end on
kind with the same quality bar as Phase 2 (`verify`, ≥85% `internal/` coverage,
e2e green).

## Phase 5 — User & authority management

- [x] [PHASE5_AUTH_SKETCH.md](PHASE5_AUTH_SKETCH.md) — CR sketch mapped from
  reference MQSC; e2e fixture [`test/e2e/fixtures/channel-auth-prereq.mqsc`](../test/e2e/fixtures/channel-auth-prereq.mqsc).
- [ ] Extend the API toward MQ access control: authority records / channel auth /
  user-style resources (exact CRDs decided when reached).
- [ ] Corresponding `MQAdmin` operations, adapter support, and tests at all layers.

Exit criteria: declarative channel auth (and at least one user/authority resource)
reconciled on kind with e2e coverage.

## Repo visibility

- [x] README badges — CI, MIT license, Codecov, Go module / pkg.go.dev
  ([konih/kurator](https://github.com/konih/kurator)).
- [x] User guide — [INSTALL_AND_USE.md](INSTALL_AND_USE.md) + annotated
  [config/samples/README.md](../config/samples/README.md).
- [x] CI coverage export — `coverage.out` artifact, job summary, Codecov upload
  (`codecov.yml`; first green `main` run registers the project).
- [x] **Go Report Card** — badge in [README.md](../README.md); refresh at
  [goreportcard.com/report/github.com/konradheimel/kurator](https://goreportcard.com/report/github.com/konradheimel/kurator)
  after significant API changes (uses module path from `go.mod`).
- [x] Release badge — [`README.md`](../README.md) links GitHub Releases (`v0.1.0`).

## Later / candidate work

- Optional PCF adapter behind the existing `MQAdmin` port for environments
  without `mqweb`.
- Metrics/dashboards and richer status reporting.
- Documentation site and OCI Helm chart registry (release today attaches `.tgz`
  to GitHub Releases; GHCR chart push is optional follow-up).
- Commit generated `docs/schemas/mqweb-swagger.json` per target MQ version.
