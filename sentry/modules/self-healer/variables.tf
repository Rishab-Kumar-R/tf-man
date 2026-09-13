variable "project_name" {
  type = string
}

variable "alarm_sns_topic_arn" {
  type = string
}

variable "ecs_cluster_name" {
  type = string
}

variable "ecs_service_name" {
  type = string
}

variable "desired_count" {
  type = number
}

variable "lambda_zip_path" {
  type    = string
  default = "lambdas/self-healer/function.zip"
}

variable "tags" {
  type    = map(string)
  default = {}
}
