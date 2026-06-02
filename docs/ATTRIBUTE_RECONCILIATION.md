# Attribute reconciliation model

Kurator applies IBM MQ objects through **mqweb `runCommandJSON`** (`DEFINE … REPLACE`).
Reconcilers compare desired `spec.attributes` to **DISPLAY** results before re-applying.

Implementation lives in `internal/adapter/mqrest/mqsc_params.go` (DISPLAY parameter lists)
and `internal/mqadmin/attrmatch.go` (value comparison). See
[IBM_MQ_OBJECTS.md](IBM_MQ_OBJECTS.md) for MQSC semantics.

## How it works

| Layer | Behaviour |
|-------|-----------|
| **DEFINE** | Any lowercase key in `spec.attributes` is forwarded (numeric coercion where configured; topic `topstr` → mqweb `topicStr`; topic `pub`/`sub` uppercased for DEFINE). |
| **DISPLAY** | Only attributes listed per object type are requested from mqweb (some keywords return `MQWB0120E` on IBM MQ 9.4.x and are omitted). |
| **Drift** | For each desired key, observed DISPLAY value must match (`AttributeValueMatches` — case-insensitive for policies, numeric-normalized for counters). |
| **`Synced=True`** | Object exists and every **desired** key that we can observe matches; define-only keys are not verified after apply. |

## Reconciled object types (v1alpha1)

| CRD | MQ object | `spec.type` |
|-----|-----------|-------------|
| `Queue` | `QLOCAL`, `QALIAS`, `QREMOTE` | `local` (default), `alias`, `remote` |
| `Topic` | `TOPIC` | n/a |
| `Channel` | `CHANNEL` | `svrconn` only (default) |
| `QueueManagerConnection` | (connectivity, not MQSC) | n/a |

Planned: `SET AUTHREC` / `SET CHLAUTH` (Phase 5).

## Attribute coverage by object

### Queue — `type: local` (`QLOCAL`)

| Attribute | DEFINE | Drift (DISPLAY) | Notes |
|-----------|--------|-----------------|-------|
| `maxdepth` | yes | yes | Numeric |
| `descr` | yes | yes | |
| `defpsist` | yes | yes | Case-insensitive match |
| `get`, `put` | yes | yes | Case-insensitive |
| `maxmsglen` | yes | **no** | mqweb 9.4 rejects on DISPLAY (`MQWB0120E`) |
| `share`, `defopts`, `bothresh`, `boqname`, `usage`, trigger fields | yes | **no** | Passthrough; not in safe DISPLAY list |
| `cluster`, `clusnl` | yes | **no** | Clustering — future work |

### Queue — `type: alias` (`QALIAS`)

| Attribute | DEFINE | Drift (DISPLAY) | Notes |
|-----------|--------|-----------------|-------|
| `targq` | yes | yes | Target queue name |
| `targtype` | yes | yes | `QUEUE` or `TOPIC` |
| `descr` | yes | yes | |

### Queue — `type: remote` (`QREMOTE`)

| Attribute | DEFINE | Drift (DISPLAY) | Notes |
|-----------|--------|-----------------|-------|
| `rname` | yes | yes | Remote queue name (blank for QM alias) |
| `rqmname` | yes | yes | Remote queue manager |
| `xmitq` | yes | yes | Transmission queue |
| `descr` | yes | yes | |

### Topic (`TOPIC`)

| Attribute | DEFINE | Drift (DISPLAY) | Notes |
|-----------|--------|-----------------|-------|
| `topstr` | yes | yes | Stored as `topicStr` in mqweb JSON |
| `descr` | yes | yes | |
| `pub`, `sub` | yes | yes | Uppercased on DEFINE; case-insensitive drift |
| `defpsist` | yes | yes | |
| `pubscope`, `subscope` | yes | yes | Omitted from DISPLAY if mqweb returns `MQWB0120E` on your QM level |
| `toptype`, `cluster` | yes | **no** | Passthrough only |

### Channel (`CHLTYPE(SVRCONN)`)

| Attribute | DEFINE | Drift (DISPLAY) | Notes |
|-----------|--------|-----------------|-------|
| `trptype` | yes | yes | Case-insensitive |
| `descr` | yes | yes | |
| `maxmsgl` | yes | yes | Numeric |
| `sharecnv` | yes | yes | Numeric |
| `mcauser` | yes | yes | |
| `maxinst`, `maxinstc` | yes | yes | Numeric |
| `sslciph`, `sslcauth` | yes | **no** | TLS — passthrough; not in DISPLAY list (Phase 5 / gitops samples) |

## Out of scope (not CRDs today)

| MQ surface | MQSC | Phase |
|------------|------|-------|
| OAM | `SET AUTHREC` | 5 |
| Channel auth | `SET CHLAUTH` | 5 |
| Durable subscription | `DEFINE SUB` | Later |
| Alias / remote / model queue | `QALIAS`, `QREMOTE`, `QMODEL` | 4 follow-on |
| Message channels | `CHLTYPE(SDR\|RCVR\|…)` | Later |
| Connection auth | `AUTHINFO`, `ALTER QMGR CONNAUTH` | Platform |

Sketch for Phase 5: [PHASE4_CHANNEL_AUTH.md](PHASE4_CHANNEL_AUTH.md).

## Known limitations

1. **Manual MQ changes** to define-only attributes are not detected; re-applying the CR does not force a new DEFINE unless a drift-checked key changes.
2. **mqweb version** — DISPLAY safe lists are tuned for 9.4.x; older queue managers may need list adjustments (see Phase 2 roadmap note on `maxmsglen`).
3. **Open attribute map** — typos in keys fail at MQ apply time with MQSC errors, not Kubernetes schema validation.

## Related docs

- User-facing field tables: [INSTALL_AND_USE.md](INSTALL_AND_USE.md#attribute-reconciliation)
- Delivery plan: [ROADMAP.md](ROADMAP.md)
