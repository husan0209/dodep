# Runbooks для Opus Casino Platform

## Индекс runbooks

| ID | Название | Категория | Критичность |
|----|----------|-----------|-------------|
| RB-001 | High Error Rate | Incident | P1 |
| RB-002 | Database Connection Pool Exhausted | Incident | P1 |
| RB-003 | High P99 Latency | Incident | P2 |
| RB-004 | Disk Usage Critical | Warning | P2 |
| RB-005 | Memory Usage High | Warning | P2 |
| RB-006 | Pod CrashLoopBackOff | Incident | P1 |
| RB-007 | Service Unavailable | Incident | P1 |
| RB-008 | Certificate Expiring Soon | Warning | P3 |
| RB-009 | Backup Failed | Incident | P1 |
| RB-010 | DDoS Attack Detected | Incident | P1 |

---

## RB-001: High Error Rate

**Критичность:** P1  
**Симптомы:** Error rate > 1% в течение 5 минут  
**Alert:** `High Error Rate` → PagerDuty

### Диагностика

```bash
# 1. Проверить dashboard
open https://grafana.opus.casino/d/error-rate

# 2. Определить затронутые сервисы
kubectl get pods -n production | grep -v Running

# 3. Проверить логи
kubectl logs -n production -l app=<service-name> --tail=100 | grep ERROR

# 4. Проверить трейсы
open https://jaeger.opus.casino/search?service=<service-name>&tags=error
```

### Действия

1. **Определить масштаб**
   - Один сервис или несколько?
   - Какие endpoint'ы затронуты?

2. **Проверить недавние деплои**
   ```bash
   kubectl rollout history deployment/<service-name> -n production
   ```

3. **Если деплой был < 30 мин назад → Rollback**
   ```bash
   kubectl rollout undo deployment/<service-name> -n production
   ```

4. **Если не деплой → Проверить зависимости**
   - Базы данных доступны?
   - Redis/Dragonfly отвечает?
   - Redpanda работает?

5. **Эскалация**
   - Если не resolved за 15 мин → On-call lead
   - Если не resolved за 30 мин → Incident commander

### Post-Incident

- [ ] Создать post-mortem в Confluence
- [ ] Добавить runbook update если нужно
- [ ] Обновить alerting rules если false positive

---

## RB-002: Database Connection Pool Exhausted

**Критичность:** P1  
**Симптомы:** `db_pool_available < 5` в течение 2 минут  
**Alert:** `Database Connection Pool Exhausted` → PagerDuty

### Диагностика

```bash
# 1. Проверить метрики пула
open https://grafana.opus.casino/d/postgresql-pool

# 2. Подключиться к PostgreSQL
kubectl port-forward svc/postgresql -n data 5432:5432
psql "postgres://postgres:password@localhost:5432/opus_casino"

# 3. Проверить активные подключения
SELECT count(*) FROM pg_stat_activity;

# 4. Проверить долгие запросы
SELECT pid, now() - pg_stat_activity.query_start AS duration, query
FROM pg_stat_activity
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';
```

### Действия

1. **Определить виновника**
   - Какой сервис держит больше всего коннекций?
   - Есть ли долгие запросы?

2. **Kill долгие запросы**
   ```sql
   SELECT pg_terminate_backend(pid)
   WHERE (now() - pg_stat_activity.query_start) > interval '30 minutes';
   ```

3. **Увеличить размер пула (временное решение)**
   ```bash
   kubectl set env deployment/postgresql -n data \
     MAX_CONNECTIONS=200
   ```

4. **Проверить PgBouncer настройки**
   ```bash
   kubectl logs -n data -l app=pgbouncer --tail=50
   ```

5. **Если проблема в коде → Escalate к команде**

### Post-Incident

- [ ] Review connection pool настройки для каждого сервиса
- [ ] Добавить connection limits per service
- [ ] Рассмотреть connection pooling на уровне приложения

---

## RB-003: High P99 Latency

**Критичность:** P2  
**Симптомы:** `p99_latency > 500ms` в течение 10 минут  
**Alert:** `High P99 Latency` → PagerDuty

### Диагностика

```bash
# 1. Проверить latency dashboard
open https://grafana.opus.casino/d/latency

# 2. Определить медленные endpoint'ы
open https://jaeger.opus.casino/search?minDuration=500ms

# 3. Проверить CPU/Memory у подов
kubectl top pods -n production | sort -k2 -nr | head -20

# 4. Проверить throttling
kubectl get pods -n production -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.containerStatuses[*].state}{"\n"}{end}'
```

### Действия

1. **Определить bottleneck**
   - CPU bound? → Проверить profiling
   - I/O bound? → Проверить БД, кэш
   - Network bound? → Проверить external API

2. **Проверить external зависимости**
   - Платежные провайдеры
   - Game providers
   - SMS/Email сервисы

3. **Проверить кэш hit rate**
   ```bash
   # DragonflyDB stats
   kubectl port-forward svc/dragonfly -n data 6379:6379
   redis-cli INFO stats
   ```

4. **Если проблема в БД → Смотри RB-002**

5. **Scale up если нужно**
   ```bash
   kubectl scale deployment/<service-name> -n production --replicas=10
   ```

### Post-Incident

- [ ] Профилировать медленные сервисы (Pyroscope)
- [ ] Оптимизировать медленные запросы
- [ ] Рассмотреть кэширование

---

## RB-004: Disk Usage Critical

**Критичность:** P2  
**Симптомы:** `disk_usage > 85%`  
**Alert:** `Disk Usage Critical` → PagerDuty

### Диагностика

```bash
# 1. Определить какой volume
open https://grafana.opus.casino/d/disk-usage

# 2. Проверить PVC
kubectl get pvc -n <namespace>

# 3. Проверить использование внутри pod
kubectl exec -it <pod-name> -n <namespace> -- df -h

# 4. Найти большие файлы
kubectl exec -it <pod-name> -n <namespace> -- \
  du -ah /var/lib | sort -rh | head -20
```

### Действия

1. **Очистить логи (если application logs)**
   ```bash
   kubectl exec -it <pod-name> -n <namespace> -- \
     truncate -s 0 /var/log/app/*.log
   ```

2. **Очистить старые данные (если применимо)**
   ```bash
   # ClickHouse старые партиции
   kubectl exec -it clickhouse-0 -n data -- \
     clickhouse-client --query "ALTER TABLE events DELETE WHERE timestamp < now() - INTERVAL 30 DAY"
   ```

3. **Увеличить volume (если нужно)**
   ```bash
   kubectl edit pvc/<pvc-name> -n <namespace>
   # Изменить spec.resources.requests.storage
   ```

4. **Проверить retention policies**
   - Логи: 30 дней?
   - Метрики: 90 дней?
   - Трейсы: 7 дней?

### Post-Incident

- [ ] Настроить alerting на 75% (раньше)
- [ ] Review retention policies
- [ ] Настроить автоматическую очистку

---

## RB-005: Memory Usage High

**Критичность:** P2  
**Симптомы:** `memory_usage > 90%` в течение 5 минут  
**Alert:** `Memory Usage High` → PagerDuty

### Диагностика

```bash
# 1. Проверить memory dashboard
open https://grafana.opus.casino/d/memory

# 2. Проверить pod'ы
kubectl top pods -n production --sort-by=memory

# 3. Проверить OOM kills
kubectl get pods -n production -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.containerStatuses[*].lastState.terminated.reason}{"\n"}{end}' | grep OOMKilled

# 4. Проверить memory limits
kubectl get pods -n production -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[*].resources.limits.memory}{"\n"}{end}'
```

### Действия

1. **Если OOM kill → Увеличить limits**
   ```bash
   kubectl set resources deployment/<service-name> -n production \
     --limits=memory=4Gi
   ```

2. **Если memory leak → Restart pod**
   ```bash
   kubectl rollout restart deployment/<service-name> -n production
   ```

3. **Профилировать memory usage**
   - Rust: jemalloc stats
   - Go: pprof
   - Python: memory_profiler

### Post-Incident

- [ ] Профилировать memory usage
- [ ] Исправить memory leaks
- [ ] Настроить правильные limits

---

## RB-006: Pod CrashLoopBackOff

**Критичность:** P1  
**Симптомы:** Pod перезапускается > 3 раз  
**Alert:** Автоматически из Kubernetes

### Диагностика

```bash
# 1. Проверить статус pod'а
kubectl describe pod <pod-name> -n <namespace>

# 2. Проверить логи
kubectl logs <pod-name> -n <namespace> --previous

# 3. Проверить events
kubectl get events -n <namespace> --sort-by='.lastTimestamp'

# 4. Проверить liveness/readiness probes
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.spec.containers[*].livenessProbe}'
```

### Действия

1. **Определить причину**
   - Application error? → Смотри логи
   - OOM kill? → Увеличь memory
   - Failed probe? → Проверь health endpoint

2. **Если application error**
   ```bash
   # Проверить конфиги
   kubectl get configmap <config-name> -n <namespace> -o yaml
   
   # Проверить секреты
   kubectl get secret <secret-name> -n <namespace> -o jsonpath='{.data}'
   ```

3. **Если failed probe → Увеличить thresholds**
   ```bash
   kubectl edit deployment/<service-name> -n <namespace>
   # initialDelaySeconds: 30 → 60
   # failureThreshold: 3 → 5
   ```

### Post-Incident

- [ ] Исправить root cause
- [ ] Добавить better health checks
- [ ] Update deployment manifest

---

## RB-007: Service Unavailable

**Критичность:** P1  
**Симптомы:** Service не отвечает, 503 ошибки  
**Alert:** Автоматически из monitoring

### Диагностика

```bash
# 1. Проверить endpoint'ы
kubectl get endpoints <service-name> -n <namespace>

# 2. Проверить pod'ы
kubectl get pods -n <namespace> -l app=<service-name>

# 3. Проверить Istio
istioctl analyze -n <namespace>

# 4. Проверить network policies
kubectl get networkpolicy -n <namespace>
```

### Действия

1. **Если нет endpoint'ов → Проверить pod'ы**
   - Все pod'ы down? → Смотри RB-006
   - Pod'ы есть но не ready? → Проверь readiness probe

2. **Если Istio проблема**
   ```bash
   # Проверить virtual service
   kubectl get virtualservice <service-name> -n <namespace> -o yaml
   
   # Проверить destination rule
   kubectl get destinationrule <service-name> -n <namespace> -o yaml
   ```

3. **Emergency bypass (если критично)**
   ```bash
   # Scale up другую версию
   kubectl scale deployment/<service-name>-v1 -n production --replicas=10
   ```

### Post-Incident

- [ ] Review Istio конфигурацию
- [ ] Добавить circuit breakers
- [ ] Update runbook

---

## RB-008: Certificate Expiring Soon

**Критичность:** P3  
**Симптомы:** Сертификат истекает < 7 дней  
**Alert:** `Certificate Expiring` → Slack

### Диагностика

```bash
# 1. Проверить сертификаты
kubectl get certificates -n <namespace>

# 2. Проверить детали
kubectl describe certificate <cert-name> -n <namespace>

# 3. Проверить cert-manager логи
kubectl logs -n cert-manager -l app=cert-manager --tail=50
```

### Действия

1. **Если cert-manager работает → Подождать**
   - Автоматическое обновление должно сработать

2. **Если не обновляется → Renew manually**
   ```bash
   kubectl delete certificate <cert-name> -n <namespace>
   # Cert-manager создаст новый
   ```

3. **Проверить что новый создан**
   ```bash
   kubectl get certificate <cert-name> -n <namespace>
   ```

### Post-Incident

- [ ] Проверить cert-manager конфигурацию
- [ ] Настроить alerting на 14 дней (раньше)

---

## RB-009: Backup Failed

**Критичность:** P1  
**Симптомы:** Backup не завершился успешно  
**Alert:** `Backup Failed` → PagerDuty

### Диагностика

```bash
# 1. Проверить backup статус
kubectl get backup -n data

# 2. Проверить логи backup job
kubectl logs job/<backup-job-name> -n data

# 3. Проверить S3 bucket
aws s3 ls s3://opus-casino-backups/data/
```

### Действия

1. **Определить причину failure**
   - S3 недоступен? → Проверить IAM permissions
   - Нет места? → Проверить S3 bucket
   - Timeout? → Увеличить timeout

2. **Запустить backup manually**
   ```bash
   kubectl create job --from=cronjob/backup-postgresql manual-backup -n data
   ```

3. **Проверить что backup создан**
   ```bash
   aws s3 ls s3://opus-casino-backups/data/ | tail -5
   ```

### Post-Incident

- [ ] Исправить root cause
- [ ] Проверить что backup restore работает
- [ ] Update backup configuration

---

## RB-010: DDoS Attack Detected

**Критичность:** P1  
**Симптомы:** Аномальный трафик, WAF триггеры  
**Alert:** CloudFlare → PagerDuty

### Диагностика

```bash
# 1. Проверить CloudFlare dashboard
open https://dash.cloudflare.com/opus.casino/security/ddos

# 2. Проверить WAF logs
open https://dash.cloudflare.com/opus.casino/security/waf

# 3. Проверить ingress трафик
kubectl top nodes | grep -E 'CPU|MEMORY'
```

### Действия

1. **Enable Under Attack mode (CloudFlare)**
   ```bash
   # Через CloudFlare dashboard или API
   curl -X PATCH "https://api.cloudflare.com/client/v4/zones/<zone>/settings/security_level" \
     -H "Authorization: Bearer <token>" \
     -H "Content-Type: application/json" \
     --data '{"value":"under_attack"}'
   ```

2. **Add rate limiting rules**
   ```bash
   # CloudFlare Rate Limiting
   # 100 requests per minute per IP
   ```

3. **Block attacking IPs**
   ```bash
   # CloudFlare WAF
   # Add IP access rules
   ```

4. **Scale up ingress**
   ```bash
   kubectl scale deployment/istio-ingressgateway -n istio-system --replicas=20
   ```

### Post-Incident

- [ ] Analyze attack patterns
- [ ] Update WAF rules
- [ ] Review capacity planning

---

## Emergency Contacts

| Роль | Имя | Телефон | Telegram |
|------|-----|---------|----------|
| On-call Engineer | — | — | @oncall |
| DevOps Lead | — | — | @devops-lead |
| Security Lead | — | — | @security-lead |
| Incident Commander | — | — | @incident-commander |

## Эскалация

```
P1 Incident:
  0-15 min: On-call Engineer
  15-30 min: DevOps Lead + Security Lead
  30+ min: Incident Commander + Management

P2 Incident:
  0-60 min: On-call Engineer
  60+ min: DevOps Lead

P3 Incident:
  0-24 hours: On-call Engineer (Slack)
  24+ hours: Create ticket
```
