output "vpc_id" {
  value = module.vpc.vpc_id
}

output "public_subnets" {
  value = module.vpc.public_subnets
}

output "private_subnets" {
  value = module.vpc.private_subnets
}

output "flow_log_bucket" {
  value = module.log_bucket.s3_bucket_id
}

output "web_sg_id" {
  value = module.web_sg.security_group_id
}

output "db_sg_id" {
  value = module.db_sg.security_group_id
}

output "app_role_arn" {
  value = module.app_role.arn
}

output "tags_applied" {
  value = module.tags.tags
}
