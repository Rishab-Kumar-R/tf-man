terraform {
  backend "s3" {
    bucket       = "GENERATED_BUCKET_ID"
    key          = "workspace/terraform.tfstate"
    region       = "ap-south-1"
    encrypt      = true
    use_lockfile = true
  }
}
