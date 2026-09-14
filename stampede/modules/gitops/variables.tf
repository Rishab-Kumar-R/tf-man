variable "argocd_namespace" {
  type    = string
  default = "argocd"
}

variable "argocd_chart_version" {
  type    = string
  default = "10.9.0"
}

variable "argo_rollouts_namespace" {
  type    = string
  default = "argo-rollouts"
}

variable "argo_rollouts_chart_version" {
  type    = string
  default = "2.43.1"
}

variable "tags" {
  type    = map(string)
  default = {}
}
