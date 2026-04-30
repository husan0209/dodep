# Observability Stack для Opus Casino

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                    OBSERVABILITY STACK                       │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Metrics    │  │    Logs      │  │   Tracing    │      │
│  │              │  │              │  │              │      │
│  │ Victoria     │  │   Vector →   │  │ OpenTelemetry│      │
│  │ Metrics      │  │ ClickHouse   │  │   + Jaeger   │      │
│  │              │  │              │  │              │      │
│  │   Grafana    │  │   Grafana    │  │   Grafana    │      │
│  │   (UI)       │  │   (UI)       │  │   (UI)       │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Alerting (PagerDuty)                     │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Компоненты

### Metrics (VictoriaMetrics + Grafana)

| Компонент | Версия | Реплики | CPU | Memory |
|-----------|--------|---------|-----|--------|
| vmselect | 1.95 | 2 | 500m | 1Gi |
| vminsert | 1.95 | 2 | 500m | 1Gi |
| vmstorage | 1.95 | 3 | 1000m | 4Gi |
| Grafana | 10.2 | 2 | 250m | 512Mi |

**Retention:** 90 дней raw, 1 год downsampled

### Logging (Vector → ClickHouse)

| Компонент | Версия | Реплики | CPU | Memory |
|-----------|--------|---------|-----|--------|
| Vector (DaemonSet) | 0.35 | 1 per node | 200m | 256Mi |
| ClickHouse | 23.12 | 3 | 2000m | 8Gi |

**Retention:** 30 дней raw, 1 год aggregated

### Tracing (OpenTelemetry + Jaeger)

| Компонент | Версия | Реплики | CPU | Memory |
|-----------|--------|---------|-----|--------|
| OTEL Collector | 0.90 | 2 | 500m | 1Gi |
| Jaeger | 1.50 | 1 | 1000m | 2Gi |

**Sampling:** 100% errors, 10% normal

## Быстрый старт

```bash
# Install monitoring namespace
kubectl apply -f infra/k8s/monitoring/namespace.yaml

# Install VictoriaMetrics
kubectl apply -f infra/k8s/monitoring/victoriametrics/

# Install Grafana
kubectl apply -f infra/k8s/monitoring/grafana/

# Install Vector logging
kubectl apply -f infra/k8s/logging/vector/

# Install Jaeger tracing
kubectl apply -f infra/k8s/tracing/jaeger/
```

## Дашборды

- K8s Cluster Overview
- Node Resources
- Pod Resources
- Istio Mesh Dashboard
- PostgreSQL Dashboard
- DragonflyDB Dashboard
- ClickHouse Dashboard
- Business Metrics Template

## Alerting

| Severity | Канал | Response Time |
|----------|-------|---------------|
| P1 | PagerDuty (page) | < 5 min |
| P2 | PagerDuty (notify) | < 15 min |
| P3 | Slack | < 1 hour |
| P4 | Email (daily digest) | < 24 hours |
