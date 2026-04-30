# Variables for ElastiCache Redis Module

variable "environment" {
  description = "Environment name (dev, staging, production)"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where the cluster will be deployed"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for the subnet group"
  type        = list(string)
}

variable "allowed_cidr_blocks" {
  description = "List of CIDR blocks allowed to access the cluster"
  type        = list(string)
  default     = ["10.0.0.0/8"]
}

# Cluster Configuration
variable "node_type" {
  description = "Redis node type"
  type        = string
  default     = "cache.r5.2xlarge"
}

variable "num_shards" {
  description = "Number of shards (node groups)"
  type        = number
  default     = 3
}

variable "replicas_per_shard" {
  description = "Number of replicas per shard"
  type        = number
  default     = 1  # 1 primary + 1 replica = 2 nodes per shard
}

variable "redis_version" {
  description = "Redis engine version"
  type        = string
  default     = "7.0"
}

# Authentication
variable "auth_token" {
  description = "Authentication token for Redis"
  type        = string
  sensitive   = true
}

# Encryption
variable "enable_encryption" {
  description = "Enable encryption (in-transit and at-rest)"
  type        = bool
  default     = true
}

# High Availability
variable "multi_az" {
  description = "Enable Multi-AZ"
  type        = bool
  default     = true
}

variable "auto_failover" {
  description = "Enable automatic failover"
  type        = bool
  default     = true
}

# Maintenance
variable "maintenance_window" {
  description = "Weekly maintenance window"
  type        = string
  default     = "mon:03:00-mon:04:00"
}

variable "snapshot_window" {
  description = "Daily snapshot window"
  type        = string
  default     = "02:00-03:00"
}

variable "snapshot_retention_days" {
  description = "Number of days to retain snapshots"
  type        = number
  default     = 7
}

variable "snapshot_name" {
  description = "Name of a snapshot from which to restore data"
  type        = string
  default     = null
}

# Notifications
variable "notification_topic_arn" {
  description = "SNS topic ARN for notifications"
  type        = string
  default     = null
}

# Custom Parameters
variable "custom_parameters" {
  description = "List of custom Redis parameters"
  type = list(object({
    name  = string
    value = string
  }))
  default = []
}

# Slow Log
variable "enable_slow_log" {
  description = "Enable slow log delivery to CloudWatch"
  type        = bool
  default     = true
}

variable "log_retention_days" {
  description = "CloudWatch log retention days"
  type        = number
  default     = 30
}

# Alarm Thresholds
variable "cpu_threshold" {
  description = "CPU utilization threshold for alarm (%)"
  type        = number
  default     = 80
}

variable "memory_threshold" {
  description = "Memory usage threshold for alarm (%)"
  type        = number
  default     = 85
}

variable "evictions_threshold" {
  description = "Evictions threshold for alarm"
  type        = number
  default     = 100
}

variable "cache_hit_threshold" {
  description = "Cache hit rate threshold for alarm (%)"
  type        = number
  default     = 70
}

variable "network_threshold" {
  description = "Network output threshold for alarm (bytes/sec)"
  type        = number
  default     = 100000000  # 100 MB/s
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

# Tags
variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}
