variable "kubeconfig" {
  description = "Path to kubeconfig used by Terraform."
  type        = string
}

variable "ingress_class_name" {
  description = "IngressClass name used by the HAProxy ingress controller."
  type        = string
  default     = "haproxy"
}

variable "tls_namespace" {
  description = "Namespace that holds the shared/default TLS secret for the ingress controller."
  type        = string
  default     = "ingress-system"
}

variable "tls_secret_name" {
  description = "Secret name for the mkcert TLS material."
  type        = string
  default     = "wildcard-localhost-tls"
}

variable "tls_cert_string" {
  description = "Base64-encoded TLS cert (PEM)."
  type        = string
  default     = ""
}

variable "tls_key_string" {
  description = "Base64-encoded TLS key (PEM)."
  type        = string
  default     = ""
}

variable "grafana_admin_user" {
  description = "Grafana admin username (kube-prometheus-stack)."
  type        = string
  default     = "admin"
}

variable "grafana_admin_password" {
  description = "Grafana admin password (kube-prometheus-stack)."
  type        = string
  default     = "admin"
  sensitive   = true
}

variable "enable_monitoring" {
  description = "Whether to install kube-prometheus-stack (Prometheus + Grafana)."
  type        = bool
  default     = true
}

variable "enable_argocd" {
  description = "Whether to install Argo CD on the local kind cluster."
  type        = bool
  default     = false
}

variable "mq_namespace" {
  description = "Namespace for the IBM MQ queue manager."
  type        = string
  default     = "ibm-mq"
}

variable "mq_chart_version" {
  description = "Version of the upstream IBM MQ Helm chart (helm repo add ibm-messaging-mq https://ibm-messaging.github.io/mq-helm)."
  type        = string
  # renovate: datasource=helm depName=ibm-mq registryUrl=https://ibm-messaging.github.io/mq-helm
  default     = "12.0.1"
}

variable "mq_image_tag" {
  description = "IBM MQ container image tag (icr.io/ibm-messaging/mq)."
  type        = string
  # renovate: datasource=docker depName=icr.io/ibm-messaging/mq
  default     = "9.4.2.0-r1"
}

variable "mq_queue_manager_name" {
  description = "Name of the IBM MQ queue manager."
  type        = string
  default     = "QM1"
}

variable "mq_admin_password" {
  description = "Password for the MQ 'admin' user (MQWebAdmin role / REST admin API)."
  type        = string
  default     = "passw0rd"
  sensitive   = true
}

variable "mq_app_password" {
  description = "Password for the MQ 'app' user."
  type        = string
  default     = "passw0rd"
  sensitive   = true
}

variable "state_dir" {
  description = "Path to the local .state directory where generated secrets are written."
  type        = string
  default     = ""
}
