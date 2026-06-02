# ADR-0013: Finalizers and deletion (MQ object before CR removal)

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

When a user deletes a `Queue`, `Topic`, `Channel`, or `QueueManagerConnection`,
Kubernetes removes the CR immediately unless something blocks removal. If the CR
vanishes while the object still exists on IBM MQ, we leak MQ resources and break
the declarative “CR is source of truth” model.

`QueueManagerConnection` deletion is special: workloads reference it, and
removing the connection while MQ objects remain can strand admin state.

## Decision

We will use **controller-runtime finalizers** on shipped reconciled types:

| Resource | Finalizer purpose |
|----------|-------------------|
| `Queue`, `Topic`, `Channel` | On delete: remove the MQ object via `MQAdmin` delete, then remove finalizer |
| `QueueManagerConnection` | On delete: release pooled mqweb client (`ReleaseConnection`), then remove finalizer |

Behaviour:

- **Add finalizer** on first reconcile of a non-deleting object.
- **Deletion path**: set `Synced` / status to deleting where applicable; call MQ
  delete (or connection cleanup); emit Normal `Deleting` / `Deleted` Events per
  [ADR-0015](0015-kubernetes-events-on-transitions.md); remove finalizer only after
  successful cleanup (or idempotent not-found on MQ).
- **Failed MQ delete**: finalizer remains; error classified per
  [ADR-0014](0014-mq-error-taxonomy-and-requeue.md); user retries via reconcile.
- **QMC delete with dependents**: blocked at **admission** ([ADR-0009](0009-validating-admission-webhooks.md));
  reconcile does not need to implement orphan MQ sweeps for connection delete.

We do not implement Kubernetes `ownerReferences` garbage-collection from QMC to
workloads; explicit `connectionRef` and admission rules enforce ordering.

## Consequences

- `kubectl delete` on a Queue triggers MQ `DELETE` (MQSC) before the CR disappears.
- Stuck terminating CRs indicate MQ or permission failures — inspect status and Events.
- Finalizer RBAC (`*/finalizers` update) is required on the operator ClusterRole.
- Connection delete does not auto-delete all queues on that QM; users delete
  workload CRs first (enforced for QMC delete at admission).

## Alternatives considered

- **No finalizer — orphan MQ objects**: violates cleanup expectation. Rejected.
- **Foreground cascade delete all MQ objects when QMC deleted**: dangerous and
  surprising; admission block preferred.
- **Owner references from QMC to Queue/Topic/Channel**: implicit cascade on QMC
  delete; rejected in favour of explicit user order + webhook deny.

## References

- Controllers: `internal/controller/*_controller.go`
- [ARCHITECTURE.md](../ARCHITECTURE.md)
