resource "aws_sqs_queue" "dlq" {
  name                      = "${local.name_prefix}-dlq"
  message_retention_seconds = local.config.queue_retention_seconds

  tags = {
    Environment = local.env
  }
}

resource "aws_sqs_queue" "main" {
  name                       = "${local.name_prefix}-queue"
  message_retention_seconds  = local.config.queue_retention_seconds
  visibility_timeout_seconds = local.config.visibility_timeout

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq.arn
    maxReceiveCount     = local.config.max_receive_count
  })

  tags = {
    Environment = local.env
  }
}

resource "aws_sns_topic" "events" {
  name = "${local.name_prefix}-topic"

  tags = {
    Environment = local.env
  }
}

resource "aws_sns_topic_subscription" "to_queue" {
  topic_arn = aws_sns_topic.events.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.main.arn
}

resource "aws_sqs_queue_policy" "allow_sns" {
  queue_url = aws_sqs_queue.main.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "sns.amazonaws.com" }
      Action    = "sqs:SendMessage"
      Resource  = aws_sqs_queue.main.arn
      Condition = {
        ArnEquals = { "aws:SourceArn" = aws_sns_topic.events.arn }
      }
    }]
  })
}
