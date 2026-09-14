output "db_endpoint" {
  value = aws_db_instance.this.address
}

output "db_port" {
  value = 5432
}

output "secret_arn" {
  value = aws_secretsmanager_secret.db.arn
}

output "security_group_id" {
  value = aws_security_group.rds.id
}
