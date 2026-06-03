# Phase 5 — authority and channel-auth API sketch

Planning document for Kurator **Phase 5** ([ROADMAP.md](ROADMAP.md)). It maps
reference MQSC from
ibm-messaging/mq-gitops-samples `qmdemo-mqsc-config-map.yaml` (mirrored in [`test/e2e/fixtures/channel-auth-prereq.mqsc`](../test/e2e/fixtures/channel-auth-prereq.mqsc))
and patterns in [IBM_MQ_OBJECTS.md](IBM_MQ_OBJECTS.md) to CRD fields.

**Already shipped (Phase 4):** the [`Channel`](../api/v1alpha1/channel_types.go)
CRD reconciles `DEFINE CHANNEL` … `CHLTYPE(SVRCONN)` with drift detection. See
[INSTALL_AND_USE.md](INSTALL_AND_USE.md) and
[ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md).

**Shipped on `main` (Phase 5):**

- [`ChannelAuthRule`](../api/v1alpha1/channelauthrule_types.go) — `SET CHLAUTH`
  with `ACTION(REPLACE)` / `ACTION(REMOVE)` on delete
- [`AuthorityRecord`](../api/v1alpha1/authorityrecord_types.go) — `SET AUTHREC`
  with `AUTHADD` / `AUTHRMV(ALL)` on delete
- Samples: [`config/samples/`](../config/samples/) · integration tests in
  [`test/integration/mq/`](../test/integration/mq/)

**Remaining:** extended CHLAUTH rule types, CI proof on release tag, and optional
integration coverage for additional rule/object types — see
[ROADMAP.md](ROADMAP.md#phase-5--user--authority-management).

Kurator reconciles Phase 5 objects via the existing **mqweb `/mqsc`** path
([ADR-0002](adr/0002-manage-mq-via-mqweb-rest.md)), not via IBM’s ConfigMap-at-
`QueueManager` bootstrap model.

## Reference MQSC (gitops basic deployment)

Source: IBM `mq-gitops-samples` (Apache-2.0 header in upstream file). Kurator e2e
fixture: [`test/e2e/fixtures/channel-auth-prereq.mqsc`](../test/e2e/fixtures/channel-auth-prereq.mqsc).

```mqsc
DEFINE CHANNEL('DEV.APP.SVRCONN.0TLS') CHLTYPE(SVRCONN) TRPTYPE(TCP) +
  MCAUSER('app') SSLCIPH('') SSLCAUTH(OPTIONAL) REPLACE

SET CHLAUTH('DEV.APP.SVRCONN.0TLS') TYPE(ADDRESSMAP) ADDRESS('*') +
  USERSRC(CHANNEL) CHCKCLNT(REQUIRED) +
  DESCR('Allows connection via APP channel') ACTION(REPLACE)
```

The `DEFINE CHANNEL` portion is covered by the shipped `Channel` CRD. The
`SET CHLAUTH` portion is covered by `ChannelAuthRule`.

## Shipped resources

### `ChannelAuthRule` (CHLAUTH)

| CRD field | MQSC | Maps from gitops example |
|-----------|------|---------------------------|
| `spec.connectionRef` | — | |
| `spec.channelName` | channel name in `SET CHLAUTH('…')` | `DEV.APP.SVRCONN.0TLS` |
| `spec.ruleType` | `TYPE` | `ADDRESSMAP` |
| `spec.address` | `ADDRESS` | `*` |
| `spec.userSource` | `USERSRC` | `CHANNEL` |
| `spec.checkClient` | `CHCKCLNT` | `REQUIRED` |
| `spec.description` | `DESCR` | |

**Reconcile:** `SET CHLAUTH(...) ACTION(REPLACE)`; **delete:** `SET CHLAUTH(...) ACTION(REMOVE)`.

Additional rule types in the OpenAPI enum (not yet covered by samples/e2e):

| `ruleType` | Typical use |
|------------|-------------|
| `BLOCKUSER` | `USERLIST` — deny privileged IDs |
| `USERMAP` | Map `CLNTUSER` to `MCAUSER` |
| `SSLPEERMAP` | Map TLS DN |
| `QMGRMAP` | Map remote QM name |
| `BLOCKADDR` | Block IPs at listener |

### `AuthorityRecord` (OAM — `SET AUTHREC`)

| CRD field | MQSC |
|-----------|------|
| `spec.connectionRef` | — |
| `spec.profile` | `PROFILE('…')` queue or channel name |
| `spec.objectType` | `OBJTYPE` — `QUEUE`, `CHANNEL`, … |
| `spec.principal` / `spec.group` | `PRINCIPAL` / `GROUP` |
| `spec.authorities` | `AUTHADD` list — `GET`, `PUT`, `CONNECT`, … |

**Reconcile:** `SET AUTHREC ... AUTHADD(...) ACTION(REPLACE)`; **delete:**
`SET AUTHREC ... AUTHRMV(ALL)`.

## `MQAdmin` port (shipped)

```go
SetChannelAuth(ctx context.Context, spec ChannelAuthSpec) error
DeleteChannelAuth(ctx context.Context, spec ChannelAuthSpec) error
SetAuthority(ctx context.Context, spec AuthoritySpec) error
DeleteAuthority(ctx context.Context, spec AuthoritySpec) error
```

Adapter implementation: [`internal/adapter/mqrest/auth.go`](../internal/adapter/mqrest/auth.go)
via `RunMQSC` / `runCommand`.

**GET paths (shipped):** `GetChannelAuth` and `GetAuthority` run `DISPLAY CHLAUTH` /
`DISPLAY AUTHREC` for observed state (foundation for drift detection). Reconcilers
still use replace-on-apply; drift-aware auth reconcile is a follow-up.

## What we are not copying from IBM samples

| IBM pattern | Kurator approach |
|-------------|------------------|
| `spec.queueManager.mqsc` ConfigMap on `QueueManager` | Per-object CRs + continuous reconcile |
| Dynamic MQSC volume reload (gitops `queue-manager-deployment`) | Operator observes CR spec generation |
| IBM MQ Operator webhook / OLM install | Out of scope — Kurator targets existing mqweb |

## E2e and fixtures

Channel/auth MQSC used to validate mqweb lives under
[`test/e2e/fixtures/`](../test/e2e/fixtures/). Queue/Topic/Channel/auth reconcile
e2e is in [`test/e2e/mq_e2e_test.go`](../test/e2e/mq_e2e_test.go). Remaining
Phase 5 items are tracked in [ROADMAP.md](ROADMAP.md#phase-5--user--authority-management).
