mock_provider "aws" {}

variables {
  region            = "us-west-2"
  state_bucket_name = "investment-analytics-tofu-state-test"
}

run "protected_remote_state_foundation" {
  command = plan

  assert {
    condition     = aws_s3_bucket.state.bucket == "investment-analytics-tofu-state-test"
    error_message = "The requested remote-state bucket name was not preserved."
  }

  assert {
    condition     = aws_s3_bucket_versioning.state.versioning_configuration[0].status == "Enabled"
    error_message = "Remote state must have S3 versioning enabled."
  }

  assert {
    condition     = aws_s3_bucket_public_access_block.state.restrict_public_buckets
    error_message = "Remote state must block public bucket policies."
  }

  assert {
    condition     = aws_kms_key.state.enable_key_rotation
    error_message = "The remote-state KMS key must rotate."
  }
}

run "reject_invalid_state_bucket_name" {
  command = plan

  variables {
    state_bucket_name = "INVALID_BUCKET_NAME"
  }

  expect_failures = [var.state_bucket_name]
}
