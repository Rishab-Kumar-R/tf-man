output "vpc_id" {
  value = module.network.vpc_id
}

output "cluster_name" {
  value = module.eks_cluster.cluster_name
}

output "cluster_endpoint" {
  value = module.eks_cluster.cluster_endpoint
}

output "oidc_provider_arn" {
  value = module.eks_cluster.oidc_provider_arn
}

output "messaging_topic_arn" {
  value = module.messaging.topic_arn
}

output "waf_web_acl_arn" {
  value = module.ingress.waf_web_acl_arn
}

output "orders_db_endpoint" {
  value = module.rds_orders.db_endpoint
}

output "orders_db_secret_arn" {
  value = module.rds_orders.secret_arn
}

output "catalog_db_endpoint" {
  value = module.rds_catalog.db_endpoint
}

output "catalog_db_secret_arn" {
  value = module.rds_catalog.secret_arn
}

output "redis_endpoint" {
  value = module.redis.primary_endpoint_address
}

output "opensearch_endpoint" {
  value = module.opensearch.domain_endpoint
}

output "grafana_admin_secret_arn" {
  value = module.observability.grafana_admin_secret_arn
}

output "ecr_repository_urls" {
  value = module.ecr.repository_urls
}

output "cluster_security_group_id" {
  value = module.eks_cluster.cluster_security_group_id
}

output "karpenter_node_instance_profile_name" {
  value = module.autoscaling.node_instance_profile_name
}

output "codebuild_source_bucket" {
  value = module.codebuild.source_bucket
}

output "codebuild_project_name" {
  value = module.codebuild.project_name
}
