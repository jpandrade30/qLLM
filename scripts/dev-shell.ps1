#Requires -Version 5.1
<#
.SYNOPSIS
  Loads gcc (MSYS2 UCRT64), CGO, and duckdblib into the current PowerShell session.

.EXAMPLE
  .\scripts\dev-shell.ps1

.EXAMPLE
  # Double-click:
  scripts\dev-shell.cmd
#>

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent $PSScriptRoot
if (-not (Test-Path (Join-Path $RepoRoot "go.mod"))) {
    $RepoRoot = (Get-Location).Path
}

$MsysGccBin = "C:\ghcup\msys64\ucrt64\bin"
$DuckDbLib  = Join-Path $RepoRoot "duckdblib"

if (-not (Test-Path (Join-Path $MsysGccBin "gcc.exe"))) {
    Write-Host "gcc not found at $MsysGccBin" -ForegroundColor Red
    Write-Host "In MSYS2 UCRT64 run: pacman -S mingw-w64-ucrt-x86_64-gcc" -ForegroundColor Yellow
    exit 1
}

if (-not (Test-Path (Join-Path $DuckDbLib "duckdb.dll"))) {
    Write-Host "Warning: duckdb.dll not found in $DuckDbLib" -ForegroundColor Yellow
}

$env:PATH = "$MsysGccBin;$DuckDbLib;$env:PATH"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CGO_CFLAGS = "-I$DuckDbLib"
$env:CGO_LDFLAGS = "-L$DuckDbLib -lduckdb"
$env:QLLM_CRM_PG_HOST='127.0.0.1'
$env:QLLM_CRM_PG_USER='qllm'
$env:QLLM_CRM_PG_PASSWORD='qllm'
$env:QLLM_BILLING_MYSQL_HOST='127.0.0.1'
$env:QLLM_BILLING_MYSQL_USER='qllm'
$env:QLLM_BILLING_MYSQL_PASSWORD='qllm'
$env:QLLM_EVENTS_MONGO_URI='mongodb://127.0.0.1:27017'
$env:QLLM_LEGACY_API_BASE_URL='http://127.0.0.1:18080'

Set-Location $RepoRoot

function global:prompt {
    "qLLM-dev $($executionContext.SessionState.Path.CurrentLocation)> "
}

Write-Host ""
Write-Host "qLLM dev shell loaded" -ForegroundColor Cyan
Write-Host "  repo:        $RepoRoot"
Write-Host "  gcc:         $((Get-Command gcc).Source)"
Write-Host "  CGO_ENABLED: $env:CGO_ENABLED"
Write-Host "  duckdblib:   $DuckDbLib"
Write-Host ""
Write-Host "Try:  go test -tags duckdb ./internal/duckdblocal/" -ForegroundColor DarkGray
Write-Host "      go run -tags duckdb .\scripts\duckdb_smoke.go" -ForegroundColor DarkGray

Write-Host "      go build -o qllm.exe .\cmd\qllm" -ForegroundColor DarkGray
Write-Host ""
