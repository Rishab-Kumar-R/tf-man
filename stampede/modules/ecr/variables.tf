variable "project_name" {
  type = string
}

variable "repository_names" {
  type = list(string)
}

variable "image_tag_mutability" {
  description = "IMMUTABLE: every build gets a unique tag, never overwritten. A manifest change is what triggers a new deploy."
  type        = string
  default     = "IMMUTABLE"
}

variable "keep_last_n_images" {
  type    = number
  default = 10
}

variable "tags" {
  type    = map(string)
  default = {}
}
