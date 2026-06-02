# Architecture

This document describes the design of **Kurator**: its
components, the custom resources it manages, the reconcile flow, and the local
development topology. For conventions and tooling see [DEVELOPMENT.md](DEVELOPMENT.md); for the delivery
plan see [ROADMAP.md](ROADMAP.md). Attribute DEFINE vs drift behaviour:
[ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md).

## Scope

The operator manages **administrative objects on an existing IBM MQ Queue
Manager** declaratively. It is explicitly **not** responsible for deploying or
operating Queue Manager installations. The Queue Manager already exists and
exposes the IBM MQ Administrative REST API (`mqweb`).

The initial `v1alpha1` API targets:

- `QueueManagerConnection` — how to reach a Queue Manager (endpoint + creds).
- `Queue`, `Topic`, `Channel` — MQSC objects on a referenced Queue Manager.

## Components

```mermaid
flowchart TB
  subgraph cluster [Kubernetes Cluster]
    apiserver["API server"]
    secret["Secret (mqweb creds)"]
    subgraph operator [Operator Pod]
      mgr["controller-runtime Manager"]
      wh["validating webhooks"]
      crec["QueueManagerConnectionReconciler"]
      qrec["QueueReconciler"]
      trec["TopicReconciler"]
      chrec["ChannelReconciler"]
      port["mqadmin.Admin port"]
      rest["mqrest adapter (mqweb client)"]
    end
  end
  qm["Existing Queue Manager + mqweb"]

  apiserver -->|"admission"| wh
  apiserver --> mgr
  wh --> mgr
  mgr --> crec
  mgr --> qrec
  mgr --> trec
  mgr --> chrec
  secret -.->|"resolved by"| crec
  qrec --> port
  trec --> port
  chrec --> port
  crec --> port
  port --> rest
  rest -->|"HTTPS REST / MQSC"| qm
```

| Component | Responsibility |
|-----------|----------------|
| **Manager** (`cmd/`) | Wires reconcilers, validating webhooks, caches, health/metrics, leader election. |
| **Validating webhooks** (`internal/webhook`, `internal/validation`) | Reject invalid CR specs at admission (`failurePolicy: Fail`); same-namespace `connectionRef` and Secret checks only — **no mqweb**. |
| **Reconcilers** (`internal/controller`) | Thin control loops for `QueueManagerConnection`, `Queue`, `Topic`, and `Channel`. Translate desired vs. observed state and call the `mqadmin.Admin` port. No HTTP/MQ details. |
| **MQAdmin port** (`internal/mqadmin`) | Go interface (`Admin`) describing MQ operations (ping, queue/topic/channel define/inspect/delete) plus domain types. The seam that makes controllers testable and backends swappable. |
| **mqrest adapter** (`internal/adapter/mqrest`) | The only `MQAdmin` implementation today. Talks to `mqweb` over HTTPS, posting MQSC commands and parsing responses. |
| **Secret** | Holds mqweb credentials (and optionally TLS material), referenced by `QueueManagerConnection`. Never inlined in specs. |

### The MQAdmin port

The live interface in `internal/mqadmin/admin.go` (abbreviated):

```go
// Admin is the seam between reconcilers and IBM MQ.
type Admin interface {
    Ping(ctx context.Context) error
    GetQueue(ctx context.Context, spec QueueSpec) (*QueueState, error)
    DefineQueue(ctx context.Context, spec QueueSpec) error
    DeleteQueue(ctx context.Context, spec QueueSpec) error
    GetTopic(ctx context.Context, name string) (*TopicState, error)
    DefineTopic(ctx context.Context, spec TopicSpec) error
    DeleteTopic(ctx context.Context, name string) error
    GetChannel(ctx context.Context, spec ChannelSpec) (*ChannelState, error)
    DefineChannel(ctx context.Context, spec ChannelSpec) error
    DeleteChannel(ctx context.Context, spec ChannelSpec) error
}
```

- Reconcilers depend only on this interface.
- `mockery` generates a mock from it for unit tests (`test/mocks`).
- A future PCF backend can implement the same interface with no controller
  changes (see [ADR-0002](adr/0002-manage-mq-via-mqweb-rest.md)).

## Operator runtime concerns

The `cmd/` entrypoint wires a single controller-runtime **Manager** that owns
all cross-cutting runtime behaviour. These are first-class requirements, not
afterthoughts (NFRs in [NON_FUNCTIONAL_REQUIREMENTS.md](NON_FUNCTIONAL_REQUIREMENTS.md)).

| Concern | Approach |
|---------|----------|
| **Leader election** | Enabled (`--leader-elect`) so a multi-replica Deployment has exactly one active reconciler; standby replicas give fast failover. Uses a `Lease` in the operator namespace. |
| **Health / readiness** | `healthz` and `readyz` on `:8081`; wired to Deployment liveness/readiness probes. Readiness gates on manager cache sync. |
| **Metrics** | controller-runtime Prometheus metrics on `:8443` (HTTPS, authn/authz-protected) plus custom MQ counters/histograms. A `ServiceMonitor` is shipped (optional) for the local kube-prometheus-stack. |
| **Graceful shutdown** | Manager stops on `SIGTERM`/`SIGINT`, draining in-flight reconciles within `terminationGracePeriodSeconds`. |
| **Configuration** | Flags + env for metrics/health addresses, leader election, log level/format, and reconcile concurrency. No MQ endpoints in operator config — those live in `QueueManagerConnection` CRs. |
| **Logging** | Structured logging via **`logr` in application code** and **`slog` at bootstrap** ([ADR-0007](adr/0007-structured-logging-logr-slog.md), [LOGGING.md](LOGGING.md)); configurable via file, `KURATOR_LOG_*` env, or flags. JSON in cluster, text optional locally. Never log secrets or full credentialed request bodies. |
| **Concurrency** | `MaxConcurrentReconciles` tuned per controller; work is queued and rate-limited by controller-runtime. |

### RBAC & least privilege

The operator ships a tightly scoped `ClusterRole` generated from
`+kubebuilder:rbac` markers:

- Full access to its own API group (`messaging.kurator.dev`): `queues`,
  `topics`, `channels`, `queuemanagerconnections`, and their `/status` and
  `/finalizers` subresources.
- `get`/`list`/`watch` on the referenced **`Secrets`** (credentials, CA bundles)
  — and nothing broader on core resources.
- `create`/`patch` on `Events`; reconcilers emit **Warning** Events on terminal
  errors (`recordTerminalEvent`) in addition to status conditions.
- `Lease` access in the operator namespace for leader election.

No wildcard verbs, no cluster-admin. RBAC drift is caught by `task verify`.

### Connection & client lifecycle

- A `QueueManagerConnection` resolves to an `mqadmin.Admin` client: endpoint + TLS
  trust (from `caSecretRef`) + credentials (from `credentialsSecretRef`).
- The adapter keeps a **pooled HTTPS client** per connection (reused across
  reconciles) rather than dialing per request. The cache key includes the
  connection **generation** and referenced Secret **`resourceVersion`** values
  (credentials and optional CA bundle). `ReleaseConnection` drops the cached
  client when a connection CR is deleted.
- TLS is verified by default; `insecureSkipVerify` is opt-in and intended only
  for local dev. The mqweb CSRF header (`ibm-mq-rest-csrf-token`) is sent on all
  mutating calls (see [IBM_MQ_REST_API.md](IBM_MQ_REST_API.md)).

### Error handling & requeue strategy

Errors are classified at the `MQAdmin` port boundary so controllers can react
without string-parsing:

| Class | Examples | Reconciler response |
|-------|----------|---------------------|
| **Terminal** | invalid MQSC, 400/403, auth misconfig | Set a failing condition with a clear reason; emit a Warning Event; do **not** hot-loop. |
| **Transient** | 5xx, network timeout, QM not running (503) | Return the error (or `RequeueAfter` with backoff) so controller-runtime retries with rate limiting. |
| **NotFound** | object absent on QM | Treated as "needs create" on ensure, or "already gone" on delete. |

Principles: wrap with `%w` and context; use `errors.Is`/`errors.As`; let
controller-runtime own backoff for transient failures; never panic in a
reconcile.

Workload reconcilers (`Queue`, `Topic`, `Channel`) **watch**
`QueueManagerConnection` Ready/status changes so they requeue promptly when a
connection becomes healthy instead of polling on a fixed interval only.

The `mqrest.Client` also exposes `RunMQSC` for ad-hoc MQSC (e2e fixtures,
future Phase 5 objects). It is **not** part of the `mqadmin.Admin` port —
reconcilers depend only on typed Admin methods.

## Security model

- **No inline credentials**: all secrets come from referenced `Secret`s; specs
  never carry passwords or keys.
- **Least-privilege RBAC** as above; the operator can read only the Secrets it
  needs.
- **TLS everywhere**: HTTPS to mqweb with verification on by default; custom CA
  bundles via `caSecretRef`.
- **Defense in logging**: structured logs scrub credentials; request/response
  bodies are not logged at default levels.
- **Supply chain**: CGO-free static binary, distroless nonroot image,
  `govulncheck` + image scanning in CI (see [CICD.md](CICD.md)).

Full requirements and rationale: [NON_FUNCTIONAL_REQUIREMENTS.md](NON_FUNCTIONAL_REQUIREMENTS.md).

## Custom resources

### QueueManagerConnection

Describes how to reach a Queue Manager. Cluster- or namespace-scoped (TBD in
Phase 2; namespaced by default for multi-tenant isolation).

```yaml
apiVersion: messaging.kurator.dev/v1alpha1
kind: QueueManagerConnection
metadata:
  name: qm1
spec:
  queueManager: QM1            # MQ Queue Manager name
  endpoint: https://mq.example.com:9443
  tls:
    insecureSkipVerify: false
    caSecretRef:               # optional CA bundle
      name: qm1-ca
  credentialsSecretRef:        # username/password for mqweb
    name: qm1-mqweb
status:
  conditions:                  # Ready=True once Ping succeeds
    - type: Ready
      status: "True"
```

### Queue

A queue maintained on a referenced Queue Manager (`QLOCAL`, `QALIAS`, or
`QREMOTE`).

```yaml
apiVersion: messaging.kurator.dev/v1alpha1
kind: Queue
metadata:
  name: orders
spec:
  connectionRef:
    name: qm1                  # references a QueueManagerConnection
  queueName: APP.ORDERS        # MQ object name
  type: local                  # local | alias | remote
  attributes:                  # lowercase MQSC keys
    maxdepth: "5000"
    descr: "Orders intake queue"
status:
  conditions:                  # Synced=True when MQSC matches spec
    - type: Synced
      status: "True"
  observedGeneration: 3
```

### Topic

An administrative topic object (`DEFINE TOPIC`) on a referenced Queue Manager.

```yaml
apiVersion: messaging.kurator.dev/v1alpha1
kind: Topic
metadata:
  name: retail-orders
spec:
  connectionRef:
    name: qm1
  topicName: RETAIL.ORDERS
  attributes:
    topstr: retail/orders
    descr: Retail order events
status:
  conditions:
    - type: Synced
      status: "True"
```

### Channel

A server-connection channel (`CHLTYPE(SVRCONN)`) on a referenced Queue Manager.

```yaml
apiVersion: messaging.kurator.dev/v1alpha1
kind: Channel
metadata:
  name: orders-app
spec:
  connectionRef:
    name: qm1
  channelName: ORDERS.APP
  type: svrconn
  attributes:
    descr: Application SVRCONN channel
    trptype: tcp
status:
  conditions:
    - type: Synced
      status: "True"
```

Design choices (Queue, Topic, Channel):

- `connectionRef` decouples object definitions from connection details and lets
  many resources share one connection.
- `attributes` map to MQSC parameters (lowercase keys) so new attributes can be
  supported without API churn. Drift-checked vs define-only keys are documented
  in [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md).

## Reconcile flow

`Queue`, `Topic`, and `Channel` reconcilers share the same lifecycle pattern
(connection wait → finalizer → display/define/delete via `mqadmin.Admin`).
Example for a `Queue`:

```mermaid
sequenceDiagram
  participant API as API server
  participant R as QueueReconciler
  participant P as mqadmin.Admin port
  participant MQ as Queue Manager (mqweb)

  API->>R: Queue created/updated
  R->>R: resolve QueueManagerConnection (wait until Ready)
  alt deletion (finalizer set)
    R->>P: DeleteQueue(spec)
    P->>MQ: DELETE QLOCAL/QALIAS/QREMOTE(...)
    R->>API: remove finalizer
  else ensure desired state
    R->>P: GetQueue(spec)
    P->>MQ: DISPLAY ...
    R->>P: DefineQueue(spec) (create or alter on drift)
    P->>MQ: DEFINE ... REPLACE
    R->>API: update status (Synced, observedGeneration)
  end
```

`TopicReconciler` and `ChannelReconciler` call the corresponding topic/channel
port methods with the same ensure/delete structure.

Principles:

- **Idempotent**: define/alter MQSC so repeated reconciles converge; safe to
  re-run.
- **Drift detection**: compare observed MQSC attributes against spec each loop
  and correct (see [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md)).
- **Finalizers**: a finalizer guarantees the MQ object is removed before the CR
  disappears.
- **Status conditions**: `Ready` on `QueueManagerConnection` (connectivity) and
  `Synced` on workload CRs (object matches spec), plus `observedGeneration`,
  give clear, machine-readable state.

## Why REST over PCF

| Aspect | mqweb REST (chosen) | PCF via `ibm-messaging/mq-golang` |
|--------|--------------------|-----------------------------------|
| Build | Pure Go, `CGO_ENABLED=0` | Requires MQ C client libs + CGO |
| Image | Slim, static binary | Must bundle native MQ client |
| Testability | Easy: `httptest` + mockable port | Harder: native client, command queues |
| Transport | HTTPS, firewall-friendly | MQ channels |

REST keeps the binary pure Go and the project easy to test and ship. The
`MQAdmin` port preserves the option to add a PCF adapter later if a deployment
requires it, without disturbing controllers. The full rationale and trade-offs
are recorded in [ADR-0002](adr/0002-manage-mq-via-mqweb-rest.md).

## Local development topology

Day-to-day development and e2e run against a self-contained local platform under
`hack/kind-cluster`: a **kind** cluster with **HAProxy Ingress** (NodePorts
30080/30443), **cert-manager**, an optional **kube-prometheus-stack**, and a
real **IBM MQ** Queue Manager (Helm chart) — all provisioned with **Terraform**.
mkcert provides trusted TLS for `*.localhost`, so the web console and REST API
are reachable over real HTTPS without `curl -k`. See
[DEVELOPMENT.md](DEVELOPMENT.md) for commands.

```mermaid
flowchart LR
  host["developer host (mkcert trust)"]
  subgraph kind [kind cluster]
    ing["HAProxy Ingress (NodePorts 30080/30443)"]
    subgraph mqns [namespace ibm-mq]
      qm["IBM MQ Queue Manager + mqweb (Helm)"]
    end
    subgraph opns [operator namespace]
      op["operator (controller-manager)"]
      crs["QMC / Queue / Topic / Channel CRs"]
    end
    mon["kube-prometheus-stack (optional)"]
  end
  host -->|"https://mq.localhost:30443"| ing --> qm
  op -->|"reconcile"| crs
  op -->|"HTTPS mqweb (in-cluster svc:9443)"| qm
  op -.->|"/metrics"| mon
```

- **kind** hosts both day-to-day dev and e2e runs; Terraform provisions HAProxy
  ingress, TLS, monitoring, and the Queue Manager.
- The operator reaches mqweb in-cluster (e.g. `https://ibm-mq.ibm-mq.svc:9443`);
  humans reach the console/REST via ingress at `https://mq.localhost:30443`.
- e2e (`KURATOR_E2E_MQ=1`) asserts that applying Queue, Topic, and Channel CRs
  produces the expected MQSC objects on the live Queue Manager.
- Unit/envtest layers need no MQ at all (port is mocked), keeping the inner loop
  fast.
