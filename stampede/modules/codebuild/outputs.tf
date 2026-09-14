output "source_bucket" {
  value = aws_s3_bucket.source.bucket
}

output "project_name" {
  value = aws_codebuild_project.images.name
}
