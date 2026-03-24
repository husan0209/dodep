# RDS PostgreSQL Module

Production-ready PostgreSQL модуль для Opus Casino с поддержкой Multi-AZ, read replicas и автоматического масштабирования.

## Особенности

- **Multi-AZ deployment** для высокой доступности
- **Read replicas** для масштабирования read-операций
- **Автоматическое масштабирование storage** (до max_allocated_storage)
- **Шифрование** через KMS (at rest и in transit)
- **Performance Insights** для мониторинга производительности
- **Enhanced Monitoring** с 60-секундным интервалом
- **CloudWatch Alarms** для CPU, storage и connections
- **Backup** с 30-дневным retention и PITR

## Использование

```hcl
module "postgres" {
  source = "../modules/rds-postgresql"

  environment = "production"
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.private_subnet_ids

  # Database
  database_name   = "opus_casino"
  master_username = var.db_master_username
  master_password = var.db_master_password

  # Instance
  instance_class       = "db.r5.2xlarge"
  replica_instance_class = "db.r5.xlarge"
  allocated_storage    = 500
  max_allocated_storage = 1000

  # High Availability
  multi_az           = true
  read_replica_count = 2

  # Encryption
  enable_encryption = true
  multi_region      = true  # Для cross-region replication

  # Monitoring
  enable_performance_insights = true
  enable_enhanced_monitoring  = true

  # Alarms
  alarm_actions = [aws_sns_topic.platform_alerts.arn]

  tags = {
    Project = "opus-casino"
  }
}
```

## Архитектура

```
┌─────────────────────────────────────────────────────────┐
│                      VPC                                 │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │          Private Subnet 1 (AZ-a)                │    │
│  │  ┌──────────────────┐                          │    │
│  │  │   Primary        │ ← Writer endpoint        │    │
│  │  │   PostgreSQL     │                          │    │
│  │  │   db.r5.2xlarge  │                          │    │
│  │  └────────┬─────────┘                          │    │
│  │           │ synchronous                        │    │
│  │           ▼ standby                            │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │          Private Subnet 2 (AZ-b)                │    │
│  │  ┌──────────────────┐                          │    │
│  │  │   Standby        │ ← Auto failover          │    │
│  │  │   (Multi-AZ)     │                          │    │
│  │  └──────────────────┘                          │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │          Private Subnet 3 (AZ-c)                │    │
│  │  ┌──────────────────┐  ┌──────────────────┐    │    │
│  │  │   Read Replica 1 │  │   Read Replica 2 │    │    │
│  │  │   db.r5.xlarge   │  │   db.r5.xlarge   │    │    │
│  │  └──────────────────┘  └──────────────────┘    │    │
│  │         ↑                        ↑             │    │
│  │         └──── Reader endpoint ───┘            │    │
│  │              (load balanced)                  │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## Параметры

### Обязательные

| Название | Описание | Тип |
|----------|----------|-----|
| `environment` | Имя окружения | `string` |
| `vpc_id` | VPC ID | `string` |
| `subnet_ids` | Список subnet IDs | `list(string)` |
| `master_username` | Master username | `string` (sensitive) |
| `master_password` | Master password | `string` (sensitive) |

### Опциональные

| Название | Описание | По умолчанию |
|----------|----------|--------------|
| `database_name` | Имя базы данных | `"opus_casino"` |
| `postgres_version` | Версия PostgreSQL | `"15"` |
| `instance_class` | Instance class primary | `"db.r5.2xlarge"` |
| `replica_instance_class` | Instance class реплик | `"db.r5.xlarge"` |
| `allocated_storage` | Начальный storage (GB) | `500` |
| `max_allocated_storage` | Макс. storage (GB) | `1000` |
| `iops` | IOPS для io1 | `12000` |
| `multi_az` | Multi-AZ deployment | `true` |
| `read_replica_count` | Количество реплик | `2` |
| `enable_encryption` | Шифрование storage | `true` |
| `backup_retention_days` |Retention backup (дней) | `30` |

## Выходные параметры

| Название | Описание |
|----------|----------|
| `primary_endpoint` | Endpoint primary instance |
| `reader_endpoint` | Endpoint для read replicas (load balanced) |
| `read_replica_endpoints` | Endpoints всех реплик |
| `security_group_id` | Security group ID |
| `kms_key_arn` | KMS key ARN для шифрования |

## Мониторинг

Модуль создаёт CloudWatch Alarms:

- **High CPU** > 80% (2 evaluation periods)
- **Low Storage** < 50 GB
- **High Connections** > 500

## Best Practices

### Безопасность

1. Используйте security groups для ограничения доступа
2. Включите шифрование KMS
3. Храните пароль в AWS Secrets Manager или HashiCorp Vault
4. Используйте IAM authentication при возможности

### Производительность

1. Выберите правильный instance class под нагрузку
2. Настройте read replicas для read-heavy workloads
3. Используйте Performance Insights для анализа query
4. Настройте custom parameters под вашу нагрузку

### Надёжность

1. Multi-AZ обязателен для production
2. Минимум 2 read replicas для failover
3. Backup retention 30+ дней
4. Включите deletion protection

## Стоимость (пример для us-east-1)

```
Primary (db.r5.2xlarge, Multi-AZ):    ~$2,500/месяц
2x Read Replicas (db.r5.xlarge):      ~$1,000/месяц
Storage 500GB io1:                    ~$625/месяц
IOPS 12000:                           ~$600/месяц
Backup storage:                       ~$50/месяц
─────────────────────────────────────────────────────
Итого:                                ~$4,775/месяц
```

## Миграции

Для применения изменений в параметрах БД:

```bash
# Применить изменения с reboot
terraform apply -target=aws_db_parameter_group.main

# RDS потребует reboot для применения параметров
```

## Disaster Recovery

Для cross-region replication:

1. Установите `multi_region = true`
2. Создайте KMS key в DR регионе
3. Настройте replication через AWS DMS или native streaming replication

## Поддержка

- PostgreSQL 15 (рекомендуется)
- PostgreSQL 14
- PostgreSQL 13

Auto minor version upgrade включено по умолчанию.
