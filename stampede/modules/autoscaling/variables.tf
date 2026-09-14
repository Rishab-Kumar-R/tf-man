variable "project_name" {
  type = string
}

variable "cluster_name" {
  type = string
}

variable "cluster_endpoint" {
  type = string
}

variable "oidc_provider_arn" {
  type = string
}

variable "oidc_provider_url" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "cluster_security_group_id" {
  type = string
}

variable "karpenter_version" {
  type    = string
  default = "1.14.1"
}

variable "nodepool_manifest_path" {
  type        = string
  description = "Path to the Karpenter EC2NodeClass/NodePool manifest to apply automatically once Karpenter is installed."
}

variable "tags" {
  type    = map(string)
  default = {}
}
