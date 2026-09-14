variable "project_name" {
  type = string
}

variable "cluster_name" {
  description = "Anticipated EKS cluster name, used for subnet auto-discovery tags"
  type        = string
}

variable "vpc_cidr" {
  type    = string
  default = "10.60.0.0/16"
}

variable "azs" {
  type = list(string)
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.60.0.0/24", "10.60.1.0/24"]
}

variable "private_subnet_cidrs" {
  type    = list(string)
  default = ["10.60.10.0/24", "10.60.11.0/24"]
}

variable "single_nat_gateway" {
  description = "Use one shared NAT gateway instead of one per AZ"
  type        = bool
  default     = true
}

variable "tags" {
  type    = map(string)
  default = {}
}
