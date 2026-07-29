variable "region" {
  description = "AWS region that stores the OpenTofu state."
  type        = string
  default     = "us-west-2"
}

variable "state_bucket_name" {
  description = "Globally unique S3 bucket name for OpenTofu state."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$", var.state_bucket_name))
    error_message = "state_bucket_name must be a valid lowercase S3 bucket name."
  }
}

variable "project" {
  description = "Project tag applied to state-foundation resources."
  type        = string
  default     = "investment-analytics"
}

variable "tags" {
  description = "Additional resource tags."
  type        = map(string)
  default     = {}
}
