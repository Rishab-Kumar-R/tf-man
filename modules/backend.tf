terraform {
  backend "s3" {
    bucket       = "REPLACE_WITH_STATE_BUCKET"
    key          = "modules/terraform.tfstate"
    region       = "ap-south-1"
    encrypt      = true
    use_lockfile = true
  }
}
