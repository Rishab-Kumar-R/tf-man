variable "project_name" {
  type = string
}

variable "cluster_name" {
  type = string
}

variable "service_name" {
  type = string
}

variable "log_group_name" {
  type = string
}

variable "alb_arn_suffix" {
  type = string
}

variable "target_group_arn_suffix" {
  type = string
}

variable "min_healthy_count" {
  description = "RunningTaskCount below this triggers ALARM"
  type        = number
  default     = 1
}

variable "alarm_evaluation_periods" {
  description = "Consecutive breaching periods required before alarming (guards against single-task blips)"
  type        = number
  default     = 3
}

variable "alarm_period_seconds" {
  type    = number
  default = 60
}

variable "tags" {
  type    = map(string)
  default = {}
}
