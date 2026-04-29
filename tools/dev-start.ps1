# dev-start.ps1 — Полный dev-запуск: инфра + сервисы (Windows PowerShell)
# Использование: pwsh -File tools\dev-start.ps1 [infra|services|all]
param(
    [string]$Mode = "all"  # infra | services | all
)

$Root = Split-Path -Parent $PSScriptRoot

function Write-Step([string]$msg) {
    Write-Host "`n═══ $msg ═══" -ForegroundColor Yellow
}

# ── 1. Docker инфраструктура ──────────────────────────────────────────────
if ($Mode -in @("infra", "all")) {
    Write-Step "Starting infrastructure (Docker Compose)"
    Set-Location $Root
    docker compose -f infra/docker/docker-compose.dev.yml up -d

    Write-Host "Waiting for PostgreSQL to be ready..." -ForegroundColor Gray
    $maxWait = 30; $waited = 0
    do {
        Start-Sleep 2; $waited += 2
        $ready = docker exec opus-postgres pg_isready -U postgres 2>$null
    } while ($ready -notmatch "accepting" -and $waited -lt $maxWait)

    if ($waited -ge $maxWait) {
        Write-Host "WARNING: PostgreSQL did not become ready in time" -ForegroundColor Red
    } else {
        Write-Host "PostgreSQL ready." -ForegroundColor Green
    }
}

# ── 2. Go-сервисы в отдельных окнах ──────────────────────────────────────
if ($Mode -in @("services", "all")) {
    Write-Step "Starting Go services"

    # Общие dev переменные окружения
    $commonEnv = @{
        DATABASE_URL     = "postgres://postgres:changeme@localhost:5433/opus_casino?sslmode=disable"
        REDIS_HOST       = "localhost"
        REDIS_PORT       = "6379"
        REDIS_PASSWORD   = "changeme"
        KAFKA_BROKERS    = "localhost:9092"
        APP_ENV          = "development"
        OTEL_ENABLED     = "false"
    }

    function Start-Service([string]$name, [string]$dir, [hashtable]$extraEnv = @{}) {
        $envBlock = ($commonEnv + $extraEnv).GetEnumerator() |
            ForEach-Object { "`$env:$($_.Key) = '$($_.Value)'" } |
            Join-String -Separator "; "

        $cmd = "$envBlock; Set-Location '$dir'; go run ."
        Start-Process pwsh -ArgumentList "-NoExit", "-Command", $cmd `
            -WindowStyle Normal
        Write-Host "  Started: $name" -ForegroundColor Green
    }

    # Auth (port 8083)
    Start-Service "auth" "$Root\services\go\auth" @{
        PORT                     = "8083"
        ENCRYPTION_KEY           = "dev-encryption-key-32bytes!!!!!"
        JWT_ED25519_PRIVATE_KEY  = ""
        JWT_ED25519_PUBLIC_KEY   = ""
    }
    Start-Sleep 1

    # Payment (port 8084)
    Start-Service "payment" "$Root\services\go\payment" @{}
    # Payment requires NOWPayments keys — use dev-payment.ps1 separately:
    Write-Host "  NOTE: Run .\tools\dev-payment.ps1 for payment service with NOWPayments keys" -ForegroundColor Cyan
    Start-Sleep 1

    # Casino (port 8086)
    Start-Service "casino" "$Root\services\go\casino" @{
        PORT             = "8086"
        GRPC_PORT        = "50057"
        WALLET_GRPC_ADDR = "localhost:50053"
        USER_GRPC_ADDR   = "localhost:50052"
        PRAGMATIC_ENABLED = "false"
        PGSOFT_ENABLED    = "false"
        AMATIC_ENABLED    = "false"
        AMUSNET_ENABLED   = "false"
    }
    Start-Sleep 1
}

Write-Step "Dev environment ready"
Write-Host ""
Write-Host "Services:" -ForegroundColor White
Write-Host "  Auth      → http://localhost:8083/health"
Write-Host "  Payment   → http://localhost:8084/healthz  (run dev-payment.ps1)"
Write-Host "  Casino    → http://localhost:8086/health"
Write-Host ""
Write-Host "Infrastructure:"
Write-Host "  PostgreSQL → localhost:5433"
Write-Host "  Redis      → localhost:6379"
Write-Host "  Redpanda   → localhost:9092"
Write-Host "  ClickHouse → localhost:8123"
Write-Host "  Jaeger     → http://localhost:16686"
Write-Host ""
Write-Host "Frontend:"
Write-Host "  Web   → cd apps/web && npm run dev   (localhost:3000)"
Write-Host "  Admin → cd apps/admin && npm run dev (localhost:3001)"
Write-Host ""
