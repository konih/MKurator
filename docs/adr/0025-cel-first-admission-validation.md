# ADR-0025: CEL-first admission validation; webhooks for stateful checks only

- **Status**: Accepted
- **Date**: 2026-06-09
- **Extends**: [ADR-0009](0009-validating-admission-webhooks.md) (validating webhooks)

## Context

[ADR-0009](0009-validating-admission-webhooks.md) put all admission validation
in validating webhooks with `failurePolicy: Fail`. Most rules in
`internal/validation` are **stateless** — name constraints, enums, cross-field
requirements within a single object (alias/remote required attributes,
`svrconn`-only channels). Only a minority need cluster state: `connectionRef`
existence, Channel↔ChannelAuthRule referential checks, and the
QueueManagerConnection delete guard.

Consequences of the current shape:

- The webhook is an **availability SPOF**: with `failurePolicy: Fail`, a
  webhook outage rejects every write to all six kinds, cluster-wide.
- **cert-manager is a hard install prerequisite** for serving certs — the
  single largest install friction for new adopters (docs audit DOC-03), and
  it buys nothing for the stateless majority of the rules.
- Stateless rules in webhook code are invisible to `kubectl explain`, dry-run
  server-side validation in older tooling, and GitOps diff previews.

Kubernetes ≥1.25 ships `x-kubernetes-validations` (CEL) on CRD schemas:
stateless rules evaluated by the API server itself, with no operator pod, no
TLS, and no failure mode beyond the API server's own.

## Decision

1. **Stateless validation moves to CRD CEL** (`+kubebuilder:validation:XValidation`
   markers on `api/v1alpha1` types): MQ name constraints, enum tightening
   ([ADR-0024](0024-mqsc-command-construction-hygiene.md) auth fields),
   queue-type-conditional required attributes, policy enums
   ([ADR-0022](0022-deletion-and-adoption-policy.md)), and the
   typed-field/attributes exclusivity rule ([ADR-0021](0021-attribute-api-shape.md)).
   `internal/validation` keeps mirror implementations only where a rule cannot
   be expressed in CEL; everything expressible is deleted from webhook code
   once its CEL twin is golden-tested.
2. **Webhooks shrink to stateful checks**: `connectionRef` exists / same
   namespace / not deleting, Channel↔CHLAUTH references, QMC delete guard,
   and unknown-attribute warnings (warnings are not expressible in CEL).
   `failurePolicy: Fail` is retained for this reduced surface — these checks
   guard referential integrity and are worth the stricter posture, but the
   blast radius of an outage no longer covers basic field validation.
3. **Schema goldens assert the CEL rules** (`test/schema/`): each migrated
   rule lands with a golden-fragment update plus envtest proving acceptance/
   rejection parity with the webhook rule it replaces.
4. **Install modes**: with stateless rules in CEL, document
   `webhooks.enabled=false` as a supported degraded mode (field validation
   intact, referential checks deferred to reconcile-time conditions) — making
   cert-manager an optional rather than hard prerequisite. The default
   install keeps webhooks on.

## Consequences

- Webhook outage no longer blocks routine CR edits; only referential
  enforcement degrades.
- New-adopter install on a vanilla cluster works without cert-manager (with
  the documented trade-off), removing the top install friction.
- Validation behaviour becomes visible in the CRD schema (`kubectl explain`,
  server-side dry-run, GitOps preview).
- Migration discipline required: every moved rule needs CEL + golden +
  envtest parity before the webhook copy is deleted; no window with neither.
- CEL has budget/complexity limits per rule; the rare over-budget rule stays
  in the webhook (documented per case).
- Minimum Kubernetes version rises to one where CEL validation is GA (≥1.29
  for all features used); record in INSTALL prerequisites.

## Alternatives considered

- **Status quo (webhook-only)**: keeps one validation engine but retains the
  SPOF and the cert-manager wall. Rejected.
- **`failurePolicy: Ignore` for the existing webhook**: removes the SPOF but
  silently drops *all* validation under outage, including referential checks.
  Rejected; CEL keeps stateless guarantees API-server-side instead.
- **ValidatingAdmissionPolicy (CEL with parameters) for stateful checks too**:
  could eventually replace the webhook entirely, but cross-object lookups
  (Secret/QMC existence) still need informers; revisit when the project's
  minimum Kubernetes version makes VAP + variables ubiquitous.

## References

- [ADR-0009](0009-validating-admission-webhooks.md) — webhook baseline (narrowed, not superseded)
- [Kubernetes CEL validation](https://kubernetes.io/docs/reference/using-api/cel/)
- Docs audit 2026-06-09: DOC-03 (cert-manager prerequisite); improvement brief MKR-04/MKR-11 (internal)
