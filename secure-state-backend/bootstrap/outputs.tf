output "state_bucket_name" {
  value = aws_s3_bucket.state.id
}

output "lock_table_name" {
  value = aws_dynamodb_table.lock.name
}

output "kms_key_arn" {
  value = aws_kms_key.state.arn
}

output "admin_role_arn" {
  value = aws_iam_role.tf_admin.arn
}

output "readonly_role_arn" {
  value = aws_iam_role.tf_readonly.arn
}
