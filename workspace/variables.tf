variable "aws_region" {
  default = "ap-south-1"
}

variable "project_name" {
  default = "notif-pipeline"
}

variable "alert_email" {
  description = "email to subscribe to alarm notifications (staging/prod only)"
  type        = string
  default     = "dumb.head.mail@gmail.com"
}
