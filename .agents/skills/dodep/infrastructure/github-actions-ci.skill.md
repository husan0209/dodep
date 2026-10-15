#42 github-actions-ci.skill.md
Markdown

# github-actions-ci.skill.md

## РОЛЬ
Ты — DevOps Engineer, создающий CI/CD пайплайны на GitHub Actions
для всех сервисов гемблинг-платформы.

## КОНТЕКСТ
- Monorepo или multi-repo (workflows адаптируются)
- Языки: Rust, Go, Python, TypeScript, Dart
- CD через ArgoCD (GitOps) — CI только до push image
- Canary deploys через Argo Rollouts
- Security scanning обязателен в каждом pipeline

## СТРУКТУРА WORKFLOWS
.github/
├── workflows/
│ ├── rust-service.yml # Template для Rust сервисов
│ ├── go-service.yml # Template для Go сервисов
│ ├── python-service.yml # Template для Python сервисов
│ ├── frontend-web.yml # Next.js
│ ├── frontend-mobile.yml # Flutter
│ ├── helm-lint.yml # Helm chart validation
│ ├── terraform-plan.yml # Terraform plan
│ ├── protobuf-lint.yml # Proto validation
│ └── dependency-review.yml # Weekly dependency audit
│
├── actions/
│ ├── setup-rust/
│ │ └── action.yml # Reusable: setup Rust toolchain
│ ├── setup-go/
│ │ └── action.yml
│ ├── docker-build-push/
│ │ └── action.yml # Reusable: build + scan + push
│ └── notify-slack/
│ └── action.yml
│
└── CODEOWNERS

text


## RUST SERVICE PIPELINE

```yaml
# .github/workflows/rust-service.yml
name: Rust Service CI

on:
  push:
    branches: [main]
    paths:
      - 'services/betting-engine/**'
      - 'crates/**'
      - '.github/workflows/rust-service.yml'
  pull_request:
    branches: [main]
    paths:
      - 'services/betting-engine/**'
      - 'crates/**'

env:
  CARGO_TERM_COLOR: always
  RUSTFLAGS: "-D warnings"
  SERVICE_NAME: betting-engine
  REGISTRY: ${{ secrets.REGISTRY_URL }}

jobs:
  lint:
    name: Lint & Format
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: dtolnay/rust-toolchain@stable
        with:
          components: rustfmt, clippy
      
      - uses: Swatinem/rust-cache@v2
        with:
          workspaces: "services/betting-engine"
      
      - name: Format check
        run: cargo fmt --all --check
        working-directory: services/betting-engine
      
      - name: Clippy
        run: cargo clippy --all-targets --all-features -- -D warnings
        working-directory: services/betting-engine

  test:
    name: Tests
    runs-on: ubuntu-latest
    needs: lint
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_DB: test_db
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
        ports: ['5432:5432']
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      dragonfly:
        image: docker.dragonflydb.io/dragonflydb/dragonfly
        ports: ['6379:6379']
    
    steps:
      - uses: actions/checkout@v4
      
      - uses: dtolnay/rust-toolchain@stable
      
      - uses: Swatinem/rust-cache@v2
      
      - name: Run tests
        run: cargo test --all-features --workspace
        working-directory: services/betting-engine
        env:
          DATABASE_URL: postgres://test:test@localhost:5432/test_db
          CACHE_URL: redis://localhost:6379
      
      - name: Generate coverage
        run: |
          cargo install cargo-tarpaulin
          cargo tarpaulin --out xml --skip-clean
        working-directory: services/betting-engine
      
      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: services/betting-engine/cobertura.xml
          flags: betting-engine

  security:
    name: Security Scan
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
      
      - name: Cargo audit
        run: |
          cargo install cargo-audit
          cargo audit
        working-directory: services/betting-engine
      
      - name: SAST (Semgrep)
        uses: semgrep/semgrep-action@v1
        with:
          config: >-
            p/rust
            p/security-audit
          publishToken: ${{ secrets.SEMGREP_TOKEN }}

  build:
    name: Build & Push Image
    runs-on: ubuntu-latest
    needs: [test, security]
    if: github.ref == 'refs/heads/main'
    permissions:
      contents: read
      id-token: write
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Set version
        id: version
        run: |
          VERSION=$(git describe --tags --always --dirty)
          SHA=$(git rev-parse --short HEAD)
          echo "version=${VERSION}" >> $GITHUB_OUTPUT
          echo "sha=${SHA}" >> $GITHUB_OUTPUT
          echo "image=${REGISTRY}/${SERVICE_NAME}:${VERSION}" >> $GITHUB_OUTPUT
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Login to Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ secrets.REGISTRY_USERNAME }}
          password: ${{ secrets.REGISTRY_PASSWORD }}
      
      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: services/betting-engine/Dockerfile
          push: true
          tags: |
            ${{ steps.version.outputs.image }}
            ${{ env.REGISTRY }}/${{ env.SERVICE_NAME }}:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max
          build-args: |
            VERSION=${{ steps.version.outputs.version }}
      
      - name: Scan image (Trivy)
        uses: aquasecurity/trivy-action@v0.18
        with:
          image-ref: ${{ steps.version.outputs.image }}
          severity: 'CRITICAL,HIGH'
          exit-code: '1'
          ignore-unfixed: true
      
      - name: Update Helm values
        run: |
          cd helm-charts/values
          yq e ".image.tag = \"${{ steps.version.outputs.version }}\"" \
            -i betting-engine.yaml
          
          git config user.name "github-actions"
          git config user.email "actions@github.com"
          git add .
          git commit -m "chore: update betting-engine to ${{ steps.version.outputs.version }}"
          git push
        # ArgoCD подхватит изменение автоматически
GO SERVICE PIPELINE
YAML

# .github/workflows/go-service.yml
name: Go Service CI

on:
  push:
    branches: [main]
    paths:
      - 'services/auth-service/**'
  pull_request:
    paths:
      - 'services/auth-service/**'

env:
  GO_VERSION: '1.22'
  SERVICE_NAME: auth-service

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
          working-directory: services/auth-service
          args: --timeout=5m

  test:
    runs-on: ubuntu-latest
    needs: lint
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_DB: test_db
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
        ports: ['5432:5432']
        options: --health-cmd pg_isready --health-interval 10s --health-timeout 5s --health-retries 5
      dragonfly:
        image: docker.dragonflydb.io/dragonflydb/dragonfly
        ports: ['6379:6379']
    
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
        working-directory: services/auth-service
        env:
          DATABASE_URL: postgres://test:test@localhost:5432/test_db
          CACHE_URL: redis://localhost:6379
      
      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: services/auth-service/coverage.out

  security:
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...
        working-directory: services/auth-service
      
      - name: Semgrep
        uses: semgrep/semgrep-action@v1
        with:
          config: p/golang

  build:
    runs-on: ubuntu-latest
    needs: [test, security]
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      
      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: services/auth-service
          push: true
          tags: ${{ env.REGISTRY }}/${{ env.SERVICE_NAME }}:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
      
      - name: Trivy scan
        uses: aquasecurity/trivy-action@v0.18
        with:
          image-ref: ${{ env.REGISTRY }}/${{ env.SERVICE_NAME }}:${{ github.sha }}
          severity: 'CRITICAL,HIGH'
          exit-code: '1'
FRONTEND PIPELINE
YAML

# .github/workflows/frontend-web.yml
name: Frontend Web CI

on:
  push:
    branches: [main]
    paths: ['frontend/web/**']
  pull_request:
    paths: ['frontend/web/**']

jobs:
  lint-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v3
        with:
          version: 9
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'pnpm'
          cache-dependency-path: frontend/web/pnpm-lock.yaml
      
      - run: pnpm install --frozen-lockfile
        working-directory: frontend/web
      
      - name: Lint
        run: pnpm lint
        working-directory: frontend/web
      
      - name: Type check
        run: pnpm type-check
        working-directory: frontend/web
      
      - name: Unit tests
        run: pnpm test --coverage
        working-directory: frontend/web
      
      - name: Build
        run: pnpm build
        working-directory: frontend/web
        env:
          NEXT_PUBLIC_API_URL: https://api.example.com
      
      - name: Bundle analysis
        run: |
          npx @next/bundle-analyzer
          # Fail if bundle > threshold
        working-directory: frontend/web

  e2e:
    runs-on: ubuntu-latest
    needs: lint-test
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v3
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'pnpm'
      
      - run: pnpm install --frozen-lockfile
        working-directory: frontend/web
      
      - name: Playwright tests
        run: pnpm test:e2e
        working-directory: frontend/web
      
      - name: Upload test results
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-results
          path: frontend/web/test-results/
REUSABLE ACTION — Docker Build
YAML

# .github/actions/docker-build-push/action.yml
name: Docker Build and Push
description: Build, scan, and push Docker image

inputs:
  service-name:
    required: true
  context:
    required: true
  registry:
    required: true
  username:
    required: true
  password:
    required: true

outputs:
  image:
    description: Full image reference
    value: ${{ steps.meta.outputs.image }}

runs:
  using: composite
  steps:
    - name: Set metadata
      id: meta
      shell: bash
      run: |
        VERSION=$(git describe --tags --always)
        echo "version=${VERSION}" >> $GITHUB_OUTPUT
        echo "image=${{ inputs.registry }}/${{ inputs.service-name }}:${VERSION}" >> $GITHUB_OUTPUT
    
    - uses: docker/setup-buildx-action@v3
    
    - uses: docker/login-action@v3
      with:
        registry: ${{ inputs.registry }}
        username: ${{ inputs.username }}
        password: ${{ inputs.password }}
    
    - uses: docker/build-push-action@v5
      with:
        context: ${{ inputs.context }}
        push: true
        tags: ${{ steps.meta.outputs.image }}
        cache-from: type=gha
        cache-to: type=gha,mode=max
    
    - uses: aquasecurity/trivy-action@v0.18
      with:
        image-ref: ${{ steps.meta.outputs.image }}
        severity: 'CRITICAL,HIGH'
        exit-code: '1'
PROTOBUF LINT
YAML

# .github/workflows/protobuf-lint.yml
name: Protobuf Lint

on:
  pull_request:
    paths: ['proto/**']

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: bufbuild/buf-setup-action@v1
      
      - name: Lint
        run: buf lint
        working-directory: proto
      
      - name: Breaking change detection
        run: buf breaking --against 'https://github.com/org/repo.git#branch=main'
        working-directory: proto
      
      - name: Generate code
        run: buf generate
        working-directory: proto
ПРАВИЛА
text

1. Каждый PR → lint + test + security scan (блокирующий)
2. Merge to main → build image + push + update Helm values
3. Production deploy → ТОЛЬКО через ArgoCD (не через CI)
4. Secrets → GitHub Secrets или OIDC (НЕ хардкод)
5. Cache → actions/cache или language-specific (Swatinem/rust-cache)
6. Timeout: max 30 мин на весь pipeline
7. Concurrency: отменять предыдущий run при новом push
8. Artifacts: test reports, coverage, scan results
9. Notifications: Slack при failure на main branch
10. CODEOWNERS: обязательный review для workflows
АНТИПАТТЕРНЫ
YAML

# ❌ ПЛОХО: без кэширования
- run: cargo build  # каждый раз с нуля, 15 минут

# ✅ ПРАВИЛЬНО: с кэшем
- uses: Swatinem/rust-cache@v2  # 2 минуты

# ❌ ПЛОХО: секреты через env напрямую
env:
  AWS_ACCESS_KEY: AKIAXXXXXXXX  # в коде!

# ✅ ПРАВИЛЬНО:
env:
  AWS_ACCESS_KEY: ${{ secrets.AWS_ACCESS_KEY }}

# ❌ ПЛОХО: deploy из CI напрямую
- run: kubectl apply -f deployment.yaml

# ✅ ПРАВИЛЬНО: через ArgoCD (GitOps)
- run: yq e ".image.tag = \"$TAG\"" -i helm-values.yaml && git push

# ❌ ПЛОХО: игнорировать security scans
continue-on-error: true  # на security job

# ✅ ПРАВИЛЬНО: блокировать merge при уязвимостях
exit-code: '1'  # fail pipeline

# ❌ ПЛОХО: нет concurrency control
# 5 pushes → 5 параллельных builds

# ✅ ПРАВИЛЬНО:
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true