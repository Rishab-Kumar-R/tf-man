variable "project" {
  type = string
}

variable "environment" {
  type = string
}

variable "owner" {
  type    = string
  default = "rishabkumar"
}

variable "additional_tags" {
  type    = map(string)
  default = {}
}
