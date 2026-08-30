locals {
  env = terraform.workspace

  env_config = {
    default = {
      queue_retention_seconds = 60
      max_receive_count       = 2
      alarm_enabled           = false
      visibility_timeout      = 30
    }
    dev = {
      queue_retention_seconds = 3600
      max_receive_count       = 3
      alarm_enabled           = false
      visibility_timeout      = 30
    }
    staging = {
      queue_retention_seconds = 86400
      max_receive_count       = 5
      alarm_enabled           = true
      visibility_timeout      = 60
    }
    prod = {
      queue_retention_seconds = 1209600
      max_receive_count       = 5
      alarm_enabled           = true
      visibility_timeout      = 60
    }
  }

  config = lookup(local.env_config, local.env, local.env_config["default"])

  name_prefix = "${var.project_name}-${local.env}"
}
