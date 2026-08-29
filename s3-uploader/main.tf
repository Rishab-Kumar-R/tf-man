resource "aws_s3_bucket" "upload_bucket" {
  bucket = var.bucket_name
}

resource "aws_s3_bucket_public_access_block" "block" {
  bucket                  = aws_s3_bucket.upload_bucket.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "versioning" {
  bucket = aws_s3_bucket.upload_bucket.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_object" "uploads" {
  for_each = fileset("${path.module}/${var.upload_folder}", "**")

  bucket = aws_s3_bucket.upload_bucket.id
  key    = each.value
  source = "${path.module}/${var.upload_folder}/${each.value}"
  etag   = filemd5("${path.module}/${var.upload_folder}/${each.value}")

  tags = {
    UploadedBy = "terraform"
  }
}
