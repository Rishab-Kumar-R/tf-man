resource "aws_s3_bucket" "demo" {
  bucket = "example-consumer-demo-bucket-${random_id.suffix.hex}"
}

resource "random_id" "suffix" {
  byte_length = 4
}
