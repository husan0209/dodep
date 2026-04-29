# dev-casino.ps1 — Запуск Casino Service в режиме разработки

$env:APP_ENV             = "development"
$env:PORT                = "8086"
$env:GRPC_PORT           = "50057"
$env:DATABASE_URL        = "postgres://postgres:changeme@localhost:5433/opus_casino?sslmode=disable"
$env:REDIS_HOST          = "localhost"
$env:REDIS_PORT          = "6379"
$env:REDIS_PASSWORD      = "changeme"
$env:WALLET_GRPC_ADDR    = "localhost:50053"
$env:USER_GRPC_ADDR      = "localhost:50052"
$env:KAFKA_BROKERS       = "localhost:9092"

# Providers (все выключены по умолчанию — включай по одному)
$env:PRAGMATIC_ENABLED   = "false"
$env:PGSOFT_ENABLED      = "false"
$env:AMATIC_ENABLED      = "false"
$env:AMUSNET_ENABLED     = "false"

# Чтобы включить Pragmatic Play в dev:
# $env:PRAGMATIC_ENABLED    = "true"
# $env:PRAGMATIC_AGENT_ID   = "your_agent_id"
# $env:PRAGMATIC_SECRET_KEY = "your_secret_key"

Write-Host "Starting Casino Service (port 8086)..." -ForegroundColor Cyan
Set-Location "$PSScriptRoot\..\services\go\casino"
go run .
