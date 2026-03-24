# RDS PostgreSQL Module for Opus Casino Production
# Multi-AZ deployment с read replicas для высокой доступности

# =============================================================================
# PostgreSQL Cluster (Aurora-compatible)
# =============================================================================

resource "aws_db_subnet_group" "main" {
  name       = "${var.environment}-postgres-subnet-group"
  subnet_ids = var.subnet_ids

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-subnet-group"
    Environment = var.environment
  })
}

resource "aws_security_group" "postgres" {
  name        = "${var.environment}-postgres-sg"
  description = "Security group for RDS PostgreSQL"
  vpc_id      = var.vpc_id

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-sg"
    Environment = var.environment
  })
}

# Allow access from EKS nodes
resource "aws_security_group_rule" "postgres_ingress" {
  description       = "Allow PostgreSQL access from EKS nodes"
  from_port         = 5432
  to_port           = 5432
  protocol          = "tcp"
  cidr_blocks       = var.allowed_cidr_blocks
  security_group_id = aws_security_group.postgres.id
  type              = "ingress"
}

resource "aws_security_group_rule" "postgres_egress" {
  description       = "Allow all outbound traffic"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.postgres.id
  type              = "egress"
}

# =============================================================================
# KMS Key for Database Encryption
# =============================================================================

resource "aws_kms_key" "postgres" {
  count = var.enable_encryption ? 1 : 0

  description             = "KMS key for RDS PostgreSQL encryption (${var.environment})"
  deletion_window_in_days = 30
  enable_key_rotation     = true
  multi_region            = var.multi_region

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-key"
    Environment = var.environment
  })
}

resource "aws_kms_alias" "postgres" {
  count = var.enable_encryption ? 1 : 0

  name          = "alias/${var.environment}-rds-postgres"
  target_key_id = aws_kms_key.postgres[0].key_id
}

# =============================================================================
# RDS Parameter Group
# =============================================================================

resource "aws_db_parameter_group" "main" {
  name   = "${var.environment}-postgres-params"
  family = "postgres${var.postgres_version}"

  dynamic "parameter" {
    for_each = var.custom_parameters
    content {
      name         = parameter.value.name
      value        = parameter.value.value
      apply_method = lookup(parameter.value, "apply_method", "pending-reboot")
    }
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-params"
    Environment = var.environment
  })
}

# =============================================================================
# Primary RDS Instance
# =============================================================================

resource "aws_db_instance" "primary" {
  identifier = "${var.environment}-postgres-primary"

  # Engine
  engine               = "postgres"
  engine_version       = var.postgres_version
  instance_class       = var.instance_class
  license_model        = "postgresql-license"

  # Storage
  allocated_storage     = var.allocated_storage
  max_allocated_storage = var.max_allocated_storage
  storage_type          = "io1"
  iops                  = var.iops
  storage_encrypted     = var.enable_encryption
  kms_key_id            = var.enable_encryption ? aws_kms_key.postgres[0].arn : null

  # Database
  db_name  = var.database_name
  username = var.master_username
  password = var.master_password
  port     = 5432

  # Network
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.postgres.id]
  publicly_accessible    = false
  multi_az               = var.multi_az

  # Backup
  backup_retention_period      = var.backup_retention_days
  backup_window               = var.backup_window
  maintenance_window          = var.maintenance_window
  copy_tags_to_snapshot       = true
  delete_automated_backups    = false
  final_snapshot_identifier   = "${var.environment}-postgres-final-snapshot"
  skip_final_snapshot         = var.skip_final_snapshot

  # Monitoring
  enabled_cloudwatch_logs_exports   = ["postgresql", "upgrade"]
  performance_insights_enabled      = var.enable_performance_insights
  performance_insights_retention_period = var.enable_performance_insights ? 7 : null
  monitoring_interval               = var.enable_enhanced_monitoring ? 60 : 0
  monitoring_role_arn              = var.enable_enhanced_monitoring ? aws_iam_role.rds_monitoring[0].arn : null

  # Parameters
  parameter_group_name = aws_db_parameter_group.main.name

  # Auto minor version upgrade
  auto_minor_version_upgrade = var.auto_minor_version_upgrade

  # Deletion protection
  deletion_protection = var.deletion_protection

  # Replication
  replica_mode = "open-read-only"

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-primary"
    Environment = var.environment
    Type        = "primary"
  })
}

# =============================================================================
# Read Replicas
# =============================================================================

resource "aws_db_instance" "read_replica" {
  count = var.read_replica_count

  identifier = "${var.environment}-postgres-replica-${count.index + 1}"

  # Replication
  replicate_source_db = aws_db_instance.primary.identifier
  replica_mode        = "open-read-only"

  # Instance
  instance_class = var.replica_instance_class
  engine         = aws_db_instance.primary.engine
  engine_version = aws_db_instance.primary.engine_version

  # Storage
  allocated_storage     = aws_db_instance.primary.allocated_storage
  storage_type          = aws_db_instance.primary.storage_type
  iops                  = aws_db_instance.primary.iops
  storage_encrypted     = var.enable_encryption
  kms_key_id            = var.enable_encryption ? aws_kms_key.postgres[0].arn : null

  # Network
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.postgres.id]
  publicly_accessible    = false
  multi_az               = var.multi_az

  # Monitoring
  enabled_cloudwatch_logs_exports   = ["postgresql"]
  performance_insights_enabled      = var.enable_performance_insights
  monitoring_interval               = var.enable_enhanced_monitoring ? 60 : 0
  monitoring_role_arn              = var.enable_enhanced_monitoring ? aws_iam_role.rds_monitoring[0].arn : null

  # Parameters
  parameter_group_name = aws_db_parameter_group.main.name

  # Auto minor version upgrade
  auto_minor_version_upgrade = var.auto_minor_version_upgrade

  # Deletion protection
  deletion_protection = var.deletion_protection

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-replica-${count.index + 1}"
    Environment = var.environment
    Type        = "read-replica"
  })
}

# =============================================================================
# IAM Role for Enhanced Monitoring
# =============================================================================

resource "aws_iam_role" "rds_monitoring" {
  count = var.enable_enhanced_monitoring ? 1 : 0

  name = "${var.environment}-rds-monitoring-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "monitoring.rds.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(var.tags, {
    Name        = "${var.environment}-rds-monitoring-role"
    Environment = var.environment
  })
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  count = var.enable_enhanced_monitoring ? 1 : 0

  role       = aws_iam_role.rds_monitoring[0].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

# =============================================================================
# CloudWatch Alarm for High CPU
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "high_cpu" {
  alarm_name          = "${var.environment}-postgres-high-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "CPUUtilization"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = var.cpu_threshold
  alarm_description   = "RDS PostgreSQL CPU utilization is too high"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.primary.identifier
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-high-cpu"
    Environment = var.environment
  })
}

# =============================================================================
# CloudWatch Alarm for Free Storage
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "low_storage" {
  alarm_name          = "${var.environment}-postgres-low-storage"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 2
  metric_name         = "FreeStorageSpace"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = var.storage_threshold
  alarm_description   = "RDS PostgreSQL free storage is too low"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.primary.identifier
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-low-storage"
    Environment = var.environment
  })
}

# =============================================================================
# CloudWatch Alarm for Database Connections
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "high_connections" {
  alarm_name          = "${var.environment}-postgres-high-connections"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "DatabaseConnections"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = var.connections_threshold
  alarm_description   = "RDS PostgreSQL connections count is too high"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.primary.identifier
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-postgres-high-connections"
    Environment = var.environment
  })
}

# =============================================================================
# Outputs
# =============================================================================

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
