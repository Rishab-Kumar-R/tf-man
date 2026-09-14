variable "project_name" {
  type = string
}

variable "aws_region" {
  type = string
}

variable "ecr_repository_arns" {
  type        = map(string)
  description = "Map of service name -> ECR repository ARN, scopes the build role to push only these repos."
}

variable "services" {
  type        = list(string)
  description = "Service directory names under apps/, built in one CodeBuild run."
}

variable "tags" {
  type    = map(string)
  default = {}
}
