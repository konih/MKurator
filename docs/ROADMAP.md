# Roadmap

Phased delivery plan for **MKurator**. Each phase is shippable
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

- [x] [ADR-0006](adr/0006-project-name-kurator.md) — original project name and
  module identity (superseded by [ADR-0018](adr/0018-project-rename-mkurator.md):
  API group `messaging.mkurator.dev`).
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
- [x] **`charts/mkurator/`** — publishable Helm chart, `helm:sync-crds`, kind
  `values-kind.yaml`, sample Secret + CRs under `charts/mkurator/samples/resources/`.
- [x] Local workflow tasks: `deploy:helm`, `deploy:samples`, `local:up` /
  `local:deploy` / `local:info` / `local:down`.
- [x] README + [DEVELOPMENT.md](DEVELOPMENT.md) + [LOCAL_SETUP.md](LOCAL_SETUP.md)
  — local setup documented; `hack/kind-cluster/README.md` cross-linked.
- [x] Optional local `docs/REFERENCES.md` (from [REFERENCES.md.example](REFERENCES.md.example)) + `references/` clones (both gitignored)
  for vendored IBM MQ sample trees — not published in the repository.
- [x] `setup-envtest` wired in `Taskfile.test.yml` (`KUBEBUILDER_ASSETS`).
- [x] Manual validation on kind: `QueueManagerConnection` reaches **Ready** against
  live `QM1`; operator reaches mqweb in-cluster.
- [x] Fix **DISPLAY QLOCAL** on live MQ — drop `maxmsglen` from display parameters
  (mqweb 9.4 returns `MQWB0120E`); coerce numeric DEFINE attrs; **Queue** reaches
  **Synced=True** on `task local:up`.
- [x] Raise `internal/` coverage to **≥90%** (enforced in `task test:run`).

Exit criteria: envtest + adapter tests + live queue on kind — **met**.

## Phase 3 — E2E, CI & release

- [x] e2e scaffold (`test/e2e`, build tag `e2e`) — controller pod, metrics, suite
  wiring on kind.
- [x] MQ e2e scaffold (`test/e2e/mq_e2e_test.go`, `mq_helpers.go`) gated by
  `KURATOR_E2E_MQ=1` — QueueManagerConnection + **Queue** CR apply, MQSC fixture
  apply via mqweb; assert real MQSC objects for create/update/delete and re-apply
  idempotency (NFR REL-1). Topic, Channel, and auth CR scenarios were added in
  Phase 4/5 (same test file).
- [x] `test/e2e/fixtures/` — MQSC bootstrap for channels/auth (from
  mq-gitops-samples); see [PHASE5_AUTH_SKETCH.md](PHASE5_AUTH_SKETCH.md).
- [x] Wire e2e into CI (`.github/workflows/e2e.yaml`: kind + IBM MQ + `task test:e2e`;
  `task ci:e2e` for local parity).
- [x] Release workflow (`.github/workflows/release.yaml`): multi-arch distroless
  image publish to GHCR + Trivy image scan + published Kustomize/Helm install
  manifests (`hack/release-assets.sh`, `charts/mkurator/samples/values-release.yaml`,
  `.trivyignore`) on `v*.*.*` tags (NFR OPS-1/OPS-2, SEC-4/SEC-6).
- [x] Release supply chain — OCI SBOM + SLSA provenance (`docker/build-push-action`),
  SPDX SBOM on GitHub Releases (`anchore/sbom-action`), cosign keyless signing
  (`sigstore/cosign-installer`), Helm chart OCI push to GHCR (`helm push`).

**Also delivered in Phase 3:**

- [x] Helm install path for local and release publish (`charts/mkurator`, `task helm:*`).
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
  `charts/mkurator/samples/resources/`.
- [x] Unit + envtest coverage; [x] e2e scenarios on kind against live `QM1`
  (Queue, Topic, Channel — see [`test/e2e/mq_e2e_test.go`](../test/e2e/mq_e2e_test.go)).
- [x] [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md) — DEFINE vs DISPLAY
  drift matrix per object; user tables in [INSTALL_AND_USE.md](INSTALL_AND_USE.md).
- [x] `Queue.spec.type` OpenAPI aligned with reconcilers (`local`, `alias`, `remote`).
- [x] Drift detection: case-insensitive `pub`/`sub`/policies; channel `maxinst` /
  `maxinstc`; topic `pubscope`/`subscope` where mqweb DISPLAY allows.
- [x] **Alias** and **remote** queue types (`QALIAS`, `QREMOTE`) with drift detection.
- [x] TLS channel attrs (`sslciph`, `sslcauth`) drift-checked (shipped; see
  [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md)).
- [ ] Queue attrs `share`, `defopts`, `bothresh`, `boqname`, `usage` remain
  DEFINE-only on mqweb 9.4 (`MQWB0120E` on DISPLAY); drift for those deferred
  until mqweb allows it (capability probing direction:
  [ADR-0024](adr/0024-mqsc-command-construction-hygiene.md)).

Exit criteria: at least **Topic** and one **Channel** kind reconcile end-to-end on
kind with the same quality bar as Phase 2 (`verify`, ≥90% `internal/` coverage,
e2e green) — **met** (optional DISPLAY/TLS drift extensions above remain open).

## Phase 4b — Validating admission webhooks (pre–Phase 5)

- [x] Validating webhooks (no mutating) for `QueueManagerConnection`, `Queue`, `Topic`, `Channel`.
- [x] cert-manager TLS for webhook serving; Kustomize + Helm wired.
- [x] Referential checks: `connectionRef` exists, same namespace, not deleting.
- [x] Queue: MQ name constraints; alias/remote required attributes; optional unknown-attribute warnings.
- [x] Topic/Channel: name constraints; channel `svrconn` only.
- [x] Unit tests (`internal/validation`) + envtest admission tests; optional e2e negative apply.
- [x] Optional: deny `QueueManagerConnection` delete while dependent CRs exist.

Exit criteria: **met** — invalid manifests rejected by `kubectl apply` on kind (Kustomize and Helm verified); `task test:run` includes webhook envtest suite.

## Phase 5 — User & authority management

Planning doc: [PHASE5_AUTH_SKETCH.md](PHASE5_AUTH_SKETCH.md) (CR sketch mapped from
reference MQSC; e2e fixture
[`test/e2e/fixtures/channel-auth-prereq.mqsc`](../test/e2e/fixtures/channel-auth-prereq.mqsc)).

**Shipped on `main`:**

- [x] `ChannelAuthRule` and `AuthorityRecord` CRDs — API, CRDs, `MQAdmin` port,
  `mqrest` adapter (`SET CHLAUTH` / `SET AUTHREC`), thin reconcilers, validating
  webhooks, samples, Docker integration tests (including auth delete/update paths).
- [x] **E2e on kind** — `ChannelAuthRule` and `AuthorityRecord` reconcile and
  delete in [`test/e2e/mq_e2e_test.go`](../test/e2e/mq_e2e_test.go); adapter GET
  helpers for auth assertions; admission negative tests for invalid Queue and
  ChannelAuthRule without a matching Channel.
- [x] **MQAdmin GET + drift** — `GetChannelAuth` / `GetAuthority`; replace-on-diff
  in auth reconcilers; `status.desiredMQSC` on auth CRs.
- [x] Release tags **`v0.5.0`**–**`v0.5.3`** and **`v0.6.0`** with GitHub Releases;
  publish only after the [RELEASE.md](RELEASE.md) gate is green on the tag SHA
  (historical note: **`v0.5.2`** was cut before e2e was consistently green on `main`).
- [x] **CI ergonomics (Phase 5)** — [`preflight.yaml`](../.github/workflows/preflight.yaml)
  (fail-fast `go mod tidy` + `task verify`), [`nightly.yaml`](../.github/workflows/nightly.yaml)
  (Mon 03:00 UTC full pyramid), [`release-gate.yaml`](../.github/workflows/release-gate.yaml)
  (`workflow_dispatch` on SHA), Phase C PR e2e filter `(smoke || mq) && !slow`,
  JUnit artifacts (`integration-junit.xml`, `e2e-junit.xml`), composite caches under
  [`.github/actions/`](../.github/actions/) — see [CICD.md](CICD.md).

**Remaining:**

- [x] **Pipeline green on `main`** — **CI**, **Integration**, and **E2E (kustomize)**
  succeeded on `main` before **`v0.6.0`** (evidenced 2026-06-03/04; use
  [release-gate](RELEASE.md#automated-release-gate-workflow) or `gh run list` before
  the next tag).
- [ ] **`task ci:e2e` green locally** — maintainer verification of full kind + MQ
  stack (Kustomize deploy path); respect `exclusive-test.lock`.
- [x] Helm **ClusterRole** includes auth CRDs; `hack/helm-verify-rbac.sh` in `task helm:lint`.
- [x] E2e **BLOCKUSER** `ChannelAuthRule` on kind ([`test/e2e/mq_e2e_test.go`](../test/e2e/mq_e2e_test.go)).
- [x] Optional: integration **BLOCKUSER** CHLAUTH
  ([`test/integration/mq/auth_integration_test.go`](../test/integration/mq/auth_integration_test.go)).
- [ ] Additional CHLAUTH rule types (`USERMAP`, `SSLPEERMAP`, …) — schema present;
  extend API fields, integration, and e2e when needed. **Resequenced after
  Phase 7** (2026-06-09 audit): production-hardening table stakes serve
  adopters deciding whether to commit; extended rule types serve users already
  committed.

Exit criteria: declarative channel auth and OAM authority records reconciled on
kind with e2e specs — **met** for core auth **code** and **`v0.6.0`** pipeline
closure on `main`; extended CHLAUTH rule types remain optional; local **`task ci:e2e`**
verification remains a maintainer checklist item.

## Repo visibility

- [x] README badges — CI, MIT license, Codecov, Go module / pkg.go.dev
  ([conduit-ops/MKurator](https://github.com/conduit-ops/MKurator)).
- [x] User guide — [INSTALL_AND_USE.md](INSTALL_AND_USE.md) + annotated
  [config/samples/README.md](../config/samples/README.md).
- [x] CI coverage export — `coverage.out` artifact, job summary, Codecov upload
  (`codecov.yml`; first green `main` run registers the project).
- [x] **Go Report Card** — badge in [README.md](../README.md); refresh at
  [goreportcard.com/report/github.com/conduit-ops/mkurator](https://goreportcard.com/report/github.com/conduit-ops/mkurator)
  after significant API changes (uses module path from `go.mod`).
- [x] Release badge — [`README.md`](../README.md) links GitHub Releases (latest tag).
- [x] [LOCAL_SETUP.md](LOCAL_SETUP.md) — tiered dev tool install (`Brewfile`,
  `task tools:check` / `task tools:install`, updated `.devcontainer/`).

## Phase 6 — OSS maturity (shipped, v0.7.0)

Meta-engineering wave per [ADR-0019](adr/0019-oss-maturity-posture.md) /
[ADR-0020](adr/0020-merge-gate-matrix.md):

- [x] MkDocs Material docs site + GitHub Pages (`docs.yaml`), README badge.
- [x] CodeQL, OpenSSF Scorecard, RBAC audit job; SonarCloud scaffolded (disabled).
- [x] Release attestations (cosign sign-blob, `actions/attest`, signed Helm OCI).
- [x] Community docs (`CODE_OF_CONDUCT.md`, `GOVERNANCE.md`, DCO), engineering
  standards split (`docs/development/*`), assurance docs.
- [x] Grafana dashboard + PrometheusRule (0.6.0/0.7.0); metrics catalog in
  [OBSERVABILITY.md](OBSERVABILITY.md).

Exit criteria: docs site live, posture workflows green — **met**. Further badge
work is deprioritized in favour of Phase 7 (2026-06-09 audit litmus test: *does
the artifact help a user run the operator, or help the repo look run?*).

## Phase 7 — Production hardening & operator table stakes

Driven by the 2026-06-09 audit train (architecture, edge-case, test-quality,
docs, release lanes + critical design review). These items serve adopters
**deciding whether to commit** and therefore precede further MQ-surface depth.
New decisions: [ADR-0021](adr/0021-attribute-api-shape.md) –
[ADR-0025](adr/0025-cel-first-admission-validation.md).

### 7a — Reliability fixes (P0/P1, land first)

- [x] **Deletion deadlocks** (EC-P0-01/EC-P0-02): evaluate `DeletionTimestamp`
  before requiring a ready connection; `ReleaseConnection` tolerates missing
  Secrets; envtest locks T1/T2. Per [ADR-0022](adr/0022-deletion-and-adoption-policy.md)
  / [ADR-0023](adr/0023-connection-client-cache-lifecycle.md). *(v0.7.1)*
- [x] **QMC hot loop** (ARCH-01/EC-P1-02): stop `Ready` False→True flapping on
  every reconcile; status-change-only patches + predicates; one `Available`
  event per transition (ADR-0015 compliance); envtest lock T4. *(v0.7.1, refined v0.8.0)*
- [x] **MQSC injection** (EC-P1-04): enum/pattern validation for
  `userSource`/`checkClient`/`authorities[]` + shared quoting helper per
  [ADR-0024](adr/0024-mqsc-command-construction-hygiene.md); webhook envtest T6. *(v0.7.1)*
- [x] **Periodic drift resync** (ARCH-04/EC-P1-01, ADR-0010 gap): configurable
  jittered `RequeueAfter` on successful syncs (default 5–10 min) + drift
  detection metric; makes `/readyz` MQ-health real again (EC-P2-06); envtest
  locks T3/T3b. *(v0.7.1)*
- [x] **Client cache lifecycle** (ARCH-02/03, EC-P2-01): identity-keyed cache,
  replace-on-rotation with transport close, bounded size — per
  [ADR-0023](adr/0023-connection-client-cache-lifecycle.md). *(v0.7.1)*
- [x] `RecoverPanic` on all controllers (EC-P1-05); log swallowed `List` errors
  in connection fan-out (EC-P2-02, ARCH-10); fix transient
  `RequeueAfter`+error conflict (EC-P3-02). *(v0.7.1)*

### 7b — Operator table stakes

- [x] **`spec.suspend` + reconcile-now annotation** (MKR-01; makes the FAQ
  claim true — previously documented a non-existent annotation, DOC-02). *(v0.8.0)*
- [x] **Watch referenced Secrets** (MKR-02/EC-P1-03): credential rotation and
  fixed-Secret recovery become event-driven; envtest lock T5. *(v0.8.0)*
- [x] **`spec.deletionPolicy` (`Delete`/`Orphan`) + force-orphan annotation;
  `spec.adoptionPolicy` (`Adopt`/`AdoptIfMatching`/`FailIfExists`)** — per
  [ADR-0022](adr/0022-deletion-and-adoption-policy.md); brownfield-safe semantics. *(v0.8.0)*
- [x] **Retry/backoff + circuit breaker around mqweb** (MKR-03) with breaker
  state metric; per-request context deadlines distinct from the 60 s client cap. *(v0.8.0)*
- [x] **Configurable, jittered requeue intervals** (MKR-05): QMC health 30 s,
  connection-wait 15 s, transient 30 s, drift resync — manager flags. *(v0.8.0)*
- [x] **CEL-first validation** per [ADR-0025](adr/0025-cel-first-admission-validation.md):
  stateless rules on CRDs; webhooks for referential checks only; golden + envtest
  parity; cert-manager optional with `webhooks.enabled=false`.
- [x] **Secret RBAC/cache scoping** (ARCH-05): filtered informer cache for
  Secrets; align ARCHITECTURE.md least-privilege claim. Warn on the `admin`
  default when username keys are absent (ARCH-12). *(v0.8.0)*

### 7c — Code & test health

- [x] **Collapse the 5-way type switches** (ARCH-06, critical review §2.6):
  `MQObject` interface or generics across `reconcile_shared.go` /
  `status_helpers.go` / `drift_policy.go`; one generic workload reconcile
  skeleton. **Precondition for any new CRD kind.**
- [x] **Wrong-behavior fixes** (test audit): observe-only CHLAUTH/AUTHREC must
  not SET missing objects (WB-01/F01); typed wrap errors replace substring
  event classification (WB-02/F02, ADR-0014 compliance); per-kind status-patch
  tests assert re-read state (WB-03/F03). *(v0.8.0)*
- [x] Delete dead helpers + padding tests (ARCH-09/F04); envtest gaps for
  Channel/Topic/AuthorityRecord paths (F07) — lifts `internal/controller`
  coverage (87.6%) honestly. *(v0.8.0)*
- [ ] **E2E flake triage** (ARCH-07): 2 of last 6 main runs failed with
  reruns green; duration variance 13–34 min (note added v0.8.0; root cause open).
- [x] Fix `task changelog` clobbering `CHANGELOG.md` (release audit P1-2:
  `cliff.toml` `output` vs preview tasks). *(v0.7.1)*

### 7d — Docs truth wave

- [x] Fix drift-semantics contradiction in INSTALL_AND_USE.md (DOC-01);
  remove/implement FAQ `suspend` entry (DOC-02 — closes with 7b);
  cert-manager in prerequisites (DOC-03); uninstall covers auth CRs (DOC-09);
  broken ADR-0006 links (DOC-04); cert `Ready` wait in UPGRADE.md (DOC-07);
  version pins → current release (DOC-08); remaining link/anchor fixes
  (DOC-05/10/11/12); doc-map sync (DOC-13); CRD field comments (DOC-14);
  ADR-0018 `KURATOR_*` prefix note (DOC-17).

Exit criteria: all EC-P0/P1 closed with envtest locks; suspend/deletion/
adoption/resync shipped and documented; CEL migration complete with golden
parity; coverage floor intact without padding; e2e flake rate addressed.

## Phase 8 — API maturation (v1beta1 readiness)

- [ ] **Typed attribute fields + escape hatch** per
  [ADR-0021](adr/0021-attribute-api-shape.md): promote drift-checked keys to
  typed, CEL-validated spec fields; exclusivity rule; schema goldens.
- [x] Published **API stability statement**: [API_STABILITY.md](API_STABILITY.md) —
  what `v1alpha1` guarantees, what graduation to `v1beta1` requires (conversion
  webhook, deprecation policy).
- [ ] Optional: DISPLAY **capability probing** per
  [ADR-0024](adr/0024-mqsc-command-construction-hygiene.md) §4, replacing
  hand-maintained per-version safe lists.
- [ ] `v1beta1` graduation of all six kinds with conversion webhook once 8a/8b
  are stable for one minor release.

## Phase 9 — MQ surface depth (resequenced from Phase 5)

- [ ] Additional CHLAUTH rule types: `USERMAP`, `SSLPEERMAP`, `QMGRMAP` —
  CRD fields, mqrest SET/GET + drift, integration + e2e (daily-backlog
  AUTH-3a…AUTH-6).
- [ ] AuthorityRecord channel/namelist profile parity with queue profiles.
- [ ] BLOCKADDR integration + e2e coverage (AUTH-1/AUTH-2).
- [ ] Further channel types (SDR/RCVR) behind the same quality bar — only
  after the ARCH-06 refactor (7c) makes new kinds cheap.

## Later / candidate work

- PCF adapter behind `MQAdmin` — **parked** per the 2026-06-09 note in
  [ADR-0017](adr/0017-pcf-adapter-behind-mqadmin.md): implemented only if a
  concrete no-mqweb adopter commits. Scaffold stays as the compile-time
  contract check.
- Multi-tenancy / governance boundary (`MQProject`-style allowlists, MKR-09)
  and `QueueSet`-style generators (MKR-12) — evaluate on demand signal only.
- Property-based tests for MQSC/attribute reconciliation (MKR-10,
  `pgregory.net/rapid`): idempotency and determinism properties.
- `testcontainers-go` for the Docker MQ integration tier (MKR-07) — prototype
  and compare flake/runtime before replacing `hack/mq-docker`.
- OpenTelemetry tracing `Reconcile → mqweb` (MKR-08 tail).
- Commit generated `docs/schemas/mqweb-swagger.json` per target MQ version.
- [x] **Admission:** envtest assertion for unknown-attribute warnings (Queue, Topic,
  Channel) via `internal/webhook/v1alpha1/suite_test.go`.
- [x] **e2e Helm deploy path** (`KURATOR_E2E_DEPLOY=helm`, `task test:e2e:helm`) —
  CI job `e2e (helm)` on `main` push and `workflow_dispatch` ([`e2e.yaml`](../.github/workflows/e2e.yaml)).
- [x] **Runtime cleanup:** migrated to `mgr.GetEventRecorder` (events.k8s.io API).
