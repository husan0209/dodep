# Outputs for RDS PostgreSQL Module

output "primary_endpoint" {
  description = "The connection endpoint for the primary instance"
  value       = aws_db_instance.primary.endpoint
}

output "primary_address" {
  description = "The hostname of the primary instance"
  value       = aws_db_instance.primary.address
}

output "primary_port" {
  description = "The port of the primary instance"
  value       = aws_db_instance.primary.port
}

output "reader_endpoint" {
  description = "The connection endpoint for read replicas (load balanced)"
  value       = aws_db_instance.primary.reader_endpoint
}

output "read_replica_endpoints" {
  description = "The connection endpoints for read replicas"
  value       = aws_db_instance.read_replica[*].endpoint
}

output "database_name" {
  description = "The name of the database"
  value       = aws_db_instance.primary.db_name
}

output "master_username" {
  description = "The master username"
  value       = aws_db_instance.primary.username
  sensitive   = true
}

output "security_group_id" {
  description = "The security group ID for the database"
  value       = aws_security_group.postgres.id
}

output "kms_key_arn" {
  description = "The ARN of the KMS key used for encryption"
  value       = var.enable_encryption ? aws_kms_key.postgres[0].arn : null
}

output "arn" {
  description = "The ARN of the primary RDS instance"
  value       = aws_db_instance.primary.arn
}

output "replica_arns" {
  description = "The ARNs of the read replicas"
  value       = aws_db_instance.read_replica[*].arn
}

output "parameter_group_name" {
  description = "The name of the DB parameter group"
  value       = aws_db_parameter_group.main.name
}

output "subnet_group_name" {
  description = "The name of the DB subnet group"
  value       = aws_db_subnet_group.main.name
}
