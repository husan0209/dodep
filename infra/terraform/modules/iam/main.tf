terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

# =============================================================================
# EKS Cluster Role
# =============================================================================

resource "aws_iam_role" "eks_cluster" {
  count = var.enable_eks_roles ? 1 : 0

  name = "${var.environment}-eks-cluster-role"

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

  tags = merge(var.tags, {
    Name        = "${var.environment}-eks-cluster-role"
    Environment = var.environment
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  count = var.enable_eks_roles ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.eks_cluster[0].name
}

resource "aws_iam_role_policy_attachment" "eks_vpc_resource_controller" {
  count = var.enable_eks_roles ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSVPCResourceController"
  role       = aws_iam_role.eks_cluster[0].name
}

# =============================================================================
# EKS Node Group Role
# =============================================================================

resource "aws_iam_role" "eks_node" {
  count = var.enable_eks_roles ? 1 : 0

  name = "${var.environment}-eks-node-role"

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

  tags = merge(var.tags, {
    Name        = "${var.environment}-eks-node-role"
    Environment = var.environment
  })
}

resource "aws_iam_role_policy_attachment" "eks_worker_node_policy" {
  count = var.enable_eks_roles ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.eks_node[0].name
}

resource "aws_iam_role_policy_attachment" "eks_cni_policy" {
  count = var.enable_eks_roles ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.eks_node[0].name
}

resource "aws_iam_role_policy_attachment" "eks_container_registry" {
  count = var.enable_eks_roles ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.eks_node[0].name
}

resource "aws_iam_role_policy_attachment" "eks_cloudwatch_agent" {
  count = var.enable_eks_roles ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
  role       = aws_iam_role.eks_node[0].name
}

# =============================================================================
# EKS OIDC Provider
# =============================================================================

data "aws_iam_policy_document" "eks_oidc_assume_role" {
  count = var.enable_eks_roles && var.oidc_provider_url != "" ? 1 : 0

  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = "${replace(var.oidc_provider_url, "https://", "")}:sub"
      values   = ["system:serviceaccount:kube-system:aws-load-balancer-controller"]
    }

    principals {
      identifiers = [var.oidc_provider_url]
      type        = "Federated"
    }
  }
}

# =============================================================================
# RDS Enhanced Monitoring Role
# =============================================================================

resource "aws_iam_role" "rds_enhanced_monitoring" {
  count = var.enable_rds_roles ? 1 : 0

  name = "${var.environment}-rds-enhanced-monitoring-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "monitoring.rds.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(var.tags, {
    Name        = "${var.environment}-rds-monitoring-role"
    Environment = var.environment
  })
}

resource "aws_iam_role_policy_attachment" "rds_enhanced_monitoring" {
  count = var.enable_rds_roles ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
  role       = aws_iam_role.rds_enhanced_monitoring[0].name
}

# =============================================================================
# Outputs
# =============================================================================

output "eks_cluster_role_arn" {
  description = "ARN of EKS cluster role"
  value       = var.enable_eks_roles ? aws_iam_role.eks_cluster[0].arn : null
}

output "eks_cluster_role_name" {
  description = "Name of EKS cluster role"
  value       = var.enable_eks_roles ? aws_iam_role.eks_cluster[0].name : null
}

output "eks_node_role_arn" {
  description = "ARN of EKS node role"
  value       = var.enable_eks_roles ? aws_iam_role.eks_node[0].arn : null
}

output "eks_node_role_name" {
  description = "Name of EKS node role"
  value       = var.enable_eks_roles ? aws_iam_role.eks_node[0].name : null
}

output "rds_enhanced_monitoring_role_arn" {
  description = "ARN of RDS enhanced monitoring role"
  value       = var.enable_rds_roles ? aws_iam_role.rds_enhanced_monitoring[0].arn : null
}

output "rds_enhanced_monitoring_role_name" {
  description = "Name of RDS enhanced monitoring role"
  value       = var.enable_rds_roles ? aws_iam_role.rds_enhanced_monitoring[0].name : null
}
