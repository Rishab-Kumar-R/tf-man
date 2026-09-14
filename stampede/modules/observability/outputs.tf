output "grafana_admin_secret_arn" {
  value = aws_secretsmanager_secret.grafana_admin.arn
}

output "prometheus_namespace" {
  value = helm_release.kube_prometheus_stack.namespace
}
