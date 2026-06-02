# ADR-0011: Layered testing strategy (mock → envtest → MQ → kind)

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

The operator touches Kubernetes APIs, HTTPS mqweb, and a real Queue Manager in
production. A single `go test ./...` cannot cover all of that without becoming
slow, flaky, and impossible to run in every environment. We need tiers that
trade fidelity for speed and hermeticism, aligned with [ADR-0005](0005-keep-tooling-lean.md)
and CI parity via Task ([ADR-0004](0004-task-as-task-runner.md)).

## Decision

We will use **four test tiers**, each with explicit gates:

| Tier | Scope | MQ / cluster | How to run |
|------|--------|--------------|------------|
| **Unit** | Reconcilers, `mqrest` adapter, validation, webhooks (table/unit) | Mock `mqadmin.Admin` (`mockery`); `httptest` for REST | `task test:run` (default) |
| **envtest** | Controller + API server + admission webhooks | Mock `Admin`; CRDs from `config/` | `task test:run` (Ginkgo suites) |
| **Integration** | `mqrest` CRUD against live mqweb | Docker IBM MQ (`hack/mq-docker`) | `task test:integration` — build tag `integration`, env `KURATOR_INTEGRATION_MQ=1` |
| **e2e** | Operator on kind + real QM1 scenarios | `hack/kind-cluster` + `KURATOR_E2E_MQ=1` | `task test:e2e` / `task ci:e2e` — build tag `e2e` |

Additional rules:

- **Race detector** on default CI unit/envtest: `-race`.
- **Ginkgo + Gomega** for controller/envtest/e2e suites; stdlib `testing` allowed
  where simpler (e.g. integration packages).
- **Coverage floor** on `internal/` enforced in CI (see `Taskfile.test.yml`).
- **No real MQ** in unit or envtest — keeps PR feedback fast and deterministic.
- **e2e and integration** are opt-in locally; CI runs them on PRs/main per
  [../CICD.md](../CICD.md).

A change touching reconcile behaviour needs the **lowest tier that exercises it**;
MQ adapter changes should include integration; cross-cutting install paths need
e2e when feasible.

## Consequences

- Contributors run `task test:run` often; MQ tiers only when touching adapter/MQ
  paths or before release.
- Build tags prevent accidental e2e runs in IDE “test package”.
- Maintaining mocks (`task test:generate` / mockery) is mandatory when the
  `MQAdmin` port changes.
- Flaky e2e/MQ tests are investigated; not ignored — but scoped to their tier.

## Alternatives considered

- **e2e only, no mocks**: slow PRs, hard onboarding. Rejected.
- **Unit tests hitting shared QM**: shared state, flaky parallel runs. Rejected.
- **Testcontainers for all MQ tests**: extra dependency; Docker Compose path is
  enough for integration today.

## References

- [DEVELOPMENT.md](../DEVELOPMENT.md#test-tiers)
- [CICD.md](../CICD.md)
