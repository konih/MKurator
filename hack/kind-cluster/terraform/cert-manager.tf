resource "helm_release" "cert_manager" {
  name             = "cert-manager"
  namespace        = "cert-manager"
  create_namespace = true

  repository = "https://charts.jetstack.io"
  chart      = "cert-manager"

  # Pinned to avoid surprise upgrades in local dev.
  version = "v1.20.2"

  wait    = true
  timeout = 600

  values = [
    yamlencode({
      crds = {
        enabled = true
      }
      replicaCount = 1
      webhook = {
        replicaCount = 1
      }
      cainjector = {
        replicaCount = 1
      }
    })
  ]
}
