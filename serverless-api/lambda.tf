resource "aws_lambda_function" "tasks" {
  function_name = "${var.project_name}-fn"
  role          = aws_iam_role.lambda_role.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]

  filename         = "${path.module}/lambda.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda.zip")

  environment {
    variables = {
      TABLE_NAME = aws_dynamodb_table.tasks.name
    }
  }
}
