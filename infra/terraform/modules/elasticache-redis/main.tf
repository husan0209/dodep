# ElastiCache Redis Cluster Module for Opus Casino
# Redis Cluster mode с Multi-AZ для кэширования и сессий

# =============================================================================
# Security Group
# =============================================================================

resource "aws_security_group" "redis" {
  name        = "${var.environment}-redis-sg"
  description = "Security group for ElastiCache Redis"
  vpc_id      = var.vpc_id

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-sg"
    Environment = var.environment
  })
}

resource "aws_security_group_rule" "redis_ingress" {
  description       = "Allow Redis access from EKS nodes"
  from_port         = 6379
  to_port           = 6379
  protocol          = "tcp"
  cidr_blocks       = var.allowed_cidr_blocks
  security_group_id = aws_security_group.redis.id
  type              = "ingress"
}

resource "aws_security_group_rule" "redis_egress" {
  description       = "Allow all outbound traffic"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.redis.id
  type              = "egress"
}

# =============================================================================
# KMS Key for Encryption
# =============================================================================

resource "aws_kms_key" "redis" {
  count = var.enable_encryption ? 1 : 0

  description             = "KMS key for ElastiCache Redis encryption (${var.environment})"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-key"
    Environment = var.environment
  })
}

resource "aws_kms_alias" "redis" {
  count = var.enable_encryption ? 1 : 0

  name          = "alias/${var.environment}-elasticache-redis"
  target_key_id = aws_kms_key.redis[0].key_id
}

# =============================================================================
# Subnet Group
# =============================================================================

resource "aws_elasticache_subnet_group" "main" {
  name       = "${var.environment}-redis-subnet-group"
  subnet_ids = var.subnet_ids

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-subnet-group"
    Environment = var.environment
  })
}

# =============================================================================
# Parameter Group
# =============================================================================

resource "aws_elasticache_parameter_group" "main" {
  name   = "${var.environment}-redis-params"
  family = "redis${var.redis_version}"

  dynamic "parameter" {
    for_each = var.custom_parameters
    content {
      name  = parameter.value.name
      value = parameter.value.value
    }
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-params"
    Environment = var.environment
  })
}

# =============================================================================
# Redis Cluster (Cluster Mode Enabled)
# =============================================================================

resource "aws_elasticache_replication_group" "main" {
  replication_group_id          = "${var.environment}-redis"
  description                   = "ElastiCache Redis Cluster for ${var.environment}"
  node_type                     = var.node_type
  num_node_groups               = var.num_shards
  replicas_per_node_group       = var.replicas_per_shard
  port                          = 6379

  # Network
  subnet_group_name    = aws_elasticache_subnet_group.main.name
  security_group_ids   = [aws_security_group.redis.id]
  ip_discovery         = "ipv4"
  network_type         = "ipv4"

  # Cluster mode
  cluster_mode {
    num_node_groups         = var.num_shards
    replicas_per_node_group = var.replicas_per_shard
  }

  # Engine
  engine               = "redis"
  engine_version       = var.redis_version
  parameter_group_name = aws_elasticache_parameter_group.main.name

  # Auth
  auth_token               = var.auth_token
  auth_token_update_strategy = "ROTATE"
  user_group_ids           = [aws_elasticache_user_group.main[0].id]

  # Encryption
  transit_encryption_enabled = var.enable_encryption
  at_rest_encryption_enabled = var.enable_encryption
  kms_key_id                 = var.enable_encryption ? aws_kms_key.redis[0].arn : null

  # Multi-AZ
  automatic_failover_enabled = var.auto_failover
  multi_az_enabled           = var.multi_az

  # Maintenance
  maintenance_window     = var.maintenance_window
  snapshot_window        = var.snapshot_window
  snapshot_retention_limit = var.snapshot_retention_days
  snapshot_name          = var.snapshot_name

  # Notification
  notification_topic_arn = var.notification_topic_arn

  # Monitoring
  log_delivery_configuration {
    destination      = aws_cloudwatch_log_group.slowlog[0].arn
    destination_type = "cloudwatch-logs"
    log_format       = "json"
    log_type         = "slow-log"
  }

  # Tags
  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-cluster"
    Environment = var.environment
  })

  lifecycle {
    ignore_changes = [
      num_node_groups,
      replicas_per_node_group,
    ]
  }
}

# =============================================================================
# ElastiCache User Group
# =============================================================================

resource "aws_elasticache_user_group" "main" {
  count = var.enable_encryption ? 1 : 0

  user_group_id = "${var.environment}-redis-user-group"
  engine        = "REDIS"

  user_ids = [aws_elasticache_user.main[0].user_id]

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-user-group"
    Environment = var.environment
  })
}

# =============================================================================
# ElastiCache User
# =============================================================================

resource "aws_elasticache_user" "main" {
  count = var.enable_encryption ? 1 : 0

  user_id       = "${var.environment}-redis-user"
  user_name     = "${var.environment}-redis-user"
  access_string = "on ~* +@all"
  engine        = "REDIS"
  authentication_mode {
    type      = "password"
    passwords = [var.auth_token]
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-user"
    Environment = var.environment
  })
}

# =============================================================================
# CloudWatch Log Group for Slow Log
# =============================================================================

resource "aws_cloudwatch_log_group" "slowlog" {
  count = var.enable_slow_log ? 1 : 0

  name              = "/aws/elasticache/${var.environment}-redis-slowlog"
  retention_in_days = var.log_retention_days

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-slowlog"
    Environment = var.environment
  })
}

# =============================================================================
# CloudWatch Alarms
# =============================================================================

# CPU Utilization
resource "aws_cloudwatch_metric_alarm" "high_cpu" {
  alarm_name          = "${var.environment}-redis-high-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ElastiCache"
  period              = 300
  statistic           = "Average"
  threshold           = var.cpu_threshold
  alarm_description   = "ElastiCache CPU utilization is too high"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.main.replication_group_id
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-high-cpu"
    Environment = var.environment
  })
}

# Memory Utilization
resource "aws_cloudwatch_metric_alarm" "high_memory" {
  alarm_name          = "${var.environment}-redis-high-memory"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "DatabaseMemoryUsagePercentage"
  namespace           = "AWS/ElastiCache"
  period              = 300
  statistic           = "Average"
  threshold           = var.memory_threshold
  alarm_description   = "ElastiCache memory usage is too high"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.main.replication_group_id
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-high-memory"
    Environment = var.environment
  })
}

# Evictions
resource "aws_cloudwatch_metric_alarm" "high_evictions" {
  alarm_name          = "${var.environment}-redis-high-evictions"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "Evictions"
  namespace           = "AWS/ElastiCache"
  period              = 300
  statistic           = "Sum"
  threshold           = var.evictions_threshold
  alarm_description   = "ElastiCache evictions are too high"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.main.replication_group_id
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-high-evictions"
    Environment = var.environment
  })
}

# Cache Hits
resource "aws_cloudwatch_metric_alarm" "low_cache_hits" {
  alarm_name          = "${var.environment}-redis-low-cache-hits"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 2
  metric_name         = "CacheHitRate"
  namespace           = "AWS/ElastiCache"
  period              = 300
  statistic           = "Average"
  threshold           = var.cache_hit_threshold
  alarm_description   = "ElastiCache cache hit rate is too low"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.main.replication_group_id
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-low-cache-hits"
    Environment = var.environment
  })
}

# Network Output
resource "aws_cloudwatch_metric_alarm" "high_network" {
  alarm_name          = "${var.environment}-redis-high-network"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "NetworkBytesPerSecond"
  namespace           = "AWS/ElastiCache"
  period              = 300
  statistic           = "Average"
  threshold           = var.network_threshold
  alarm_description   = "ElastiCache network output is too high"
  alarm_actions       = var.alarm_actions
  ok_actions          = var.ok_actions

  dimensions = {
    ReplicationGroupId = aws_elasticache_replication_group.main.replication_group_id
  }

  tags = merge(var.tags, {
    Name        = "${var.environment}-redis-high-network"
    Environment = var.environment
  })
}

# =============================================================================
# Outputs
# =============================================================================

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
