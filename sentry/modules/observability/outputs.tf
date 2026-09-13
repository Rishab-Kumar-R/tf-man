output "dashboard_name" {
  value = aws_cloudwatch_dashboard.this.dashboard_name
}

output "alarm_arn" {
  value = aws_cloudwatch_metric_alarm.running_task_count.arn
}

output "sns_topic_arn" {
  value = aws_sns_topic.alarms.arn
}
