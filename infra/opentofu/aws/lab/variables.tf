variable "region" {
  description = "AWS region for the ephemeral lab."
  type        = string
  default     = "us-west-2"
}

variable "project" {
  description = "Short project name used in resource names and tags."
  type        = string
  default     = "investment-analytics"
}

variable "environment" {
  description = "Environment name. This stack is intentionally lab-only."
  type        = string
  default     = "aws-lab"

  validation {
    condition     = var.environment == "aws-lab"
    error_message = "This root module is limited to the aws-lab environment."
  }
}

variable "vpc_cidr" {
  description = "IPv4 CIDR for the lab VPC."
  type        = string
  default     = "10.40.0.0/16"

  validation {
    condition     = can(cidrnetmask(var.vpc_cidr))
    error_message = "vpc_cidr must be a valid IPv4 CIDR."
  }
}

variable "api_server_allowed_cidrs" {
  description = "CIDRs allowed to reach the public EKS API endpoint. Never use 0.0.0.0/0."
  type        = list(string)

  validation {
    condition = (
      length(var.api_server_allowed_cidrs) > 0 &&
      alltrue([for cidr in var.api_server_allowed_cidrs : can(cidrnetmask(cidr))]) &&
      alltrue([
        for cidr in var.api_server_allowed_cidrs :
        try(tonumber(split("/", cidr)[1]), 0) >= 24
      ])
    )
    error_message = "Provide at least one valid operator IPv4 CIDR with a /24-or-narrower prefix."
  }
}

variable "cluster_admin_principal_arn" {
  description = "IAM role ARN granted temporary EKS cluster-administrator access through an access entry."
  type        = string

  validation {
    condition     = can(regex("^arn:aws:iam::[0-9]{12}:role/.+$", var.cluster_admin_principal_arn))
    error_message = "cluster_admin_principal_arn must be an IAM role ARN, not a user or access key."
  }
}

variable "kubernetes_version" {
  description = "Supported Amazon EKS Kubernetes minor version."
  type        = string
  default     = "1.35"
}

variable "node_instance_types" {
  description = "EC2 instance types eligible for the managed node group."
  type        = list(string)
  default     = ["m7i.large", "m6i.large", "m5.large"]
}

variable "node_capacity_type" {
  description = "SPOT is cost-oriented and can be interrupted; use ON_DEMAND for controlled failure experiments."
  type        = string
  default     = "SPOT"

  validation {
    condition     = contains(["SPOT", "ON_DEMAND"], var.node_capacity_type)
    error_message = "node_capacity_type must be SPOT or ON_DEMAND."
  }
}

variable "node_min_size" {
  description = "Minimum managed-node count in each zonal node group."
  type        = number
  default     = 1
}

variable "node_desired_size" {
  description = "Desired managed-node count in each zonal node group."
  type        = number
  default     = 1
}

variable "node_max_size" {
  description = "Maximum managed-node count in each zonal node group."
  type        = number
  default     = 2
}

variable "monthly_budget_usd" {
  description = "Monthly AWS cost budget for resources matching this project and environment."
  type        = number
  default     = 100

  validation {
    condition     = var.monthly_budget_usd > 0
    error_message = "monthly_budget_usd must be greater than zero."
  }
}

variable "budget_alert_email" {
  description = "Optional email for forecast and actual budget notifications."
  type        = string
  default     = ""

  validation {
    condition     = var.budget_alert_email == "" || can(regex("^[^@[:space:]]+@[^@[:space:]]+\\.[^@[:space:]]+$", var.budget_alert_email))
    error_message = "budget_alert_email must be empty or a valid email address."
  }
}

variable "force_destroy_data" {
  description = "Allow OpenTofu to delete versioned synthetic lab data during teardown. Never enable for retained or personal data."
  type        = bool
  default     = false
}

variable "tags" {
  description = "Additional resource tags."
  type        = map(string)
  default     = {}
}
