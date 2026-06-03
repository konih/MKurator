resource "helm_release" "haproxy_ingress" {
  name             = "haproxy-ingress"
  namespace        = "ingress-system"
  create_namespace = true

  repository = "https://haproxy-ingress.github.io/charts"
  chart      = "haproxy-ingress"
  version    = "0.16.1"

  wait    = true
  timeout = 600

  values = [
    file("${path.module}/haproxy-ingress-values.yaml"),
  ]

  set {
    name  = "controller.ingressClassResource.enabled"
    value = "true"
  }

  set {
    name  = "controller.ingressClassResource.name"
    value = var.ingress_class_name
  }

  set {
    name  = "controller.ingressClass"
    value = var.ingress_class_name
  }
}
