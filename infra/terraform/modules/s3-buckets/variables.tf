# Variables for S3 Buckets Module

variable "environment" {
  description = "Environment name (dev, staging, production)"
  type        = string
}

variable "buckets" {
  description = "Map of bucket names to create"
  type        = map(string)
  default = {
    uploads      = ""
    backups      = ""
    logs         = ""
    assets       = ""
    archives     = ""
  }
}

# Versioning
variable "enable_versioning" {
  description = "Enable versioning for all buckets"
  type        = bool
  default     = true
}

# Encryption
variable "enable_encryption" {
  description = "Enable KMS encryption for all buckets"
  type        = bool
  default     = true
}

# Lifecycle Rules
variable "lifecycle_rules" {
  description = "Lifecycle rules per bucket"
  type = map(list(object({
    id     = string
    status = optional(string, "Enabled")
    prefix = optional(string, "")
    transitions = optional(list(object({
      days          = number
      storage_class = string
    })), [])
    expiration = optional(object({
      days = number
    }))
    noncurrent_version_transitions = optional(list(object({
      noncurrent_days = number
      storage_class   = string
    })), [])
    noncurrent_version_expiration = optional(object({
      noncurrent_days = number
    }))
  })))
  default = {}
}

# Cross-Region Replication
variable "enable_cross_region_replication" {
  description = "Enable cross-region replication"
  type        = bool
  default     = false
}

variable "replication_destination_bucket_arn" {
  description = "Destination bucket ARN for replication"
  type        = string
  default     = null
}

variable "replication_storage_class" {
  description = "Storage class for replicated objects"
  type        = map(string)
  default     = {}
}

# Bucket Policies
variable "bucket_policies" {
  description = "Optional bucket policies (JSON)"
  type        = map(string)
  default     = {}
}

# Alarms
variable "enable_size_alarm" {
  description = "Enable bucket size alarm"
  type        = bool
  default     = true
}

variable "size_threshold_bytes" {
  description = "Bucket size threshold in bytes"
  type        = number
  default     = 1099511627776  # 1 TB
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
