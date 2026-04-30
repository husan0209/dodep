# Staging Environment Variables

# AWS Configuration
variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

# VPC Configuration
variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.1.0.0/16"
}

# Cluster Configuration
variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
  default     = "opus-casino-staging"
}

variable "cluster_version" {
  description = "Kubernetes version"
  type        = string
  default     = "1.28"
}

# Node Group Configuration
variable "system_node_min_size" {
  description = "Minimum size of system node group"
  type        = number
  default     = 3
}

variable "system_node_max_size" {
  description = "Maximum size of system node group"
  type        = number
  default     = 8
}

variable "system_node_desired_size" {
  description = "Desired size of system node group"
  type        = number
  default     = 5
}

variable "application_node_min_size" {
  description = "Minimum size of application node group"
  type        = number
  default     = 3
}

variable "application_node_max_size" {
  description = "Maximum size of application node group"
  type        = number
  default     = 15
}

variable "application_node_desired_size" {
  description = "Desired size of application node group"
  type        = number
  default     = 5
}

variable "data_node_min_size" {
  description = "Minimum size of data node group"
  type        = number
  default     = 2
}

variable "data_node_max_size" {
  description = "Maximum size of data node group"
  type        = number
  default     = 6
}

variable "data_node_desired_size" {
  description = "Desired size of data node group"
  type        = number
  default     = 3
}

# Database Configuration
variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.r5.large"
}

variable "db_allocated_storage" {
  description = "RDS allocated storage in GB"
  type        = number
  default     = 100
}

# Redis Configuration
variable "redis_node_type" {
  description = "ElastiCache node type"
  type        = string
  default     = "cache.r5.large"
}

variable "redis_num_cache_nodes" {
  description = "Number of ElastiCache nodes"
  type        = number
  default     = 2  # 2 узла для отказоустойчивости
}

# Tags
variable "additional_tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}
