output "alb_dns_name" {
  value = module.workload.alb_dns_name
}

output "ecs_cluster_name" {
  value = module.workload.cluster_name
}

output "ecs_service_name" {
  value = module.workload.service_name
}

output "dashboard_name" {
  value = module.observability.dashboard_name
}

output "alarm_arn" {
  value = module.observability.alarm_arn
}

output "sns_topic_arn" {
  value = module.observability.sns_topic_arn
}

output "self_healer_lambda_name" {
  value = module.self_healer.lambda_function_name
}

output "self_healer_lambda_arn" {
  value = module.self_healer.lambda_function_arn
}

output "guardrail_lambda_name" {
  value = module.guardrail.lambda_function_name
}

output "guardrail_sns_topic_arn" {
  value = module.guardrail.sns_topic_arn
}

output "reaper_lambda_name" {
  value = module.reaper.lambda_function_name
}

output "reaper_sns_topic_arn" {
  value = module.reaper.sns_topic_arn
}
