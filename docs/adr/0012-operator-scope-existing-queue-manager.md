# ADR-0012: Operator scope — existing Queue Manager only

- **Status**: Accepted
- **Date**: 2026-06-02

## Context

IBM MQ can be deployed in many ways: native install, containers, Kubernetes
Operators (e.g. IBM MQ on Kubernetes), cloud managed services. Teams adopting
Kurator may already run a Queue Manager elsewhere and only want **declarative
administration** of queues, topics, and channels.

A common expectation for “MQ operators” is lifecycle management of the queue
manager itself (install, scale, upgrade, storage). That is a different product
surface with different RBAC, storage, and day-2 operations.

## Decision

Kurator **manages administrative objects on an existing Queue Manager** that
already exposes the **mqweb Administrative REST API**. It explicitly **does
not**:

- Deploy, install, or upgrade Queue Manager software or pods.
- Scale queue manager instances or manage persistent volumes for QM data.
- Replace IBM’s MQ Kubernetes operator or cloud MQ offerings.
- Manage messages, payloads, or application connectivity (only admin objects:
  queues, topics, SVRCONN channels in v1alpha1).

Prerequisites for users:

- A running Queue Manager with mqweb enabled and reachable from the cluster.
- Credentials supplied via Kubernetes `Secret` through
  `QueueManagerConnection` ([ADR-0003](0003-connection-model.md)).

Transport to MQ is HTTPS mqweb only ([ADR-0002](0002-manage-mq-via-mqweb-rest.md)).

## Consequences

- Clear positioning: Kurator is a **GitOps/admin CRD layer**, not a QM installer.
- Documentation and samples assume QM exists (kind dev stack provisions QM for
  convenience, but that platform is dev-only — not shipped as product scope).
- Feature requests for `QueueManager` CRDs, pod templates, or OLM lifecycle
  are out of scope unless this ADR is superseded.
- Users must operate MQ availability, mqweb config, and network paths themselves.

## Alternatives considered

- **Full QM lifecycle operator**: large scope overlap with IBM charts; rejected
  for this project’s focus.
- **Bundle QM Helm chart as required install**: couples releases; we provide
  `hack/kind-cluster` for dev only instead.
- **PCF-only without mqweb**: rejected as default in ADR-0002; would still not
  deploy QM.

## References

- [ARCHITECTURE.md](../ARCHITECTURE.md#scope)
- [INSTALL_AND_USE.md](../INSTALL_AND_USE.md)
- [ROADMAP.md](../ROADMAP.md) — Phase 5 auth objects still on existing QM
