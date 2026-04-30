terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.23"
    }
    helm = {
      source  = "hashicorp/helm"
      version = ">= 2.11"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}
data "aws_region" "current" {}

# =============================================================================
# IAM Role for EKS Cluster
# =============================================================================

resource "aws_iam_role" "cluster" {
  count = var.iam_role_arn == "" ? 1 : 0

  name = "${var.cluster_name}-cluster-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "eks.amazonaws.com"
        }
      }
    ]
  })

  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "cluster_policy" {
  count = var.iam_role_arn == "" ? 1 : 0

  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.cluster[0].name
}

resource "aws_iam_role_policy_attachment" "cluster_vpc_policy" {
  count = var.iam_role_arn == "" ? 1 : 0

  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSVPCResourceController"
  role       = aws_iam_role.cluster[0].name
}

# =============================================================================
# EKS Cluster
# =============================================================================

resource "aws_eks_cluster" "main" {
  name     = var.cluster_name
  version  = var.cluster_version
  role_arn = var.iam_role_arn != "" ? var.iam_role_arn : aws_iam_role.cluster[0].arn

  vpc_config {
    subnet_ids              = var.subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = true
    security_group_ids      = [aws_security_group.cluster[0].id]
  }

  enabled_cluster_log_types = var.enable_cluster_logging ? var.cluster_logging_types : []

  encryption_config {
    provider {
      key_arn = var.enable_secrets_encryption && var.kms_key_id != "" ? var.kms_key_id : aws_kms_key.cluster[0].arn
    }
    resources = ["secrets"]
  }

  tags = merge(var.tags, {
    Name = var.cluster_name
  })

  depends_on = [
    aws_iam_role_policy_attachment.cluster_policy,
    aws_iam_role_policy_attachment.cluster_vpc_policy,
    aws_security_group_rule.cluster_ingress,
  ]
}

# =============================================================================
# KMS Key for Secrets Encryption
# =============================================================================

resource "aws_kms_key" "cluster" {
  count = var.enable_secrets_encryption && var.kms_key_id == "" ? 1 : 0

  description             = "EKS cluster secrets encryption key for ${var.cluster_name}"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = merge(var.tags, {
    Name = "${var.cluster_name}-secrets-key"
  })
}

resource "aws_kms_alias" "cluster" {
  count = var.enable_secrets_encryption && var.kms_key_id == "" ? 1 : 0

  name          = "alias/${var.cluster_name}-secrets"
  target_key_id = aws_kms_key.cluster[0].key_id
}

# =============================================================================
# Security Group for EKS Cluster
# =============================================================================

resource "aws_security_group" "cluster" {
  count = var.iam_role_arn == "" ? 1 : 0

  name        = "${var.cluster_name}-cluster-sg"
  description = "Security group for EKS cluster"
  vpc_id      = var.vpc_id

  tags = merge(var.tags, {
    Name = "${var.cluster_name}-cluster-sg"
  })
}

resource "aws_security_group_rule" "cluster_ingress" {
  count = var.iam_role_arn == "" ? 1 : 0

  description       = "Allow inbound HTTPS from VPC"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["10.0.0.0/8"]  # Разрешить из VPC
  security_group_id = aws_security_group.cluster[0].id
  type              = "ingress"
}

resource "aws_security_group_rule" "cluster_egress" {
  count = var.iam_role_arn == "" ? 1 : 0

  description       = "Allow all outbound traffic"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.cluster[0].id
  type              = "egress"
}

# =============================================================================
# EKS Node Groups
# =============================================================================

resource "aws_eks_node_group" "main" {
  for_each = var.node_groups

  cluster_name    = aws_eks_cluster.main.name
  node_group_name = each.key
  node_role_arn   = aws_iam_role.node_group[0].arn
  subnet_ids      = var.subnet_ids

  instance_types = each.value.instance_types
  capacity_type  = each.value.capacity_type
  disk_size      = each.value.disk_size

  scaling_config {
    min_size     = each.value.min_size
    max_size     = each.value.max_size
    desired_size = each.value.desired_size
  }

  labels = each.value.labels

  dynamic "taint" {
    for_each = each.value.taints
    content {
      key    = taint.value.key
      value  = taint.value.value
      effect = taint.value.effect
    }
  }

  lifecycle {
    ignore_changes = [
      scaling_config[0].desired_size,
    ]
  }

  tags = merge(var.tags, {
    Name = "${var.cluster_name}-${each.key}"
  })

  depends_on = [
    aws_iam_role_policy_attachment.node_group_policy,
    aws_iam_role_policy_attachment.node_group_cni_policy,
    aws_iam_role_policy_attachment.node_group_container_registry,
  ]
}

# =============================================================================
# IAM Role for Node Groups
# =============================================================================

resource "aws_iam_role" "node_group" {
  count = length(var.node_groups) > 0 ? 1 : 0

  name = "${var.cluster_name}-node-group-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })

  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "node_group_policy" {
  count = length(var.node_groups) > 0 ? 1 : 0

  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.node_group[0].name
}

resource "aws_iam_role_policy_attachment" "node_group_cni_policy" {
  count = length(var.node_groups) > 0 ? 1 : 0

  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.node_group[0].name
}

resource "aws_iam_role_policy_attachment" "node_group_container_registry" {
  count = length(var.node_groups) > 0 ? 1 : 0

  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.node_group[0].name
}

resource "aws_iam_role_policy_attachment" "node_group_cloudwatch" {
  count = length(var.node_groups) > 0 ? 1 : 0

  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/CloudWatchAgentServerPolicy"
  role       = aws_iam_role.node_group[0].name
}

resource "aws_iam_role_policy_attachment" "node_group_ssm" {
  count = length(var.node_groups) > 0 ? 1 : 0

  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonSSMManagedInstanceCore"
  role       = aws_iam_role.node_group[0].name
}

# =============================================================================
# OIDC Provider for IRSA
# =============================================================================

resource "aws_iam_openid_connect_provider" "oidc" {
  count = var.enable_irsa ? 1 : 0

  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.eks.certificates[0].sha1_fingerprint]
  url             = aws_eks_cluster.main.identity[0].oidc[0].issuer

  tags = merge(var.tags, {
    Name = "${var.cluster_name}-oidc-provider"
  })
}

data "tls_certificate" "eks" {
  url = aws_eks_cluster.main.identity[0].oidc[0].issuer
}

# =============================================================================
# EKS Addons
# =============================================================================

resource "aws_eks_addon" "vpc_cni" {
  count = var.enable_irsa ? 1 : 0

  cluster_name      = aws_eks_cluster.main.name
  addon_name        = "vpc-cni"
  resolve_conflicts = "OVERWRITE"

  tags = var.tags
}

resource "aws_eks_addon" "coredns" {
  count = var.enable_irsa ? 1 : 0

  cluster_name      = aws_eks_cluster.main.name
  addon_name        = "coredns"
  resolve_conflicts = "OVERWRITE"

  tags = var.tags
}

resource "aws_eks_addon" "kube_proxy" {
  count = var.enable_irsa ? 1 : 0

  cluster_name      = aws_eks_cluster.main.name
  addon_name        = "kube-proxy"
  resolve_conflicts = "OVERWRITE"

  tags = var.tags
}

# =============================================================================
# Outputs
# =============================================================================

output "cluster_name" {
  description = "EKS cluster name"
  value       = aws_eks_cluster.main.name
}

output "cluster_endpoint" {
  description = "EKS cluster endpoint"
  value       = aws_eks_cluster.main.endpoint
}

output "cluster_arn" {
  description = "EKS cluster ARN"
  value       = aws_eks_cluster.main.arn
}

output "cluster_security_group_id" {
  description = "EKS cluster security group ID"
  value       = aws_security_group.cluster[0].id
}

output "oidc_provider_arn" {
  description = "EKS OIDC provider ARN"
  value       = var.enable_irsa ? aws_iam_openid_connect_provider.oidc[0].arn : null
}

output "oidc_provider_url" {
  description = "EKS OIDC provider URL"
  value       = aws_eks_cluster.main.identity[0].oidc[0].issuer
}

output "node_group_arns" {
  description = "ARNs of node groups"
  value       = { for k, v in aws_eks_node_group.main : k => v.arn }
}

output "cluster_certificate_authority" {
  description = "Base64 encoded certificate data for cluster CA"
  value       = aws_eks_cluster.main.certificate_authority[0].data
}

output "cluster_identity" {
  description = "EKS cluster identity"
  value       = aws_eks_cluster.main.identity
}
