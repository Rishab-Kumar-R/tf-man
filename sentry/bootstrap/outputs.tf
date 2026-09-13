output "state_bucket_name" {
  value = aws_s3_bucket.sentry_state.id
}

output "state_bucket_arn" {
  value = aws_s3_bucket.sentry_state.arn
}
