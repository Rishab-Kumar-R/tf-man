output "domain_endpoint" {
  value = aws_opensearch_domain.this.endpoint
}

output "domain_arn" {
  value = aws_opensearch_domain.this.arn
}

output "security_group_id" {
  value = aws_security_group.opensearch.id
}
