# ElastiCache Redis Cluster Module

Production-ready Redis Cluster модуль для Opus Casino с поддержкой cluster mode, Multi-AZ и автоматического failover.

## Особенности

- **Cluster Mode Enabled** — шардирование данных
- **Multi-AZ** — реплики в разных AZ для отказоустойчивости
- **Automatic Failover** — автоматический failover при сбое
- **Encryption** — шифрование in-transit и at-rest
- **Auth Token** — Redis AUTH аутентификация
- **Slow Log** — логирование медленных запросов в CloudWatch
- **CloudWatch Alarms** — мониторинг CPU, memory, evictions, cache hits

## Использование

```hcl
module "redis" {
  source = "../modules/elasticache-redis"

  environment = "production"
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.private_subnet_ids

  # Cluster configuration
  node_type            = "cache.r5.2xlarge"
  num_shards           = 3
  replicas_per_shard   = 1  # 1 primary + 1 replica per shard
  redis_version        = "7.0"

  # Authentication
  auth_token = var.redis_auth_token

  # Encryption
  enable_encryption = true

  # High Availability
  multi_az       = true
  auto_failover  = true

  # Maintenance
  maintenance_window        = "mon:03:00-mon:04:00"
  snapshot_window           = "02:00-03:00"
  snapshot_retention_days   = 7

  # Monitoring
  alarm_actions = [aws_sns_topic.platform_alerts.arn]

  tags = {
    Project = "opus-casino"
  }
}
```

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                         VPC                                  │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │           Private Subnet 1 (AZ-a)                    │   │
│  │                                                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │   │
│  │  │  Shard 0    │  │  Shard 1    │  │  Shard 2    │  │   │
│  │  │   Primary   │  │   Primary   │  │   Primary   │  │   │
│  │  │ cache.r5.2x │  │ cache.r5.2x │  │ cache.r5.2x │  │   │
│  │  │  Slots 0-   │  │  Slots 5462-│  │  Slots 10923-│ │   │
│  │  │  5461       │  │  10922      │  │  16383      │  │   │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  │   │
│  │         │                │                │         │   │
│  └─────────┼────────────────┼────────────────┼─────────┘   │
│            │                │                │             │
│  ┌─────────┼────────────────┼────────────────┼─────────┐   │
│  │         ▼                ▼                ▼         │   │
│  │           Private Subnet 2 (AZ-b)                  │   │
│  │                                                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │   │
│  │  │  Shard 0    │  │  Shard 1    │  │  Shard 2    │  │   │
│  │  │   Replica   │  │   Replica   │  │   Replica   │  │   │
│  │  │ cache.r5.2x │  │ cache.r5.2x │  │ cache.r5.2x │  │   │
│  │  │  (async)    │  │  (async)    │  │  (async)    │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │   │
│  │                                                       │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
│  Configuration Endpoint: cluster.xxx.cfg.use1.cache.amazonaws.com:6379
│  Primary Endpoint: cluster-primary.xxx.ng.0001.use1.cache.amazonaws.com:6379
│  Reader Endpoint: cluster-reader.xxx.ng.0001.use1.cache.amazonaws.com:6379
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Параметры

### Обязательные

| Название | Описание | Тип |
|----------|----------|-----|
| `environment` | Имя окружения | `string` |
| `vpc_id` | VPC ID | `string` |
| `subnet_ids` | Список subnet IDs | `list(string)` |
| `auth_token` | Redis auth token | `string` (sensitive) |

### Опциональные

| Название | Описание | По умолчанию |
|----------|----------|--------------|
| `node_type` | Node type | `"cache.r5.2xlarge"` |
| `num_shards` | Количество шардов | `3` |
| `replicas_per_shard` | Реплик на шард | `1` |
| `redis_version` | Версия Redis | `"7.0"` |
| `enable_encryption` | Шифрование | `true` |
| `multi_az` | Multi-AZ | `true` |
| `auto_failover` | Авто failover | `true` |
| `snapshot_retention_days` |Retention snapshot'ов | `7` |

## Выходные параметры

| Название | Описание |
|----------|----------|
| `configuration_endpoint_address` | Cluster config endpoint |
| `primary_endpoint_address` | Writer endpoint |
| `reader_endpoint_address` | Reader endpoint (load balanced) |
| `security_group_id` | Security group ID |
| `node_groups` | Информация о шардах |

## Redis Cluster Mode

### Шардирование

Redis Cluster использует 16384 хэш-слота:

- **Shard 0:** Slots 0-5461
- **Shard 1:** Slots 5462-10922
- **Shard 2:** Slots 10923-16383

Клиент автоматически определяет, какой шард содержит нужный ключ.

### Подключение из приложения

```python
# Python (redis-py cluster)
from redis.cluster import RedisCluster

rc = RedisCluster(
    host="cluster-config.xxx.cfg.use1.cache.amazonaws.com",
    port=6379,
    password=AUTH_TOKEN,
    ssl=True,
    ssl_cert_reqs="none"
)
```

```rust
// Rust (redis cluster)
use redis::cluster::ClusterClient;

let client = ClusterClient::open(vec![
    "rediss://cluster-config.xxx.cfg.use1.cache.amazonaws.com:6379"
])?;
```

## Мониторинг

Модуль создаёт CloudWatch Alarms:

| Alarm | Threshold | Описание |
|-------|-----------|----------|
| High CPU | > 80% | Высокая загрузка CPU |
| High Memory | > 85% | Высокое использование памяти |
| High Evictions | > 100 | Много вытеснений ключей |
| Low Cache Hits | < 70% | Низкий hit rate |
| High Network | > 100 MB/s | Высокий сетевой трафик |

## Best Practices

### Производительность

1. **Выберите правильный node_type:**
   - `cache.r5.large` — dev/staging
   - `cache.r5.xlarge` — small production
   - `cache.r5.2xlarge` — production
   - `cache.r6g.xlarge` — Graviton2 (дешевле)

2. **Оптимальное количество шардов:**
   - 1-3 шарда для < 100GB данных
   - 3-6 шардов для 100-500GB
   - 6+ шардов для > 500GB

3. **Replicas per shard:**
   - Минимум 1 реплика на шард
   - 2 реплики для critical workloads

### Безопасность

1. Используйте security groups для ограничения доступа
2. Включите encryption (in-transit + at-rest)
3. Храните auth token в Secrets Manager/Vault
4. Используйте ElastiCache User Groups для RBAC

### Надёжность

1. Multi-AZ обязателен для production
2. Auto failover должен быть включён
3. Делайте snapshots регулярно
4. Мониторьте evictions и memory usage

## Стоимость (пример для us-east-1)

```
3 shards × (1 primary + 1 replica) × cache.r5.2xlarge:
  6 nodes × $0.378/hour × 730 hours = ~$1,655/месяц

Storage (3 shards × 17.5GB):
  52.5GB included в стоимость nodes

Backup storage:
  ~$20/месяц

Data transfer (между AZ):
  ~$50/месяц

─────────────────────────────────────────────────────
Итого: ~$1,725/месяц
```

## Custom Parameters

Пример настройки Redis параметров:

```hcl
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
  }
]
```

## Миграции

### Scaling (увеличение шардов)

```bash
# Увеличить количество шардов с 3 до 6
# 1. Измените num_shards в terraform.tfvars
# 2. terraform apply
# 3. ElastiCache автоматически rebalance'ит данные
```

### Изменение node_type

```bash
# Изменить тип нод (требует reboot)
# 1. Измените node_type
# 2. terraform apply
# 3. ElastiCache применит изменения в maintenance window
```

## Поддержка

- Redis 7.0 (рекомендуется)
- Redis 6.2
- Redis 6.0

Auto minor version upgrade включено по умолчанию.
