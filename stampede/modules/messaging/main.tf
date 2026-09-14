resource "aws_sns_topic" "events" {
  name = "${var.project_name}-events"
  tags = var.tags
}

resource "aws_sqs_queue" "dlq" {
  for_each = var.subscribers

  name                      = "${var.project_name}-${each.key}-dlq"
  message_retention_seconds = 1209600 # 14 days

  tags = var.tags
}

resource "aws_sqs_queue" "queue" {
  for_each = var.subscribers

  name                       = "${var.project_name}-${each.key}"
  visibility_timeout_seconds = var.visibility_timeout_seconds
  message_retention_seconds  = var.message_retention_seconds

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq[each.key].arn
    maxReceiveCount     = var.max_receive_count
  })

  tags = var.tags
}

resource "aws_sqs_queue_policy" "allow_sns" {
  for_each = var.subscribers

  queue_url = aws_sqs_queue.queue[each.key].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "sns.amazonaws.com" }
      Action    = "sqs:SendMessage"
      Resource  = aws_sqs_queue.queue[each.key].arn
      Condition = {
        ArnEquals = {
          "aws:SourceArn" = aws_sns_topic.events.arn
        }
      }
    }]
  })
}

resource "aws_sns_topic_subscription" "this" {
  for_each = var.subscribers

  topic_arn                = aws_sns_topic.events.arn
  protocol                 = "sqs"
  endpoint                 = aws_sqs_queue.queue[each.key].arn
  filter_policy            = each.value.filter_policy
  raw_message_delivery     = true

  depends_on = [aws_sqs_queue_policy.allow_sns]
}
