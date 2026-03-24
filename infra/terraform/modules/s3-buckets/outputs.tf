# Outputs for S3 Buckets Module

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

output "bucket_regional_domains" {
  description = "Map of bucket regional domain names"
  value       = { for k, v in aws_s3_bucket.main : k => v.bucket_regional_domain_name }
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
