# ADR-0023: Connection client cache lifecycle and Secret handling

- **Status**: Accepted
- **Date**: 2026-06-09
- **Extends**: [ADR-0003](0003-connection-model.md) (connection model)

## Context

The mqrest `ClientFactory` ([ADR-0003](0003-connection-model.md), NFR PERF-2)
caches one client per `QueueManagerConnection`. The 2026-06-09 audits found
three defects in its lifecycle, all rooted in the cache key embedding the QMC
`generation` and the credentials/CA `Secret.resourceVersion`:

1. **Unbounded growth** (ARCH-03 / EC-P2-01): every QMC spec change or Secret
   rotation mints a new key; old entries — each owning an `http.Client` and
   idle TLS connections — are never evicted.
2. **Release deadlock** (ARCH-02 / EC-P0-02): `ReleaseConnection` recomputes
   the cache key, which performs a live `Get` of the credentials Secret. If
   the Secret was deleted first (namespace teardown), release fails forever
   and the QMC finalizer is never removed.
3. **Per-reconcile Secret reads**: `cacheKey` issues one or two Secret `Get`s
   on **every** reconcile of every dependent CR. With cluster-wide Secret
   RBAC and an unfiltered manager cache this also drags every Secret in the
   cluster into the informer cache (ARCH-05), contradicting the documented
   least-privilege posture.

Separately, the audits questioned whether `connectionRef` should stay
same-namespace-only (SEC-8) or grow cross-namespace/cluster-scoped reach.

## Decision

### Cache lifecycle

1. **Key the cache by stable identity** — `namespace/name` of the QMC only.
   Store the QMC `generation` and the Secret `resourceVersion`s *inside* the
   cached entry.
2. **Replace on mismatch**: on `ForConnection`, if the stored
   generation/resourceVersions differ from the current ones, build a new
   client, swap it in, and close the old transport's idle connections
   (`CloseIdleConnections`). The cache size is thus bounded by the number of
   live QMCs.
3. **Release must not require live Secrets**: `ReleaseConnection` evicts by
   `namespace/name` and **treats missing Secrets (and any NotFound) as
   success**. Deletion paths never depend on referenced objects still
   existing.

### Secret scoping

4. Keep reading Secrets through the manager client, but **filter the informer
   cache for Secrets** (`cache.ByObject` with a field/label scope, or a
   transform that strips data of unreferenced Secrets) so the operator does
   not cache every Secret in the cluster; align
   [ARCHITECTURE.md](../ARCHITECTURE.md)'s least-privilege claim with whatever
   scope ships. Pair with the Secret **watch** (reconcile on rotation) tracked
   in ROADMAP Phase 6 so rotation is event-driven rather than discovered via
   per-reconcile `Get`s.

### Namespace boundary

5. `connectionRef` stays **same-namespace-only**. This is a deliberate
   security boundary (SEC-8): namespace isolation governs who may use which
   credentials, and it keeps RBAC reasoning local. Cross-namespace or
   cluster-scoped connections are out of scope unless a future ADR introduces
   an explicit grant model (e.g. `allowedNamespaces` on the QMC). Multiple
   namespaces needing one QM each hold their own QMC + Secret by design.

## Consequences

- Cache memory is bounded; rotation closes stale transports instead of
  leaking them.
- QMC deletion succeeds regardless of Secret deletion order — unblocks
  namespace teardown (with [ADR-0022](0022-deletion-and-adoption-policy.md)
  covering the workload-CR side).
- Entry replacement on generation/RV mismatch preserves PERF-2 (pooled client
  per connection) with identical hit rates for steady state.
- Secret cache filtering changes manager wiring (`cmd/main.go`) and needs a
  documented RBAC statement; the e2e secret-rotation scenario must stay green.
- Unit tests must pin: replace-on-rotation closes old transport, release with
  missing Secret succeeds, cache bounded across N rotations.

## Alternatives considered

- **TTL/LRU cache**: solves growth but not the release deadlock, and evicts
  hot clients under pressure; identity-keyed replacement is simpler and exact.
- **No cache (client per reconcile)**: re-handshakes TLS per reconcile;
  violates PERF-2. Rejected.
- **Cluster-scoped `QueueManagerConnection`**: centralizes credentials and
  eases sharing, but breaks the namespace isolation model and requires a
  grant mechanism; deferred to a future ADR if multi-tenancy demand
  materialises (MKR-09).

## References

- [ADR-0003](0003-connection-model.md) — connection model (extended)
- Architecture review 2026-06-09: ARCH-02/03/05; edge-case audit: EC-P0-02,
  EC-P2-01 (internal)
- [NON_FUNCTIONAL_REQUIREMENTS.md](../NON_FUNCTIONAL_REQUIREMENTS.md) — PERF-2, SEC-8
