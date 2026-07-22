output "account_id" {
  description = "AWS account that owns the lab."
  value       = data.aws_caller_identity.current.account_id
}

output "region" {
  description = "AWS region containing the lab."
  value       = var.region
}

output "cluster_name" {
  description = "Amazon EKS cluster name."
  value       = aws_eks_cluster.this.name
}

output "analytics_bucket" {
  description = "Encrypted S3 bucket replacing local Garage."
  value       = aws_s3_bucket.analytics.id
}

output "ecr_repository_urls" {
  description = "Immutable application image repositories."
  value       = { for name, repository in aws_ecr_repository.application : name => repository.repository_url }
}

output "pod_identity_role_arns" {
  description = "Least-privilege roles associated with Kubernetes service accounts."
  value = {
    raw_event_archiver  = aws_iam_role.archive.arn
    portfolio_analytics = aws_iam_role.analytics.arn
    portfolio_api       = aws_iam_role.api.arn
  }
}

output "configure_kubectl" {
  description = "Command to add the lab EKS cluster to kubeconfig."
  value       = "aws eks update-kubeconfig --region ${var.region} --name ${aws_eks_cluster.this.name}"
}
