# Observability (metrics)

Practical guide to **Prometheus metrics** for Kurator in production. Logging is
documented separately in [LOGGING.md](LOGGING.md).

Doc index: [README.md](README.md) · Install: [INSTALL_AND_USE.md](INSTALL_AND_USE.md)

## What you get out of the box

| Capability | Default | You configure |
|------------|---------|---------------|
| Controller-runtime metrics + Kurator counters | **On** (`metrics.enabled=true`) | Scraping / dashboards |
| HTTPS metrics on port **8443** | **On** (`metrics.secure=true`) | Network policies, TLS trust |
| Kubernetes API auth on `/metrics` | **On** (secure mode) | Prometheus `ServiceMonitor` + RBAC |
| `ServiceMonitor` CR | **Off** | Enable when Prometheus Operator is installed |
| `PrometheusRule` alerts | **Off** | Enable with kube-prometheus-stack labels |
| Structured logs | **On** (JSON, `info`) | [LOGGING.md](LOGGING.md), Helm `logging.*` |

The operator does **not** install Prometheus, Grafana, or the Prometheus Operator.
You need a monitoring stack (or a vendor equivalent) to scrape and alert.

## Metrics endpoint

- **Path:** `/metrics`
- **Port:** `8443` (named port `metrics` on the manager pod)
- **Service:** `{release}-metrics` in the operator namespace (Helm fullname prefix)

When `metrics.secure=true` (default), scrapes must use **HTTPS** and present a
valid Kubernetes service account token (or a subject allowed by RBAC).

### Built-in custom metrics

- `kurator_reconcile_total{controller,result}`
- `kurator_reconcile_errors_total{controller}`
- `kurator_mq_operations_total{operation,result}`

Plus standard controller-runtime workqueue and Go runtime metrics on the same endpoint.

## Enabling Prometheus scrape (Helm)

Requires [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)
CRDs (e.g. **kube-prometheus-stack**).

```sh
helm upgrade --install kurator ./charts/kurator \
  --namespace kurator-system \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.labels.release=kube-prometheus-stack
```

Adjust `metrics.serviceMonitor.labels` to match your Prometheus `serviceMonitorSelector`.
The local kind stack uses `release: kube-prometheus-stack` — see
[`charts/kurator/samples/values-kind.yaml`](../charts/kurator/samples/values-kind.yaml).

Optional starter alerts:

```sh
--set metrics.prometheusRule.enabled=true \
--set metrics.prometheusRule.labels.release=kube-prometheus-stack
```

The chart ships a rule that fires when the metrics target is down.

### Kustomize installs

The repo includes a sample `ServiceMonitor` under `config/prometheus/` (disabled in
default kustomization). For production, either enable that overlay or use the Helm
chart’s `ServiceMonitor` template with equivalent labels.

## RBAC: metrics-reader pattern

Secure metrics use two cluster roles (created by Helm when `metrics.enabled=true`):

| ClusterRole | Purpose |
|-------------|---------|
| `{release}-metrics-reader` | `GET` on non-resource URL `/metrics` |
| `{release}-metrics-auth` | Delegates authentication/authorization to the Kubernetes API |

Prometheus needs permission to scrape. Typical pattern:

1. Create a **dedicated ServiceAccount** for Prometheus in your monitoring namespace.
2. Bind **`{release}-metrics-reader`** to that ServiceAccount (ClusterRoleBinding).
3. Point the `ServiceMonitor` at the Kurator metrics Service; the chart sets
   `bearerTokenFile` on the scrape endpoint so the Prometheus pod’s SA token is used.

E2e validates this flow with `kurator-metrics-reader` — see
[`test/e2e/e2e_test.go`](../test/e2e/e2e_test.go).

If scrapes return **403 Forbidden**, check RoleBindings and that Prometheus runs with
the bound ServiceAccount.

### Insecure metrics (not recommended)

`metrics.secure=false` exposes metrics without Kubernetes API auth. Only use behind
strict network policies in isolated environments.

## Verify scraping

```sh
# Metrics Service exists
kubectl -n kurator-system get svc -l app.kubernetes.io/name=kurator

# ServiceMonitor (when enabled)
kubectl -n kurator-system get servicemonitor

# From a debug pod with a bound metrics-reader SA (simplified)
kubectl -n kurator-system port-forward svc/kurator-metrics 8443:8443
# curl -k -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://127.0.0.1:8443/metrics
```

In Prometheus UI, query `up{namespace="kurator-system"}` for the Kurator target.

## Dashboards and SLOs

No first-party Grafana dashboard is required for operation. Start from:

- Reconcile error rate: `rate(kurator_reconcile_errors_total[5m])`
- MQ operation failures: `rate(kurator_mq_operations_total{result="error"}[5m])`
- Target up: `up` for the metrics Service

Align alerting with [NON_FUNCTIONAL_REQUIREMENTS.md](NON_FUNCTIONAL_REQUIREMENTS.md) (OBS-*).

## See also

- [LOGGING.md](LOGGING.md) — log levels and formats  
- [charts/kurator/README.md](../charts/kurator/README.md) — Helm values table  
- [ARCHITECTURE.md](ARCHITECTURE.md) — metrics component overview  
- [UPGRADE.md](UPGRADE.md) — upgrade order (operator before changing scrape config)  
