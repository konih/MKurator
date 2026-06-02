# ADR-0009: Validating admission webhooks (no MQ at admission)

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

Kurator CR specs can be invalid in ways OpenAPI markers alone do not catch:
wrong queue-type attribute combinations, missing `connectionRef` targets, or
deleting a `QueueManagerConnection` while workloads still reference it. Without
admission checks, users only see failures after reconcile (or confusing status).

We also need to avoid duplicating MQ connectivity checks in two places with
different semantics. Reachability, credentials, and MQSC correctness require
mqweb and belong in reconcile ([ADR-0002](0002-manage-mq-via-mqweb-rest.md),
[ADR-0003](0003-connection-model.md)).

Phase 4b delivered validating webhooks for all shipped v1alpha1 CRDs. See
[../plans/VALIDATING_WEBHOOKS.md](../plans/VALIDATING_WEBHOOKS.md) for the
implementation plan.

## Decision

We will run **validating** (non-mutating) admission webhooks for
`QueueManagerConnection`, `Queue`, `Topic`, and `Channel` with:

- **`failurePolicy: Fail`** — invalid manifests are rejected at `kubectl apply`.
- **Pure Kubernetes validation** — rules in `internal/validation` (pure
  functions); thin handlers in `internal/webhook/v1alpha1` using `client.Reader`
  only. **No `MQAdmin` / mqweb calls** in webhooks.
- **Referential checks** — same-namespace `connectionRef` and Secret refs exist;
  QMC delete blocked when Queue/Topic/Channel dependents exist; QMC not
  deleting when workloads reference it.
- **Cross-field rules** — queue alias/remote required attributes; channel
  `svrconn` only; MQ object naming constraints.
- **Unknown attribute keys** — `admission.Warnings` on Queue/Topic/Channel (not
  errors) for forward-compatible GitOps.
- **TLS for serving** — cert-manager `Certificate` + webhook Service; wired in
  Kustomize (`config/webhook`, `config/certmanager`) and Helm
  (`webhooks.enabled`, default `true`).

We will **not** add mutating or conversion webhooks in v1alpha1.

## Consequences

- Invalid samples fail fast with clear admission messages (NFR API-2).
- Webhook tests are hermetic: table-driven unit tests + envtest admission;
  optional e2e negative apply — no IBM MQ required for admission coverage.
- Operators must install cert-manager (or equivalent TLS) for webhooks on cluster;
  the kind dev stack already provisions it.
- Runtime readiness (`QueueManagerConnection` `Ready`, MQSC errors) remains
  reconcile-only; users must not expect admission to prove mqweb works.
- Phase 5 CRDs (AUTHREC/CHLAUTH) get their own validators when introduced.

## Alternatives considered

- **Reconcile-only validation**: simpler deploy, worse UX and noisier status.
  Rejected for shipped CRDs.
- **Mutating webhooks** (defaulting, labels): more moving parts; not needed for
  v1alpha1. Rejected.
- **Validate mqweb reachability at admission**: couples admission to external MQ,
  slows apply, duplicates QMC reconciler. Rejected.
- **CEL validation-only**: good for simple rules; cross-object lookups and
  warnings are clearer in Go. We use OpenAPI + Go validators together.

## References

- [ARCHITECTURE.md](../ARCHITECTURE.md) — webhook component row
- [INSTALL_AND_USE.md](../INSTALL_AND_USE.md) — install with webhooks enabled
