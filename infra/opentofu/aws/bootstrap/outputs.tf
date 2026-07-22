output "state_bucket_name" {
  description = "S3 bucket used by the AWS lab backend."
  value       = aws_s3_bucket.state.id
}

output "state_kms_key_arn" {
  description = "KMS key used to encrypt OpenTofu state."
  value       = aws_kms_key.state.arn
}

output "backend_init_arguments" {
  description = "Non-secret arguments for initializing the AWS lab backend."
  value = [
    "-backend-config=bucket=${aws_s3_bucket.state.id}",
    "-backend-config=key=aws-lab/platform.tfstate",
    "-backend-config=region=${var.region}",
    "-backend-config=kms_key_id=${aws_kms_key.state.arn}",
  ]
}
