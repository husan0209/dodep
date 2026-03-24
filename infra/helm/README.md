# Helm Charts для Opus Casino Platform

## Структура

```
charts/
├── service-chart/       ← Базовый chart для всех сервисов
├── betting-engine/      ← Betting Engine service
├── wallet-core/         ← Wallet Core service
├── auth/                ← Auth service
└── ...
```

## Быстрый старт

### Установка chart

```bash
# Install
helm install betting-engine ./charts/betting-engine \
  --namespace production \
  --set replicaCount=3 \
  --set image.tag=latest

# Upgrade
helm upgrade betting-engine ./charts/betting-engine \
  --namespace production \
  --set replicaCount=5

# Uninstall
helm uninstall betting-engine --namespace production
```

### Values для окружений

```bash
# Dev
helm install betting-engine ./charts/betting-engine \
  -f values-dev.yaml \
  --namespace dev

# Staging
helm install betting-engine ./charts/betting-engine \
  -f values-staging.yaml \
  --namespace staging

# Production
helm install betting-engine ./charts/betting-engine \
  -f values-production.yaml \
  --namespace production
```

## Требования

- Kubernetes >= 1.28
- Helm >= 3.13
- Ingress Controller (AWS ALB / Nginx)
- cert-manager (для HTTPS)
