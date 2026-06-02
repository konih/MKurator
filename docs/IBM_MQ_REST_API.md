# IBM MQ REST API (mqweb) — reference summary

This document summarizes how the **mqweb** server exposes IBM MQ over HTTPS. It complements [IBM_MQ_OBJECTS.md](./IBM_MQ_OBJECTS.md) (MQSC/object model).

For a **machine-readable full schema**, see [schemas/README.md](./schemas/README.md). IBM publishes the complete API as **Swagger 2.0** only from a running mqweb instance (`GET /ibm/api/docs`), not as a static product file.

---

## 1. Architecture

```
Client (curl, operator, browser)
        │ HTTPS (default 9443)
        ▼
┌───────────────────────────────────────┐
│  mqweb (WebSphere Liberty)            │
│  - IBM MQ Console  (/ibmmq/console)   │
│  - Admin REST API  (/ibmmq/rest/vN/…) │
│  - Messaging REST  (/ibmmq/rest/vN/…) │
│  - Swagger UI      (/ibm/api/explorer)│
└───────────────┬───────────────────────┘
                │ PCF / command server / pooled MQI
                ▼
        Queue manager(s) — local or remote via gateway
```

| Component | Role |
|-----------|------|
| **mqweb** | HTTP server embedded with IBM MQ; configured via `mqwebuser.xml`, `setmqweb`, `dspmqweb` |
| **Admin REST API** | CRUD-style resources for some objects + **MQSC escape** endpoint for everything else |
| **Messaging REST API** | Put, browse, and destructively get messages (and publish to topics) |
| **API Discovery** | Liberty feature `apiDiscovery-1.0` exposes Swagger at `/ibm/api/docs` |

**Default port:** HTTPS `9443` (HTTP optional). Console: `https://host:9443/ibmmq/console`.

---

## 2. URL layout and versions

All REST APIs share the prefix:

```text
https://{host}:{port}/ibmmq/rest/v{version}/
```

| Version | Introduced | Status for new work |
|---------|------------|---------------------|
| **v1** | MQ 9.1 | Legacy; first-class `GET`/`POST` on `/admin/qmgr/.../queue` etc. |
| **v2** | MQ 9.1.5 | Stable; **`/mqsc`** JSON commands; v1 queue `GET` deprecated for display |
| **v3** | MQ 9.3.0 | **Preferred** — same resource model as v2, version bump for incompatible changes |

**Target for this operator:** `v3`.

Examples:

```text
https://localhost:9443/ibmmq/rest/v3/admin/installation
https://localhost:9443/ibmmq/rest/v3/admin/qmgr/QM1/queue
https://localhost:9443/ibmmq/rest/v3/admin/action/qmgr/QM1/mqsc
https://localhost:9443/ibmmq/rest/v3/messaging/qmgr/QM1/queue/APP.IN/message
```

Queue manager names in paths are **case-sensitive**. Encode `/` → `%2F`, `.` → `%2E`, `%` → `%25`.

References: [REST API versions](https://www.ibm.com/docs/en/ibm-mq/9.3.x?topic=api-rest-versions), [Administrative REST API reference](https://www.ibm.com/docs/en/ibm-mq/9.3.x?topic=reference-administrative-rest-api).

---

## 3. Security

### 3.1 Authentication

Typical modes (can be combined):

| Mode | Notes |
|------|--------|
| **HTTP Basic** | `Authorization: Basic …` — common in dev |
| **TLS client certificate** | Enterprise / container setups |
| **LTPA / cookies** | Console and token-based flows |

### 3.2 mqweb roles

| Role | Admin REST | Messaging REST |
|------|------------|----------------|
| `MQWebAdmin` | Full admin (incl. mutating) | No (not applicable) |
| `MQWebAdminRO` | Read-only admin | No |
| `MQWebUser` | Limited admin | Yes (with OAM on QM) |

Configured in `mqwebuser.xml` (user → group → role mapping).

### 3.3 CSRF header

Required on **POST, PATCH, DELETE** (any value, including empty):

```http
ibm-mq-rest-csrf-token: 1
```

### 3.4 Remote administration gateway

For queue managers not in the same installation as mqweb:

```http
ibm-mq-rest-gateway-qmgr: GATEWAY.QM
```

Gateway QM must have channels/XMITQs to the target. See [Remote administration using the REST API](https://www.ibm.com/docs/en/ibm-mq/9.4.x?topic=api-remote-administration-using-rest).

### 3.5 OAM still applies

REST authentication ≠ object authority. The principal must still have `CONNECT` on the QM and appropriate `SET AUTHREC` / `setmqaut` grants for MQSC and messaging.

---

## 4. Administrative REST API

### 4.1 Resource inventory

IBM documents these **first-class** admin resources (v2/v3). HTTP verbs vary per resource; see IBM docs for each.

| Resource path | Purpose |
|---------------|---------|
| `/login` | Authenticated user and roles |
| `/admin/installation` | MQ installation discovery |
| `/admin/qmgr` | List queue managers |
| `/admin/qmgr/{qmgrName}` | Queue manager attributes / status |
| `/admin/qmgr/{qmgrName}/queue` | Queues collection |
| `/admin/qmgr/{qmgrName}/queue/{queueName}` | Single queue |
| `/admin/qmgr/{qmgrName}/channel` | Channels collection |
| `/admin/qmgr/{qmgrName}/channel/{channelName}` | Single channel |
| `/admin/qmgr/{qmgrName}/subscription` | Pub/sub subscriptions |
| `/admin/qmgr/{qmgrName}/subscription/{subName}` | Single subscription |
| `/admin/action/qmgr/{qmgrName}/mqsc` | **Execute arbitrary MQSC** (primary escape hatch) |
| `/admin/mft/agent` | Managed File Transfer agents |
| `/admin/mft/transfer` | MFT transfers |
| `/admin/mft/monitor` | MFT monitoring |

**Not exposed as dedicated REST resources** (use `/mqsc` instead): topics, listeners, processes, namelists, authinfo, **authority records (OAM)**, CHLAUTH, most `ALTER QMGR` settings.

Community confirmation: only QM, queue, channel, and subscription have native REST CRUD; auth uses MQSC ([MQSeries discussion](https://mqseries.net/phpBB2/viewtopic.php?t=76475)).

### 4.2 v1 vs v2+ queue access

| Approach | Version | Example |
|----------|---------|---------|
| REST object model | v1 | `GET /v1/admin/qmgr/QM1/queue/Q1?attributes=*` |
| MQSC via REST | v2+ | `POST /v2/admin/action/qmgr/QM1/mqsc` with `DISPLAY QLOCAL(Q1)` |

For **v3 greenfield**, prefer **`/mqsc`** with `runCommandJSON` for parity with [IBM_MQ_OBJECTS.md](./IBM_MQ_OBJECTS.md) and a single reconciliation path.

### 4.3 The `/mqsc` endpoint (recommended for the operator)

**URL:** `POST /ibmmq/rest/v3/admin/action/qmgr/{qmgrName}/mqsc`

**Headers:**

```http
Content-Type: application/json; charset=UTF-8
ibm-mq-rest-csrf-token: 1
Authorization: Basic …
```

#### Mode A — plain text MQSC (`runCommand`)

Request:

```json
{
  "type": "runCommand",
  "parameters": {
    "command": "DEFINE QLOCAL('APP.IN') REPLACE MAXDEPTH(100000) DEFPSIST(YES)"
  }
}
```

Response body: JSON wrapper; messages in `commandResponse[].message` or `.text`.

#### Mode B — structured JSON MQSC (`runCommandJSON`)

Request:

```json
{
  "type": "runCommandJSON",
  "command": "define",
  "qualifier": "qlocal",
  "name": "APP.IN",
  "parameters": {
    "replace": "yes",
    "maxdepth": 100000,
    "defpsist": "yes",
    "descr": "Application input"
  }
}
```

**Rules:**

- Parameter names are **lowercase** JSON keys mapping to MQSC keywords.
- No surrounding quotes for string values (unlike interactive MQSC).
- Use `"replace": "yes"` / `"noreplace": "yes"` — not `"replace": "no"`.
- Lists (e.g. `authadd`) are JSON arrays.

**DISPLAY with selected attributes:**

```json
{
  "type": "runCommandJSON",
  "command": "display",
  "qualifier": "qlocal",
  "name": "APP.IN",
  "responseParameters": ["maxdepth", "defpsist", "get", "put", "curdepth"]
}
```

Returned attributes appear under `commandResponse[].parameters`.

**JSON `runCommandJSON` does not support:** `DISPLAY ARCHIVE`, `DISPLAY CHINIT`, `DISPLAY GROUP`, `DISPLAY LOG`, `DISPLAY SECURITY`, `DISPLAY SYSTEM`, `DISPLAY THREAD`, `DISPLAY TRACE`, `DISPLAY USAGE`.

JSON Schema for request/response shapes: [schemas/mqsc-runcommand.schema.json](./schemas/mqsc-runcommand.schema.json).

#### Example: reconcile queue + authority

```bash
curl -sk -u admin:pass \
  -X POST "https://localhost:9443/ibmmq/rest/v3/admin/action/qmgr/QM1/mqsc" \
  -H "Content-Type: application/json" \
  -H "ibm-mq-rest-csrf-token: 1" \
  -d '{"type":"runCommand","parameters":{"command":"DEFINE QLOCAL(APP.IN) REPLACE MAXDEPTH(100000) DEFPSIST(YES)"}}'

curl -sk -u admin:pass \
  -X POST "https://localhost:9443/ibmmq/rest/v3/admin/action/qmgr/QM1/mqsc" \
  -H "Content-Type: application/json" \
  -H "ibm-mq-rest-csrf-token: 1" \
  -d '{"type":"runCommand","parameters":{"command":"SET AUTHREC PROFILE(APP.IN) OBJTYPE(QUEUE) PRINCIPAL(app) AUTHADD(GET,PUT,INQ,DSP)"}}'
```

#### Standard response envelope

```json
{
  "commandResponse": [
    {
      "completionCode": 0,
      "reasonCode": 0,
      "message": ["AMQ8006I: IBM MQ queue created."]
    }
  ],
  "overallCompletionCode": 0,
  "overallReasonCode": 0
}
```

HTTP **200** means the REST layer accepted the command; check `overallCompletionCode` and each `commandResponse` entry for MQ errors.

#### HTTP status codes (summary)

| Code | Meaning |
|------|---------|
| 200 | Command submitted / completed at REST layer |
| 400 | Invalid JSON or MQSC |
| 401 | Not authenticated to mqweb |
| 403 | Authenticated but not authorized |
| 404 | Queue manager not found |
| 500 | Server / MQ error |
| 503 | Queue manager not running |

Errors may also return JSON `error[]` with `msgId`, `reasonCode`, `explanation` (e.g. `MQWB0111E` if `SYSTEM.REST.REPLY.QUEUE` is missing).

### 4.4 Native queue resource (v1-style JSON model)

Still available on v1 paths; useful reference for attribute naming when mapping CRD status.

**GET** `…/admin/qmgr/QM1/queue/Q1?attributes=*` returns nested JSON, for example:

```json
{
  "queue": [{
    "name": "Q1",
    "type": "local",
    "general": {
      "description": "",
      "inhibitPut": false,
      "inhibitGet": false,
      "isTransmissionQueue": false
    },
    "storage": {
      "maximumDepth": 5000,
      "maximumMessageLength": 4194304,
      "messageDeliverySequence": "priority"
    },
    "applicationDefaults": {
      "messagePersistence": "nonPersistent",
      "messagePriority": 0
    },
    "trigger": { "enabled": false, "type": "first", "depth": 1 },
    "timestamps": { "created": "…", "altered": "…" }
  }]
}
```

**POST** create (minimal v1 body): `{"name":"MAQ1"}` — prefer explicit MQSC for production definitions.

Channel **GET** returns analogous `channel[]` with `type` (`sender`, `svrconn`, etc.) and type-specific nested objects.

---

## 5. Messaging REST API

Base: `/ibmmq/rest/v3/messaging/`

| Resource | Method | Action |
|----------|--------|--------|
| `/messaging/qmgr/{qmgr}/queue/{queue}/message` | POST | Put message (body = payload, `text/plain`) |
| `/messaging/qmgr/{qmgr}/queue/{queue}/message` | GET | Browse messages |
| `/messaging/qmgr/{qmgr}/queue/{queue}/message` | DELETE | Destructive get |
| `/messaging/qmgr/{qmgr}/topic/{topicPath}/message` | POST | Publish to topic string |

**Limits / behaviour:**

- Supported message formats: `MQSTR`, JMS `TextMessage` only for get/browse.
- No transactional once-and-only-once; failed HTTP may leave ambiguous put/get state.
- Requires `MQWebUser` + OAM on target queue/topic.
- AMS encryption uses **mqweb server** context, not end-user context.

**Example put:**

```bash
curl -sk -u myuser:pass \
  -X POST "https://localhost:9443/ibmmq/rest/v3/messaging/qmgr/QM1/queue/MSGQ/message" \
  -H "ibm-mq-rest-csrf-token: 1" \
  -H "Content-Type: text/plain;charset=utf-8" \
  --data "Hello World"
```

Reference: [Messaging REST API](https://www.ibm.com/docs/en/ibm-mq/9.3.x?topic=reference-messaging-rest-api), [Getting started](https://www.ibm.com/docs/en/ibm-mq/9.4.x?topic=api-getting-started-messaging-rest).

---

## 6. Swagger / OpenAPI (complete schema)

IBM documents the API in Knowledge Center **and** as runtime Swagger 2.0.

| URL | Content |
|-----|---------|
| `https://host:port/ibm/api/explorer` | Interactive Swagger UI |
| `https://host:port/ibm/api/docs` | Full Swagger 2.0 JSON document |

**Enable in `mqwebuser.xml`:**

```xml
<featureManager>
  <feature>apiDiscovery-1.0</feature>
</featureManager>
```

**Fetch into this repo:**

```bash
./scripts/fetch-mqweb-swagger.sh https://localhost:9443 docs/schemas/mqweb-swagger.json
```

The Swagger document includes Liberty-wide discovery plus IBM MQ paths such as:

| Swagger tag (typical) | Paths |
|-----------------------|--------|
| `login` | `/ibmmq/rest/v1/login` |
| `qmgr` | `/ibmmq/rest/vN/admin/qmgr/...` |
| Admin actions | `/ibmmq/rest/vN/admin/action/qmgr/{name}/mqsc` |
| Messaging | `/ibmmq/rest/vN/messaging/...` |

**Note:** IBM states `apiDiscovery-1.0` is stabilized; **`mpOpenAPI` is not supported** for MQ. Format is **Swagger 2.0**, not OpenAPI 3.

Reference: [REST API discovery](https://www.ibm.com/docs/en/ibm-mq/9.3.x?topic=api-rest-discovery).

---

## 7. Mapping REST to operator reconciliation

| CRD concern | Recommended REST approach |
|-------------|---------------------------|
| Queues, channels, topics, auth, CHLAUTH | `POST …/mqsc` with `runCommand` or `runCommandJSON` |
| Observe depth / status | `DISPLAY` via mqsc, or v1 `GET …/queue/{name}?attributes=…` |
| Verify existence | `DISPLAY QLOCAL(name)` → parse `commandResponse[].parameters` |
| Delete | `DELETE QLOCAL(name)` via mqsc |
| Health of QM | `GET …/admin/qmgr/{name}` (subset for remote QMs) |
| Application messaging tests | Messaging REST (optional; out of scope for admin operator) |

**Design recommendation:** Implement the operator against **`/v3/admin/action/qmgr/{qmgr}/mqsc`** so all objects in [IBM_MQ_OBJECTS.md](./IBM_MQ_OBJECTS.md) share one client. Use native `/queue` and `/channel` REST only if you need their JSON attribute model without parsing MQSC text.

**Idempotency:** Send `REPLACE` on DEFINE; use `SET AUTHREC` with `AUTHRMV(ALL)` + `AUTHADD` for permissions.

**Connection:** Operator needs `MQWEB_URL`, credentials (or cert), target `qmgrName`, optional `gatewayQmgr` header, and TLS trust config.

---

## 8. Container / Kubernetes notes

IBM MQ Helm chart exposes mqweb on the queue manager pod (default `ClusterIP`; optional Route/NodePort). `mqwebuser.xml` can be mounted from a ConfigMap — see the [upstream IBM MQ Helm chart](https://github.com/ibm-messaging/mq-helm/tree/main/charts/ibm-mq).

Typical gaps in default dev images:

- `apiDiscovery-1.0` not enabled → add feature + restart to get `/ibm/api/docs`.
- Predefined `DEV.*` auth only → custom queues need explicit `SET AUTHREC`.

`dspmqweb status` shows the REST base URL and ports.

---

## 9. Quick reference

| Task | Call |
|------|------|
| Full API schema | `GET /ibm/api/docs` |
| Define queue | `POST …/v3/admin/action/qmgr/QM1/mqsc` + `DEFINE QLOCAL(…)` |
| Display queue | mqsc `DISPLAY QLOCAL(…)` or v1 `GET …/queue/{name}` |
| Set permissions | mqsc `SET AUTHREC …` via `/mqsc` |
| Put message | `POST …/messaging/qmgr/QM1/queue/Q/message` |
| List installations | `GET …/v3/admin/installation` |

---

## 10. Related documentation

- [IBM_MQ_OBJECTS.md](./IBM_MQ_OBJECTS.md) — MQSC objects and attributes
- [schemas/mqsc-runcommand.schema.json](./schemas/mqsc-runcommand.schema.json) — JSON Schema for `/mqsc` bodies
- [schemas/README.md](./schemas/README.md) — how to capture full Swagger
- [IBM MQ Console and REST API security](https://www.ibm.com/docs/en/ibm-mq/9.3.x?topic=SSFKSJ_9.3.0/com.ibm.mq.sec.doc/q127930_.html)
- [REST API and PCF equivalents](https://www.ibm.com/docs/en/ibm-mq/9.2.x?topic=reference-rest-api-pcf-equivalents)

---

*Export the versioned Swagger snapshot from your target MQ release and commit it as `docs/schemas/mqweb-swagger-{version}.json` when you have a running queue manager.*
