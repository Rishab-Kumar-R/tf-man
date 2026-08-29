variable "aws_region" {
  default = "ap-south-1"
}

variable "bucket_name" {
  description = "must be globally unique"
  type        = string
}

variable "upload_folder" {
  default = "files-to-upload"
}
