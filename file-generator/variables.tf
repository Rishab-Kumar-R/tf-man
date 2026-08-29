variable "services" {
  description = "Map of service name to config"
  type = map(object({
    port     = number
    replicas = number
    image    = string
  }))
}

variable "owner" {
  default = "rishabkumar"
}
