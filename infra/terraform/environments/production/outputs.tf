# Production Environment Outputs

output "environment" {
  description = "Environment name"
  value       = "production"
}

output "region" {
  description = "AWS region"
  value       = var.aws_region
}

output "region_replica" {
  description = "AWS region for disaster recovery"
  value       = var.aws_region_replica
}

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

output "eks_cluster_arn" {
  description = "EKS cluster ARN"
  value       = module.eks.cluster_arn
}

output "eks_cluster_security_group_id" {
  description = "EKS cluster security group ID"
  value       = module.eks.cluster_security_group_id
}

output "eks_oidc_provider_arn" {
  description = "EKS OIDC provider ARN"
  value       = module.eks.oidc_provider_arn
}

output "eks_oidc_provider_url" {
  description = "EKS OIDC provider URL"
  value       = module.eks.oidc_provider_url
}

output "kms_key_ids" {
  description = "KMS key IDs"
  value       = module.kms.key_ids
}

output "kms_key_arns" {
  description = "KMS key ARNs"
  value       = module.kms.key_arns
}

# Connection info
output "configure_kubectl" {
  description = "Command to configure kubectl"
  value       = "aws eks update-kubeconfig --name ${module.eks.cluster_name} --region ${var.aws_region} --alias production"
}

# Cluster info
output "cluster_info" {
  description = "Cluster information"
  value = {
    name              = module.eks.cluster_name
    version           = var.cluster_version
    endpoint          = module.eks.cluster_endpoint
    security_group_id = module.eks.cluster_security_group_id
    oidc_provider_arn = module.eks.oidc_provider_arn
  }
  sensitive = false
}
