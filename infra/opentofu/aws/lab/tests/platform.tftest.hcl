mock_provider "aws" {
  mock_data "aws_caller_identity" {
    defaults = {
      account_id = "123456789012"
      arn        = "arn:aws:iam::123456789012:role/test"
      user_id    = "test"
    }
  }

  mock_data "aws_availability_zones" {
    defaults = {
      names = ["us-west-2a", "us-west-2b", "us-west-2c"]
    }
  }

  mock_data "aws_eks_addon_version" {
    defaults = {
      version = "v1.0.0-eksbuild.1"
    }
  }

  mock_data "aws_iam_policy_document" {
    defaults = {
      json = "{\"Version\":\"2012-10-17\",\"Statement\":[]}"
    }
  }

  mock_resource "aws_iam_role" {
    defaults = {
      arn = "arn:aws:iam::123456789012:role/mock-platform-role"
      id  = "mock-platform-role"
    }
  }

  mock_resource "aws_cloudwatch_log_group" {
    defaults = {
      arn = "arn:aws:logs:us-west-2:123456789012:log-group:mock-platform-log"
    }
  }

  mock_resource "aws_kms_key" {
    defaults = {
      arn    = "arn:aws:kms:us-west-2:123456789012:key/00000000-0000-0000-0000-000000000000"
      key_id = "00000000-0000-0000-0000-000000000000"
    }
  }

  mock_resource "aws_launch_template" {
    defaults = {
      id             = "lt-0123456789abcdef0"
      latest_version = 1
    }
  }
}

variables {
  region                      = "us-west-2"
  api_server_allowed_cidrs    = ["198.51.100.10/32"]
  cluster_admin_principal_arn = "arn:aws:iam::123456789012:role/federated-platform-admin"
}

run "three_zone_least_privilege_lab" {
  command = plan

  assert {
    condition     = length(aws_subnet.public) == 3
    error_message = "The Kafka lab must span three Availability Zones."
  }

  assert {
    condition = (
      aws_eks_cluster.this.vpc_config[0].endpoint_private_access &&
      aws_eks_cluster.this.vpc_config[0].endpoint_public_access &&
      aws_eks_cluster.this.vpc_config[0].public_access_cidrs == toset(["198.51.100.10/32"])
    )
    error_message = "The EKS API must retain private access and restrict public access to the operator CIDR."
  }

  assert {
    condition = (
      length(aws_eks_node_group.platform) == 3 &&
      alltrue([
        for node_group in aws_eks_node_group.platform :
        node_group.scaling_config[0].min_size == 1 &&
        node_group.scaling_config[0].desired_size == 1 &&
        node_group.scaling_config[0].max_size == 2
      ])
    )
    error_message = "Each Availability Zone must have a bounded node group for EBS-backed Kafka."
  }

  assert {
    condition     = aws_s3_bucket.analytics.force_destroy == false
    error_message = "Synthetic data destruction must require an explicit opt-in."
  }

  assert {
    condition     = alltrue([for repository in aws_ecr_repository.application : repository.image_tag_mutability == "IMMUTABLE"])
    error_message = "Every ECR repository must reject mutable image tags."
  }

  assert {
    condition = alltrue([
      for tag_specification in aws_launch_template.node.tag_specifications :
      tag_specification.tags["CostScope"] == "investment-analytics-aws-lab"
    ])
    error_message = "Managed-node instances and root volumes must carry the budget cost tag."
  }

  assert {
    condition = (
      aws_eks_pod_identity_association.archive.service_account == "raw-event-archiver" &&
      aws_eks_pod_identity_association.analytics.service_account == "portfolio-analytics" &&
      aws_eks_pod_identity_association.api.service_account == "portfolio-api"
    )
    error_message = "Application roles must map to distinct workload service accounts."
  }
}

run "reject_world_open_api" {
  command = plan

  variables {
    api_server_allowed_cidrs = ["0.0.0.0/0"]
  }

  expect_failures = [var.api_server_allowed_cidrs]
}

run "reject_broad_api_cidr" {
  command = plan

  variables {
    api_server_allowed_cidrs = ["10.0.0.0/8"]
  }

  expect_failures = [var.api_server_allowed_cidrs]
}

run "reject_non_lab_environment" {
  command = plan

  variables {
    environment = "production"
  }

  expect_failures = [var.environment]
}

run "reject_inverted_node_scaling" {
  command = plan

  variables {
    node_min_size     = 2
    node_desired_size = 1
    node_max_size     = 2
  }

  expect_failures = [aws_eks_node_group.platform]
}
