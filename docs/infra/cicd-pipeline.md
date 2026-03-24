# CI/CD Pipeline Documentation — Opus Casino

## 📋 Overview

Production CI/CD pipeline для гемблинг-платформы с canary deployment, automated rollback, security gates, и performance testing.

## 🏗 Архитектура

```
┌─────────────────────────────────────────────────────────────────┐
│                    CI/CD Pipeline Flow                           │
│                                                                  │
│  Code Commit → CI Pipeline → Build → Security Scan             │
│       │                                                          │
│       ▼                                                          │
│  Push to Registry → CD Pipeline                                  │
│       │                                                          │
│       ▼                                                          │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Environment Promotion                                   │    │
│  │                                                           │    │
│  │  dev ──▶ staging ──▶ production                          │    │
│  │    │        │           │                                │    │
│  │    │        │           └─▶ Canary (5%→25%→50%→100%)    │    │
│  │    │        │                                              │    │
│  │    │        └─▶ Performance Tests (k6)                    │    │
│  │    │        └─▶ Integration Tests                         │    │
│  │    │                                                         │    │
│  │    └─▶ Auto-deploy on push                                  │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  Monitoring & Rollback                                          │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Argo Rollouts + Istio + Prometheus                      │    │
│  │  - Automated canary analysis                             │    │
│  │  - Auto-rollback on metrics threshold breach             │    │
│  │  - Real-time monitoring                                  │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

## 📁 Структура workflows

```
.github/
├── workflows/
│   ├── ci.yaml                    # Main CI orchestrator
│   ├── ci-rust.yml                # Rust сервисы (Betting Engine, Wallet, WebSocket)
│   ├── ci-go.yml                  # Go сервисы (Auth, User, Payment, etc.)
│   ├── ci-python.yml              # Python сервисы (Fraud ML, Analytics)
│   ├── ci-frontend.yml            # Next.js Web Platform
│   ├── ci-flutter-mobile.yml      # Flutter Mobile App
│   ├── ci-admin-panel.yml         # React Admin Panel
│   ├── cd-production.yml          # Production release с canary
│   ├── cd-promotion.yml           # Environment promotion (dev→staging→prod)
│   ├── security-scan.yml          # Security scanning (Trivy, Semgrep)
│   └── terraform-plan.yml         # Terraform plan/apply
│
├── actions/
│   ├── docker-build-push/         # Composite: Build, scan, push Docker
│   ├── security-scan/             # Composite: Security gates
│   └── performance-test/          # Composite: k6 load tests
│
└── CODEOWNERS
```

## 🔄 CI Pipeline

### Для Rust сервисов

```yaml
# .github/workflows/ci-rust.yml
Stages:
  1. Lint (clippy, rustfmt)
  2. Unit Tests (cargo test)
  3. Integration Tests (testcontainers)
  4. Security Scan (Trivy, cargo-audit)
  5. Build Docker (multi-stage, distroless)
  6. Push to Registry (ghcr.io)
```

**Время выполнения:** ~15 минут

### Для Go сервисов

```yaml
# .github/workflows/ci-go.yml
Stages:
  1. Lint (golangci-lint)
  2. Unit Tests (go test -race)
  3. Integration Tests (testcontainers)
  4. Security Scan (Trivy, govulncheck)
  5. Build Docker (multi-stage, distroless)
  6. Push to Registry (ghcr.io)
```

**Время выполнения:** ~12 минут

## 🚀 CD Pipeline

### Production Release с Canary

```yaml
# .github/workflows/cd-production.yml
Stages:
  1. Pre-deployment Checks
     - Deployment window validation
     - Pending migrations check
     - ArgoCD health verification
  
  2. Build & Push Image
     - Multi-platform build
     - SBOM generation
     - Cosign signing
  
  3. Security Gate
     - Trivy image scan
     - Semgrep SAST
     - Dependency audit
     - Image signature verification
  
  4. Performance Gate (Staging)
     - k6 load tests
     - Latency thresholds
     - Error rate checks
  
  5. Canary Deployment (Production)
     - Step 1: 5% traffic, 5min pause
     - Step 2: Analysis (10min)
     - Step 3: 25% traffic, 5min pause
     - Step 4: Analysis (5min)
     - Step 5: 50% traffic, 5min pause
     - Step 6: Analysis (5min)
     - Step 7: 100% traffic
  
  6. Full Production Rollout
     - ArgoCD sync
     - Smoke tests
     - Deployment record
  
  7. Post-deployment Verification
     - Comprehensive smoke tests
     - Error rate monitoring (5min)
     - Stakeholder notification
```

**Время выполнения:** ~60 минут (с canary анализом)

### Environment Promotion

```yaml
# .github/workflows/cd-promotion.yml
Flow: dev → staging → production

dev → staging:
  - Auto-promote on successful CI
  - Run integration tests
  - Run E2E tests (Playwright)
  - Tag as staging-latest

staging → production:
  - Manual approval required
  - Deployment window check (Mon-Fri, 9AM-5PM UTC)
  - Performance validation (k6)
  - Security scan
  - Tag as production-latest
  - Trigger canary deployment
```

## 🛡 Security Gates

### Trivy Scanning

```bash
# Image scan
trivy image \
  --severity HIGH,CRITICAL \
  --timeout 10m \
  --exit-code 1 \
  ghcr.io/opus-casino/service:tag

# Filesystem scan
trivy fs \
  --severity HIGH,CRITICAL \
  --timeout 10m \
  --exit-code 1 \
  .
```

### Semgrep SAST

```yaml
# Automatic rules
semgrep ci --config auto

# Custom rules
semgrep ci --config .semgrep/
```

### Dependency Audit

| Язык | Инструмент |
|------|------------|
| Rust | `cargo audit` |
| Go | `govulncheck` |
| Python | `pip-audit` |
| Node.js | `npm audit` |

## 📊 Performance Gates

### k6 Thresholds

| Environment | p95 Latency | p99 Latency | Error Rate |
|-------------|-------------|-------------|------------|
| **Staging** | < 300ms | < 500ms | < 0.5% |
| **Production** | < 200ms | < 400ms | < 0.1% |

### Load Test Scenarios

```javascript
// tools/testing/k6/scenarios/regression.js
{
  stages: [
    { duration: '5m', target: 50 },   // Warmup
    { duration: '10m', target: 100 }, // Load
    { duration: '5m', target: 200 },  // Peak
    { duration: '5m', target: 0 },    // Cooldown
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
  }
}
```

## 🎯 Canary Deployment Strategy

### Argo Rollouts Configuration

```yaml
strategy:
  canary:
    steps:
      - setWeight: 5       # 5% traffic
      - pause: 5m          # Manual verification
      - analysis:          # Automated analysis
          templates:
            - success-rate
            - latency-p99
          duration: 10m
      - setWeight: 25      # 25% traffic
      - pause: 5m
      - analysis
      - setWeight: 50      # 50% traffic
      - pause: 5m
      - analysis
      - setWeight: 100     # Full rollout
```

### Analysis Templates

**Success Rate:**
```promql
sum(rate(http_requests_total{service="X",status=~"2.."}[5m])) 
/ 
sum(rate(http_requests_total{service="X"}[5m]))
Threshold: >= 0.99 (99% success rate)
```

**P99 Latency:**
```promql
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{service="X"}[5m])) by (le))
Threshold: <= 0.5s (500ms)
```

**Error Rate:**
```promql
sum(rate(http_requests_total{service="X",status=~"5.."}[5m])) 
/ 
sum(rate(http_requests_total{service="X"}[5m]))
Threshold: <= 0.01 (1%)
```

## 🔄 Automated Rollback

### Rollback Triggers

1. **Canary Analysis Failure**
   - Success rate < 99%
   - P99 latency > 500ms
   - Error rate > 1%

2. **Kubernetes Health Checks**
   - Liveness probe failures (3 consecutive)
   - Readiness probe failures (3 consecutive)

3. **ArgoCD Sync Failure**
   - Resource validation error
   - Deployment timeout (600s)

### Rollback Process

```bash
# Automatic rollback command
kubectl argo rollouts abort <rollout-name> -n platform

# Scale down canary replicas
kubectl scale deployment <canary-deployment> --replicas=0 -n platform

# Restore stable version
kubectl argo rollouts set image <rollout-name> <container>=<stable-image>
```

**Rollback Time:** < 2 минут

## 📈 Monitoring & Observability

### Dashboards

- **Grafana:** `Production Deployments`
  - Deployment frequency
  - Lead time for changes
  - Mean time to recovery (MTTR)
  - Change failure rate

- **Argo Rollouts:** `Canary Analysis`
  - Traffic distribution
  - Metrics comparison (canary vs stable)
  - Analysis run history

### Alerts

| Alert | Severity | Condition | Action |
|-------|----------|-----------|--------|
| **Canary Analysis Failed** | P1 | Analysis template failure | Auto-rollback |
| **Deployment Timeout** | P1 | Rollout > 600s | Notify on-call |
| **Error Rate Spike** | P1 | Error rate > 5% post-deploy | Auto-rollback |
| **Performance Degradation** | P2 | p99 latency > 1s | Investigate |

## 🔐 Access Control

### Environment Protection Rules

| Environment | Branch | Approval | Deployment Window |
|-------------|--------|----------|-------------------|
| **dev** | develop | Auto | Anytime |
| **staging** | develop | Auto | Anytime |
| **production** | main | Required (1) | Mon-Fri, 9AM-5PM UTC |

### Required Permissions

```yaml
permissions:
  contents: read
  id-token: write      # OIDC for AWS
  packages: write      # Push to ghcr.io
  deployments: write   # Deployment status
```

## 📝 Deployment Checklist

### Pre-deployment

- [ ] All CI checks passed
- [ ] Security scan clean (no HIGH/CRITICAL)
- [ ] Performance tests passed
- [ ] Database migrations reviewed
- [ ] Rollback plan documented

### During deployment

- [ ] Canary analysis passing
- [ ] Error rates normal
- [ ] Latency within thresholds
- [ ] Business metrics stable

### Post-deployment

- [ ] Smoke tests passed
- [ ] Monitoring dashboards healthy
- [ ] No increase in error rates
- [ ] Stakeholders notified

## 🚨 Incident Response

### Deployment Failure

1. **Automatic rollback** triggered by Argo Rollouts
2. **PagerDuty alert** sent to on-call engineer
3. **Slack notification** to #deployments channel
4. **Incident created** in Jira

### Post-Mortem Template

```markdown
## Deployment Incident Report

**Date:** YYYY-MM-DD
**Service:** <service-name>
**Version:** <version>

### Timeline
- HH:MM - Deployment started
- HH:MM - Canary analysis failed
- HH:MM - Auto-rollback triggered
- HH:MM - Rollback complete

### Root Cause
<Description>

### Impact
- Users affected: X%
- Duration: X minutes
- Financial impact: $X

### Action Items
- [ ] Fix <issue>
- [ ] Add test for <scenario>
- [ ] Update runbook
```

## 🔧 Troubleshooting

### Common Issues

**1. Canary analysis failing**
```bash
# Check metrics
kubectl get analysisrun <name> -n platform -o yaml

# View Prometheus query
curl http://prometheus.monitoring:9090/api/v1/query?query=<query>
```

**2. Deployment stuck**
```bash
# Check rollout status
kubectl argo rollouts get rollout <name> -n platform

# Force abort
kubectl argo rollouts abort <name> -n platform --force
```

**3. Image pull errors**
```bash
# Verify image exists
docker pull ghcr.io/opus-casino/service:tag

# Check image pull secrets
kubectl get secret regcred -n platform
```

## 📚 Related Documentation

- [Argo Rollouts Docs](https://argoproj.github.io/argo-rollouts/)
- [k6 Documentation](https://k6.io/docs/)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [Opus Casino Architecture](./architecture-overview.md)

---

**Document Owner:** DevOps Lead  
**Review Cycle:** Monthly  
**Last Updated:** 2026-03-24
