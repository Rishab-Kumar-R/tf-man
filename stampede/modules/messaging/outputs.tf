output "topic_arn" {
  value = aws_sns_topic.events.arn
}

output "queue_arns" {
  value = { for k, v in aws_sqs_queue.queue : k => v.arn }
}

output "queue_urls" {
  value = { for k, v in aws_sqs_queue.queue : k => v.url }
}

output "dlq_arns" {
  value = { for k, v in aws_sqs_queue.dlq : k => v.arn }
}
