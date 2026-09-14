variable "project_name" {
  description = "Name prefix for all stampede resources"
  type        = string
  default     = "stampede"
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
