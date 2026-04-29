# dev-auth.ps1 — Запуск Auth Service в режиме разработки

$env:APP_ENV             = "development"
$env:PORT                = "8083"
$env:DATABASE_URL        = "postgres://postgres:changeme@localhost:5433/opus_casino?sslmode=disable"
$env:REDIS_URL           = "redis://:changeme@localhost:6379/0"
$env:JWT_ACCESS_EXPIRY   = "900"
$env:JWT_REFRESH_EXPIRY  = "604800"
$env:ENCRYPTION_KEY      = "dev-encryption-key-32bytes!!!!!"
$env:APP_BASE_URL        = "http://localhost:3000"

# Генерируй ключи один раз и вставляй сюда:
# openssl genpkey -algorithm ed25519 -out ed25519-priv.pem
# openssl pkey -in ed25519-priv.pem -outform DER | [Convert]::ToBase64String([IO.File]::ReadAllBytes("ed25519-priv.pem"))
$env:JWT_ED25519_PRIVATE_KEY = ""  # base64 DER
$env:JWT_ED25519_PUBLIC_KEY  = ""  # base64 DER

Write-Host "Starting Auth Service (port 8083)..." -ForegroundColor Cyan
Set-Location "$PSScriptRoot\..\services\go\auth"
go run .
