#Requires -Version 5.1
# Run SQL goldens from the repo root against the live qLLM MCP (compose).
# Usage:
#   .\scripts\dev\check-live.ps1
#   .\scripts\dev\check-live.ps1 -Filter rest_json
#   .\scripts\dev\check-live.ps1 -Oracle
# Needs compose up (HTTP 8088 / MCP 8089) unless -Oracle.
param(
    [switch]$Oracle,
    [string]$Filter = ""
)
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$VenvPy = Join-Path $Root "fixtures\seed\.venv\Scripts\python.exe"
if (-not (Test-Path $VenvPy)) {
    Write-Host "seed venv missing - running dev-seed-fake.ps1 first" -ForegroundColor Yellow
    & (Join-Path $PSScriptRoot "dev-seed-fake.ps1")
    if ($LASTEXITCODE -ne 0) { throw "dev-seed-fake failed" }
}
$cmd = if ($Oracle) { "check-oracle" } else { "mcp" }
$argv = @("-m", "sqlcheck", $cmd)
if ($Filter) { $argv += @("--filter", $Filter) }
Write-Host "Using $VenvPy" -ForegroundColor DarkGray
$filtHint = ""
if ($Filter) { $filtHint = " --filter $Filter" }
Write-Host "  python -m sqlcheck $cmd$filtHint" -ForegroundColor DarkGray
if (-not $Oracle) {
    $mcpUrl = $env:QLLM_MCP_URL
    if (-not $mcpUrl) { $mcpUrl = "http://127.0.0.1:8089/mcp" }
    Write-Host "Expecting MCP at $mcpUrl" -ForegroundColor DarkGray
}
Push-Location (Join-Path $Root "fixtures")
try {
    & $VenvPy @argv
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
