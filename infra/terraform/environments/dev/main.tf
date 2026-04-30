# Dev Environment Configuration for Opus Casino
# Development окружение с минимальными ресурсами

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

  backend "s3" {
    bucket         = "opus-casino-terraform-state"
    key            = "environments/dev/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "opus-casino-terraform-locks"
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "opus-casino"
      Environment = "dev"
      ManagedBy   = "terraform"
    }
  }
}

provider "kubernetes" {
  host                   = data.aws_eks_cluster.cluster.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.cluster.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.cluster.token
}

provider "helm" {
  kubernetes {
    host                   = data.aws_eks_cluster.cluster.endpoint
    cluster_ca_certificate = base64decode(data.aws_eks_cluster.cluster.certificate_authority[0].data)
    token                  = data.aws_eks_cluster_auth.cluster.token
  }
}

# =============================================================================
# Data Sources
# =============================================================================

data "aws_caller_identity" "current" {}
data "aws_availability_zones" "available" {
  state = "available"
}

# =============================================================================
# VPC Module
# =============================================================================

module "vpc" {
  source = "../modules/vpc"

  environment = "dev"
  vpc_cidr    = var.vpc_cidr
  azs         = slice(data.aws_availability_zones.available.names, 0, 2)

  public_subnet_cidrs  = ["10.0.1.0/24", "10.0.2.0/24"]
  private_subnet_cidrs = ["10.0.11.0/24", "10.0.12.0/24"]
  isolated_subnet_cidrs = ["10.0.21.0/24", "10.0.22.0/24"]

  enable_nat_gateway  = true
  single_nat_gateway  = true  # Экономия для dev
  enable_flow_logs    = true
  flow_logs_retention_days = 7

  tags = {
    Name = "opus-casino-dev-vpc"
  }
}

# =============================================================================
# S3 State Backend Module
# =============================================================================

module "state_backend" {
  source = "../modules/s3-state"

  environment         = "dev"
  bucket_name         = "opus-casino-terraform-state-dev-${data.aws_caller_identity.current.account_id}"
  dynamodb_table_name = "opus-casino-terraform-locks-dev"
  enable_versioning   = true
  enable_encryption   = true

  tags = {
    Name = "opus-casino-dev-state"
  }
}

# =============================================================================
# IAM Module
# =============================================================================

module "iam" {
  source = "../modules/iam"

  environment       = "dev"
  enable_eks_roles  = true
  enable_rds_roles  = true
  enable_lambda_roles = false

  tags = {
    Name = "opus-casino-dev-iam"
  }
}

# =============================================================================
# KMS Module
# =============================================================================

module "kms" {
  source = "../modules/kms"

  environment = "dev"

  keys = {
    rds = {
      description             = "KMS key for RDS encryption"
      deletion_window_in_days = 30
      enable_key_rotation     = true
    }
    s3 = {
      description             = "KMS key for S3 encryption"
      deletion_window_in_days = 30
      enable_key_rotation     = true
    }
    secrets = {
      description             = "KMS key for secrets encryption"
      deletion_window_in_days = 30
      enable_key_rotation     = true
    }
  }

  tags = {
    Name = "opus-casino-dev-kms"
  }
}

# =============================================================================
# EKS Module
# =============================================================================

module "eks" {
  source = "../modules/eks"

  environment        = "dev"
  cluster_name       = "opus-casino-dev"
  cluster_version    = "1.28"
  vpc_id             = module.vpc.vpc_id
  subnet_ids         = module.vpc.private_subnet_ids
  iam_role_arn       = module.iam.eks_cluster_role_arn

  # Node groups
  node_groups = {
    system = {
      instance_types = ["t3.large"]
      min_size       = 2
      max_size       = 5
      desired_size   = 3
      capacity_type  = "SPOT"  # Spot для экономии в dev
      disk_size      = 50
      labels = {
        "node-type" = "system"
      }
      taints = []
    }
    application = {
      instance_types = ["t3.xlarge"]
      min_size       = 2
      max_size       = 10
      desired_size   = 3
      capacity_type  = "SPOT"
      disk_size      = 100
      labels = {
        "node-type" = "application"
      }
      taints = []
    }
  }

  # Addons
  enable_irsa = true
  enable_ebs_csi = true

  tags = {
    Name = "opus-casino-dev-eks"
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "vpc_id" {
  description = "VPC ID"
  value       = module.vpc.vpc_id
}

output "vpc_cidr" {
  description = "VPC CIDR"
  value       = module.vpc.vpc_cidr
}

output "public_subnet_ids" {
  description = "Public subnet IDs"
  value       = module.vpc.public_subnet_ids
}

output "private_subnet_ids" {
  description = "Private subnet IDs"
  value       = module.vpc.private_subnet_ids
}

output "eks_cluster_name" {
  description = "EKS cluster name"
  value       = module.eks.cluster_name
}

output "eks_cluster_endpoint" {
  description = "EKS cluster endpoint"
  value       = module.eks.cluster_endpoint
}

output "eks_oidc_provider_arn" {
  description = "EKS OIDC provider ARN"
  value       = module.eks.oidc_provider_arn
}

output "eks_cluster_security_group_id" {
  description = "EKS cluster security group ID"
  value       = module.eks.cluster_security_group_id
}
