# Frequently asked questions

Common questions about using MKurator with an existing IBM MQ queue manager on Kubernetes.

## General

### Does MKurator install or run a queue manager?

No. MKurator assumes a queue manager already exists and exposes the **IBM MQ Administrative REST
API** (`mqweb`) over HTTPS. It reconciles administrative objects (queues, topics, channels, auth
rules) on that queue manager — it does not deploy MQ pods or manage MQ licensing.

### How is this different from the IBM MQ Operator?

The IBM MQ Operator focuses on **deploying and operating** queue manager instances on Kubernetes.
MKurator focuses on **declarative administration** of objects on a queue manager you already run
(on or off cluster). They can complement each other: deploy MQ with IBM's operator, connect with
`QueueManagerConnection`, then manage objects with MKurator CRs.

### What API version is stable?

`messaging.mkurator.dev/v1alpha1` is the current **alpha** API — it may change between
releases until `v1beta1`. What is and is not guaranteed, and the path to graduation, is
in [API_STABILITY.md](API_STABILITY.md). Breaking changes are documented in
[CHANGELOG.md](../CHANGELOG.md) and [UPGRADE.md](UPGRADE.md) before release.

## Connectivity

### What is a `QueueManagerConnection` (QMC)?

A **QueueManagerConnection** stores how the operator reaches a queue manager: HTTPS endpoint, queue
manager name, and credentials via a referenced Kubernetes `Secret`. Resource CRs (`Queue`, `Topic`,
`Channel`, auth CRs) reference a QMC by name in the same namespace.

See [INSTALL_AND_USE.md](INSTALL_AND_USE.md) and the [queue walkthrough](examples/queue-and-connection.md).

### Can one operator manage multiple queue managers?

Yes. Create one `QueueManagerConnection` per queue manager (each with its own endpoint and Secret).
Resource CRs point at the QMC they target.

### Why does reconciliation fail with TLS or certificate errors?

MKurator validates the mqweb server certificate unless you configure trust for private CAs. On kind
with mkcert, follow [LOCAL_SETUP.md](LOCAL_SETUP.md). In production, prefer proper CA-issued certs
or a documented trust bundle — see [Engineering guidelines](development/guidelines.md).

## Reconciliation

### What happens when I delete a CR?

Finalizers block removal until the operator deletes (or confirms absence of) the corresponding MQ
object. If MQ deletion fails, the CR stays with a `Deleting` condition explaining why.

### What is drift and how does MKurator handle it?

**Drift** is when someone changes an MQ object outside Kubernetes (for example via `runmqsc`). For
supported attributes, MKurator compares desired spec to MQ `DISPLAY` output and re-applies DEFINE when
values differ. See [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md).

### Can I suspend reconciliation?

Set **`spec.suspend: true`** on a workload CR (`Queue`, `Topic`, `Channel`,
`ChannelAuthRule`, or `AuthorityRecord`) to pause MQ reconciliation for that object
without deleting it. Status shows `Synced=False` with `Reason=Suspended`. Set
`spec.suspend: false` (or omit the field) to resume.

To force an immediate reconcile after clearing suspend or changing spec, update the
annotation `messaging.mkurator.dev/reconcile-requested-at` to a new value (for
example the current UTC time in RFC3339).

## Authentication and authorization

### What auth CRs are supported?

- **`ChannelAuthRule`** — `SET CHLAUTH` rules (for example `ADDRESSMAP`, `BLOCKUSER`).
- **`AuthorityRecord`** — `SET AUTHREC` object authority (OAM) for queue-style profiles.

Rule types accepted by the API but not yet fully exercised in CI are listed in
[PHASE5_AUTH_SKETCH.md](PHASE5_AUTH_SKETCH.md).

### Why does my `ChannelAuthRule` fail validation?

Admission checks schema and cross-field rules. Some failures (invalid MQ rule combinations) surface
only when MQ applies the rule — check CR `.status.conditions` and operator logs. See the
[channel authentication example](examples/channel-authentication.md).

## Operations

### How do I upgrade the operator?

Follow [UPGRADE.md](UPGRADE.md): upgrade CRDs and the manager deployment (Helm or manifests), then
verify webhooks and sample CRs. The [upgrade walkthrough](examples/upgrade-walkthrough.md) shows a
typical path.

### cert-manager webhooks time out during install — what now?

This usually means the cluster cannot reach the validating webhook Service or TLS is misconfigured.
See [INSTALL_AND_USE.md#diagnostics-and-troubleshooting](INSTALL_AND_USE.md#diagnostics-and-troubleshooting)
and ensure webhook certificates and network policies allow apiserver → webhook traffic.

### Where are metrics and logs?

Prometheus metrics and structured logging are documented in [OBSERVABILITY.md](OBSERVABILITY.md) and
[LOGGING.md](LOGGING.md).

## Contributing and support

### How do I report a security issue?

Use the process in [SECURITY.md](../SECURITY.md) — do not open public issues for vulnerabilities.

### Where is the full documentation site?

Published docs: [conduit-ops.github.io/MKurator](https://conduit-ops.github.io/MKurator/). Source lives under
[`docs/`](README.md).
