resource "aws_kms_key" "state" {
  description             = "KMS key for encrypting tf state"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "EnableRootAccountFullAccess"
        Effect = "Allow"
        Principal = {
          AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
        }
        Action   = "kms:*"
        Resource = "*"
      },
      {
        Sid    = "AllowTerraformRolesUseOfKey"
        Effect = "Allow"
        Principal = {
          AWS = [
            aws_iam_role.tf_admin.arn,
            aws_iam_role.tf_readonly.arn
          ]
        }
        Action = [
          "kms:Decrypt",
          "kms:GenerateDataKey"
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_kms_alias" "state" {
  name          = "alias/${var.project_name}-state-key"
  target_key_id = aws_kms_key.state.key_id
}
