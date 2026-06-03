locals {
  monitoring_namespace = "monitoring"
  grafana_host         = "grafana.localhost"
}

resource "kubernetes_namespace_v1" "monitoring" {
  count = var.enable_monitoring ? 1 : 0

  metadata {
    name = local.monitoring_namespace
  }
}

# TLS secrets are namespace-scoped, so copy our mkcert material into monitoring.
resource "kubernetes_secret_v1" "monitoring_tls" {
  count = var.enable_monitoring ? 1 : 0

  metadata {
    name      = var.tls_secret_name
    namespace = kubernetes_namespace_v1.monitoring[0].metadata[0].name
  }

  type = "kubernetes.io/tls"

  data = {
    "tls.crt" = base64decode(var.tls_cert_string)
    "tls.key" = base64decode(var.tls_key_string)
  }
}

resource "helm_release" "kube_prometheus_stack" {
  count = var.enable_monitoring ? 1 : 0

  name             = "kube-prometheus-stack"
  namespace        = kubernetes_namespace_v1.monitoring[0].metadata[0].name
  create_namespace = false

  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "kube-prometheus-stack"
  version    = "84.5.0"

  wait    = true
  timeout = 900

  # Keep it light for kind while still delivering Prometheus Operator + Grafana.
  values = [
    yamlencode({
      grafana = {
        enabled       = true
        adminUser     = var.grafana_admin_user
        adminPassword = var.grafana_admin_password
        ingress = {
          enabled          = true
          ingressClassName = var.ingress_class_name
          hosts            = [local.grafana_host]
          path             = "/"
          pathType         = "Prefix"
          tls = [
            {
              hosts      = [local.grafana_host]
              secretName = var.tls_secret_name
            }
          ]
        }
      }
      # Commonly disabled in small local clusters:
      kubeEtcd              = { enabled = false }
      kubeControllerManager = { enabled = false }
      kubeScheduler         = { enabled = false }
      kubeProxy             = { enabled = false }
    })
  ]

  depends_on = [
    helm_release.haproxy_ingress,
    kubernetes_secret_v1.monitoring_tls,
  ]
}
