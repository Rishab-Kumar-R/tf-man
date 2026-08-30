resource "aws_dynamodb_table" "tasks" {
  name         = "${var.project_name}-table"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }
}
