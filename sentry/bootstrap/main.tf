resource "aws_s3_bucket" "sentry_state" {
  bucket = "sentry-tfstate-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket_versioning" "sentry_state" {
  bucket = aws_s3_bucket.sentry_state.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "sentry_state" {
  bucket = aws_s3_bucket.sentry_state.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "sentry_state" {
  bucket = aws_s3_bucket.sentry_state.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

data "aws_caller_identity" "current" {}
