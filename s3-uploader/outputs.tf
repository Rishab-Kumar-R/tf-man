output "bucket_name" {
  value = aws_s3_bucket.upload_bucket.id
}

output "uploaded_files" {
  value = keys(aws_s3_object.uploads)
}
