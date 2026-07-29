locals {
  name = "${var.project}-${var.environment}"
  azs  = slice(data.aws_availability_zones.available.names, 0, 3)

  common_tags = merge({
    Project     = var.project
    Environment = var.environment
    CostScope   = "${var.project}-${var.environment}"
    ManagedBy   = "OpenTofu"
  }, var.tags)

  application_service_accounts = {
    raw_event_archiver = {
      namespace       = "analytics-apps"
      service_account = "raw-event-archiver"
    }
    portfolio_analytics = {
      namespace       = "analytics-apps"
      service_account = "portfolio-analytics"
    }
    portfolio_api = {
      namespace       = "analytics-apps"
      service_account = "portfolio-api"
    }
  }

  pod_identity_service_accounts = merge(local.application_service_accounts, {
    ebs_csi = {
      namespace       = "kube-system"
      service_account = "ebs-csi-controller-sa"
    }
  })
}
