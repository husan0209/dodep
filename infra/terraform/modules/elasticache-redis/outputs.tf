# Outputs for ElastiCache Redis Module

output "replication_group_id" {
  description = "Replication group ID"
  value       = aws_elasticache_replication_group.main.replication_group_id
}

output "replication_group_arn" {
  description = "Replication group ARN"
  value       = aws_elasticache_replication_group.main.arn
}

output "configuration_endpoint_address" {
  description = "Configuration endpoint address (for cluster mode)"
  value       = aws_elasticache_replication_group.main.configuration_endpoint_address
}

output "configuration_endpoint_port" {
  description = "Configuration endpoint port"
  value       = aws_elasticache_replication_group.main.configuration_endpoint_port
}

output "primary_endpoint_address" {
  description = "Primary endpoint address (writer)"
  value       = aws_elasticache_replication_group.main.primary_endpoint_address
}

output "reader_endpoint_address" {
  description = "Reader endpoint address (load balanced)"
  value       = aws_elasticache_replication_group.main.reader_endpoint_address
}

output "auth_token" {
  description = "Auth token for Redis"
  value       = var.auth_token
  sensitive   = true
}

output "security_group_id" {
  description = "Security group ID"
  value       = aws_security_group.redis.id
}

output "kms_key_arn" {
  description = "KMS key ARN for encryption"
  value       = var.enable_encryption ? aws_kms_key.redis[0].arn : null
}

output "node_groups" {
  description = "Information about node groups"
  value       = aws_elasticache_replication_group.main.node_group
}

output "cache_nodes" {
  description = "List of cache nodes"
  value       = aws_elasticache_replication_group.main.cache_nodes
}

output "parameter_group_name" {
  description = "Parameter group name"
  value       = aws_elasticache_parameter_group.main.name
}

output "subnet_group_name" {
  description = "Subnet group name"
  value       = aws_elasticache_subnet_group.main.name
}
