## #43 istio-service-mesh.skill.md

```markdown
# istio-service-mesh.skill.md

## РОЛЬ
Ты — Platform Engineer, настраивающий Istio service mesh
для гемблинг-платформы. Istio обеспечивает mTLS, traffic management,
observability и security между микросервисами.

## КОНТЕКСТ
- Istio 1.20+ (ambient mode рассмотреть в будущем)
- mTLS STRICT между всеми сервисами
- Traffic management: canary, circuit breaker, retry
- Authorization policies: deny-all по умолчанию
- Интеграция с Jaeger (tracing), VictoriaMetrics (metrics)

## mTLS CONFIGURATION

```yaml
# PeerAuthentication: STRICT mTLS для всего mesh
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: default
  namespace: istio-system
spec:
  mtls:
    mode: STRICT
---
# Для namespace platform
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: platform-mtls
  namespace: platform
spec:
  mtls:
    mode: STRICT
AUTHORIZATION POLICIES
YAML

# Deny all по умолчанию
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: deny-all
  namespace: platform
spec:
  {}  # пустой spec = deny all
---
# Разрешить API Gateway → все сервисы
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-from-gateway
  namespace: platform
spec:
  action: ALLOW
  rules:
    - from:
        - source:
            principals: ["cluster.local/ns/istio-system/sa/istio-ingressgateway"]
      to:
        - operation:
            ports: ["8080"]
---
# Betting Engine ← только API Gateway + Wallet Service
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: betting-engine-policy
  namespace: platform
spec:
  selector:
    matchLabels:
      app: betting-engine
  action: ALLOW
  rules:
    - from:
        - source:
            principals:
              - "cluster.local/ns/istio-system/sa/istio-ingressgateway"
              - "cluster.local/ns/platform/sa/wallet-service"
              - "cluster.local/ns/platform/sa/risk-engine"
      to:
        - operation:
            ports: ["8080", "9000"]
    # Metrics всегда доступны для мониторинга
    - from:
        - source:
            namespaces: ["monitoring"]
      to:
        - operation:
            ports: ["9090"]
---
# Wallet Service ← только Betting + Payment + Casino
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: wallet-service-policy
  namespace: platform
spec:
  selector:
    matchLabels:
      app: wallet-service
  action: ALLOW
  rules:
    - from:
        - source:
            principals:
              - "cluster.local/ns/platform/sa/betting-engine"
              - "cluster.local/ns/platform/sa/payment-service"
              - "cluster.local/ns/platform/sa/casino-service"
              - "cluster.local/ns/platform/sa/bonus-service"
      to:
        - operation:
            ports: ["9000"]  # только gRPC
---
# PostgreSQL ← только сервисы из platform namespace
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: postgres-policy
  namespace: data
spec:
  selector:
    matchLabels:
      app: postgresql
  action: ALLOW
  rules:
    - from:
        - source:
            namespaces: ["platform"]
      to:
        - operation:
            ports: ["5432"]
VIRTUAL SERVICE — TRAFFIC MANAGEMENT
YAML

# Canary deployment: 95% stable, 5% canary
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: betting-engine
  namespace: platform
spec:
  hosts:
    - betting-engine
  http:
    - match:
        - headers:
            x-canary:
              exact: "true"
      route:
        - destination:
            host: betting-engine
            subset: canary
    - route:
        - destination:
            host: betting-engine
            subset: stable
          weight: 95
        - destination:
            host: betting-engine
            subset: canary
          weight: 5
      timeout: 10s
      retries:
        attempts: 3
        perTryTimeout: 3s
        retryOn: 5xx,reset,connect-failure,retriable-4xx
---
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: betting-engine
  namespace: platform
spec:
  host: betting-engine
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 1000
        connectTimeout: 5s
      http:
        h2UpgradePolicy: UPGRADE
        maxRequestsPerConnection: 100
    outlierDetection:
      consecutive5xxErrors: 5
      interval: 30s
      baseEjectionTime: 30s
      maxEjectionPercent: 30
    loadBalancer:
      simple: LEAST_REQUEST
  subsets:
    - name: stable
      labels:
        version: stable
    - name: canary
      labels:
        version: canary
CIRCUIT BREAKER
YAML

# DestinationRule с circuit breaker для внешних сервисов
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: payment-provider
  namespace: platform
spec:
  host: api.stripe.com
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 100
        connectTimeout: 10s
      http:
        maxRequestsPerConnection: 10
        maxRetries: 3
    outlierDetection:
      consecutive5xxErrors: 3
      interval: 10s
      baseEjectionTime: 60s
      maxEjectionPercent: 50
GATEWAY — INGRESS
YAML

# Istio Gateway (вместо отдельного Ingress Controller)
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: platform-gateway
  namespace: istio-system
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 443
        name: https
        protocol: HTTPS
      tls:
        mode: SIMPLE
        credentialName: platform-tls-cert
      hosts:
        - "api.example.com"
        - "www.example.com"
        - "ws.example.com"
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "*.example.com"
      tls:
        httpsRedirect: true
---
# VirtualService для внешнего роутинга
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: api-routing
  namespace: platform
spec:
  hosts:
    - "api.example.com"
  gateways:
    - istio-system/platform-gateway
  http:
    - match:
        - uri:
            prefix: /api/v1/auth
      route:
        - destination:
            host: auth-service
            port:
              number: 8080
    - match:
        - uri:
            prefix: /api/v1/bets
      route:
        - destination:
            host: betting-engine
            port:
              number: 8080
    - match:
        - uri:
            prefix: /api/v1/wallet
      route:
        - destination:
            host: wallet-service
            port:
              number: 8080
    - match:
        - uri:
            prefix: /api/v1/payments
      route:
        - destination:
            host: payment-service
            port:
              number: 8080
    - match:
        - uri:
            prefix: /api/v1/casino
      route:
        - destination:
            host: casino-service
            port:
              number: 8080
---
# WebSocket routing
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: ws-routing
  namespace: platform
spec:
  hosts:
    - "ws.example.com"
  gateways:
    - istio-system/platform-gateway
  http:
    - match:
        - uri:
            prefix: /
          headers:
            upgrade:
              exact: websocket
      route:
        - destination:
            host: websocket-gateway
            port:
              number: 8080
      timeout: 3600s  # 1 час для WebSocket
RATE LIMITING
YAML

# EnvoyFilter для rate limiting
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: rate-limit
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        listener:
          filterChain:
            filter:
              name: envoy.filters.network.http_connection_manager
      patch:
        operation: INSERT_BEFORE
        value:
          name: envoy.filters.http.local_ratelimit
          typed_config:
            "@type": type.googleapis.com/udpa.type.v1.TypedStruct
            type_url: type.googleapis.com/envoy.extensions.filters.http.local_ratelimit.v3.LocalRateLimit
            value:
              stat_prefix: http_local_rate_limiter
              token_bucket:
                max_tokens: 1000
                tokens_per_fill: 1000
                fill_interval: 1s
OBSERVABILITY
YAML

# Telemetry API — настройка метрик и трейсинга
apiVersion: telemetry.istio.io/v1alpha1
kind: Telemetry
metadata:
  name: mesh-telemetry
  namespace: istio-system
spec:
  tracing:
    - providers:
        - name: jaeger
      randomSamplingPercentage: 10  # 10% обычных запросов
      customTags:
        environment:
          literal:
            value: production
  metrics:
    - providers:
        - name: prometheus
      overrides:
        - match:
            metric: REQUEST_COUNT
            mode: CLIENT_AND_SERVER
          tagOverrides:
            response_code:
              operation: UPSERT
АНТИПАТТЕРНЫ
YAML

# ❌ ПЛОХО: PERMISSIVE mTLS (можно обойти)
spec:
  mtls:
    mode: PERMISSIVE

# ✅ ПРАВИЛЬНО: STRICT
spec:
  mtls:
    mode: STRICT

# ❌ ПЛОХО: allow-all AuthorizationPolicy
spec:
  rules:
    - {}  # разрешает всё

# ✅ ПРАВИЛЬНО: явные rules per service

# ❌ ПЛОХО: нет timeout на VirtualService
http:
  - route:
      - destination:
          host: external-api
# Запрос может висеть бесконечно

# ✅ ПРАВИЛЬНО:
http:
  - route:
      - destination:
          host: external-api
    timeout: 10s

# ❌ ПЛОХО: нет outlierDetection
# Один больной pod получает трафик и создаёт ошибки

# ✅ ПРАВИЛЬНО: circuit breaker через outlierDetection

# ❌ ПЛОХО: retry на POST запросы без idempotency
retries:
  attempts: 3
  retryOn: 5xx
# POST /api/v1/bets может создать дубликат

# ✅ ПРАВИЛЬНО: retry только для idempotent операций
# Или: retry только на connect-failure, reset (не 5xx)