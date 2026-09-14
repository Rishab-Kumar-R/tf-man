variable "cluster_name" {
  type = string
}

variable "irsa_role_arn" {
  description = "ARN of the IRSA role for the aws-load-balancer-controller service account"
  type        = string
}

variable "vpc_id" {
  type = string
}

variable "namespace" {
  type    = string
  default = "kube-system"
}

variable "chart_version" {
  type    = string
  default = "3.5.0"
}

variable "waf_rate_limit" {
  description = "Max requests per 5-minute window per IP before WAF blocks it"
  type        = number
  default     = 2000
}

variable "tags" {
  type    = map(string)
  default = {}
}
