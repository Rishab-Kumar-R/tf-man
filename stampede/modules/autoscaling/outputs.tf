output "controller_role_arn" {
  value = aws_iam_role.controller.arn
}

output "node_role_arn" {
  value = aws_iam_role.node.arn
}

output "node_instance_profile_name" {
  value = aws_iam_instance_profile.node.name
}

output "interruption_queue_name" {
  value = aws_sqs_queue.interruption.name
}
