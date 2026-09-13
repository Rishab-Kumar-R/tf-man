terraform {
  backend "s3" {
    bucket       = "sentry-tfstate-<ACCOUNT_ID>"
    key          = "sentry/terraform.tfstate"
    region       = "ap-south-1"
    encrypt      = true
    use_lockfile = true
  }
}
