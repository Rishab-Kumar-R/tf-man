output "state_bucket_name" {
  value = aws_s3_bucket.stampede_state.id
}

output "state_bucket_arn" {
  value = aws_s3_bucket.stampede_state.arn
}
