variable "role_name" {
  type = string
}

variable "oidc_provider_arn" {
  type = string
}

variable "oidc_provider_url" {
  description = "OIDC issuer URL, e.g. https://oidc.eks.<region>.amazonaws.com/id/XXXX"
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace the service account lives in"
  type        = string
}

variable "service_account_name" {
  type = string
}

variable "policy_json" {
  description = "Inline IAM policy document (JSON). Optional if managed_policy_arns covers everything needed."
  type        = string
  default     = null
}

variable "managed_policy_arns" {
  description = "AWS managed policy ARNs to attach, e.g. official controller policies"
  type        = list(string)
  default     = []
}

variable "tags" {
  type    = map(string)
  default = {}
}
