variable "project_name" {
  type = string
}

variable "lambda_zip_path" {
  type    = string
  default = "lambdas/guardrail/function.zip"
}

variable "tags" {
  type    = map(string)
  default = {}
}
