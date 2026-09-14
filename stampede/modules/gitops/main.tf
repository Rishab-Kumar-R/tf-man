resource "helm_release" "argocd" {
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  namespace        = var.argocd_namespace
  version          = var.argocd_chart_version
  create_namespace = true
  timeout = 900
}

resource "helm_release" "argo_rollouts" {
  name             = "argo-rollouts"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-rollouts"
  namespace        = var.argo_rollouts_namespace
  version          = var.argo_rollouts_chart_version
  create_namespace = true
}
