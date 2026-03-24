# Istio Service Mesh Configuration
# Service mesh для Opus Casino platform

## Структура

```
istio/
├── base/              ← Базовая установка Istio
├── base-operator/     ← Istio Operator
├── gateway/           ← Ingress/Egress gateways
├── config/            ← Конфигурация (DestinationRules, VirtualServices)
└── policies/          ← Authorization policies
```

## Быстрый старт

### Установка Istio

```bash
# Install Istio Operator
kubectl apply -f base-operator/

# Install Istio base
kubectl apply -f base/

# Install Istio gateway
kubectl apply -f gateway/

# Apply configuration
kubectl apply -f config/
```

### Проверка установки

```bash
# Check Istio pods
kubectl get pods -n istio-system

# Check gateway
kubectl get svc -n istio-system

# Verify mTLS
istioctl analyze
```

## Конфигурация

### mTLS

- **Режим:** STRICT для production
- **Исключения:** Dev окружение (PERMISSIVE для отладки)

### Traffic Management

- **Timeout:** 30s по умолчанию
- **Retries:** 3 попытки
- **Circuit Breaker:** 5 consecutive errors

### Rate Limiting

- **Login:** 10 req/min per IP
- **API:** 100 req/min per user
- **WebSocket:** 1000 connections per IP
