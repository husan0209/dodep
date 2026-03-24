# Variables for RDS PostgreSQL Module

variable "environment" {
  description = "Environment name (dev, staging, production)"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where the database will be deployed"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for the database subnet group"
  type        = list(string)
}

variable "allowed_cidr_blocks" {
  description = "List of CIDR blocks allowed to access the database"
  type        = list(string)
  default     = ["10.0.0.0/8"]
}

# Database Configuration
variable "database_name" {
  description = "The name of the database to create"
  type        = string
  default     = "opus_casino"
}

variable "master_username" {
  description = "The master username for the database"
  type        = string
  sensitive   = true
}

variable "master_password" {
  description = "The master password for the database"
  type        = string
  sensitive   = true
}

variable "postgres_version" {
  description = "PostgreSQL version"
  type        = string
  default     = "15"
}

# Instance Configuration
variable "instance_class" {
  description = "The instance class for the primary instance"
  type        = string
  default     = "db.r5.2xlarge"
}

variable "replica_instance_class" {
  description = "The instance class for read replicas"
  type        = string
  default     = "db.r5.xlarge"
}

variable "allocated_storage" {
  description = "The allocated storage in GB"
  type        = number
  default     = 500
}

variable "max_allocated_storage" {
  description = "The maximum allocated storage in GB (autoscaling)"
  type        = number
  default     = 1000
}

variable "iops" {
  description = "The IOPS for the storage"
  type        = number
  default     = 12000
}

variable "multi_az" {
  description = "Enable Multi-AZ deployment"
  type        = bool
  default     = true
}

variable "read_replica_count" {
  description = "Number of read replicas to create"
  type        = number
  default     = 2
}

# Encryption
variable "enable_encryption" {
  description = "Enable storage encryption"
  type        = bool
  default     = true
}

variable "multi_region" {
  description = "Enable multi-region KMS key for cross-region replication"
  type        = bool
  default     = false
}

# Backup Configuration
variable "backup_retention_days" {
  description = "Number of days to retain backups"
  type        = number
  default     = 30
}

variable "backup_window" {
  description = "The daily backup window (UTC)"
  type        = string
  default     = "03:00-04:00"
}

variable "maintenance_window" {
  description = "The weekly maintenance window (UTC)"
  type        = string
  default     = "Mon:04:00-Mon:05:00"
}

# Monitoring
variable "enable_performance_insights" {
  description = "Enable Performance Insights"
  type        = bool
  default     = true
}

variable "enable_enhanced_monitoring" {
  description = "Enable Enhanced Monitoring"
  type        = bool
  default     = true
}

# Custom Parameters
variable "custom_parameters" {
  description = "List of custom database parameters"
  type = list(object({
    name         = string
    value        = string
    apply_method = optional(string, "pending-reboot")
  }))
  default = []
}

# Alarm Configuration
variable "cpu_threshold" {
  description = "CPU utilization threshold for alarm (%)"
  type        = number
  default     = 80
}

variable "storage_threshold" {
  description = "Free storage threshold for alarm (GB)"
  type        = number
  default     = 50
}

variable "connections_threshold" {
  description = "Database connections threshold for alarm"
  type        = number
  default     = 500
}

variable "alarm_actions" {
  description = "List of SNS topic ARNs for alarm actions"
  type        = list(string)
  default     = []
}

variable "ok_actions" {
  description = "List of SNS topic ARNs for OK actions"
  type        = list(string)
  default     = []
}

# General
variable "auto_minor_version_upgrade" {
  description = "Enable auto minor version upgrades"
  type        = bool
  default     = true
}

variable "deletion_protection" {
  description = "Enable deletion protection"
  type        = bool
  default     = true
}

variable "skip_final_snapshot" {
  description = "Skip final snapshot when deleting"
  type        = bool
  default     = false
}

variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}
