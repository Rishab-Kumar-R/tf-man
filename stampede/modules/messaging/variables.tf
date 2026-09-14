variable "project_name" {
  type = string
}

variable "subscribers" {
  description = "One entry per consuming service. filter_policy is a JSON string, or null to receive every event."
  type = map(object({
    filter_policy = optional(string)
  }))
}

variable "visibility_timeout_seconds" {
  type    = number
  default = 30
}

variable "message_retention_seconds" {
  type    = number
  default = 86400
}

variable "max_receive_count" {
  description = "How many times a message can be retried before going to the DLQ"
  type        = number
  default     = 5
}

variable "tags" {
  type    = map(string)
  default = {}
}
