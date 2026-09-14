resource "random_password" "grafana_admin" {
  length           = 20
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "aws_secretsmanager_secret" "grafana_admin" {
  name = "${var.project_name}-grafana-admin"
  recovery_window_in_days = 0
  tags                     = var.tags
}

resource "aws_secretsmanager_secret_version" "grafana_admin" {
  secret_id = aws_secretsmanager_secret.grafana_admin.id

  secret_string = jsonencode({
    username = "admin"
    password = random_password.grafana_admin.result
  })
}

resource "helm_release" "kube_prometheus_stack" {
  name             = "kube-prometheus-stack"
  repository       = "https://prometheus-community.github.io/helm-charts"
  chart            = "kube-prometheus-stack"
  namespace        = var.namespace
  version          = var.chart_version
  create_namespace = true
  timeout = 900

  set_sensitive = [
    {
      name  = "grafana.adminPassword"
      value = random_password.grafana_admin.result
    },
  ]
}

resource "aws_eks_addon" "cloudwatch_observability" {
  cluster_name = var.cluster_name
  addon_name   = "amazon-cloudwatch-observability"
  addon_version            = "v6.6.0-eksbuild.1"
  service_account_role_arn = var.container_insights_role_arn

  tags = var.tags
}

