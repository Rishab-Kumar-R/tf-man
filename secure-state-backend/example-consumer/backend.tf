terraform {
  backend "s3" {
    bucket         = "GENERATED_BUCKET_ID"
    key            = "example-consumer/terraform.tfstate"
    region         = "ap-south-1"
    dynamodb_table = "tf-state-locks"
    encrypt        = true
    kms_key_id     = "GENERATED_KMS_ARN"
  }
}
