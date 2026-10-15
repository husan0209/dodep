## #51 alerting-rules.skill.md

```markdown
# alerting-rules.skill.md

## РОЛЬ
Ты настраиваешь правила алертинга для гемблинг-платформы.
Каждый алерт должен быть actionable — иметь runbook и чёткое действие.

## КОНТЕКСТ
- Alerting: Grafana Alerting
- Notification: PagerDuty (P1/P2), Slack (P3/P4)
- Каждый алерт имеет severity, runbook, escalation
- False positive rate целевой: < 5%

## SEVERITY LEVELS
P1 — CRITICAL: Платформа не работает или потеря денег
Response time: < 5 минут
Notification: PagerDuty → phone call + SMS
Escalation: 5 мин → Team Lead, 15 мин → CTO
Примеры:
- Все API возвращают 500
- БД недоступна
- Финансовая рассогласованность (reconciliation failed)
- Платёжная система не работает
- Zero bets для > 5 минут (в рабочие часы)

P2 — HIGH: Деградация, часть пользователей затронута
Response time: < 15 минут
Notification: PagerDuty → push notification
Escalation: 30 мин → Team Lead
Примеры:
- p99 latency > 500ms
- Error rate > 1%
- Один сервис недоступен
- Disk usage > 85%
- Consumer lag > 10K messages
- Cache miss rate > 30%

P3 — MEDIUM: Потенциальная проблема
Response time: < 1 часа
Notification: Slack #alerts
Примеры:
- p99 latency > 200ms
- CPU > 70% sustained
- Certificate expiry < 14 days
- Deployment failed (auto-rollback сработал)

P4 — LOW: Информационные
Response time: < 24 часа
Notification: Slack #alerts-low, daily digest email
Примеры:
- Disk usage > 70%
- Dependency vulnerability detected
- Unusual traffic pattern

text


## INFRASTRUCTURE ALERTS

```yaml
# Kubernetes alerts
- alert: PodCrashLooping
  expr: rate(kube_pod_container_status_restarts_total[15m]) > 0.1
  for: 10m
  severity: P2
  runbook: docs/runbooks/pod-crash-loop.md
  annotations:
    summary: "Pod {{ $labels.pod }} is crash looping"
    description: "Pod has restarted {{ $value }} times in 15 minutes"

- alert: PodNotReady
  expr: kube_pod_status_ready{condition="false"} == 1
  for: 5m
  severity: P2
  runbook: docs/runbooks/pod-not-ready.md

- alert: NodeNotReady
  expr: kube_node_status_condition{condition="Ready",status="true"} == 0
  for: 2m
  severity: P1
  runbook: docs/runbooks/node-not-ready.md

- alert: HighCPUUsage
  expr: |
    (1 - avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))) > 0.85
  for: 15m
  severity: P2
  runbook: docs/runbooks/high-cpu.md

- alert: HighMemoryUsage
  expr: |
    (node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes)
    / node_memory_MemTotal_bytes > 0.90
  for: 10m
  severity: P2
  runbook: docs/runbooks/high-memory.md

- alert: DiskSpaceCritical
  expr: |
    (node_filesystem_avail_bytes / node_filesystem_size_bytes) < 0.10
  for: 5m
  severity: P1
  runbook: docs/runbooks/disk-space.md

- alert: DiskSpaceWarning
  expr: |
    (node_filesystem_avail_bytes / node_filesystem_size_bytes) < 0.20
  for: 30m
  severity: P3
  runbook: docs/runbooks/disk-space.md
APPLICATION ALERTS
YAML

# HTTP Error Rate
- alert: HighErrorRate
  expr: |
    sum(rate(http_requests_total{status=~"5.."}[5m])) by (service)
    / sum(rate(http_requests_total[5m])) by (service)
    > 0.01
  for: 3m
  severity: P1
  runbook: docs/runbooks/high-error-rate.md
  annotations:
    summary: "{{ $labels.service }} error rate is {{ $value | humanizePercentage }}"

- alert: HighLatencyP99
  expr: |
    histogram_quantile(0.99,
      sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service)
    ) > 0.5
  for: 5m
  severity: P2
  runbook: docs/runbooks/high-latency.md

- alert: HighLatencyP99Critical
  expr: |
    histogram_quantile(0.99,
      sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service)
    ) > 2.0
  for: 2m
  severity: P1
  runbook: docs/runbooks/high-latency.md

# Service-specific
- alert: BettingEngineDown
  expr: up{job="betting-engine"} == 0
  for: 1m
  severity: P1
  runbook: docs/runbooks/betting-engine-down.md

- alert: ZeroBetsPlaced
  expr: |
    sum(rate(betting_bets_placed_total[5m])) == 0
    and ON() hour() >= 8 and ON() hour() <= 23
  for: 5m
  severity: P1
  runbook: docs/runbooks/zero-bets.md
  annotations:
    summary: "No bets placed for 5 minutes during business hours"
DATABASE ALERTS
YAML

- alert: PostgresConnectionPoolExhausted
  expr: |
    pgbouncer_pools_server_active_connections
    / pgbouncer_pools_server_max_connections > 0.90
  for: 3m
  severity: P1
  runbook: docs/runbooks/db-pool-exhausted.md

- alert: PostgresReplicationLag
  expr: pg_replication_lag_seconds > 5
  for: 2m
  severity: P2
  runbook: docs/runbooks/db-replication-lag.md

- alert: PostgresSlowQueries
  expr: |
    rate(pg_stat_statements_mean_exec_time_ms[5m]) > 100
  for: 10m
  severity: P3
  runbook: docs/runbooks/db-slow-queries.md

- alert: PostgresDeadlocks
  expr: rate(pg_stat_database_deadlocks[5m]) > 0
  for: 5m
  severity: P2
  runbook: docs/runbooks/db-deadlocks.md

- alert: DragonflyDBHighMemory
  expr: dragonfly_memory_used_bytes / dragonfly_memory_max_bytes > 0.85
  for: 5m
  severity: P2
  runbook: docs/runbooks/cache-high-memory.md

- alert: ClickHouseIngestionLag
  expr: |
    clickhouse_async_insert_queue_size > 10000
  for: 5m
  severity: P3
  runbook: docs/runbooks/clickhouse-ingestion.md
BUSINESS ALERTS
YAML

- alert: FinancialReconciliationFailed
  expr: platform_reconciliation_discrepancy_total > 0
  for: 0m  # немедленно!
  severity: P1
  runbook: docs/runbooks/reconciliation-failed.md
  annotations:
    summary: "CRITICAL: Financial reconciliation discrepancy detected"
    description: "Wallet balance != sum of transactions. Immediate investigation required."

- alert: PaymentSuccessRateLow
  expr: |
    sum(rate(platform_deposits_total{status="success"}[15m]))
    / sum(rate(platform_deposits_total[15m]))
    < 0.90
  for: 10m
  severity: P2
  runbook: docs/runbooks/payment-low-success.md

- alert: WithdrawalQueueBacklog
  expr: platform_withdrawal_queue_depth > 100
  for: 30m
  severity: P3
  runbook: docs/runbooks/withdrawal-backlog.md

- alert: FraudAlertSpike
  expr: |
    rate(platform_fraud_blocks_total[15m])
    > 3 * avg_over_time(rate(platform_fraud_blocks_total[15m])[7d:1h])
  for: 15m
  severity: P2
  runbook: docs/runbooks/fraud-spike.md

- alert: CasinoRTPDeviation
  expr: |
    abs(platform_casino_actual_rtp - platform_casino_theoretical_rtp) > 3
  for: 1h
  severity: P2
  runbook: docs/runbooks/casino-rtp-deviation.md
  annotations:
    summary: "Casino RTP deviation > 3% for game {{ $labels.game_id }}"

- alert: AbnormalBetVolume
  expr: |
    sum(rate(betting_bets_placed_total[5m]))
    > 3 * avg_over_time(sum(rate(betting_bets_placed_total[5m]))[7d:1h])
  for: 10m
  severity: P3
  runbook: docs/runbooks/abnormal-bet-volume.md
MESSAGE QUEUE ALERTS
YAML

- alert: RedpandaConsumerLag
  expr: |
    redpanda_kafka_consumer_group_lag > 10000
  for: 5m
  severity: P2
  runbook: docs/runbooks/consumer-lag.md
  annotations:
    summary: "Consumer {{ $labels.group }} lag is {{ $value }} for topic {{ $labels.topic }}"

- alert: RedpandaDLQMessages
  expr: |
    increase(redpanda_kafka_topic_messages_total{topic=~".*\\.dlq"}[5m]) > 0
  for: 0m
  severity: P2
  runbook: docs/runbooks/dlq-messages.md
  annotations:
    summary: "Messages in Dead Letter Queue: {{ $labels.topic }}"

- alert: RedpandaBrokerDown
  expr: up{job="redpanda"} == 0
  for: 1m
  severity: P1
  runbook: docs/runbooks/redpanda-broker-down.md
SECURITY ALERTS
YAML

- alert: HighAuthFailureRate
  expr: |
    sum(rate(auth_login_attempts_total{status="failure"}[5m])) by (ip)
    > 10
  for: 2m
  severity: P2
  runbook: docs/runbooks/auth-brute-force.md

- alert: WAFBlockSpike
  expr: |
    rate(cloudflare_waf_blocked_total[5m])
    > 5 * avg_over_time(rate(cloudflare_waf_blocked_total[5m])[7d:1h])
  for: 10m
  severity: P2
  runbook: docs/runbooks/waf-block-spike.md

- alert: CertificateExpiringSoon
  expr: |
    (probe_ssl_earliest_cert_expiry - time()) / 86400 < 14
  for: 1h
  severity: P3
  runbook: docs/runbooks/cert-expiry.md
RUNBOOK TEMPLATE
Markdown

# Runbook: High Error Rate

## Alert
- Name: HighErrorRate
- Severity: P1
- Condition: Error rate > 1% for > 3 minutes

## Impact
Users cannot place bets / make deposits / login

## Diagnosis Steps
1. Check Grafana dashboard: [Service Overview](link)
2. Check recent deployments: `kubectl rollout history -n platform`
3. Check logs: Grafana → Explore → ClickHouse
   Filter: service={affected_service}, level=error, last 15min
4. Check dependencies:
   - Database: `kubectl exec -n data pg-0 -- pg_isready`
   - Cache: `kubectl exec -n data dragonfly-0 -- redis-cli ping`
   - Redpanda: check consumer lag dashboard
5. Check trace: Jaeger → find error traces

## Resolution
- If recent deployment → rollback:
  `kubectl argo rollouts undo {service} -n platform`
- If database issue → see DB runbook
- If cache issue → see Cache runbook
- If external dependency → check status page, enable circuit breaker

## Escalation
- 5 min no resolution → escalate to Team Lead
- 15 min no resolution → escalate to CTO
- If financial impact → notify Finance team immediately
АНТИПАТТЕРНЫ
YAML

# ❌ ПЛОХО: алерт без runbook
- alert: SomethingBroken
  expr: something > threshold
  # Что делать? Кто знает...

# ✅ ПРАВИЛЬНО: всегда с runbook
  runbook: docs/runbooks/something-broken.md

# ❌ ПЛОХО: слишком чувствительный (alert fatigue)
- alert: HighCPU
  expr: cpu > 50%
  for: 1m
  # Срабатывает 100 раз в день → игнорируется

# ✅ ПРАВИЛЬНО: разумный порог и duration
  expr: cpu > 85%
  for: 15m

# ❌ ПЛОХО: нет for (мгновенный алерт)
- alert: ErrorRate
  expr: error_rate > 0.01
  # Один spike → alert → resolve → alert → resolve

# ✅ ПРАВИЛЬНО: with for duration
  for: 3m  # стабильная проблема, не spike

# ❌ ПЛОХО: один алерт для всех severity
- alert: Latency
  expr: latency > 100ms
  severity: P1
  # 100ms для betting = P2, для notifications = P4

# ✅ ПРАВИЛЬНО: разные пороги для разных сервисов
- alert: BettingLatencyHigh
  expr: latency{service="betting-engine"} > 50ms
  severity: P2
- alert: NotificationLatencyHigh
  expr: latency{service="notification-service"} > 1000ms
  severity: P4