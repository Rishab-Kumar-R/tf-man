variable "project_name" {
  type = string
}

variable "schedule_expression" {
  description = "How often reaper scans for orphaned resources"
  type        = string
  default     = "rate(1 hour)"
}

variable "grace_period_seconds" {
  description = "How long a flagged resource must remain orphaned before it's deleted"
  type        = number
  default     = 86400
}

variable "lambda_zip_path" {
  type    = string
  default = "lambdas/reaper/function.zip"
}

variable "tags" {
  type    = map(string)
  default = {}
}
