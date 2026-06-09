# ADR-0017: PCF adapter behind the MQAdmin port

- **Status**: Accepted (scaffold only) — full adapter **parked**, see 2026-06-09 note
- **Date**: 2026-06-03 (status updated 2026-06-09)

> **2026-06-09 status note.** The scaffold described below shipped at
> `internal/adapter/mqpcf` (v0.5.2) and is accepted as the compile-time
> contract check on the `Admin` port. The **full PCF adapter is parked**: it
> implies CGO builds, MQ C client redistribution, a second integration-test
> matrix, and a second image variant — a project-defining commitment for a
> single-maintainer operator. It will not be implemented unless a concrete
> adopter with a no-mqweb environment commits to using and helping validate
> it. The port seam preserves the option at zero ongoing cost; the connection
> model questions below remain open until then.

## Context

[ADR-0002](0002-manage-mq-via-mqweb-rest.md) chose **mqweb REST** as the default
transport for IBM MQ administration and placed all MQ interaction behind the
**`MQAdmin` port** (`internal/mqadmin`). That decision remains correct for
MKurator's primary build: pure Go, CGO-free, slim images, and straightforward
`httptest` unit tests.

Some deployments cannot expose **mqweb** (firewall policy, legacy queue managers,
or environments where only MQI/PCF client channels are available). IBM's
**Programmable Command Format (PCF)** via `ibm-messaging/mq-golang` offers a
native alternative but requires the MQ C client and **CGO**, complicating builds,
images, and CI.

The port seam in ADR-0002 explicitly reserved a future PCF adapter implementing
the same `Admin` interface with zero controller changes. Phase 5 auth objects
(CHLAUTH, AUTHREC) are now on the port; a PCF backend must eventually cover the
full surface, not just queues.

## Decision

We will add a **scaffold PCF adapter** at `internal/adapter/mqpcf` that:

1. Declares a `Client` type implementing `mqadmin.Admin` (compile-time checked).
2. Exposes `NewClient(Config)` with minimal configuration placeholders.
3. Returns **`errNotImplemented`** from every port method until PCF commands are
   wired incrementally.
4. Remains **unwired** from `cmd/main.go` and reconcilers — **mqrest stays the
   sole production adapter** until this ADR moves to Accepted with a chosen
   connection/factory model.

This ADR **does not supersede [ADR-0002](0002-manage-mq-via-mqweb-rest.md)**.
REST remains the default; PCF is an optional future backend for environments
without mqweb.

### Admin → PCF mapping outline

Implementation will map each `Admin` method to IBM MQ PCF commands (via
`mq-golang` or equivalent). Order of delivery TBD; mapping is the target shape:

| `Admin` method | PCF direction | Notes |
|----------------|---------------|-------|
| `Ping` | Connect + lightweight inquiry (e.g. `MQCMD_INQUIRE_Q_MGR`) | Validates QM reachability |
| `GetQueue` | `MQCMD_INQUIRE_Q` | Map PCF attributes → `QueueState.Attributes` |
| `DefineQueue` | `MQCMD_CREATE_Q` / `MQCMD_CHANGE_Q` with replace semantics | Mirror DEFINE … REPLACE drift path |
| `DeleteQueue` | `MQCMD_DELETE_Q` | |
| `GetTopic` | `MQCMD_INQUIRE_TOPIC` | |
| `DefineTopic` | `MQCMD_CREATE_TOPIC` / `MQCMD_CHANGE_TOPIC` | |
| `DeleteTopic` | `MQCMD_DELETE_TOPIC` | |
| `GetChannel` | `MQCMD_INQUIRE_CHANNEL` | SVRCONN focus matches v1alpha1 |
| `DefineChannel` | `MQCMD_CREATE_CHANNEL` / `MQCMD_CHANGE_CHANNEL` | |
| `DeleteChannel` | `MQCMD_DELETE_CHANNEL` | |
| `SetChannelAuth` | `MQCMD_SET_CHLAUTH_REC` | Map `ChannelAuthSpec` → PCF filters |
| `GetChannelAuth` | `MQCMD_INQUIRE_CHLAUTH_REC` | |
| `DeleteChannelAuth` | `MQCMD_DELETE_CHLAUTH_REC` | |
| `SetAuthority` | `MQCMD_SET_AUTHORITY_REC` | AUTHADD equivalent |
| `GetAuthority` | `MQCMD_INQUIRE_AUTHORITY_REC` | |
| `DeleteAuthority` | `MQCMD_DELETE_AUTHORITY_REC` | |

Port-level errors (`ErrNotFound`, `ErrTerminal`, `ErrTransient`) must be
returned consistently with [ADR-0014](0014-mq-error-taxonomy-and-requeue.md);
PCF reason/comp codes map in the adapter, not in controllers.

### Connection model deferral

[ADR-0003](0003-connection-model.md) models connectivity through
`QueueManagerConnection` and the mqrest `ClientFactory` (endpoint, TLS, Secrets).
**This ADR does not define the PCF connection model.** Open questions deferred
to a follow-up ADR or Accepted revision of this one:

- MQI channel parameters vs. CCDT / JSON connection definitions.
- TLS trust and credentials from the same `Secret` keys as mqrest, or MQ-specific
  binding files.
- Whether `Factory.ForConnection` selects mqrest vs mqpcf via a new
  `connectionRef` field, build tag, or operator flag.
- Client pooling, reconnect, and `ReleaseConnection` lifecycle under CGO.

The scaffold `Config` carries only a queue manager name placeholder; factory
wiring and `cmd/main.go` registration are explicitly out of scope for the
scaffold phase.

## Consequences

- **Positive**: The `Admin` contract is exercised by a second package early;
  interface drift is caught at compile time. Incremental PCF work can land
  method-by-method without touching reconcilers.
- **Positive**: Documentation records the PCF command mapping target before
  implementation detail obscures it.
- **Neutral**: Default builds remain CGO-free; `mqpcf` is inert until linked
  and called.
- **Negative**: A full PCF adapter implies CGO, native MQ client redistribution,
  and heavier integration-test infrastructure ([ADR-0011](0011-layered-testing-strategy.md)).
- **Follow-up**: Accept this ADR (or supersede with connection decision),
  implement methods in priority order (Ping → queue CRUD → topic/channel → auth),
  add `mqpcf` factory, optional build tag or separate image variant for CGO builds.

## Alternatives considered

- **Implement PCF immediately as production backend**: rejected; violates lean
  default in ADR-0002 and expands CI/release scope prematurely.
- **Skip scaffold; document mapping only**: rejected; compile-time `Admin`
  implementation catches port changes and proves package layout before CGO lands.
- **Replace REST with PCF**: rejected; would supersede ADR-0002 and regress
  testability and image size for all users.

## References

- [ADR-0002](0002-manage-mq-via-mqweb-rest.md) — REST default (unchanged)
- [ADR-0003](0003-connection-model.md) — `QueueManagerConnection`
- [ADR-0014](0014-mq-error-taxonomy-and-requeue.md) — port error taxonomy
- [ARCHITECTURE.md](../ARCHITECTURE.md) — Why REST over PCF
- [ROADMAP.md](../ROADMAP.md) — optional PCF adapter candidate work
