# Disaster Recovery Plan — Opus Casino Production

## 1. Overview

### 1.1 Цель документа

Этот документ описывает план аварийного восстановления (Disaster Recovery Plan, DRP) для платформы Opus Casino в production окружении.

### 1.2 Область применения

- **Primary Region:** US-East-1 (N. Virginia)
- **DR Region:** US-West-2 (Oregon)
- **RTO (Recovery Time Objective):** < 30 минут
- **RPO (Recovery Point Objective):** < 5 минут

### 1.3 Типы сбоев

| Тип | Описание | Пример |
|-----|----------|--------|
| **Regional Outage** | Полный отказ региона | AWS region down |
| **Service Outage** | Отказ отдельного сервиса | RDS, ElastiCache down |
| **Data Corruption** | Повреждение данных | Ошибка приложения, баг миграции |
| **Security Incident** | Атака на инфраструктуру | DDoS, data breach |
| **Human Error** | Ошибка оператора | Случайное удаление данных |

---

## 2. Архитектура High Availability

### 2.1 Primary Region (US-East-1)

```
┌─────────────────────────────────────────────────────────────┐
│                      US-East-1 (Primary)                     │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  CloudFlare (DNS + WAF + Load Balancer)              │   │
│  │  - Geo routing                                       │   │
│  │  - Health checks                                     │   │
│  │  - Automatic failover                                │   │
│  └──────────────────────────────────────────────────────┘   │
│                            │                                 │
│         ┌──────────────────┼──────────────────┐             │
│         ▼                  ▼                  ▼             │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐       │
│  │  EKS        │   │  RDS        │   │  ElastiCache│       │
│  │  Cluster    │   │  PostgreSQL │   │  Redis      │       │
│  │  Multi-AZ   │   │  Multi-AZ   │   │  Cluster    │       │
│  │  3 AZs      │   │  + 2 RR     │   │  3 shards   │       │
│  └─────────────┘   └─────────────┘   └─────────────┘       │
│         │                  │                  │             │
│         └──────────────────┼──────────────────┘             │
│                            ▼                                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  MSK (Kafka) + S3 + DynamoDB                         │   │
│  │  - Event streaming                                   │   │
│  │  - Object storage                                    │   │
│  │  - Session storage                                   │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 DR Region (US-West-2)

```
┌─────────────────────────────────────────────────────────────┐
│                      US-West-2 (DR)                          │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  CloudFlare (DNS Failover)                           │   │
│  │  - Passive standby                                   │   │
│  │  - Health check failover                             │   │
│  └──────────────────────────────────────────────────────┘   │
│                            │                                 │
│         ┌──────────────────┼──────────────────┐             │
│         ▼                  ▼                  ▼             │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐       │
│  │  EKS        │   │  RDS        │   │  ElastiCache│       │
│  │  Cluster    │   │  Read       │   │  Replica    │       │
│  │  (scaled    │   │  Replica    │   │  (async     │       │
│  │   down)     │   │  (async)    │   │   repl)     │       │
│  └─────────────┘   └─────────────┘   └─────────────┘       │
│         │                  │                  │             │
│         └──────────────────┼──────────────────┘             │
│                            ▼                                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  MSK Replica + S3 CRR + DynamoDB Global              │   │
│  │  - Cross-region replication                          │   │
│  │  - S3 Cross-Region Replication                       │   │
│  │  - Global Tables                                     │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Стратегия Репликации

### 3.1 Базы данных (RDS PostgreSQL)

**Метод:** Cross-Region Read Replica

```hcl
# Primary (US-East-1)
module "rds_primary" {
  source = "../../modules/rds-postgresql"
  environment = "production"
  multi_az    = true
  multi_region = true  # Включает multi-region KMS
}

# DR Replica (US-West-2)
resource "aws_db_instance" "dr_replica" {
  identifier          = "opus-casino-dr-replica"
  replicate_source_db = module.rds_primary.arn
  
  # Асинхронная репликация
  # RPO: ~1-5 минут в зависимости от нагрузки
}
```

**RPO:** 1-5 минут  
**RTO:** 5-10 минут (promote replica → update DNS)

### 3.2 Кэш (ElastiCache Redis)

**Метод:** Async Replication через приложение

```
Primary Cluster (US-East-1)
         │
         │ Приложение пишет в оба региона
         ▼
DR Cluster (US-West-2)
```

**RPO:** Зависит от частоты записи (обычно < 1 минута для сессий)  
**RTO:** 2-5 минут (переключение клиентов на DR cluster)

### 3.3 Event Streaming (MSK)

**Метод:** MSK Replicator

```hcl
resource "aws_msk_replicator" "dr" {
  replicator_name  = "opus-casino-dr-replicator"
  source_cluster_arn = aws_msk_cluster.primary.arn
  target_cluster_arn = aws_msk_cluster.dr.arn
  
  replication_info_list {
    topic_replication {
      topics_to_replicate = [
        "user-events",
        "transaction-events",
        "bet-events",
        "casino-events"
      ]
    }
  }
}
```

**RPO:** < 1 минута  
**RTO:** 5 минут (переключение consumers на DR cluster)

### 3.4 Object Storage (S3)

**Метод:** Cross-Region Replication (CRR)

```hcl
resource "aws_s3_bucket_replication_configuration" "dr" {
  bucket = aws_s3_bucket.primary.id
  role   = aws_iam_role.replication.arn
  
  rule {
    status = "Enabled"
    destination {
      bucket        = aws_s3_bucket.dr.arn
      storage_class = "STANDARD"
    }
  }
}
```

**RPO:** < 15 минут (S3 репликация асинхронная)  
**RTO:** Мгновенно (DNS failover переключает на DR bucket)

### 3.5 DynamoDB

**Метод:** Global Tables

```hcl
resource "aws_dynamodb_table" "sessions" {
  name           = "opus-casino-sessions"
  billing_mode   = "PAY_PER_REQUEST"
  
  stream_enabled   = true
  stream_view_type = "NEW_AND_OLD_IMAGES"
  
  replica {
    region_name = "us-west-2"
  }
}
```

**RPO:** < 1 секунда  
**RTO:** Мгновенно (Global Tables автоматически доступны в обоих регионах)

---

## 4. План Восстановления

### 4.1 Сценарий 1: Regional Outage

**Обнаружение:**
- CloudFlare health checks failing (> 3 consecutive failures)
- Grafana alerts: "Region US-East-1 Unreachable"
- PagerDuty notification → On-call engineer

**Действия (автоматические):**

1. **CloudFlare Failover** (0-2 минуты)
   - CloudFlare автоматически переключает трафик на DR pool
   - DNS TTL: 60 секунд

2. **RDS Promote** (2-5 минут)
   ```bash
   # AWS CLI command для promote replica
   aws rds promote-read-replica \
     --db-instance-identifier opus-casino-dr-replica \
     --region us-west-2
   ```

3. **EKS Scale Up** (5-10 минут)
   ```bash
   # Увеличиваем количество нод в DR регионе
   kubectl scale nodeset --replicas=15 -n platform
   ```

4. **MSK Consumer Switch** (5-7 минут)
   - Consumers переключаются на DR cluster
   - Конфигурация через environment variables

**Восстановление (после fix primary region):**

1. Развернуть репликацию в обратном направлении (DR → Primary)
2. Дождаться синхронизации
3. Переключить трафик обратно через CloudFlare
4. Вернуть DR в standby mode

### 4.2 Сценарий 2: Database Corruption

**Обнаружение:**
- Application errors (data integrity violations)
- CloudWatch alarms: "Database Errors High"
- User reports

**Действия:**

1. **Stop Writes** (0-1 минута)
   ```bash
   # Блокируем запись на уровне приложения
   kubectl set env deployment/wallet-core MAINTENANCE_MODE=true
   ```

2. **Assess Damage** (1-5 минут)
   - Определить затронутые таблицы
   - Проверить audit log

3. **Point-in-Time Recovery** (10-30 минут)
   ```bash
   # Restore из snapshot до инцидента
   aws rds restore-db-instance-to-point-in-time \
     --source-db-instance-identifier opus-casino-production \
     --target-db-instance-identifier opus-casino-restored \
     --restore-time 2024-03-24T10:00:00Z
   ```

4. **Data Validation** (5-10 минут)
   - Запустить data integrity checks
   - Сверить с backup logs

5. **Resume Operations**
   - Переключить application на restored database
   - Отключить maintenance mode

### 4.3 Сценарий 3: Security Incident (DDoS)

**Обнаружение:**
- CloudFlare DDoS detection alert
- Spike in traffic (10x+ normal)
- Increased latency

**Действия (автоматические):**

1. **CloudFlare Under Attack Mode** (мгновенно)
   - Включается автоматически при detection
   - Challenge все suspicious requests

2. **WAF Rules Enhancement** (1-2 минуты)
   ```bash
   # Усилить rate limiting
   aws wafv2 update-web-acl --name opus-casino-production-waf \
     --default-action Block
   ```

3. **Scale Infrastructure** (2-5 минут)
   - HPA автоматически увеличивает количество pods
   - K8s Cluster Autoscaler добавляет ноды

4. **Block Attack Vectors** (по мере выявления)
   - Добавить IP ranges в block list
   - Update WAF rules

---

## 5. Тестирование DR

### 5.1 Типы тестов

| Тип | Частота | Описание |
|-----|---------|----------|
| **Tabletop Exercise** | Ежемесячно | Прохождение сценариев на бумаге |
| **Component Failover** | Ежеквартально | Тестирование отдельных компонентов |
| **Full DR Drill** | Ежегодно | Полное переключение на DR регион |

### 5.2 Checklist для Full DR Drill

**Preparation:**
- [ ] Уведомить stakeholders за 2 недели
- [ ] Запланировать maintenance window (суббота 02:00-06:00 UTC)
- [ ] Подготовить rollback plan
- [ ] Создать backup перед тестом

**Execution:**
- [ ] 02:00 — Начать drill
- [ ] 02:05 — CloudFlare failover test
- [ ] 02:15 — RDS replica promote test
- [ ] 02:30 — EKS scale up test
- [ ] 02:45 — Application smoke tests
- [ ] 03:00 — Performance tests
- [ ] 03:30 — Failback to primary
- [ ] 04:00 — Verify primary region working
- [ ] 04:30 — Restore DR to standby
- [ ] 05:00 — Завершить drill

**Post-Drill:**
- [ ] Document lessons learned
- [ ] Update runbooks
- [ ] Share report with stakeholders

---

## 6. Monitoring & Alerting

### 6.1 Ключевые метрики

| Метрика | Threshold | Action |
|---------|-----------|--------|
| **Region Health** | Primary unreachable | Trigger failover |
| **RDS Replication Lag** | > 300 seconds | Alert on-call |
| **MSK Replication Lag** | > 60 seconds | Alert on-call |
| **S3 Replication Latency** | > 15 minutes | Alert on-call |
| **Error Rate** | > 1% | Investigate |
| **Latency p99** | > 500ms | Investigate |

### 6.2 Dashboards

- **Grafana:** `Production DR Status`
  - Replication lag для всех компонентов
  - Health status по регионам
  - Traffic distribution

### 6.3 Alerts

```yaml
# Alert: RDS Replication Lag High
alert: RDSReplicationLagHigh
expr: aws_rds_replica_lag_seconds > 300
for: 5m
labels:
  severity: warning
annotations:
  summary: "RDS replication lag is high"
  description: "Replication lag is {{ $value }} seconds"

# Alert: Primary Region Unreachable
alert: PrimaryRegionUnreachable
expr: probe_success{region="us-east-1"} == 0
for: 2m
labels:
  severity: critical
annotations:
  summary: "Primary region is unreachable"
  action: "Initiate DR failover"
```

---

## 7. Контакты и Эскалация

### 7.1 On-Call Rotation

| Роль | Контакт | Доступность |
|------|---------|-------------|
| **On-Call Engineer** | PagerDuty: ops-oncall | 24/7 |
| **DevOps Lead** | +1-XXX-XXX-XXXX | 24/7 |
| **CTO** | +1-XXX-XXX-XXXX | Critical only |

### 7.2 Escalation Matrix

| Время | Уровень | Контакт |
|-------|---------|---------|
| **0-15 мин** | L1 | On-call engineer |
| **15-30 мин** | L2 | DevOps lead + Team lead |
| **30+ мин** | L3 | CTO + VP Engineering |

---

## 8. Приложения

### 8.1 Terraform Variables для DR

```hcl
# environments/dr-replica/terraform.tfvars
aws_region       = "us-west-2"
environment      = "dr-replica"
is_dr            = true

primary_region   = "us-east-1"

rds_replicate_source_db_arn = "arn:aws:rds:us-east-1:123456789012:db:opus-casino-production"
msk_replicate_source_cluster_arn = "arn:aws:kafka:us-east-1:123456789012:cluster/opus-casino-production/xxx"

# Scale down для cost savings (DR в standby)
eks_node_min_size = 3
eks_node_desired_size = 5
```

### 8.2 Scripts

**failover.sh:**
```bash
#!/bin/bash
# Automated failover script

set -e

echo "Starting failover to DR region..."

# 1. Promote RDS replica
echo "Promoting RDS replica..."
aws rds promote-read-replica \
  --db-instance-identifier opus-casino-dr-replica \
  --region us-west-2

# 2. Wait for RDS to be available
echo "Waiting for RDS to be available..."
aws rds wait db-instance-available \
  --db-instance-identifier opus-casino-dr-replica \
  --region us-west-2

# 3. Scale up EKS
echo "Scaling up EKS cluster..."
kubectl --context=dr scale nodeset platform-nodes --replicas=15

# 4. Update CloudFlare (через API)
echo "Updating CloudFlare DNS..."
curl -X PUT "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records/$RECORD_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"content":"DR_IP","proxied":true}'

echo "Failover complete!"
```

### 8.3 Runbook: DR Failover

**URL:** `https://runbooks.opus-casino.com/dr-failover`

**Steps:**
1. Confirm primary region is down
2. Notify stakeholders
3. Execute failover.sh
4. Verify application health in DR
5. Monitor metrics
6. Prepare for failback

---

## 9. Cost Optimization для DR

### 9.1 DR в Standby Mode

| Компонент | Production | DR (Standby) | Savings |
|-----------|------------|--------------|---------|
| **EKS Nodes** | 15 nodes | 5 nodes | 67% |
| **RDS** | db.r5.2xlarge + 2 RR | db.r5.xlarge (single) | 60% |
| **ElastiCache** | 6 nodes | 3 nodes | 50% |
| **MSK** | 3 brokers | 3 brokers (smaller) | 30% |

**Monthly DR Cost:** ~$5,000 (vs $15,000 production)

### 9.2 Auto-Scaling для DR

```hcl
# DR autoscaling policy
resource "aws_autoscaling_policy" "dr_scale_up" {
  name                   = "dr-emergency-scale-up"
  scaling_adjustment     = 10
  adjustment_type        = "ChangeInCapacity"
  autoscaling_group_name = aws_autoscaling_group.dr.name
}
```

---

## 10. Compliance и Аудит

### 10.1 Требования

- **SOC 2:** Требуется DR plan и регулярное тестирование
- **GDPR:** Data residency compliance
- **Gambling License:** Business continuity requirements

### 10.2 Audit Trail

Все DR события логируются:
- CloudTrail: API calls
- CloudWatch Logs: Application logs
- S3: Backup logs

**Retention:** 7 лет для compliance

---

## 11. История Изменений

| Версия | Дата | Автор | Изменения |
|--------|------|-------|-----------|
| 1.0 | 2024-03-24 | DevOps Team | Initial version |
| | | | |

---

**Document Owner:** DevOps Lead  
**Review Cycle:** Quarterly  
**Next Review:** 2024-06-24
