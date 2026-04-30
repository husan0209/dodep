# Production Database Configuration
# RDS PostgreSQL + ElastiCache Redis

# =============================================================================
# Data Sources
# =============================================================================

data "aws_secretsmanager_secret_version" "db_credentials" {
  secret_id = "opus-casino/production/db-credentials"
}

data "aws_secretsmanager_secret_version" "redis_auth" {
  secret_id = "opus-casino/production/redis-auth"
}

locals {
  db_credentials = jsondecode(data.aws_secretsmanager_secret_version.db_credentials.secret_string)
  redis_auth_token = data.aws_secretsmanager_secret_version.redis_auth.secret_string
}

# =============================================================================
# RDS PostgreSQL
# =============================================================================

module "rds_postgresql" {
  source = "../../modules/rds-postgresql"

  environment = "production"
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.private_subnet_ids

  # Database credentials from Secrets Manager
  database_name   = "opus_casino"
  master_username = local.db_credentials.username
  master_password = local.db_credentials.password

  # Instance configuration - Production
  instance_class       = "db.r5.2xlarge"
  replica_instance_class = "db.r5.xlarge"
  allocated_storage    = 500
  max_allocated_storage = 1000
  iops                 = 12000

  # High Availability
  multi_az           = true
  read_replica_count = 2

  # Encryption
  enable_encryption = true
  multi_region      = true  # Для cross-region replication

  # Backup
  backup_retention_days = 30
  backup_window         = "03:00-04:00"
  maintenance_window    = "Mon:04:00-Mon:05:00"

  # Monitoring
  enable_performance_insights = true
  enable_enhanced_monitoring  = true

  # Custom parameters для гемблинг платформы
  custom_parameters = [
    {
      name  = "max_connections"
      value = "500"
    },
    {
      name  = "shared_buffers"
      value = "4GB"
    },
    {
      name  = "effective_cache_size"
      value = "12GB"
    },
    {
      name  = "work_mem"
      value = "64MB"
    },
    {
      name  = "maintenance_work_mem"
      value = "1GB"
    },
    {
      name  = "wal_level"
      value = "replica"
    },
    {
      name  = "max_wal_senders"
      value = "10"
    },
    {
      name  = "wal_keep_size"
      value = "1GB"
    },
    {
      name  = "log_min_duration_statement"
      value = "1000"  # Логировать медленные запросы > 1s
    },
    {
      name  = "pg_stat_statements.track"
      value = "all"
    }
  ]

  # Alarm thresholds
  cpu_threshold         = 80
  storage_threshold     = 50
  connections_threshold = 400

  # Alarm actions
  alarm_actions = [aws_sns_topic.platform_alerts.arn]
  ok_actions    = [aws_sns_topic.platform_alerts.arn]

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    ManagedBy   = "terraform"
  }
}

# =============================================================================
# ElastiCache Redis Cluster
# =============================================================================

module "elasticache_redis" {
  source = "../../modules/elasticache-redis"

  environment = "production"
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.private_subnet_ids

  # Cluster configuration
  node_type            = "cache.r5.2xlarge"
  num_shards           = 3
  replicas_per_shard   = 1  # 1 primary + 1 replica per shard = 6 nodes total
  redis_version        = "7.0"

  # Authentication
  auth_token = local.redis_auth_token

  # Encryption
  enable_encryption = true

  # High Availability
  multi_az      = true
  auto_failover = true

  # Maintenance
  maintenance_window        = "mon:03:00-mon:04:00"
  snapshot_window           = "02:00-03:00"
  snapshot_retention_days   = 7

  # Custom parameters
  custom_parameters = [
    {
      name  = "maxmemory-policy"
      value = "volatile-lru"
    },
    {
      name  = "timeout"
      value = "300"
    },
    {
      name  = "tcp-keepalive"
      value = "60"
    },
    {
      name  = "slowlog-log-slower-than"
      value = "10000"  # 10ms
    },
    {
      name  = "slowlog-max-len"
      value = "128"
    },
    {
      name  = "notify-keyspace-events"
      value = "Ex"  # Keyspace events for expired/evicted keys
    }
  ]

  # Alarm thresholds
  cpu_threshold        = 80
  memory_threshold     = 85
  evictions_threshold  = 100
  cache_hit_threshold  = 70
  network_threshold    = 100000000  # 100 MB/s

  # Alarm actions
  alarm_actions = [aws_sns_topic.platform_alerts.arn]
  ok_actions    = [aws_sns_topic.platform_alerts.arn]

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    ManagedBy   = "terraform"
  }
}

# =============================================================================
# PgBouncer Connection Pooler (опционально, для больших нагрузок)
# =============================================================================

# PgBouncer можно развернуть как отдельный сервис в K8s
# или использовать RDS Proxy (дороже, но managed)

resource "aws_db_proxy" "main" {
  count = var.enable_rds_proxy ? 1 : 0

  name                   = "opus-casino-production-proxy"
  debug_logging          = false
  engine_family          = "POSTGRESQL"
  idle_client_timeout    = 1800
  require_tls            = true
  role_arn               = aws_iam_role.rds_proxy[0].arn
  vpc_subnet_ids         = module.vpc.private_subnet_ids
  vpc_security_group_ids = [module.rds_postgresql.security_group_id]

  auth {
    auth_scheme = "SECRETS"
    iam_auth    = "DISABLED"
    secret_arn  = "opus-casino/production/db-credentials"
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_db_proxy_default_target_group" "main" {
  count = var.enable_rds_proxy ? 1 : 0

  db_proxy_name = aws_db_proxy.main[0].name

  connection_pool_config {
    connection_borrow_timeout    = 120
    init_query                   = "SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;"
    max_connections_percent      = 100
    max_idle_connections_percent = 50
  }
}

resource "aws_db_proxy_target" "main" {
  count = var.enable_rds_proxy ? 1 : 0

  db_instance_identifier = module.rds_postgresql.arn
  db_proxy_name          = aws_db_proxy.main[0].name
  target_group_name      = aws_db_proxy_default_target_group.main[0].name
}

resource "aws_iam_role" "rds_proxy" {
  count = var.enable_rds_proxy ? 1 : 0

  name = "opus-casino-production-rds-proxy-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "rds.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_iam_role_policy_attachment" "rds_proxy" {
  count = var.enable_rds_proxy ? 1 : 0

  role       = aws_iam_role.rds_proxy[0].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSPreview"
}

# =============================================================================
# Outputs
# =============================================================================

output "rds_primary_endpoint" {
  description = "RDS PostgreSQL primary endpoint"
  value       = module.rds_postgresql.primary_endpoint
}

output "rds_reader_endpoint" {
  description = "RDS PostgreSQL reader endpoint (load balanced)"
  value       = module.rds_postgresql.reader_endpoint
}

output "rds_read_replica_endpoints" {
  description = "RDS PostgreSQL read replica endpoints"
  value       = module.rds_postgresql.read_replica_endpoints
}

output "rds_security_group_id" {
  description = "RDS PostgreSQL security group ID"
  value       = module.rds_postgresql.security_group_id
}

output "redis_configuration_endpoint" {
  description = "ElastiCache Redis configuration endpoint"
  value       = module.elasticache_redis.configuration_endpoint_address
}

output "redis_primary_endpoint" {
  description = "ElastiCache Redis primary endpoint"
  value       = module.elasticache_redis.primary_endpoint_address
}

output "redis_reader_endpoint" {
  description = "ElastiCache Redis reader endpoint"
  value       = module.elasticache_redis.reader_endpoint_address
}

output "redis_security_group_id" {
  description = "ElastiCache Redis security group ID"
  value       = module.elasticache_redis.security_group_id
}

output "rds_proxy_endpoint" {
  description = "RDS Proxy endpoint (если включён)"
  value       = var.enable_rds_proxy ? aws_db_proxy.main[0].endpoint : null
}
