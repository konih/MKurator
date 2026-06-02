# ADR-0014: MQ error taxonomy and requeue strategy

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

Reconcilers must react differently to “bad MQSC syntax”, “QM down”, and “object
not found”. String-matching HTTP bodies in controllers does not scale and breaks
when mqweb wording changes. controller-runtime already provides rate-limited
requeues; we must not hot-loop on permanent failures.

MQ interaction is behind the `MQAdmin` port ([ADR-0002](0002-manage-mq-via-mqweb-rest.md)).

## Decision

We will classify errors at the **`internal/mqadmin` port boundary** (implemented
in `mqrest`, consumed by controllers):

| Class | Types / signals | Controller behaviour |
|-------|-----------------|----------------------|
| **Terminal** | `*TerminalError` (`ErrTerminal`), invalid MQSC, 4xx auth/validation | Failing status condition; Warning Event ([ADR-0015](0015-kubernetes-events-on-transitions.md)); return **without** unbounded requeue |
| **Transient** | 5xx, timeouts, QM unavailable (503) | Return error to trigger controller-runtime **backoff requeue** |
| **NotFound** | `*NotFoundError` (`ErrNotFound`) | Ensure: treat as create needed; Delete: treat as already gone |

Principles:

- Wrap with context: `fmt.Errorf("define queue: %w", err)`.
- Inspect with `errors.Is` / `errors.As` only — no substring checks in reconcilers.
- Never panic in reconcile.
- **Workload reconcilers** register a **watch** on `QueueManagerConnection` status
  so queues/topics/channels requeue when a connection becomes `Ready` instead of
  relying solely on periodic requeue.

`RunMQSC` on the REST client is for fixtures/e2e and future work — not part of
`Admin`; reconcilers use typed port methods only.

## Consequences

- New mqweb failure modes map to port errors in one place (`mqrest`).
- Terminal misconfiguration surfaces once with a stable `Reason` on status/Events.
- Transient outages self-heal without manual CR edits when MQ returns.
- Tests assert classification via mock `Admin` errors and adapter unit tests.

## Alternatives considered

- **Classify in each reconciler**: duplicated logic. Rejected.
- **Always requeue forever**: hides terminal MQSC mistakes. Rejected.
- **Custom workqueue rate limiter per CR**: unnecessary; controller-runtime defaults
  suffice with terminal vs transient returns.

## References

- `internal/mqadmin/admin.go` — `TerminalError`, `NotFoundError`
- [ARCHITECTURE.md](../ARCHITECTURE.md#error-handling--requeue-strategy)
