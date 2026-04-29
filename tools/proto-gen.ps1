# proto-gen.ps1 — Generate protobuf code (Windows, no make required)
# Replaces: cd libs/proto && make gen
# Usage: pwsh -File tools\proto-gen.ps1 [go|rust|ts|all|lint]
param(
    [string]$Target = "all"
)

$Root      = Split-Path -Parent $PSScriptRoot
$ProtoDir  = Join-Path $Root "libs\proto"
$GenDir    = Join-Path $ProtoDir "gen"

# Check buf is installed
if (-not (Get-Command buf -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: buf is not installed." -ForegroundColor Red
    Write-Host "Install from: https://github.com/bufbuild/buf/releases"
    Write-Host ""
    Write-Host "  Windows (scoop):   scoop install buf"
    Write-Host "  Windows (winget):  winget install bufbuild.buf"
    Write-Host "  Manual: download buf.exe and add to PATH"
    exit 1
}

Set-Location $ProtoDir

switch ($Target) {
    "all" {
        Write-Host "Generating protobuf code (Go + Rust + TypeScript)..." -ForegroundColor Cyan
        buf generate
    }
    "go" {
        Write-Host "Generating Go protobuf code..." -ForegroundColor Cyan
        buf generate --template buf.gen.yaml --include-imports
    }
    "rust" {
        Write-Host "Generating Rust protobuf code..." -ForegroundColor Cyan
        Write-Host "Note: requires protoc-gen-prost and protoc-gen-tonic"
        Write-Host "  cargo install protoc-gen-prost protoc-gen-tonic"
        buf generate --template buf.gen.yaml
    }
    "ts" {
        Write-Host "Generating TypeScript protobuf code..." -ForegroundColor Cyan
        buf generate --template buf.gen.yaml
    }
    "lint" {
        Write-Host "Linting proto files..." -ForegroundColor Cyan
        buf lint
    }
    "breaking" {
        Write-Host "Checking for breaking changes..." -ForegroundColor Cyan
        buf breaking --against ".git#branch=main,subdir=libs/proto"
    }
    default {
        Write-Host "Unknown target: $Target" -ForegroundColor Red
        Write-Host "Usage: proto-gen.ps1 [all|go|rust|ts|lint|breaking]"
        exit 1
    }
}

if ($LASTEXITCODE -eq 0) {
    Write-Host "Done. Generated code in: $GenDir" -ForegroundColor Green
} else {
    Write-Host "Generation failed (exit code $LASTEXITCODE)" -ForegroundColor Red
    exit $LASTEXITCODE
}
