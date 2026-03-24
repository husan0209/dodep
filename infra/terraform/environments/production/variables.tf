# Production Environment Variables

# AWS Configuration
variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "aws_region_replica" {
  description = "AWS region for disaster recovery"
  type        = string
  default     = "us-west-2"
}

# VPC Configuration
variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.2.0.0/16"
}

# Cluster Configuration
variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
  default     = "opus-casino-production"
}

variable "cluster_version" {
  description = "Kubernetes version"
  type        = string
  default     = "1.28"
}

# Node Group Configuration - Production
variable "system_node_min_size" {
  description = "Minimum size of system node group"
  type        = number
  default     = 5
}

variable "system_node_max_size" {
  description = "Maximum size of system node group"
  type        = number
  default     = 15
}

variable "system_node_desired_size" {
  description = "Desired size of system node group"
  type        = number
  default     = 8
}

variable "application_node_min_size" {
  description = "Minimum size of application node group"
  type        = number
  default     = 10
}

variable "application_node_max_size" {
  description = "Maximum size of application node group"
  type        = number
  default     = 40
}

variable "application_node_desired_size" {
  description = "Desired size of application node group"
  type        = number
  default     = 15
}

variable "data_node_min_size" {
  description = "Minimum size of data node group"
  type        = number
  default     = 6
}

variable "data_node_max_size" {
  description = "Maximum size of data node group"
  type        = number
  default     = 20
}

variable "data_node_desired_size" {
  description = "Desired size of data node group"
  type        = number
  default     = 10
}

variable "spot_node_min_size" {
  description = "Minimum size of spot node group"
  type        = number
  default     = 5
}

variable "spot_node_max_size" {
  description = "Maximum size of spot node group"
  type        = number
  default     = 30
}

variable "spot_node_desired_size" {
  description = "Desired size of spot node group"
  type        = number
  default     = 10
}

# Database Configuration - Production
variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.r5.2xlarge"
}

variable "db_allocated_storage" {
  description = "RDS allocated storage in GB"
  type        = number
  default     = 500
}

variable "db_multi_az" {
  description = "Enable Multi-AZ for RDS"
  type        = bool
  default     = true
}

variable "db_read_replicas" {
  description = "Number of read replicas"
  type        = number
  default     = 2
}

# Redis Configuration - Production
variable "redis_node_type" {
  description = "ElastiCache node type"
  type        = string
  default     = "cache.r5.2xlarge"
}

variable "redis_num_cache_nodes" {
  description = "Number of ElastiCache nodes"
  type        = number
  default     = 3  # Primary + 2 replicas
}

variable "redis_cluster_mode" {
  description = "Enable cluster mode for Redis"
  type        = bool
  default     = true
}

# Tags
variable "additional_tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default = {
    Environment = "production"
    Team        = "platform"
    CostCenter  = "production"
    Compliance  = "SOC2"
  }
}
