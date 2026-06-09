# ADR-0022: Deletion and adoption policies for MQ object CRs

- **Status**: Accepted
- **Date**: 2026-06-09
- **Extends**: [ADR-0013](0013-finalizers-and-deletion.md) (finalizers and deletion)

## Context

Two lifecycle gaps surfaced in the 2026-06-09 audits:

1. **Deletion can brick CRs.** [ADR-0013](0013-finalizers-and-deletion.md)
   removes the finalizer only after the MQ object is deleted. Every workload
   reconciler resolves the `QueueManagerConnection`, waits for `Ready`, and
   builds the MQ client **before** checking `DeletionTimestamp`
   (EC-P0-01). If the Queue Manager is decommissioned, permanently
   unreachable, or the QMC/credentials `Secret` is deleted first (typical in
   namespace teardown), the finalizer is never removed: the CR — and its
   namespace — is stuck `Terminating` forever, with terminal errors that never
   requeue. There is no supported escape hatch short of manual finalizer
   surgery.
2. **Adoption is implicit and destructive.** `DEFINE … REPLACE` silently
   adopts **and overwrites** any pre-existing MQ object of the same name. For
   the brownfield queue managers this operator explicitly targets
   ([ADR-0012](0012-operator-scope-existing-queue-manager.md)), applying a CR
   that happens to collide with an existing production queue rewrites its
   attributes without warning.

## Decision

### Deletion policy

Add `spec.deletionPolicy` to all five workload kinds (`Queue`, `Topic`,
`Channel`, `ChannelAuthRule`, `AuthorityRecord`):

- `Delete` (default — current behaviour): the finalizer deletes the MQ object
  before the CR is released.
- `Orphan`: the finalizer is removed without touching MQ; the MQ object is
  left in place. Status records `Synced=False / Orphaned` on the way out and a
  Normal Event is emitted ([ADR-0015](0015-kubernetes-events-on-transitions.md)).

Additionally:

- **Force-orphan escape hatch**: the annotation
  `messaging.mkurator.dev/force-orphan: "true"` causes the next reconcile of a
  deleting CR to skip MQ cleanup and remove the finalizer, regardless of
  `deletionPolicy`. This is the documented break-glass path for "the QM is
  never coming back", replacing manual finalizer patching.
- **Deletion-path ordering fix**: reconcilers must evaluate
  `DeletionTimestamp` (and the orphan paths above) **before** requiring a
  ready connection, so orphan-deletes succeed with no QMC/Secret present. A
  `Delete`-policy CR whose connection chain is broken keeps requeueing with a
  clear condition instead of failing terminally with no requeue.

### Adoption policy

Add `spec.adoptionPolicy` to the same kinds, governing the first reconcile
when the MQ object **already exists**:

- `Adopt` (default — current behaviour, now explicit): take ownership and
  reconcile attributes (`DEFINE … REPLACE` on drift).
- `AdoptIfMatching`: take ownership only if all drift-checked attributes
  already match the spec; otherwise set `Synced=False / AdoptionConflict` and
  do not write to MQ.
- `FailIfExists`: never adopt; if the object pre-exists, set
  `Synced=False / AlreadyExists` and do not write. For users who want CRs to
  only ever create.

Defaults preserve today's semantics, so this is **non-breaking** for existing
CRs; CRD defaulting makes the previously implicit behaviour visible in specs.

## Consequences

- Namespace teardown and QM decommissioning become safe, documented flows
  (INSTALL_AND_USE troubleshooting must document both policies and the
  force-orphan annotation).
- Brownfield users get a guard against accidental overwrite of pre-existing
  objects; GitOps users can express Retain-like semantics (`Orphan`).
- Webhook/CEL validation: `deletionPolicy`/`adoptionPolicy` are simple enums
  (CEL-friendly, [ADR-0025](0025-cel-first-admission-validation.md)).
- Envtest locks required (audit T1/T2): delete-after-Secret-gone,
  delete-while-QMC-not-ready, force-orphan, and adoption-conflict paths.
- The `Orphaned`/`AdoptionConflict`/`AlreadyExists` reasons extend the
  condition vocabulary; events fire on these transitions only.
- Crossplane's `deletionPolicy` and ACK's adoption annotations are prior art;
  we follow their naming where it fits.

## Alternatives considered

- **Keep ADR-0013 as-is, document manual finalizer removal**: pushes a routine
  operational situation onto kubectl surgery; rejected.
- **Auto-orphan after N failed deletion attempts**: implicit data-loss-ish
  behaviour, hard to reason about; rejected in favour of explicit policy +
  annotation.
- **Annotation-only (no spec fields)**: annotations are not validated,
  not defaulted, and invisible to `kubectl explain`; spec fields with CRD
  defaults are the API-conventions-correct shape. Annotation retained only for
  the break-glass case where spec edits may be undesirable mid-deletion.

## References

- [ADR-0013](0013-finalizers-and-deletion.md) — base deletion flow (extended, not superseded)
- [ADR-0012](0012-operator-scope-existing-queue-manager.md) — brownfield scope
- Edge-case audit 2026-06-09: EC-P0-01, EC-P0-02 (internal)
- [Crossplane deletionPolicy](https://docs.crossplane.io/latest/concepts/managed-resources/#deletionpolicy)
