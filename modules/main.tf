module "tags" {
  source = "./local-modules/tags"

  project     = var.project_name
  environment = var.environment
  owner       = var.owner
}

module "log_bucket" {
  source  = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 4.0"

  bucket = "${var.project_name}-flow-logs-${data.aws_caller_identity.current.account_id}"

  attach_policy = true
  policy        = data.aws_iam_policy_document.flow_log_bucket_policy.json

  versioning = {
    enabled = true
  }

  server_side_encryption_configuration = {
    rule = {
      apply_server_side_encryption_by_default = {
        sse_algorithm = "AES256"
      }
    }
  }

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true

  tags = module.tags.tags
}

data "aws_iam_policy_document" "flow_log_bucket_policy" {
  statement {
    sid    = "AWSLogDeliveryWrite"
    effect = "Allow"
    principals {
      identifiers = ["delivery.logs.amazonaws.com"]
      type        = "Service"
    }
    actions   = ["s3:PutObject"]
    resources = ["arn:aws:s3:::${var.project_name}-flow-logs-${data.aws_caller_identity.current.account_id}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
    condition {
      test     = "ArnLike"
      variable = "aws:SourceArn"
      values   = ["arn:aws:ec2:${var.aws_region}:${data.aws_caller_identity.current.account_id}:vpc-flow-log/*"]
    }
  }
  statement {
    sid    = "AWSLogDeliveryAclCheck"
    effect = "Allow"
    principals {
      identifiers = ["delivery.logs.amazonaws.com"]
      type        = "Service"
    }
    actions   = ["s3:GetBucketAcl"]
    resources = ["arn:aws:s3:::${var.project_name}-flow-logs-${data.aws_caller_identity.current.account_id}"]
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
    condition {
      test     = "ArnLike"
      variable = "aws:SourceArn"
      values   = ["arn:aws:ec2:${var.aws_region}:${data.aws_caller_identity.current.account_id}:vpc-flow-log/*"]
    }
  }
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${var.project_name}-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["${var.aws_region}a", "${var.aws_region}b"]
  public_subnets  = ["10.0.1.0/24", "10.0.2.0/24"]
  private_subnets = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway = true
  single_nat_gateway = true

  enable_flow_log                   = true
  flow_log_destination_type         = "s3"
  flow_log_destination_arn          = module.log_bucket.s3_bucket_arn
  flow_log_max_aggregation_interval = 600

  tags = module.tags.tags

  depends_on = [module.log_bucket]
}

module "web_sg" {
  source  = "terraform-aws-modules/security-group/aws//modules/http-80"
  version = "~> 5.0"

  name        = "${var.project_name}-web-sg"
  vpc_id      = module.vpc.vpc_id
  description = "Allow HTTP from anywhere - web tier"

  ingress_cidr_blocks = ["0.0.0.0/0"]

  tags = module.tags.tags
}

module "db_sg" {
  source  = "terraform-aws-modules/security-group/aws//modules/postgresql"
  version = "~> 5.0"

  name        = "${var.project_name}-db-sg"
  vpc_id      = module.vpc.vpc_id
  description = "Allow Postgres only from private subnets"

  ingress_cidr_blocks = module.vpc.private_subnets_cidr_blocks

  tags = module.tags.tags
}

module "app_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "~> 6.0"

  name = "${var.project_name}-app-role"

  trust_policy_permissions = {
    EC2AssumeRole = {
      actions = ["sts:AssumeRole"]
      principals = [{
        type        = "Service"
        identifiers = ["ec2.amazonaws.com"]
      }]
    }
  }

  policies = {
    S3ReadOnly = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
  }

  tags = module.tags.tags
}
