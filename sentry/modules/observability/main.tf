resource "aws_sns_topic" "alarms" {
  name = "${var.project_name}-alarms"
  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "running_task_count" {
  alarm_name          = "${var.project_name}-running-task-count-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = var.alarm_evaluation_periods
  period              = var.alarm_period_seconds
  threshold           = var.min_healthy_count
  statistic           = "Average"
  namespace           = "ECS/ContainerInsights"
  metric_name         = "RunningTaskCount"
  treat_missing_data  = "breaching"

  dimensions = {
    ClusterName = var.cluster_name
    ServiceName = var.service_name
  }

  alarm_description = "Fires only after ${var.alarm_evaluation_periods} consecutive periods below ${var.min_healthy_count} running tasks - filters out ECS's own instant self-heal on a single killed task"
  alarm_actions     = [aws_sns_topic.alarms.arn]
  ok_actions        = [aws_sns_topic.alarms.arn]

  tags = var.tags
}

resource "aws_cloudwatch_dashboard" "this" {
  dashboard_name = "${var.project_name}-dashboard"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "ECS Running Task Count"
          region = data.aws_region.current.name
          metrics = [
            ["ECS/ContainerInsights", "RunningTaskCount", "ClusterName", var.cluster_name, "ServiceName", var.service_name]
          ]
          period = var.alarm_period_seconds
          stat   = "Average"
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "ALB Requests / 5xx"
          region = data.aws_region.current.name
          metrics = [
            ["AWS/ApplicationELB", "RequestCount", "LoadBalancer", var.alb_arn_suffix, { stat = "Sum" }],
            ["AWS/ApplicationELB", "HTTPCode_Target_5XX_Count", "LoadBalancer", var.alb_arn_suffix, { stat = "Sum" }]
          ]
          period = var.alarm_period_seconds
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6
        properties = {
          title  = "Target Group Healthy Hosts"
          region = data.aws_region.current.name
          metrics = [
            ["AWS/ApplicationELB", "HealthyHostCount", "TargetGroup", var.target_group_arn_suffix, "LoadBalancer", var.alb_arn_suffix]
          ]
          period = var.alarm_period_seconds
          stat   = "Average"
        }
      },
      {
        type   = "log"
        x      = 12
        y      = 6
        width  = 12
        height = 6
        properties = {
          title  = "Recent Task Logs"
          region = data.aws_region.current.name
          query  = "SOURCE '${var.log_group_name}' | fields @timestamp, @message | sort @timestamp desc | limit 20"
        }
      }
    ]
  })
}

data "aws_region" "current" {}
