output "argocd_namespace" {
  value = helm_release.argocd.namespace
}

output "argo_rollouts_namespace" {
  value = helm_release.argo_rollouts.namespace
}
