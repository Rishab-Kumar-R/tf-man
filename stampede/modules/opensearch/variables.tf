variable "project_name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "allowed_security_group_ids" {
  type = list(string)
}

variable "allowed_principal_arns" {
  description = "IAM role ARNs permitted to call this domain (e.g. catalog-service's IRSA role)"
  type        = list(string)
}

variable "instance_type" {
  type    = string
  default = "t3.small.search"
}

variable "engine_version" {
  type    = string
  default = "OpenSearch_3.7"
}

variable "instance_count" {
  type    = number
  default = 1
}

variable "zone_awareness_enabled" {
  type    = bool
  default = false
}

variable "volume_size" {
  type    = number
  default = 10
}

variable "tags" {
  type    = map(string)
  default = {}
}
