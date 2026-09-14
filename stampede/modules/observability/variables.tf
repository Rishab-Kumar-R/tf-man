variable "project_name" {
  type = string
}

variable "cluster_name" {
  type = string
}

variable "container_insights_role_arn" {
  description = "IRSA role ARN for the CloudWatch observability add-on"
  type        = string
}

variable "namespace" {
  type    = string
  default = "monitoring"
}

variable "chart_version" {
  type    = string
  default = "91.0.0"
}

variable "tags" {
  type    = map(string)
  default = {}
}
