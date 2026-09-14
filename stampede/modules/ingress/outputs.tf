output "waf_web_acl_arn" {
  value = aws_wafv2_web_acl.this.arn
}

output "load_balancer_controller_service_account" {
  value = "${var.namespace}/aws-load-balancer-controller"
}
