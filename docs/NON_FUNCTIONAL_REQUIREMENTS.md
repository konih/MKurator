# Non-Functional Requirements

This document defines the **non-functional requirements (NFRs)** for
**Kurator**: the cross-cutting quality attributes every release must satisfy. Functional behaviour lives in [ARCHITECTURE.md](ARCHITECTURE.md); the
delivery order lives in [ROADMAP.md](ROADMAP.md).

Each requirement has an ID (`NFR-x`), a priority (**MUST** / **SHOULD** /
**MAY**), and a verification approach so it is testable rather than aspirational.

## 1. Security

| ID | Req | Priority | Verification |
|----|-----|----------|--------------|
| SEC-1 | No credentials, keys, or tokens in CR specs, code, images, or logs. Secrets are referenced via Kubernetes `Secret`s only. | MUST | Code review, **gosec** in `task lint`, grep in CI |
| SEC-2 | All mqweb traffic is HTTPS with certificate verification on by default; custom CA via `caSecretRef`. `insecureSkipVerify` is opt-in and dev-only. | MUST | Adapter unit tests; default-config test |
| SEC-3 | Operator RBAC is least-privilege: own API group + referenced Secrets + Events + leader-election Lease. No wildcards, no cluster-admin. | MUST | RBAC manifest review; `task verify` |
| SEC-4 | Container runs as **nonroot**, read-only root filesystem, dropped capabilities, no privilege escalation; distroless base. | MUST | Pod SecurityContext; image inspection |
| SEC-5 | Structured logs scrub secrets and do not emit credentialed request/response bodies at default levels. | MUST | Logging unit tests; review |
| SEC-6 | No known high/critical vulnerabilities in dependencies or image at release. | MUST | `govulncheck` + Trivy in CI |
| SEC-7 | Supply-chain hardening: pinned tool/action versions, committed `go.sum`, reproducible CGO-free build. | SHOULD | CI config review |
| SEC-8 | Multi-tenancy: namespaced CRs and Secret references respect namespace boundaries; the operator does not cross namespaces implicitly. | SHOULD | envtest for cross-namespace refs |

## 2. Reliability & availability

| ID | Req | Priority | Verification |
|----|-----|----------|--------------|
| REL-1 | Reconciliation is **idempotent**: repeated reconciles converge to the same state with no side effects. | MUST | envtest + e2e re-apply |
| REL-2 | Finalizers guarantee the MQ object is removed (or the failure surfaced) before the CR is deleted. | MUST | envtest deletion test |
| REL-3 | Leader election ensures exactly one active reconciler; standby replicas fail over automatically. | MUST | e2e / manual failover |
| REL-4 | Transient MQ/network errors are retried with backoff; terminal errors stop hot-looping and surface via status. | MUST | Adapter + reconciler unit tests |
| REL-5 | The operator recovers from API-server disconnects (cache re-sync) without manual intervention. | SHOULD | Provided by controller-runtime; smoke test |
| REL-6 | Graceful shutdown drains in-flight reconciles within the termination grace period. | SHOULD | Manual / e2e |

## 3. Observability

| ID | Req | Priority | Verification |
|----|-----|----------|--------------|
| OBS-1 | Every managed resource exposes machine-readable status `conditions` (e.g. `Ready`, `Synced`) plus `observedGeneration`. | MUST | envtest assertions |
| OBS-2 | Lifecycle transitions and terminal failures surface as Kubernetes `Events` with actionable reasons/messages; transient retries do not emit Events. | SHOULD | envtest / unit (`events_test.go`, `queue_reconciler_test.go`) |
| OBS-3 | Prometheus metrics are exposed (controller-runtime defaults + custom MQ counters/latency histograms) on a protected endpoint. | MUST | `/metrics` scrape test |
| OBS-4 | Structured (JSON-capable) logging with per-object context and configurable level. | MUST | Manual / unit |
| OBS-5 | A `ServiceMonitor` (and optionally a starter Grafana dashboard) ships for the local monitoring stack. | MAY | Local stack |

## 4. Performance & scalability

| ID | Req | Priority | Verification |
|----|-----|----------|--------------|
| PERF-1 | A steady-state reconcile (no drift) completes well under 1s, dominated by mqweb round-trips, not operator overhead. | SHOULD | Benchmark / e2e timing |
| PERF-2 | HTTPS clients are pooled/reused per `QueueManagerConnection`; no per-reconcile TLS handshake. | MUST | Adapter test |
| PERF-3 | Per-controller `MaxConcurrentReconciles` is configurable to scale with object count without overwhelming mqweb. | SHOULD | Flag test |
| PERF-4 | The operator handles hundreds of `Queue` objects per connection without unbounded memory/CPU growth. | SHOULD | Soak/e2e (later) |
| PERF-5 | Idle resource footprint is modest (sensible CPU/memory requests/limits in the shipped manager manifest). | SHOULD | Manifest review |

## 5. Maintainability & quality

| ID | Req | Priority | Verification |
|----|-----|----------|--------------|
| MNT-1 | Reconcilers stay **thin**; all external I/O sits behind the `MQAdmin` port. | MUST | Review |
| MNT-2 | High unit/envtest coverage on `internal/`; coverage regressions are investigated. | MUST | CI coverage report |
| MNT-3 | All generated artifacts (CRDs, RBAC, deepcopy, mocks) are committed and verified fresh (`task verify`). | MUST | pre-commit + CI |
| MNT-4 | Lint and formatting are clean (golangci-lint v2, `gofmt`/`goimports`/`golines`). | MUST | CI |
| MNT-5 | Significant decisions are recorded as ADRs in [docs/adr/](adr/). | SHOULD | Review |
| MNT-6 | Dependencies are kept current via a bot and bumped deliberately. | SHOULD | Renovate/Dependabot PRs |

## 6. Compatibility & API stability

| ID | Req | Priority | Verification |
|----|-----|----------|--------------|
| API-1 | CRDs follow Kubernetes API conventions; the `v1alpha1` contract may change but changes are deliberate and documented. | MUST | Review |
| API-2 | OpenAPI validation (kubebuilder markers) rejects invalid specs at admission time. | MUST | OpenAPI + validating webhooks; envtest admission tests |
| API-3 | Supported Kubernetes versions are stated and tested (envtest pins an API version; e2e on kind). | SHOULD | CI matrix (later) |
| API-4 | Supported IBM MQ / mqweb REST version is stated (target REST `v3`); adapter degrades clearly on unsupported versions. | SHOULD | Adapter test |

## 7. Operability & portability

| ID | Req | Priority | Verification |
|----|-----|----------|--------------|
| OPS-1 | Install/upgrade/uninstall via Kustomize (and optionally Helm) is documented and reproducible. | MUST | `task deploy/undeploy` |
| OPS-2 | The image is multi-arch-capable (at least `amd64`/`arm64`) and CGO-free for portability. | SHOULD | CI build |
| OPS-3 | A complete local environment is reproducible from scratch via `hack/kind-cluster`. | MUST | `task cluster:up` |
| OPS-4 | Configuration is via flags/env; no rebuild needed to change runtime behaviour (log level, concurrency, addresses). | SHOULD | Flag test |

## Verification summary

NFRs are enforced continuously, not audited occasionally:

- **Every PR**: lint, format, codegen `verify`, unit + envtest (`-race`,
  coverage), build, `govulncheck`.
- **e2e job**: reliability/idempotency/finalizer behaviour against live MQ.
- **Periodic**: `govulncheck` and image scanning catch newly disclosed CVEs.
- **Release**: nonroot/distroless image, pinned actions, published manifests.

See [CICD.md](CICD.md) for how each check is wired.
