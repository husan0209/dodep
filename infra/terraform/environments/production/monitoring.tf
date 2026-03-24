# Production Monitoring Configuration
# VictoriaMetrics, Grafana, Alerting

# =============================================================================
# VictoriaMetrics Cluster
# =============================================================================

resource "aws_ecs_cluster" "victoriametrics" {
  name = "opus-casino-production-victoriametrics"

  service_connect_defaults {
    namespace = aws_service_discovery_http_namespace.monitoring.arn
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_service_discovery_http_namespace" "monitoring" {
  name        = "monitoring"
  description = "Service discovery namespace for monitoring services"

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# VictoriaMetrics vmstorage
resource "aws_ecs_service" "vmstorage" {
  name            = "vmstorage"
  cluster         = aws_ecs_cluster.victoriametrics.id
  task_definition = aws_ecs_task_definition.vmstorage.arn
  desired_count   = 3
  launch_type     = "FARGATE"

  network_configuration {
    subnets         = module.vpc.private_subnet_ids
    security_groups = [aws_security_group.victoriametrics.id]
  }

  service_connect_configuration {
    namespace = aws_service_discovery_http_namespace.monitoring.arn

    service {
      port_name      = "storage"
      discovery_name = "vmstorage"
    }
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_ecs_task_definition" "vmstorage" {
  family                   = "vmstorage"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 2048
  memory                   = 8192

  execution_role_arn = aws_iam_role.ecs_execution.arn
  task_role_arn      = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name  = "vmstorage"
    image = "victoriametrics/vmstorage:v1.95.0"
    portMappings = [{
      name          = "storage"
      containerPort = 8400
      protocol      = "tcp"
    }]
    command = [
      "--retentionPeriod=3",
      "--storageDataPath=/storage",
      "--dedup.minScrapeInterval=15s"
    ]
    mountPoints = [{
      sourceVolume  = "vmstorage-data"
      containerPath = "/storage"
    }]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/victoriametrics"
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "vmstorage"
      }
    }
  }])

  volume {
    name = "vmstorage-data"

    efs_volume_configuration {
      file_system_id     = aws_efs_file_system.vmstorage.id
      transit_encryption = "ENABLED"
    }
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# VictoriaMetrics vminsert
resource "aws_ecs_service" "vminsert" {
  name            = "vminsert"
  cluster         = aws_ecs_cluster.victoriametrics.id
  task_definition = aws_ecs_task_definition.vminsert.arn
  desired_count   = 2
  launch_type     = "FARGATE"

  network_configuration {
    subnets         = module.vpc.private_subnet_ids
    security_groups = [aws_security_group.victoriametrics.id]
  }

  service_connect_configuration {
    namespace = aws_service_discovery_http_namespace.monitoring.arn

    service {
      port_name      = "insert"
      discovery_name = "vminsert"
    }
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_ecs_task_definition" "vminsert" {
  family                   = "vminsert"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 1024
  memory                   = 4096

  execution_role_arn = aws_iam_role.ecs_execution.arn
  task_role_arn      = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name  = "vminsert"
    image = "victoriametrics/vminsert:v1.95.0"
    portMappings = [{
      name          = "insert"
      containerPort = 8480
      protocol      = "tcp"
    }]
    command = [
      "--storageNode=vmstorage.monitoring:8400",
      "--replicationFactor=2"
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/victoriametrics"
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "vminsert"
      }
    }
  }])

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# VictoriaMetrics vmselect
resource "aws_ecs_service" "vmselect" {
  name            = "vmselect"
  cluster         = aws_ecs_cluster.victoriametrics.id
  task_definition = aws_ecs_task_definition.vmselect.arn
  desired_count   = 2
  launch_type     = "FARGATE"

  network_configuration {
    subnets         = module.vpc.private_subnet_ids
    security_groups = [aws_security_group.victoriametrics.id]
  }

  service_connect_configuration {
    namespace = aws_service_discovery_http_namespace.monitoring.arn

    service {
      port_name      = "select"
      discovery_name = "vmselect"
    }
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_ecs_task_definition" "vmselect" {
  family                   = "vmselect"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 1024
  memory                   = 4096

  execution_role_arn = aws_iam_role.ecs_execution.arn
  task_role_arn      = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name  = "vmselect"
    image = "victoriametrics/vmselect:v1.95.0"
    portMappings = [{
      name          = "select"
      containerPort = 8481
      protocol      = "tcp"
    }]
    command = [
      "--storageNode=vmstorage.monitoring:8400",
      "--dedup.minScrapeInterval=15s"
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/victoriametrics"
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "vmselect"
      }
    }
  }])

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# EFS for VictoriaMetrics Storage
# =============================================================================

resource "aws_efs_file_system" "vmstorage" {
  creation_token = "opus-casino-production-vmstorage"
  encrypted      = true
  kms_key_id     = aws_kms_key.monitoring.arn

  lifecycle_policy {
    transition_to_ia = "AFTER_30_DAYS"
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_efs_mount_target" "vmstorage" {
  count = length(module.vpc.private_subnet_ids)

  file_system_id  = aws_efs_file_system.vmstorage.id
  subnet_id       = module.vpc.private_subnet_ids[count.index]
  security_groups = [aws_security_group.efs.id]
}

# =============================================================================
# Grafana
# =============================================================================

resource "aws_grafana_workspace" "main" {
  account_access_type   = "CURRENT_ACCOUNT"
  authentication_providers = ["AWS_SSO"]
  data_sources          = ["PROMETHEUS", "CLOUDWATCH", "AWSXRAY"]
  name                  = "opus-casino-production"
  organization_role_name = "ops-casino-grafana-role"
  release_channel       = "STABLE"
  role_name             = "ops-casino-grafana-role"
  stack_type            = "CUSTOMER_MANAGED_S3"

  network_access_control {
    security_group_ids = [aws_security_group.grafana.id]
    subnet_ids         = module.vpc.private_subnet_ids
  }

  s3_configuration {
    bucket_arn = aws_s3_bucket.grafana.arn
    key        = "grafana"
  }

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_s3_bucket" "grafana" {
  bucket = "opus-casino-production-grafana-${data.aws_caller_identity.current.account_id}"

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# Security Groups
# =============================================================================

resource "aws_security_group" "victoriametrics" {
  name        = "opus-casino-production-victoriametrics-sg"
  description = "Security group for VictoriaMetrics"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_security_group" "grafana" {
  name        = "opus-casino-production-grafana-sg"
  description = "Security group for Grafana"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_security_group" "efs" {
  name        = "opus-casino-production-efs-sg"
  description = "Security group for EFS"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_security_group_rule" "victoriametrics_ingress" {
  description       = "Allow VictoriaMetrics access from EKS"
  from_port         = 8400
  to_port           = 8481
  protocol          = "tcp"
  cidr_blocks       = ["10.0.0.0/8"]
  security_group_id = aws_security_group.victoriametrics.id
  type              = "ingress"
}

resource "aws_security_group_rule" "grafana_ingress" {
  description       = "Allow Grafana access from VPC"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["10.0.0.0/8"]
  security_group_id = aws_security_group.grafana.id
  type              = "ingress"
}

resource "aws_security_group_rule" "efs_ingress" {
  description       = "Allow EFS access from ECS"
  from_port         = 2049
  to_port           = 2049
  protocol          = "tcp"
  security_group_id = aws_security_group.efs.id
  type              = "ingress"
}

# =============================================================================
# KMS Key for Monitoring
# =============================================================================

resource "aws_kms_key" "monitoring" {
  description             = "KMS key for monitoring encryption"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# IAM Roles for ECS
# =============================================================================

resource "aws_iam_role" "ecs_execution" {
  name = "opus-casino-production-ecs-execution-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

resource "aws_iam_role" "ecs_task" {
  name = "opus-casino-production-ecs-task-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
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
# CloudWatch Log Group
# =============================================================================

resource "aws_cloudwatch_log_group" "victoriametrics" {
  name              = "/ecs/victoriametrics"
  retention_in_days = 30

  tags = {
    Project     = "opus-casino"
    Environment = "production"
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "victoriametrics_endpoint" {
  description = "VictoriaMetrics query endpoint"
  value       = "http://vmselect.monitoring:8481/select/0/prometheus"
}

output "victoriametrics_insert_endpoint" {
  description = "VictoriaMetrics insert endpoint"
  value       = "http://vminsert.monitoring:8480/insert/0/prometheus"
}

output "grafana_workspace_url" {
  description = "Grafana workspace URL"
  value       = aws_grafana_workspace.main.endpoint
}

output "grafana_workspace_id" {
  description = "Grafana workspace ID"
  value       = aws_grafana_workspace.main.id
}
