# Production Messaging Configuration
# MSK (Managed Kafka) или Redpanda

# =============================================================================
# MSK Cluster (Managed Kafka)
# =============================================================================

# MSK Cluster для event streaming между сервисами
resource "aws_msk_cluster" "main" {
  cluster_name           = "opus-casino-production"
  kafka_version          = "3.5.1"
  number_of_broker_nodes = 3

  broker_node_group_info {
    instance_type   = "kafka.m5.2xlarge"
    client_subnets  = module.vpc.private_subnet_ids
    security_groups = [aws_security_group.msk.id]

    storage_info {
      ebs_storage_info {
        volume_size = 1000  # 1TB per broker
      }
    }
  }

  encryption_info {
    encryption_at_rest_kms_key_arn = aws_kms_key.msk.arn

    encryption_in_transit_kms_key_arn = aws_kms_key.msk.arn
    client_broker                      = "TLS_PLAINTEXT"
    in_cluster                         = true
  }

  logging_info {
    broker_logs {
      cloudwatch_logs {
        enabled   = true
        log_group = aws_cloudwatch_log_group.msk.name
      }

      firehose_delivery_stream = null
      s3                       = null
    }
  }

  open_monitoring {
    prometheus {
      jmx_exporter {
        enabled_in_broker = true
      }
      node_exporter {
        enabled_in_broker = true
      }
    }
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
    ManagedBy   = "terraform"
  }
}

# =============================================================================
# Security Group for MSK
# =============================================================================

resource "aws_security_group" "msk" {
  name        = "opus-casino-production-msk-sg"
  description = "Security group for MSK cluster"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_security_group_rule" "msk_ingress" {
  description       = "Allow Kafka access from EKS nodes"
  from_port         = 9094
  to_port           = 9094
  protocol          = "tcp"
  cidr_blocks       = ["10.0.0.0/8"]
  security_group_id = aws_security_group.msk.id
  type              = "ingress"
}

resource "aws_security_group_rule" "msk_egress" {
  description       = "Allow all outbound traffic"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.msk.id
  type              = "egress"
}

# =============================================================================
# KMS Key for MSK Encryption
# =============================================================================

resource "aws_kms_key" "msk" {
  description             = "KMS key for MSK encryption"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_kms_alias" "msk" {
  name          = "alias/opus-casino-production-msk"
  target_key_id = aws_kms_key.msk.key_id
}

# =============================================================================
# CloudWatch Log Group for MSK
# =============================================================================

resource "aws_cloudwatch_log_group" "msk" {
  name              = "/aws/msk/opus-casino-production"
  retention_in_days = 30

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# MSK Topics (через AWS CLI или скрипты)
# =============================================================================

# Topics создаются через kafka-topics.sh или AWS CLI:
# - user-events (user registration, login, profile updates)
# - transaction-events (deposits, withdrawals, transfers)
# - bet-events (bet placed, settled, cancelled)
# - casino-events (game sessions, spins, wins)
# - notification-events (email, push, sms notifications)
# - fraud-alerts (suspicious activity alerts)
# - audit-events (audit log for compliance)

# =============================================================================
# MSK Configuration
# =============================================================================

resource "aws_msk_configuration" "main" {
  kafka_versions = ["3.5.1"]
  name           = "opus-casino-production-config"

  server_properties = <<PROPERTIES
auto.create.topics.enable = false
default.replication.factor = 3
min.insync.replicas = 2
log.retention.hours = 168
log.segment.bytes = 1073741824
num.partitions = 12
message.max.bytes = 10485760
max.request.size = 10485760
compression.type = lz4
PROPERTIES

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# MSK Replicator (для cross-region replication)
# =============================================================================

resource "aws_msk_replicator" "main" {
  count = var.enable_msk_replication ? 1 : 0

  replicator_name  = "opus-casino-dr-replicator"
  kafka_version    = "3.5.1"
  service_role_arn = aws_iam_role.msk_replicator[0].arn

  source_cluster_arn = aws_msk_cluster.main.arn
  target_cluster_arn = var.target_cluster_arn

  replication_info_list {
    consumer_group_replication {
      detect_and_copy_new_groups = true
      groups_to_exclude          = []
      groups_to_replicate        = ["*"]
    }

    offset_replication {
      offsets_replication_interval_ms = 5000
    }

    topic_replication {
      topics_to_exclude = []
      topics_to_replicate = [
        "user-events",
        "transaction-events",
        "bet-events",
        "casino-events",
        "notification-events",
        "fraud-alerts",
        "audit-events"
      ]
    }
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_iam_role" "msk_replicator" {
  count = var.enable_msk_replication ? 1 : 0

  name = "opus-casino-production-msk-replicator-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "kafka.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# CloudWatch Alarms для MSK
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "msk_cpu" {
  alarm_name          = "opus-casino-production-msk-high-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "KafkaCpuUtil"
  namespace           = "AWS/KafkaMSK"
  period              = 300
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "MSK broker CPU utilization is too high"
  alarm_actions       = [aws_sns_topic.platform_alerts.arn]

  dimensions = {
    Cluster ARN = aws_msk_cluster.main.arn
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_cloudwatch_metric_alarm" "msk_disk" {
  alarm_name          = "opus-casino-production-msk-low-disk"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 2
  metric_name         = "KafkaDataStorage"
  namespace           = "AWS/KafkaMSK"
  period              = 300
  statistic           = "Average"
  threshold           = 100  # GB
  alarm_description   = "MSK broker disk space is too low"
  alarm_actions       = [aws_sns_topic.platform_alerts.arn]

  dimensions = {
    Cluster ARN = aws_msk_cluster.main.arn
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "msk_cluster_arn" {
  description = "MSK cluster ARN"
  value       = aws_msk_cluster.main.arn
}

output "msk_cluster_name" {
  description = "MSK cluster name"
  value       = aws_msk_cluster.main.cluster_name
}

output "msk_bootstrap_brokers" {
  description = "MSK bootstrap brokers (TLS)"
  value       = aws_msk_cluster.main.bootstrap_brokers_tls
}

output "msk_bootstrap_brokers_sasl" {
  description = "MSK bootstrap brokers (SASL/SCRAM)"
  value       = aws_msk_cluster.main.bootstrap_brokers_sasl_scram
}

output "msk_security_group_id" {
  description = "MSK security group ID"
  value       = aws_security_group.msk.id
}

output "msk_kms_key_arn" {
  description = "KMS key ARN for MSK encryption"
  value       = aws_kms_key.msk.arn
}
