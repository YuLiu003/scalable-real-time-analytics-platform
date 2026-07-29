data "aws_iam_policy_document" "pod_identity_assume" {
  for_each = local.pod_identity_service_accounts

  statement {
    actions = ["sts:AssumeRole", "sts:TagSession"]
    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:RequestTag/kubernetes-namespace"
      values   = [each.value.namespace]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:RequestTag/kubernetes-service-account"
      values   = [each.value.service_account]
    }
  }
}

resource "aws_iam_role" "archive" {
  name               = "${local.name}-archive"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_assume["raw_event_archiver"].json
}

data "aws_iam_policy_document" "archive" {
  statement {
    sid       = "ListArchivePrefixes"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.analytics.arn]
    condition {
      test     = "StringLike"
      variable = "s3:prefix"
      values   = ["bronze/*", "quarantine/*"]
    }
  }

  statement {
    sid       = "ReadWriteArchiveObjects"
    actions   = ["s3:GetObject", "s3:PutObject"]
    resources = ["${aws_s3_bucket.analytics.arn}/bronze/*", "${aws_s3_bucket.analytics.arn}/quarantine/*"]
  }

  statement {
    sid       = "UseDataKey"
    actions   = ["kms:Decrypt", "kms:GenerateDataKey"]
    resources = [aws_kms_key.platform.arn]
    condition {
      test     = "StringEquals"
      variable = "kms:ViaService"
      values   = ["s3.${var.region}.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "archive" {
  name   = "s3-archive"
  role   = aws_iam_role.archive.id
  policy = data.aws_iam_policy_document.archive.json
}

resource "aws_iam_role" "analytics" {
  name               = "${local.name}-analytics"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_assume["portfolio_analytics"].json
}

data "aws_iam_policy_document" "analytics" {
  statement {
    sid       = "ListDataPrefixes"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.analytics.arn]
    condition {
      test     = "StringLike"
      variable = "s3:prefix"
      values   = ["bronze/*", "silver/*", "gold/*"]
    }
  }

  statement {
    sid     = "ReadWriteAnalyticsObjects"
    actions = ["s3:GetObject", "s3:PutObject"]
    resources = [
      "${aws_s3_bucket.analytics.arn}/bronze/*",
      "${aws_s3_bucket.analytics.arn}/silver/*",
      "${aws_s3_bucket.analytics.arn}/gold/*",
    ]
  }

  statement {
    sid       = "UseDataKey"
    actions   = ["kms:Decrypt", "kms:GenerateDataKey"]
    resources = [aws_kms_key.platform.arn]
    condition {
      test     = "StringEquals"
      variable = "kms:ViaService"
      values   = ["s3.${var.region}.amazonaws.com"]
    }
  }

  statement {
    sid     = "ResetDerivedAnalyticsObjects"
    actions = ["s3:DeleteObject"]
    resources = [
      "${aws_s3_bucket.analytics.arn}/silver/*",
      "${aws_s3_bucket.analytics.arn}/gold/*",
    ]
  }
}

resource "aws_iam_role_policy" "analytics" {
  name   = "s3-analytics"
  role   = aws_iam_role.analytics.id
  policy = data.aws_iam_policy_document.analytics.json
}

resource "aws_iam_role" "api" {
  name               = "${local.name}-api"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_assume["portfolio_api"].json
}

data "aws_iam_policy_document" "api" {
  statement {
    sid       = "ListGoldPrefix"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.analytics.arn]
    condition {
      test     = "StringLike"
      variable = "s3:prefix"
      values   = ["gold/*"]
    }
  }

  statement {
    sid       = "ReadGoldObjects"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.analytics.arn}/gold/*"]
  }

  statement {
    sid       = "DecryptAnalyticsData"
    actions   = ["kms:Decrypt"]
    resources = [aws_kms_key.platform.arn]
    condition {
      test     = "StringEquals"
      variable = "kms:ViaService"
      values   = ["s3.${var.region}.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "api" {
  name   = "s3-gold-read"
  role   = aws_iam_role.api.id
  policy = data.aws_iam_policy_document.api.json
}

resource "aws_eks_pod_identity_association" "archive" {
  cluster_name    = aws_eks_cluster.this.name
  namespace       = local.application_service_accounts.raw_event_archiver.namespace
  service_account = local.application_service_accounts.raw_event_archiver.service_account
  role_arn        = aws_iam_role.archive.arn

  depends_on = [aws_eks_addon.this["eks-pod-identity-agent"]]
}

resource "aws_eks_pod_identity_association" "analytics" {
  cluster_name    = aws_eks_cluster.this.name
  namespace       = local.application_service_accounts.portfolio_analytics.namespace
  service_account = local.application_service_accounts.portfolio_analytics.service_account
  role_arn        = aws_iam_role.analytics.arn

  depends_on = [aws_eks_addon.this["eks-pod-identity-agent"]]
}

resource "aws_eks_pod_identity_association" "api" {
  cluster_name    = aws_eks_cluster.this.name
  namespace       = local.application_service_accounts.portfolio_api.namespace
  service_account = local.application_service_accounts.portfolio_api.service_account
  role_arn        = aws_iam_role.api.arn

  depends_on = [aws_eks_addon.this["eks-pod-identity-agent"]]
}

resource "aws_iam_role" "ebs_csi" {
  name               = "${local.name}-ebs-csi"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_assume["ebs_csi"].json
}

resource "aws_iam_role_policy_attachment" "ebs_csi" {
  role       = aws_iam_role.ebs_csi.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
}
