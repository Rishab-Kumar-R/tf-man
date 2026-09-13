variable "project_name" {
  description = "Name prefix for all sentry resources"
  type        = string
  default     = "sentry"
}

variable "aws_region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "ap-south-1"
}

variable "azs" {
  description = "Availability zones for the network module"
  type        = list(string)
  default     = ["ap-south-1a", "ap-south-1b"]
}

variable "container_image" {
  description = "Placeholder demo image for the workload"
  type        = string
  default     = "public.ecr.aws/nginx/nginx:latest"
}

variable "desired_count" {
  type    = number
  default = 2
}

variable "min_healthy_count" {
  type    = number
  default = 1
}

variable "reaper_schedule_expression" {
  description = "How often reaper scans for orphaned resources"
  type        = string
  default     = "rate(5 minutes)"
}

variable "reaper_grace_period_seconds" {
  description = "How long a flagged orphaned resource waits before being deleted"
  type        = number
  default     = 300
}
