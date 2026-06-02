locals {
  mq_host         = "mq.localhost"
  mq_release_name = "ibm-mq"
}

resource "kubernetes_namespace_v1" "ibm_mq" {
  metadata {
    name = var.mq_namespace
  }
}

resource "kubernetes_secret_v1" "mq_tls" {
  metadata {
    name      = var.tls_secret_name
    namespace = kubernetes_namespace_v1.ibm_mq.metadata[0].name
  }

  type = "kubernetes.io/tls"

  data = {
    "tls.crt" = base64decode(var.tls_cert_string)
    "tls.key" = base64decode(var.tls_key_string)
  }
}

resource "kubernetes_secret_v1" "mq_credentials" {
  metadata {
    name      = "mq-credentials"
    namespace = kubernetes_namespace_v1.ibm_mq.metadata[0].name
  }

  type = "Opaque"

  data = {
    mqAdminPassword = var.mq_admin_password
    mqAppPassword   = var.mq_app_password
  }
}

resource "helm_release" "ibm_mq" {
  name      = local.mq_release_name
  namespace = kubernetes_namespace_v1.ibm_mq.metadata[0].name

  repository = "https://ibm-messaging.github.io/mq-helm"
  chart      = "ibm-mq"
  version    = var.mq_chart_version

  wait    = true
  timeout = 900

  values = [
    yamlencode({
      license = "accept"

      image = {
        repository = "icr.io/ibm-messaging/mq"
        tag        = var.mq_image_tag
      }

      queueManager = {
        name = var.mq_queue_manager_name
      }

      web = {
        enable = true
      }

      credentials = {
        enable = true
        secret = kubernetes_secret_v1.mq_credentials.metadata[0].name
      }

      metrics = {
        enabled = true
      }

      persistence = {
        qmPVC = {
          enable = true
          size   = "2Gi"
        }
      }

      resources = {
        requests = {
          cpu    = "250m"
          memory = "512Mi"
        }
        limits = {
          cpu    = "1"
          memory = "1024Mi"
        }
      }

      # Upstream chart Ingress hardcodes ingressClassName: nginx. We expose mqweb
      # via a Terraform-managed Ingress using HAProxy instead.
      route = {
        ingress = {
          webconsole = {
            enable = false
          }
        }
      }
    })
  ]

  depends_on = [
    helm_release.haproxy_ingress,
    kubernetes_secret_v1.mq_tls,
    kubernetes_secret_v1.mq_credentials,
  ]
}

resource "kubernetes_ingress_v1" "mq_web" {
  metadata {
    name      = "${local.mq_release_name}-console"
    namespace = kubernetes_namespace_v1.ibm_mq.metadata[0].name
    annotations = {
      "haproxy-ingress.github.io/backend-protocol" = "h1-ssl"
    }
  }

  spec {
    ingress_class_name = var.ingress_class_name

    tls {
      hosts       = [local.mq_host]
      secret_name = var.tls_secret_name
    }

    rule {
      host = local.mq_host
      http {
        path {
          path      = "/"
          path_type = "Prefix"
          backend {
            service {
              name = local.mq_release_name
              port {
                name = "console-https"
              }
            }
          }
        }
      }
    }
  }

  depends_on = [
    helm_release.ibm_mq,
    kubernetes_secret_v1.mq_tls,
  ]
}
