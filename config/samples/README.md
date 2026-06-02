# Sample manifests

These YAML files are **starting points** for Kurator custom resources. Adapt
namespaces, endpoints, TLS, and secrets for your environment before applying in
production.

Full install and usage guide: [docs/INSTALL_AND_USE.md](../../docs/INSTALL_AND_USE.md).

## Apply order

```sh
kubectl apply -f mq-credentials-secret.yaml   # or charts/kurator/samples/resources/
kubectl apply -f messaging_v1alpha1_queuemanagerconnection.yaml
kubectl wait --for=condition=Ready qmc/qm1 -n kurator-system --timeout=120s
kubectl apply -f messaging_v1alpha1_queue.yaml
kubectl wait --for=condition=Synced queue/orders -n kurator-system --timeout=120s
kubectl apply -f messaging_v1alpha1_topic.yaml
kubectl wait --for=condition=Synced topic/retail-orders -n kurator-system --timeout=120s
kubectl apply -f messaging_v1alpha1_channel.yaml
kubectl wait --for=condition=Synced channel/orders-app -n kurator-system --timeout=120s
```

Or apply everything via Kustomize (create the credentials Secret first — it is
not bundled in `config/samples/`):

```sh
kubectl apply -f charts/kurator/samples/resources/mq-credentials-secret.yaml
kubectl apply -k config/samples/
```

Or use `task deploy:samples` when working against the local kind cluster.

---

## `messaging_v1alpha1_queuemanagerconnection.yaml`

Points the operator at one queue manager through mqweb.

```yaml
apiVersion: messaging.kurator.dev/v1alpha1
kind: QueueManagerConnection
metadata:
  name: qm1
  namespace: kurator-system
spec:
  queueManager: QM1
  endpoint: https://ibm-mq.ibm-mq.svc:9443
  tls:
    insecureSkipVerify: true          # kind/local dev only
  credentialsSecretRef:
    name: mq-credentials
```

| Field | This sample | Production guidance |
|-------|-------------|---------------------|
| `queueManager` | `QM1` | Must match your QM name exactly |
| `endpoint` | In-cluster Service DNS | Public or corporate URL reachable from the operator pod |
| `tls.insecureSkipVerify` | `true` | Remove; use `tls.caSecretRef` instead |
| `credentialsSecretRef` | `mq-credentials` | Secret in the **same namespace** as this CR |

Helm copy (no namespace in metadata — set with `-n` or Helm release namespace):
[`charts/kurator/samples/resources/queuemanagerconnection.yaml`](../../charts/kurator/samples/resources/queuemanagerconnection.yaml).

---

## Credentials secret

Not in `config/samples/` (Kustomize bundle expects you to create it separately).
Example for kind / local dev:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mq-credentials
  namespace: kurator-system
type: Opaque
stringData:
  username: admin
  mqAdminPassword: passw0rd   # local kind default only
```

Helm copy:
[`charts/kurator/samples/resources/mq-credentials-secret.yaml`](../../charts/kurator/samples/resources/mq-credentials-secret.yaml).

**Production:** inject credentials from your secret manager; never commit real
passwords to git.

---

## `messaging_v1alpha1_queue.yaml`

Declares a local queue on the queue manager referenced by `connectionRef`.

```yaml
apiVersion: messaging.kurator.dev/v1alpha1
kind: Queue
metadata:
  name: orders
  namespace: kurator-system
spec:
  connectionRef:
    name: qm1
  queueName: APP.ORDERS
  type: local
  attributes:
    maxdepth: "5000"
    descr: Orders intake queue
```

| Field | This sample | Notes |
|-------|-------------|-------|
| `connectionRef.name` | `qm1` | Must match a **Ready** `QueueManagerConnection` |
| `queueName` | `APP.ORDERS` | Actual IBM MQ object name |
| `type` | `local` | `QLOCAL`; see also `alias` and `remote` samples below |
| `attributes.maxdepth` | `"5000"` | String in YAML; sent as numeric to mqweb |
| `attributes.descr` | Human-readable text | Mapped to MQSC `DESCR` |

Helm copy:
[`charts/kurator/samples/resources/queue.yaml`](../../charts/kurator/samples/resources/queue.yaml).

---

## `messaging_v1alpha1_queue_alias.yaml`

Alias queue pointing at `APP.ORDERS` (`targq`).

| Field | This sample | Notes |
|-------|-------------|-------|
| `type` | `alias` | `DEFINE QALIAS` |
| `attributes.targq` | `APP.ORDERS` | Target queue name |

Verify: `task mq:runmqsc -- "DISPLAY QALIAS('APP.ORDERS.ALIAS') TARGQ DESCR"`

---

## `messaging_v1alpha1_queue_remote.yaml`

Remote queue definition to `APP.ORDERS` on `QM1` (local demo).

| Field | This sample | Notes |
|-------|-------------|-------|
| `type` | `remote` | `DEFINE QREMOTE` |
| `attributes.rname` | `APP.ORDERS` | Remote queue name |
| `attributes.rqmname` | `QM1` | Remote queue manager |
| `attributes.xmitq` | `SYSTEM.DEFAULT.XMIT.QUEUE` | Transmission queue |

---

## `messaging_v1alpha1_topic.yaml`

Declares an administrative topic object on the referenced queue manager.

```yaml
apiVersion: messaging.kurator.dev/v1alpha1
kind: Topic
metadata:
  name: retail-orders
  namespace: kurator-system
spec:
  connectionRef:
    name: qm1
  topicName: RETAIL.ORDERS
  attributes:
    topstr: retail/orders
    descr: Retail order events topic
    pub: enabled
    sub: enabled
```

| Field | This sample | Notes |
|-------|-------------|-------|
| `connectionRef.name` | `qm1` | Must match a **Ready** `QueueManagerConnection` |
| `topicName` | `RETAIL.ORDERS` | IBM MQ topic object name |
| `attributes.topstr` | `retail/orders` | Topic string (`TOPICSTR` in MQSC) |
| `attributes.pub` / `sub` | `enabled` | Publish/subscribe policy on the topic node |

Verify on MQ:

```sh
task mq:runmqsc -- "DISPLAY TOPIC('RETAIL.ORDERS') TOPSTR DESCR PUB SUB"
```

Helm copy:
[`charts/kurator/samples/resources/topic.yaml`](../../charts/kurator/samples/resources/topic.yaml).

---

## `messaging_v1alpha1_channel.yaml`

Declares a server-connection channel for inbound client applications.

```yaml
apiVersion: messaging.kurator.dev/v1alpha1
kind: Channel
metadata:
  name: orders-app
  namespace: kurator-system
spec:
  connectionRef:
    name: qm1
  channelName: ORDERS.APP
  type: svrconn
  attributes:
    descr: Application server-connection channel
    trptype: tcp
    maxmsgl: "4194304"
```

| Field | This sample | Notes |
|-------|-------------|-------|
| `connectionRef.name` | `qm1` | Must match a **Ready** `QueueManagerConnection` |
| `channelName` | `ORDERS.APP` | IBM MQ channel name |
| `type` | `svrconn` | Only channel type reconciled in Phase 4 |
| `attributes.trptype` | `tcp` | Transport type |
| `attributes.maxmsgl` | `"4194304"` | Max message length (numeric in mqweb JSON) |

Verify on MQ:

```sh
task mq:runmqsc -- "DISPLAY CHANNEL('ORDERS.APP') CHLTYPE(SVRCONN) TRPTYPE DESCR MAXMSGL"
```

Helm copy:
[`charts/kurator/samples/resources/channel.yaml`](../../charts/kurator/samples/resources/channel.yaml).

---

## `logging-config.yaml`

Optional manager logging config for local `go run ./cmd/main.go` — not used by
in-cluster Deployment (which sets `KURATOR_LOG_LEVEL` / `KURATOR_LOG_FORMAT`).
See [LOGGING.md](../../docs/LOGGING.md).

---

## Verify reconciliation

```sh
kubectl get qmc,mq,tp,chl -n kurator-system
kubectl describe topic retail-orders -n kurator-system
kubectl describe channel orders-app -n kurator-system
kubectl logs -n kurator-system deployment/kurator-controller-manager -f
```

On the local kind platform:

```sh
task local:info
task mq:runmqsc -- "DISPLAY QLOCAL('APP.ORDERS') MAXDEPTH"
task mq:runmqsc -- "DISPLAY TOPIC('RETAIL.ORDERS') TOPSTR"
task mq:runmqsc -- "DISPLAY CHANNEL('ORDERS.APP') CHLTYPE(SVRCONN)"
```
