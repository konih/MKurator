# ADR-0021: Attribute API shape — typed fields with an attributes escape hatch

- **Status**: Accepted
- **Date**: 2026-06-09

## Context

Since Phase 2, every MQ object CR carries its MQSC parameters in a free-form
`spec.attributes map[string]string` (lowercase mqweb `runCommandJSON` keys).
That choice was never recorded in an ADR, although it is the most consequential
API decision in the project:

- `kubectl explain queue.spec.attributes` conveys nothing; the schema contract
  CRDs exist to provide is delegated to
  [ATTRIBUTE_RECONCILIATION.md](../ATTRIBUTE_RECONCILIATION.md).
- Typos and invalid values pass OpenAPI validation and fail only at MQ apply
  time (admission warnings exist for *unknown* keys, not invalid values).
- Drift policy (DISPLAY safe lists in `mqsc_params.go`), the documentation
  matrix, and admission warnings are three parallel bookkeeping systems that
  must agree.
- CEL validation (`x-kubernetes-validations`, [ADR-0025](0025-cel-first-admission-validation.md))
  cannot express per-attribute rules against opaque map keys.
- A later `v1beta1` that introduces typed fields needs a conversion webhook;
  the longer the map is the only surface, the more painful that migration.

The map also has real strengths: MQSC has hundreds of attributes across object
types, IBM's DISPLAY behaviour varies per mqweb version, and the passthrough
made Phases 2–5 shippable by a single maintainer without chasing a typed-field
treadmill.

## Decision

We adopt a **hybrid attribute surface**, to be implemented **before any
`v1beta1` graduation**:

1. **Promote the commonly used, drift-checked attributes to typed, documented,
   validated spec fields** per kind — on the order of 10–15 fields each, drawn
   from the drift-checked columns of the attribute matrix (for `Queue` e.g.
   `maxDepth`, `description`, `defPersistence`, `defPriority`, `get`/`put`
   enablement, alias/remote targets; analogous sets for `Topic` and `Channel`).
   Typed fields get OpenAPI types, enums, bounds, and CEL rules, and surface in
   `kubectl explain`.
2. **Keep `spec.attributes` as the documented escape hatch** for the long tail
   of MQSC parameters, with unchanged passthrough semantics.
3. **Precedence rule:** a typed field and its `attributes` key are mutually
   exclusive; admission (CEL where expressible) rejects specs that set both
   forms of the same parameter. No silent merging.
4. **Internally**, typed fields are folded into the same attribute map before
   the `mqadmin` port, so the adapter, drift logic, and DISPLAY lists are
   unaffected by where a value was declared.
5. Until the hybrid lands, the map remains the supported surface; this ADR
   retroactively records that decision and its trade-offs.

## Consequences

- New adopters get a discoverable, validated API for the 90% case without
  losing full MQSC reach for the 10%.
- The attribute matrix shrinks to "escape-hatch" documentation; typed fields
  are self-documenting via CRD descriptions.
- Implementation cost: API types, conversion of folded attributes, webhook/CEL
  exclusivity checks, schema goldens, docs — tracked as a Phase 7 roadmap item
  ([ROADMAP.md](../ROADMAP.md)).
- `v1alpha1` keeps both surfaces; `v1beta1` graduation can then deprecate map
  keys that have typed equivalents without breaking existing CRs.
- Risk: field selection bikeshedding. Mitigation: promote exactly the
  drift-checked keys of the matrix first; everything else stays in the map.

## Alternatives considered

- **Keep the map only**: zero migration cost, but permanently weak validation,
  poor discoverability, and no CEL leverage. Rejected as the end state; it
  remains the interim state.
- **Fully typed spec (no map)**: maximal validation, but chases the full MQSC
  attribute surface across mqweb versions forever and blocks users from
  attributes we have not modelled. Rejected.
- **Typed fields that mirror into the map at admission (mutating webhook)**:
  the project deliberately ships no mutating webhooks ([ADR-0009](0009-validating-admission-webhooks.md)).
  Rejected.

## References

- [ATTRIBUTE_RECONCILIATION.md](../ATTRIBUTE_RECONCILIATION.md) — current matrix
- [ADR-0010](0010-drift-based-mq-reconciliation.md) — drift-checked vs define-only keys
- [ADR-0025](0025-cel-first-admission-validation.md) — CEL validation strategy
- Critical design review 2026-06-09 §2.1 (internal)
