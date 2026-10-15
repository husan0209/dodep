## #39 kubernetes-manifests.skill.md

```markdown
# kubernetes-manifests.skill.md

## РОЛЬ
Ты — DevOps/Platform Engineer, создающий Kubernetes манифесты
для гемблинг-платформы. Все сервисы работают в K8s.

## КОНТЕКСТ
- Kubernetes: EKS 1.29+ / GKE
- Service Mesh: Istio (mTLS, traffic management)
- GitOps: ArgoCD
- Namespaces: platform, data, monitoring, security, istio-system
- Node pools: system, application, data, spot

## NAMESPACE STRUCTURE

```yaml
# Namespace с resource quotas и limits
apiVersion: v1
kind: Namespace
metadata:
  name: platform
  labels:
    istio-injection: enabled
    team: backend
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: platform-quota
  namespace: platform
spec:
  hard:
    requests.cpu: "100"
    requests.memory: "200Gi"
    limits.cpu: "150"
    limits.memory: "300Gi"
    pods: "500"
---
apiVersion: v1
kind: LimitRange
metadata:
  name: platform-limits
  namespace: platform
spec:
  limits:
    - default:
        cpu: "500m"
        memory: "512Mi"
      defaultRequest:
        cpu: "100m"
        memory: "128Mi"
      type: Container
DEPLOYMENT PATTERN
YAML

# Betting Engine — критический сервис (Rust)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: betting-engine
  namespace: platform
  labels:
    app: betting-engine
    tier: critical
    language: rust
spec:
  replicas: 6
  revisionHistoryLimit: 5
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 2
  selector:
    matchLabels:
      app: betting-engine
  template:
    metadata:
      labels:
        app: betting-engine
        tier: critical
        version: v1.2.3
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
        vault.hashicorp.com/agent-inject: "true"
        vault.hashicorp.com/role: "betting-engine"
        vault.hashicorp.com/agent-inject-secret-db: "database/creds/betting-engine"
    spec:
      serviceAccountName: betting-engine
      terminationGracePeriodSeconds: 60
      
      # Гарантированные ресурсы — QoS: Guaranteed
      containers:
        - name: betting-engine
          image: registry.example.com/betting-engine:v1.2.3
          imagePullPolicy: IfNotPresent
          
          ports:
            - name: http
              containerPort: 8080
            - name: grpc
              containerPort: 9000
            - name: metrics
              containerPort: 9090
          
          env:
            - name: RUST_LOG
              value: "info,betting_engine=debug"
            - name: ENVIRONMENT
              value: "production"
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          
          envFrom:
            - configMapRef:
                name: betting-engine-config
          
          resources:
            requests:
              cpu: "4"
              memory: "2Gi"
            limits:
              cpu: "4"
              memory: "2Gi"
          
          livenessProbe:
            httpGet:
              path: /health/live
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3
          
          readinessProbe:
            httpGet:
              path: /health/ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
            timeoutSeconds: 2
            failureThreshold: 2
          
          startupProbe:
            httpGet:
              path: /health/startup
              port: http
            failureThreshold: 30
            periodSeconds: 2
          
          lifecycle:
            preStop:
              exec:
                command: ["/bin/sh", "-c", "sleep 10"]
                # Дать время Istio/LB убрать pod из rotation
          
          securityContext:
            runAsNonRoot: true
            runAsUser: 1000
            runAsGroup: 1000
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
          
          volumeMounts:
            - name: tmp
              mountPath: /tmp
      
      volumes:
        - name: tmp
          emptyDir:
            medium: Memory
            sizeLimit: 100Mi
      
      # Anti-affinity: не на одной ноде
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: betting-engine
              topologyKey: kubernetes.io/hostname
        
        # Предпочтительно на нодах app pool
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-pool
                    operator: In
                    values: ["application"]
      
      # Распределение по зонам
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              app: betting-engine
SERVICE + CONFIGMAP
YAML

apiVersion: v1
kind: Service
metadata:
  name: betting-engine
  namespace: platform
  labels:
    app: betting-engine
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 8080
      targetPort: http
      protocol: TCP
    - name: grpc
      port: 9000
      targetPort: grpc
      protocol: TCP
    - name: metrics
      port: 9090
      targetPort: metrics
      protocol: TCP
  selector:
    app: betting-engine
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: betting-engine-config
  namespace: platform
data:
  DATABASE_HOST: "postgresql-primary.data.svc.cluster.local"
  DATABASE_PORT: "5432"
  DATABASE_NAME: "platform"
  DATABASE_POOL_SIZE: "50"
  CACHE_HOST: "dragonflydb.data.svc.cluster.local"
  CACHE_PORT: "6379"
  REDPANDA_BROKERS: "redpanda-0.data.svc.cluster.local:9092,redpanda-1.data.svc.cluster.local:9092"
  GRPC_PORT: "9000"
  HTTP_PORT: "8080"
  METRICS_PORT: "9090"
  OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector.monitoring.svc.cluster.local:4317"
HPA (Horizontal Pod Autoscaler)
YAML

apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: betting-engine
  namespace: platform
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: betting-engine
  minReplicas: 6
  maxReplicas: 30
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
        - type: Percent
          value: 50
          periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Percent
          value: 10
          periodSeconds: 120
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
    - type: Pods
      pods:
        metric:
          name: http_requests_per_second
        target:
          type: AverageValue
          averageValue: "5000"
PDB (Pod Disruption Budget)
YAML

apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: betting-engine
  namespace: platform
spec:
  minAvailable: 4    # всегда минимум 4 пода
  selector:
    matchLabels:
      app: betting-engine
NETWORK POLICY
YAML

apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: betting-engine
  namespace: platform
spec:
  podSelector:
    matchLabels:
      app: betting-engine
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Принимать только от API Gateway и других сервисов
    - from:
        - namespaceSelector:
            matchLabels:
              name: platform
        - namespaceSelector:
            matchLabels:
              name: istio-system
      ports:
        - port: 8080
        - port: 9000
        - port: 9090
  egress:
    # Только к БД, кэшу, Redpanda и DNS
    - to:
        - namespaceSelector:
            matchLabels:
              name: data
      ports:
        - port: 5432   # PostgreSQL
        - port: 6379   # DragonflyDB
        - port: 9092   # Redpanda
    - to:
        - namespaceSelector:
            matchLabels:
              name: monitoring
      ports:
        - port: 4317   # OTEL collector
    - to: []            # DNS
      ports:
        - port: 53
          protocol: UDP
        - port: 53
          protocol: TCP
АНТИПАТТЕРНЫ
YAML

# ❌ ПЛОХО: latest tag
image: betting-engine:latest

# ✅ ПРАВИЛЬНО: конкретный тег
image: registry.example.com/betting-engine:v1.2.3

# ❌ ПЛОХО: нет resource limits
containers:
  - name: app
    image: app:v1

# ✅ ПРАВИЛЬНО: всегда requests и limits

# ❌ ПЛОХО: секреты в ConfigMap
data:
  DATABASE_PASSWORD: "secret123"

# ✅ ПРАВИЛЬНО: через Vault Agent Injector или External Secrets

# ❌ ПЛОХО: запуск от root
securityContext:
  runAsUser: 0

# ✅ ПРАВИЛЬНО: non-root user
securityContext:
  runAsNonRoot: true
  runAsUser: 1000

# ❌ ПЛОХО: нет probes
# Pod может быть не готов, но получает трафик

# ✅ ПРАВИЛЬНО: все три probe (startup, liveness, readiness)

# ❌ ПЛОХО: нет PDB
# kubectl drain убьёт все поды

# ✅ ПРАВИЛЬНО: PDB с minAvailable

# ❌ ПЛОХО: все поды на одной ноде
# Нода упала → весь сервис недоступен

# ✅ ПРАВИЛЬНО: podAntiAffinity + topologySpreadConstraints