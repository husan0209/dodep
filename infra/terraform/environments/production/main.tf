# Production Environment Configuration for Opus Casino
# Production окружение - максимальная надёжность и производительность

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
    key            = "environments/production/terraform.tfstate"
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
      Environment = "production"
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

  environment = "production"
  vpc_cidr    = var.vpc_cidr
  azs         = data.aws_availability_zones.available.names

  public_subnet_cidrs   = ["10.2.1.0/24", "10.2.2.0/24", "10.2.3.0/24"]
  private_subnet_cidrs  = ["10.2.11.0/24", "10.2.12.0/24", "10.2.13.0/24"]
  isolated_subnet_cidrs = ["10.2.21.0/24", "10.2.22.0/24", "10.2.23.0/24"]

  enable_nat_gateway     = true
  single_nat_gateway     = false  # Multi-AZ для надёжности
  enable_flow_logs       = true
  flow_logs_retention_days = 90  # Долгое хранение для аудита

  tags = {
    Name = "opus-casino-production-vpc"
  }
}

# =============================================================================
# S3 State Backend Module
# =============================================================================

module "state_backend" {
  source = "../modules/s3-state"

  environment         = "production"
  bucket_name         = "opus-casino-terraform-state-prod-${data.aws_caller_identity.current.account_id}"
  dynamodb_table_name = "opus-casino-terraform-locks-prod"
  enable_versioning   = true
  enable_encryption   = true

  tags = {
    Name = "opus-casino-production-state"
  }
}

# =============================================================================
# IAM Module
# =============================================================================

module "iam" {
  source = "../modules/iam"

  environment       = "production"
  enable_eks_roles  = true
  enable_rds_roles  = true
  enable_lambda_roles = true

  tags = {
    Name = "opus-casino-production-iam"
  }
}

# =============================================================================
# KMS Module
# =============================================================================

module "kms" {
  source = "../modules/kms"

  environment = "production"

  keys = {
    rds = {
      description             = "KMS key for RDS encryption"
      deletion_window_in_days = 30
      enable_key_rotation     = true
      multi_region            = true
    }
    s3 = {
      description             = "KMS key for S3 encryption"
      deletion_window_in_days = 30
      enable_key_rotation     = true
      multi_region            = true
    }
    secrets = {
      description             = "KMS key for secrets encryption"
      deletion_window_in_days = 30
      enable_key_rotation     = true
      multi_region            = true
    }
  }

  tags = {
    Name = "opus-casino-production-kms"
  }
}

# =============================================================================
# EKS Module
# =============================================================================

module "eks" {
  source = "../modules/eks"

  environment        = "production"
  cluster_name       = "opus-casino-production"
  cluster_version    = "1.28"
  vpc_id             = module.vpc.vpc_id
  subnet_ids         = module.vpc.private_subnet_ids
  iam_role_arn       = module.iam.eks_cluster_role_arn

  # Node groups - production конфигурация
  node_groups = {
    system = {
      instance_types = ["m6i.xlarge"]
      min_size       = 5
      max_size       = 15
      desired_size   = 8
      capacity_type  = "ON_DEMAND"  # Только on-demand для production
      disk_size      = 100
      labels = {
        "node-type" = "system"
      }
      taints = []
    }
    application = {
      instance_types = ["m6i.2xlarge"]
      min_size       = 10
      max_size       = 40
      desired_size   = 15
      capacity_type  = "ON_DEMAND"
      disk_size      = 200
      labels = {
        "node-type" = "application"
      }
      taints = []
    }
    data = {
      instance_types = ["m6i.4xlarge"]
      min_size       = 6
      max_size       = 20
      desired_size   = 10
      capacity_type  = "ON_DEMAND"
      disk_size      = 500
      labels = {
        "node-type" = "data"
      }
      taints = []
    }
    spot = {
      instance_types = ["m6i.2xlarge", "m5.2xlarge"]
      min_size       = 5
      max_size       = 30
      desired_size   = 10
      capacity_type  = "SPOT"  # Spot для фоновых задач
      disk_size      = 200
      labels = {
        "node-type" = "spot"
      }
      taints = [{
        key    = "spot"
        value  = "true"
        effect = "NO_SCHEDULE"
      }]
    }
  }

  # Addons
  enable_irsa = true
  enable_ebs_csi = true
  enable_aws_load_balancer_controller = true
  enable_external_dns = true
  enable_cert_manager = true

  # Cluster autoscaler
  enable_cluster_autoscaler = true
  enable_karpenter = true

  tags = {
    Name = "opus-casino-production-eks"
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
