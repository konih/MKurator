# ADR-0015: Kubernetes Events on status transitions only

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

Status conditions on CRs are the source of truth for automation, but operators
often debug with `kubectl describe`, which surfaces **Events**. Emitting an Event
on every reconcile would flood the namespace when MQ is flaky or credentials are
wrong during retry backoff.

We want audit-friendly signals without duplicating condition noise.

## Decision

We will emit Kubernetes **Events only on transitions** (not every reconcile),
implemented in `internal/controller/events.go` and shared helpers:

| Transition | Event type | Typical reason |
|------------|------------|----------------|
| Connection `Ready` → True | Normal | `Available` |
| MQ object `Synced` → True | Normal | `Available` |
| `Synced` → False (blocked on connection) | Normal | `Progressing` |
| Deletion started | Normal | `Deleting` |
| MQ object removed | Normal | `Deleted` |
| Terminal / config failure | Warning | Classified (`MQSCError`, `ConnectionNotFound`, …) |
| **Transient** MQ/network error | **None** | Status updated; rely on conditions + logs |

Warning reasons derive from typed port errors ([ADR-0014](0014-mq-error-taxonomy-and-requeue.md))
and Kubernetes `NotFound` on connections/secrets.

RBAC: operator may `create`/`patch` Events (see generated ClusterRole).

## Consequences

- `kubectl describe queue` shows a concise timeline; retry storms stay quiet.
- Debugging transient issues uses logs (structured, [ADR-0007](0007-structured-logging-logr-slog.md))
  and conditions, not Events.
- New status transitions should consider whether an Event helps operators; do
  not add per-reconcile Events without revisiting this ADR.

## Alternatives considered

- **Event on every error return**: noisy under backoff. Rejected.
- **Events only, no conditions**: poor for GitOps/status UIs. Rejected.
- **No Events**: weaker `kubectl describe` UX. Rejected.

## References

- [ARCHITECTURE.md](../ARCHITECTURE.md#event-emission)
