locals {
  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

module "network" {
  source = "./modules/network"

  project_name = var.project_name
  azs          = var.azs
  tags         = local.tags
}

module "workload" {
  source = "./modules/workload"

  project_name      = var.project_name
  vpc_id            = module.network.vpc_id
  public_subnet_ids = module.network.public_subnet_ids
  container_image   = var.container_image
  desired_count     = var.desired_count
  tags              = local.tags
}

module "observability" {
  source = "./modules/observability"

  project_name            = var.project_name
  cluster_name            = module.workload.cluster_name
  service_name            = module.workload.service_name
  log_group_name          = module.workload.log_group_name
  alb_arn_suffix          = module.workload.alb_arn_suffix
  target_group_arn_suffix = module.workload.target_group_arn_suffix
  min_healthy_count       = var.min_healthy_count
  tags                    = local.tags
}

module "self_healer" {
  source = "./modules/self-healer"

  project_name        = var.project_name
  alarm_sns_topic_arn = module.observability.sns_topic_arn
  ecs_cluster_name    = module.workload.cluster_name
  ecs_service_name    = module.workload.service_name
  desired_count       = var.desired_count
  tags                = local.tags
}

module "audit_trail" {
  source = "./modules/audit-trail"

  project_name = var.project_name
  tags         = local.tags
}

module "guardrail" {
  source = "./modules/guardrail"

  project_name = var.project_name
  tags         = local.tags

  depends_on = [module.audit_trail]
}

module "reaper" {
  source = "./modules/reaper"

  project_name         = var.project_name
  schedule_expression  = var.reaper_schedule_expression
  grace_period_seconds = var.reaper_grace_period_seconds
  tags                 = local.tags
}
