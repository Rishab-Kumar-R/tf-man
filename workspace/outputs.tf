output "workspace" {
  value = local.env
}

output "sns_topic_arn" {
  value = aws_sns_topic.events.arn
}

output "sqs_queue_url" {
  value = aws_sqs_queue.main.id
}

output "queue_retention_seconds" {
  value = local.config.queue_retention_seconds
}

output "alarm_enabled" {
  value = local.config.alarm_enabled
}
