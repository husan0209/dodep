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
# KMS Keys
# =============================================================================

resource "aws_kms_key" "main" {
  for_each = var.keys

  description             = each.value.description != "" ? each.value.description : "KMS key for ${each.key} in ${var.environment}"
  deletion_window_in_days = each.value.deletion_window_in_days
  enable_key_rotation     = each.value.enable_key_rotation
  multi_region            = each.value.multi_region

  tags = merge(var.tags, {
    Name        = "${var.environment}-${each.key}-key"
    Environment = var.environment
  })
}

resource "aws_kms_alias" "main" {
  for_each = var.keys

  name          = "alias/${var.environment}-${each.key}"
  target_key_id = aws_kms_key.main[each.key].key_id
}

# =============================================================================
# KMS Key Policies
# =============================================================================

resource "aws_kms_key_policy" "main" {
  for_each = var.keys

  key_id = aws_kms_key.main[each.key].key_id
  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "${var.environment}-${each.key}-policy"
    Statement = concat(
      [
        {
          Sid    = "Enable IAM User Permissions"
          Effect = "Allow"
          Principal = {
            AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
          }
          Action   = "kms:*"
          Resource = "*"
        }
      ],
      # Service-specific policies
      each.value.services != null ? [
        for service in each.value.services : {
          Sid    = "Enable ${service} Service Access"
          Effect = "Allow"
          Principal = {
            Service = "${service}.amazonaws.com"
          }
          Action = [
            "kms:Encrypt",
            "kms:Decrypt",
            "kms:ReEncrypt*",
            "kms:GenerateDataKey*",
            "kms:DescribeKey"
          ]
          Resource = "*"
          Condition = {
            StringEquals = {
              "aws:SourceAccount" = data.aws_caller_identity.current.account_id
            }
          }
        }
      ] : [],
      # Customer-specific policies
      each.value.customers != null ? [
        for customer in each.value.customers : {
          Sid    = "Enable ${customer} Access"
          Effect = "Allow"
          Principal = {
            AWS = customer
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
      ] : []
    )
  })
}

data "aws_caller_identity" "current" {}

# =============================================================================
# Outputs
# =============================================================================

output "key_arns" {
  description = "ARNs of created KMS keys"
  value       = { for k, v in aws_kms_key.main : k => v.arn }
}

output "key_ids" {
  description = "IDs of created KMS keys"
  value       = { for k, v in aws_kms_key.main : k => v.key_id }
}

output "key_aliases" {
  description = "Aliases of created KMS keys"
  value       = { for k, v in aws_kms_alias.main : k => v.name }
}
