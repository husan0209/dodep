# S3 Buckets Module for Opus Casino
# Versioning, lifecycle policies, encryption, replication

# =============================================================================
# KMS Key for S3 Encryption
# =============================================================================

resource "aws_kms_key" "s3" {
  count = var.enable_encryption ? 1 : 0

  description             = "KMS key for S3 bucket encryption (${var.environment})"
  deletion_window_in_days = 30
  enable_key_rotation     = true
  multi_region            = var.enable_cross_region_replication

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "Enable IAM User Permissions"
        Effect = "Allow"
        Principal = {
          AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
        }
        Action   = "kms:*"
        Resource = "*"
      },
      {
        Sid    = "Allow S3 Service Principal"
        Effect = "Allow"
        Principal = {
          Service = "s3.amazonaws.com"
        }
        Action = [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:ReEncrypt*",
          "kms:GenerateDataKey*",
          "kms:DescribeKey"
        ]
        Resource = "*"
      }
    ]
  })

  tags = merge(var.tags, {
    Name        = "${var.environment}-s3-key"
    Environment = var.environment
  })
}

resource "aws_kms_alias" "s3" {
  count = var.enable_encryption ? 1 : 0

  name          = "alias/${var.environment}-s3"
  target_key_id = aws_kms_key.s3[0].key_id
}

# =============================================================================
# S3 Buckets
# =============================================================================

resource "aws_s3_bucket" "main" {
  for_each = var.buckets

  bucket = "${var.environment}-opus-${each.key}-${data.aws_caller_identity.current.account_id}"

  tags = merge(var.tags, {
    Name        = "${var.environment}-opus-${each.key}"
    Environment = var.environment
    Bucket      = each.key
  })
}

# =============================================================================
# Bucket Versioning
# =============================================================================

resource "aws_s3_bucket_versioning" "main" {
  for_each = var.buckets

  bucket = aws_s3_bucket.main[each.key].id

  versioning_configuration {
    status = var.enable_versioning ? "Enabled" : "Suspended"
  }
}

# =============================================================================
# Bucket Encryption
# =============================================================================

resource "aws_s3_bucket_server_side_encryption_configuration" "main" {
  for_each = var.buckets

  bucket = aws_s3_bucket.main[each.key].id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = var.enable_encryption ? "aws:kms" : "AES256"
      kms_master_key_id = var.enable_encryption ? aws_kms_key.s3[0].arn : null
    }
    bucket_key_enabled = var.enable_encryption
  }
}

# =============================================================================
# Block Public Access
# =============================================================================

resource "aws_s3_bucket_public_access_block" "main" {
  for_each = var.buckets

  bucket = aws_s3_bucket.main[each.key].id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# =============================================================================
# Lifecycle Policies
# =============================================================================

resource "aws_s3_bucket_lifecycle_configuration" "main" {
  for_each = var.buckets

  bucket = aws_s3_bucket.main[each.key].id

  dynamic "rule" {
    for_each = lookup(var.lifecycle_rules, each.key, [])
    content {
      id     = rule.value.id
      status = lookup(rule.value, "status", "Enabled")

      dynamic "transition" {
        for_each = lookup(rule.value, "transitions", [])
        content {
          days          = transition.value.days
          storage_class = transition.value.storage_class
        }
      }

      dynamic "expiration" {
        for_each = lookup(rule.value, "expiration", null) != null ? [rule.value.expiration] : []
        content {
          days = expiration.value.days
        }
      }

      dynamic "noncurrent_version_transition" {
        for_each = lookup(rule.value, "noncurrent_version_transitions", [])
        content {
          noncurrent_days = transition.value.noncurrent_days
          storage_class   = transition.value.storage_class
        }
      }

      dynamic "noncurrent_version_expiration" {
        for_each = lookup(rule.value, "noncurrent_version_expiration", null) != null ? [rule.value.noncurrent_version_expiration] : []
        content {
          noncurrent_days = noncurrent_version_expiration.value.noncurrent_days
        }
      }

      filter {
        prefix = lookup(rule.value, "prefix", "")
      }
    }
  }

  depends_on = [aws_s3_bucket_versioning.main]
}

# =============================================================================
# Cross-Region Replication
# =============================================================================

resource "aws_iam_role" "replication" {
  count = var.enable_cross_region_replication ? 1 : 0

  name = "${var.environment}-s3-replication-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "s3.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(var.tags, {
    Name        = "${var.environment}-s3-replication-role"
    Environment = var.environment
  })
}

resource "aws_iam_role_policy" "replication" {
  count = var.enable_cross_region_replication ? 1 : 0

  name = "${var.environment}-s3-replication-policy"
  role = aws_iam_role.replication[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "s3:GetReplicationConfiguration",
          "s3:ListBucket"
        ]
        Effect   = "Allow"
        Resource = aws_s3_bucket.main[*].arn
      },
      {
        Action = [
          "s3:GetObjectVersionForReplication",
          "s3:GetObjectVersionAcl",
          "s3:GetObjectVersionTagging"
        ]
        Effect   = "Allow"
        Resource = "${aws_s3_bucket.main[*].arn}/*"
      },
      {
        Action = [
          "s3:ReplicateObject",
          "s3:ReplicateDelete",
          "s3:ReplicateTags"
        ]
        Effect   = "Allow"
        Resource = "${var.replication_destination_bucket_arn}/*"
      },
      {
        Action = [
          "kms:Decrypt",
          "kms:ReEncrypt",
          "kms:GenerateDataKey"
        ]
        Effect   = "Allow"
        Resource = var.enable_encryption ? aws_kms_key.s3[0].arn : "*"
      }
    ]
  })
}

resource "aws_s3_bucket_replication_configuration" "main" {
  for_each = var.enable_cross_region_replication ? var.buckets : {}

  bucket = aws_s3_bucket.main[each.key].id
  role   = aws_iam_role.replication[0].arn

  rule {
    id     = "replicate-all"
    status = "Enabled"

    source_selection_criteria {
      replica_modifications {
        status = "Enabled"
      }
      sse_kms_encrypted_objects {
        status = var.enable_encryption ? "Enabled" : "Disabled"
      }
    }

    destination {
      bucket        = "${var.replication_destination_bucket_arn}"
      storage_class = lookup(var.replication_storage_class, each.key, "STANDARD")

      encryption_configuration {
        replica_kms_key_id = var.enable_encryption ? aws_kms_key.s3[0].arn : null
      }
    }
  }

  depends_on = [aws_s3_bucket_versioning.main]
}

# =============================================================================
# Bucket Policies (optional)
# =============================================================================

resource "aws_s3_bucket_policy" "main" {
  for_each = { for k, v in var.bucket_policies : k => v if v != null }

  bucket = aws_s3_bucket.main[each.key].id
  policy = each.value
}

# =============================================================================
# CloudWatch Metrics
# =============================================================================

resource "aws_s3_bucket_metric" "main" {
  for_each = var.buckets

  bucket = aws_s3_bucket.main[each.key].id
  name   = "EntireBucket"
}

# =============================================================================
# CloudWatch Alarms for Storage
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "bucket_size" {
  for_each = var.enable_size_alarm ? var.buckets : {}

  alarm_name          = "${var.environment}-s3-${each.key}-size"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "BucketSizeBytes"
  namespace           = "AWS/S3"
  period              = 86400  # Daily
  statistic           = "Average"
  threshold           = var.size_threshold_bytes
  alarm_description   = "S3 bucket ${each.key} size is too large"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    BucketName = aws_s3_bucket.main[each.key].bucket
    StorageType = "StandardStorage"
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-s3-${each.key}-size"
    Environment = var.environment
  })
}

# =============================================================================
# Data Source
# =============================================================================

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# =============================================================================
# Outputs
# =============================================================================

output "bucket_names" {
  description = "Map of bucket names"
  value       = { for k, v in aws_s3_bucket.main : k => v.bucket_name }
}

output "bucket_arns" {
  description = "Map of bucket ARNs"
  value       = { for k, v in aws_s3_bucket.main : k => v.arn }
}

output "bucket_domains" {
  description = "Map of bucket domain names"
  value       = { for k, v in aws_s3_bucket.main : k => v.bucket_domain_name }
}

output "kms_key_arn" {
  description = "KMS key ARN for S3 encryption"
  value       = var.enable_encryption ? aws_kms_key.s3[0].arn : null
}

output "kms_key_id" {
  description = "KMS key ID for S3 encryption"
  value       = var.enable_encryption ? aws_kms_key.s3[0].key_id : null
}

output "replication_role_arn" {
  description = "IAM role ARN for cross-region replication"
  value       = var.enable_cross_region_replication ? aws_iam_role.replication[0].arn : null
}
