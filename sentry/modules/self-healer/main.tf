data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  service_arn = "arn:aws:ecs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:service/${var.ecs_cluster_name}/${var.ecs_service_name}"
}

resource "aws_iam_role" "lambda" {
  name = "${var.project_name}-self-healer-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = var.tags
}

resource "aws_iam_role_policy" "ecs_scoped" {
  name = "${var.project_name}-self-healer-ecs-policy"
  role = aws_iam_role.lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["ecs:UpdateService", "ecs:DescribeServices"]
      Resource = [local.service_arn]
    }]
  })
}

resource "aws_iam_role_policy_attachment" "logs" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "this" {
  function_name    = "${var.project_name}-self-healer"
  role             = aws_iam_role.lambda.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  filename         = "${path.root}/${var.lambda_zip_path}"
  source_code_hash = filebase64sha256("${path.root}/${var.lambda_zip_path}")
  timeout          = 30

  environment {
    variables = {
      ECS_CLUSTER   = var.ecs_cluster_name
      ECS_SERVICE   = var.ecs_service_name
      DESIRED_COUNT = tostring(var.desired_count)
    }
  }

  tags = var.tags
}

resource "aws_lambda_permission" "sns_invoke" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.this.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = var.alarm_sns_topic_arn
}

resource "aws_sns_topic_subscription" "healer" {
  topic_arn = var.alarm_sns_topic_arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.this.arn
}
