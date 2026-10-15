## #41 helm-charts.skill.md

```markdown
# helm-charts.skill.md

## РОЛЬ
Ты — Platform Engineer, создающий Helm charts для деплоя
микросервисов гемблинг-платформы в Kubernetes.

## КОНТЕКСТ
- Helm 3 (без Tiller)
- ArgoCD для GitOps деплоев
- Общий chart для всех Go/Rust сервисов (generic)
- Специализированные charts для data-сервисов
- Values per environment: dev, staging, production

## СТРУКТУРА
helm-charts/
├── charts/
│ ├── platform-service/ # Generic chart для микросервисов
│ │ ├── Chart.yaml
│ │ ├── values.yaml # defaults
│ │ ├── templates/
│ │ │ ├── _helpers.tpl
│ │ │ ├── deployment.yaml
│ │ │ ├── service.yaml
│ │ │ ├── configmap.yaml
│ │ │ ├── hpa.yaml
│ │ │ ├── pdb.yaml
│ │ │ ├── serviceaccount.yaml
│ │ │ ├── networkpolicy.yaml
│ │ │ ├── istio-virtualservice.yaml
│ │ │ └── NOTES.txt
│ │ └── values/
│ │ ├── betting-engine.yaml
│ │ ├── auth-service.yaml
│ │ ├── wallet-service.yaml
│ │ ├── payment-service.yaml
│ │ └── ...
│ │
│ ├── platform-data/ # Charts для data-сервисов
│ │ ├── postgresql/
│ │ ├── dragonflydb/
│ │ ├── clickhouse/
│ │ └── redpanda/
│ │
│ └── platform-monitoring/ # Charts для мониторинга
│ ├── victoria-metrics/
│ ├── grafana/
│ ├── jaeger/
│ └── vector/
│
├── environments/
│ ├── dev/
│ │ ├── values-global.yaml
│ │ └── values-betting-engine.yaml
│ ├── staging/
│ │ └── ...
│ └── production/
│ ├── eu-west-1/
│ │ ├── values-global.yaml
│ │ └── values-betting-engine.yaml
│ └── ap-southeast-1/
│ └── ...
│
└── argocd/
├── applications/
│ ├── betting-engine.yaml
│ ├── auth-service.yaml
│ └── ...
└── appsets/
└── platform-services.yaml

text


## GENERIC SERVICE CHART

### Chart.yaml
```yaml
apiVersion: v2
name: platform-service
description: Generic chart for gambling platform microservices
type: application
version: 1.0.0
appVersion: "1.0.0"
values.yaml (defaults)
YAML

# Default values — override per service
replicaCount: 2

image:
  repository: ""
  tag: ""
  pullPolicy: IfNotPresent

nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  annotations: {}

service:
  type: ClusterIP
  ports:
    - name: http
      port: 8080
      targetPort: http
    - name: grpc
      port: 9000
      targetPort: grpc
    - name: metrics
      port: 9090
      targetPort: metrics

container:
  ports:
    - name: http
      containerPort: 8080
    - name: grpc
      containerPort: 9000
    - name: metrics
      containerPort: 9090
  
  env: []
  envFrom: []
  
  resources:
    requests:
      cpu: "500m"
      memory: "512Mi"
    limits:
      cpu: "1"
      memory: "1Gi"
  
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

  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    readOnlyRootFilesystem: true
    allowPrivilegeEscalation: false
    capabilities:
      drop: ["ALL"]

configMap:
  enabled: true
  data: {}

vault:
  enabled: true
  role: ""
  secrets: []

autoscaling:
  enabled: false
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300

pdb:
  enabled: true
  minAvailable: 1

networkPolicy:
  enabled: true
  ingress: []
  egress: []

istio:
  enabled: true
  virtualService:
    enabled: false
    hosts: []
    gateways: []

nodeSelector:
  node-pool: application

tolerations: []

affinity:
  podAntiAffinity: true
  topologySpread: true

monitoring:
  serviceMonitor:
    enabled: true
    interval: 30s
    path: /metrics
templates/deployment.yaml
YAML

apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "platform-service.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "platform-service.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  revisionHistoryLimit: 5
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 2
  selector:
    matchLabels:
      {{- include "platform-service.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "platform-service.selectorLabels" . | nindent 8 }}
      annotations:
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
        {{- if .Values.vault.enabled }}
        vault.hashicorp.com/agent-inject: "true"
        vault.hashicorp.com/role: {{ .Values.vault.role | quote }}
        {{- range .Values.vault.secrets }}
        vault.hashicorp.com/agent-inject-secret-{{ .name }}: {{ .path | quote }}
        {{- end }}
        {{- end }}
        {{- if .Values.monitoring.serviceMonitor.enabled }}
        prometheus.io/scrape: "true"
        prometheus.io/port: {{ .Values.monitoring.serviceMonitor.port | default "9090" | quote }}
        prometheus.io/path: {{ .Values.monitoring.serviceMonitor.path | quote }}
        {{- end }}
    spec:
      serviceAccountName: {{ include "platform-service.serviceAccountName" . }}
      terminationGracePeriodSeconds: 60
      containers:
        - name: {{ .Chart.Name }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          
          ports:
            {{- toYaml .Values.container.ports | nindent 12 }}
          
          {{- with .Values.container.env }}
          env:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          
          {{- with .Values.container.envFrom }}
          envFrom:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          
          {{- if .Values.configMap.enabled }}
          envFrom:
            - configMapRef:
                name: {{ include "platform-service.fullname" . }}-config
          {{- end }}
          
          resources:
            {{- toYaml .Values.container.resources | nindent 12 }}
          
          livenessProbe:
            {{- toYaml .Values.container.livenessProbe | nindent 12 }}
          readinessProbe:
            {{- toYaml .Values.container.readinessProbe | nindent 12 }}
          startupProbe:
            {{- toYaml .Values.container.startupProbe | nindent 12 }}
          
          securityContext:
            {{- toYaml .Values.container.securityContext | nindent 12 }}
          
          volumeMounts:
            - name: tmp
              mountPath: /tmp
      
      volumes:
        - name: tmp
          emptyDir:
            medium: Memory
            sizeLimit: 100Mi
      
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      
      {{- if .Values.affinity.podAntiAffinity }}
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  {{- include "platform-service.selectorLabels" . | nindent 18 }}
              topologyKey: kubernetes.io/hostname
      {{- end }}
      
      {{- if .Values.affinity.topologySpread }}
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              {{- include "platform-service.selectorLabels" . | nindent 14 }}
      {{- end }}
templates/_helpers.tpl
YAML

{{- define "platform-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "platform-service.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "platform-service.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "platform-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Values.image.tag | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "platform-service.selectorLabels" -}}
app.kubernetes.io/name: {{ include "platform-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "platform-service.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "platform-service.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
PER-SERVICE VALUES
YAML

# values/betting-engine.yaml
nameOverride: betting-engine
replicaCount: 6

image:
  repository: registry.example.com/betting-engine
  tag: v1.2.3

container:
  resources:
    requests:
      cpu: "4"
      memory: "2Gi"
    limits:
      cpu: "4"
      memory: "2Gi"

configMap:
  data:
    DATABASE_HOST: postgresql-primary.data.svc.cluster.local
    DATABASE_PORT: "5432"
    DATABASE_NAME: platform
    DATABASE_POOL_SIZE: "50"
    CACHE_HOST: dragonflydb.data.svc.cluster.local
    CACHE_PORT: "6379"
    REDPANDA_BROKERS: "redpanda-0.data:9092,redpanda-1.data:9092,redpanda-2.data:9092"

vault:
  role: betting-engine
  secrets:
    - name: db-creds
      path: database/creds/betting-engine

autoscaling:
  enabled: true
  minReplicas: 6
  maxReplicas: 30
  targetCPUUtilizationPercentage: 70

pdb:
  minAvailable: 4

# values/auth-service.yaml
nameOverride: auth-service
replicaCount: 3

image:
  repository: registry.example.com/auth-service
  tag: v1.0.5

container:
  resources:
    requests:
      cpu: "1"
      memory: "512Mi"
    limits:
      cpu: "2"
      memory: "1Gi"

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 15
ARGOCD APPLICATION
YAML

# argocd/applications/betting-engine.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: betting-engine
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: platform
  source:
    repoURL: https://github.com/org/helm-charts.git
    targetRevision: main
    path: charts/platform-service
    helm:
      valueFiles:
        - values/betting-engine.yaml
        - ../../environments/production/eu-west-1/values-betting-engine.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: platform
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
      - CreateNamespace=true
      - PruneLast=true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
APPLICATIONSET (все сервисы одним шаблоном)
YAML

# argocd/appsets/platform-services.yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: platform-services
  namespace: argocd
spec:
  generators:
    - list:
        elements:
          - name: betting-engine
            namespace: platform
            valuesFile: values/betting-engine.yaml
          - name: auth-service
            namespace: platform
            valuesFile: values/auth-service.yaml
          - name: wallet-service
            namespace: platform
            valuesFile: values/wallet-service.yaml
          - name: payment-service
            namespace: platform
            valuesFile: values/payment-service.yaml
          - name: user-service
            namespace: platform
            valuesFile: values/user-service.yaml
          - name: casino-service
            namespace: platform
            valuesFile: values/casino-service.yaml
          - name: bonus-service
            namespace: platform
            valuesFile: values/bonus-service.yaml
          - name: notification-service
            namespace: platform
            valuesFile: values/notification-service.yaml
          - name: kyc-service
            namespace: platform
            valuesFile: values/kyc-service.yaml
          - name: fraud-engine
            namespace: platform
            valuesFile: values/fraud-engine.yaml
  template:
    metadata:
      name: "{{name}}"
    spec:
      project: platform
      source:
        repoURL: https://github.com/org/helm-charts.git
        targetRevision: main
        path: charts/platform-service
        helm:
          valueFiles:
            - "{{valuesFile}}"
      destination:
        server: https://kubernetes.default.svc
        namespace: "{{namespace}}"
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
АНТИПАТТЕРНЫ
YAML

# ❌ ПЛОХО: хардкод в templates
containers:
  - image: registry.example.com/app:v1.2.3  # хардкод в template

# ✅ ПРАВИЛЬНО: через values
containers:
  - image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"

# ❌ ПЛОХО: один values.yaml для всех environments
# dev и production с одинаковыми ресурсами

# ✅ ПРАВИЛЬНО: values per environment через ArgoCD valueFiles

# ❌ ПЛОХО: helm install вручную
helm install betting-engine ./charts/platform-service

# ✅ ПРАВИЛЬНО: через ArgoCD (GitOps)
# Push to Git → ArgoCD auto-sync

# ❌ ПЛОХО: отсутствие checksum/config annotation
# Изменение ConfigMap не перезапускает pods

# ✅ ПРАВИЛЬНО:
annotations:
  checksum/config: {{ include ... | sha256sum }}

# ❌ ПЛОХО: Chart без NOTES.txt
# Пользователь не знает как проверить деплой

# ✅ ПРАВИЛЬНО: NOTES.txt с инструкциями post-install
HELM COMMANDS REFERENCE
Bash

# Валидация chart
helm lint charts/platform-service -f values/betting-engine.yaml

# Рендер templates (debug)
helm template betting-engine charts/platform-service \
  -f values/betting-engine.yaml \
  --namespace platform \
  --debug

# Dry-run install
helm install betting-engine charts/platform-service \
  -f values/betting-engine.yaml \
  --namespace platform \
  --dry-run

# В production: ТОЛЬКО через ArgoCD, не helm install