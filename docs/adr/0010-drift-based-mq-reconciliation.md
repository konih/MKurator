# ADR-0010: Drift-based MQ reconciliation (DEFINE + DISPLAY)

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

The operator must keep IBM MQ administrative objects aligned with CR
`spec.attributes` without treating every reconcile as a blind `DEFINE`. IBM MQ
and mqweb expose both **DEFINE** (apply desired state) and **DISPLAY** (read
observed state). Some attributes can be set but not read back reliably on all
queue manager versions (e.g. `maxmsglen` on DISPLAY for mqweb 9.4.x).

Users also need a clear **`Synced`** condition: when is the CR “done”, and what
happens if someone changes MQ manually?

## Decision

We will reconcile MQ objects using a **drift-check-then-define** loop:

1. **DISPLAY** selected attributes via mqweb (per-object safe parameter lists in
   `internal/adapter/mqrest/mqsc_params.go`).
2. **Compare** desired `spec.attributes` to observed values with
   `internal/mqadmin/attrmatch.go` (`AttributeValueMatches`).
3. **DEFINE … REPLACE** only when the object is missing or a **drift-checked**
   key differs.
4. Set **`Synced=True`** when the object exists and every desired key we can
   observe matches.

Rules:

- **DEFINE** forwards any lowercase key in `spec.attributes` (with normalizations
  documented in [../ATTRIBUTE_RECONCILIATION.md](../ATTRIBUTE_RECONCILIATION.md)).
- **Drift** applies only to keys in the DISPLAY safe list for that object/type.
- **Define-only keys** are applied on create/update but **not** verified on later
  reconciles — manual MQ edits to those fields are invisible until a
  drift-checked key changes.

Shipped types: `QLOCAL` / `QALIAS` / `QREMOTE`, `TOPIC`, `CHLTYPE(SVRCONN)`.

## Consequences

- Reduces unnecessary MQSC churn and clarifies when the operator will re-apply.
- `Synced=False` with a drift reason is actionable; typos in unknown keys still
  fail at MQ apply time (warnings at admission where implemented).
- DISPLAY lists must be maintained per mqweb/MQ version; gaps are documented in
  ATTRIBUTE_RECONCILIATION (not hidden).
- Full “desired state enforcement” for define-only attributes would require
  expanding DISPLAY coverage or accepting periodic blind DEFINE — out of scope
  unless we change this ADR.

## Alternatives considered

- **Always DEFINE on every reconcile**: simple but noisy; hides whether MQ
  already matched. Rejected.
- **Observe-only / no drift**: cannot detect manual MQ changes. Rejected.
- **Native REST GET per object instead of MQSC DISPLAY**: uneven coverage across
  object types; we standardise on MQSC via [ADR-0002](0002-manage-mq-via-mqweb-rest.md).

## References

- [ATTRIBUTE_RECONCILIATION.md](../ATTRIBUTE_RECONCILIATION.md) — per-attribute matrix
- [IBM_MQ_OBJECTS.md](../IBM_MQ_OBJECTS.md) — MQSC research (not the product contract)
