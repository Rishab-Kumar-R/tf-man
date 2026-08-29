output "alb_dns_name" {
  value = aws_lb.web.dns_name
}

output "instance_ids" {
  value = { for k, v in aws_instance.web : k => v.id }
}

output "instance_public_ips" {
  value = { for k, v in aws_instance.web : k => v.public_ip }
}
