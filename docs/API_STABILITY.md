# API stability

This document states what the **`messaging.mkurator.dev/v1alpha1`** API guarantees
today, how it may evolve before **`v1beta1`**, and what graduation requires.
It satisfies Phase 8b in [ROADMAP.md](ROADMAP.md) and NFR **API-1** in
[NON_FUNCTIONAL_REQUIREMENTS.md](NON_FUNCTIONAL_REQUIREMENTS.md).

## Current version

| Item | Value |
| --- | --- |
| API group | `messaging.mkurator.dev` |
| Version | **`v1alpha1`** (all six kinds) |
| Stability (Kubernetes meaning) | **Alpha** — no compatibility promise across minor releases |
| MQ parameter surface | `spec.attributes map[string]string` (primary today) |
| Admission | CRD CEL (`x-kubernetes-validations`) + validating webhooks ([ADR-0025](adr/0025-cel-first-admission-validation.md)) |
| Webhooks | Validating only; no mutating or conversion webhooks ([ADR-0009](adr/0009-validating-admission-webhooks.md)) |

Kinds: `QueueManagerConnection`, `Queue`, `Topic`, `Channel`, `ChannelAuthRule`,
`AuthorityRecord`.

## What `v1alpha1` guarantees

Between tagged releases on `main`, the project aims for **deliberate, documented**
changes only:

1. **Reconcile semantics** for fields documented in
   [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md) and kind-specific
   guides — drift-checked keys are corrected on the queue manager; define-only
   keys are applied on create/update but not compared on DISPLAY.
2. **OpenAPI validation** on the CRD schema (enums, patterns, CEL rules) rejects
   structurally invalid specs at admission time when the API server or webhook is
   available.
3. **Breaking changes** are called out in commit messages (`!` or
   `BREAKING CHANGE:`), [CHANGELOG.md](../CHANGELOG.md), and [UPGRADE.md](UPGRADE.md)
   before a release tag ([CONTRIBUTING.md](CONTRIBUTING.md#breaking-changes),
   [GOVERNANCE.md](../GOVERNANCE.md)).
4. **Status shape** (`conditions`, `observedGeneration`, `desiredMQSC` where
   present) remains the observability contract; new condition reasons may appear
   but existing `Synced` / `Ready` semantics are not removed without a breaking
   release.

## What `v1alpha1` does *not* guarantee

- **Field-level stability** — names, types, and requiredness of spec fields may
  change until `v1beta1`.
- **Map-only MQ parameters forever** — [ADR-0021](adr/0021-attribute-api-shape.md)
  adds typed spec fields alongside `spec.attributes`; that work may introduce new
  fields and exclusivity rules without a new API version while still on
  `v1alpha1`.
- **Silent compatibility** — typos in `spec.attributes` keys are not caught by
  OpenAPI; unknown keys may receive admission **warnings** but still apply if MQ
  accepts them.
- **Webhook availability as a hard dependency for basic validation** — stateless
  rules live in CEL; referential checks (`connectionRef`, cross-CR references)
  require the validating webhook ([ADR-0025](adr/0025-cel-first-admission-validation.md)).

## Planned maturation (Phase 8)

Before **`v1beta1` graduation**, `v1alpha1` will gain ([ROADMAP.md](ROADMAP.md#phase-8--api-maturation-v1beta1-readiness)):

| Track | Deliverable | ADR |
| --- | --- | --- |
| **8a** | Typed fields for drift-checked MQ attributes + `spec.attributes` escape hatch; mutual exclusivity (CEL); internal fold into the attribute map before `mqadmin` | [ADR-0021](adr/0021-attribute-api-shape.md) |
| **8b** | This stability statement (published) | — |
| **8c** | Optional DISPLAY capability probing | [ADR-0024](adr/0024-mqsc-command-construction-hygiene.md) §4 |
| **8d** | `v1beta1` for all six kinds + conversion webhook | [ADR-0009](adr/0009-validating-admission-webhooks.md) |

During **8a**, existing manifests that use only `spec.attributes` remain valid.
New typed fields are optional; setting both a typed field and the same key in
`attributes` is rejected at admission (no silent merge). The first promoted field
is `Queue.spec.maxDepth` (alternative to `attributes.maxdepth`).

## Graduation to `v1beta1` (future)

`v1beta1` will **not** be cut until:

1. **Hybrid attribute surface (8a)** has shipped on `v1alpha1` and baked for at
   least **one minor release** without schema churn on promoted fields.
2. A **conversion webhook** converts stored objects between `v1alpha1` and
   `v1beta1` for all six kinds (today there is none).
3. **Deprecation policy** for map keys that have typed equivalents is documented
   in UPGRADE.md (map key still accepted via conversion during deprecation;
   removal only in a later version with notice).
4. CI proves conversion + reconcile parity (envtest/e2e), and [UPGRADE.md](UPGRADE.md)
   documents migration paths.

Until then, consumers should pin the operator and CRD bundle to a **release tag**
and read CHANGELOG/UPGRADE before upgrading.

## Deprecation policy (when `v1beta1` exists)

When a drift-checked attribute gains a typed spec field:

1. **Prefer the typed field** in new manifests (`kubectl explain` documents it).
2. **`spec.attributes["<key>"]` is deprecated** for that parameter in `v1beta1`
   (admission warning, then rejection in a later release — exact timeline in
   UPGRADE.md at graduation time).
3. **Conversion** copies map values into typed fields where unambiguous so
   existing GitOps repos keep working through one upgrade cycle.

No deprecations of map keys are active while the project ships **map-only**
`v1alpha1` CRs.

## Environment prerequisites

| Dependency | Supported / required |
| --- | --- |
| Kubernetes | **1.29+** for CRD CEL validation ([INSTALL_AND_USE.md](INSTALL_AND_USE.md)) |
| IBM MQ / mqweb | Administrative REST **v3**; adapter behaviour documented in [IBM_MQ_REST_API.md](IBM_MQ_REST_API.md) |

## Related documents

| Document | Role |
| --- | --- |
| [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md) | Drift-checked vs define-only MQ keys (today's contract) |
| [adr/0021-attribute-api-shape.md](adr/0021-attribute-api-shape.md) | Typed fields + escape hatch decision |
| [adr/0025-cel-first-admission-validation.md](adr/0025-cel-first-admission-validation.md) | CEL vs webhook split |
| [UPGRADE.md](UPGRADE.md) | Release-to-release migration steps |
| [FAQ.md](FAQ.md) | Short pointers for operators |
