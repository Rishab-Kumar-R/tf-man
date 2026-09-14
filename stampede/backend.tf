terraform {
  backend "s3" {
    key          = "stampede/terraform.tfstate"
    region       = "ap-south-1"
    encrypt      = true
    use_lockfile = true
  }
}
