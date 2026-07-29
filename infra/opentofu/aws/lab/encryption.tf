resource "aws_kms_key" "platform" {
  description             = "EKS secrets, S3 data, and ECR encryption for ${local.name}"
  deletion_window_in_days = 7
  enable_key_rotation     = true
}

resource "aws_kms_alias" "platform" {
  name          = "alias/${local.name}"
  target_key_id = aws_kms_key.platform.key_id
}
