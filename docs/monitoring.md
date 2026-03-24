# Мониторинг

## Стек observability

- **VictoriaMetrics** — метрики
- **Grafana** — дашборды
- **Vector** → **ClickHouse** — логи
- **Jaeger** — трейсинг
- **Sentry** — ошибки

## Метрики

### Основные метрики

| Метрика | Описание | Alert threshold |
|---------|----------|-----------------|
| `http_requests_total` | Всего запросов | - |
| `http_request_duration_seconds` | Латентность | p99 > 100ms |
| `http_requests_error_rate` | Процент ошибок | > 1% |
| `betting_engine_bets_total` | Всего ставок | - |
| `wallet_transactions_total` | Всего транзакций | - |

### VictoriaMetrics

URL: `https://victoria-metrics.opus-casino.com`

Примеры запросов:
```promql
# Error rate
sum(rate(http_requests_total{status=~"5.."}[5m])) 
/ 
sum(rate(http_requests_total[5m]))

# p99 latency
histogram_quantile(0.99, 
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le)
)

# Bets per second
rate(betting_engine_bets_total[1m])
```

## Логи

### ClickHouse (логи)

```sql
-- Последние ошибки
SELECT *
FROM logs.vector_logs
WHERE severity_text = 'ERROR'
ORDER BY timestamp DESC
LIMIT 100;

-- Логи по сервису
SELECT *
FROM logs.vector_logs
WHERE resource_attributes['service.name'] = 'betting-engine'
ORDER BY timestamp DESC
LIMIT 100;
```

## Трейсинг

### Jaeger

URL: `https://jaeger.opus-casino.com`

Поиск трейсов:
- По сервису
- По operation name
- По duration
- По tags

## Дашборды

### Grafana

URL: `https://grafana.opus-casino.com`

Готовые дашборды:
- **System Overview** — общая картина
- **Betting Engine** — метрики ставок
- **Wallet Core** — транзакции и балансы
- **Auth Service** — аутентификация
- **Database** — PostgreSQL, Redis
- **Infrastructure** — Kubernetes, nodes

## Алертинг

### Grafana Alerting

Правила алертинга:
- High error rate (> 1% за 5 мин)
- High latency (p99 > 100ms за 5 мин)
- Pod restarts (> 3 за 10 мин)
- Low disk space (< 20%)
- High memory usage (> 85%)

### PagerDuty

Интеграция через Grafana Alerting.

## Профилирование

### Pyroscope

URL: `https://pyroscope.opus-casino.com`

Continuous profiling для:
- CPU usage
- Memory allocation
- Goroutines (Go)
