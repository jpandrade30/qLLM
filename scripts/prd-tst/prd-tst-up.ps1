# Build qllm:local into k8s.io, apply deploy/prd-tst, wait for deploy/qllm.
# Does not start port-forward (run prd-tst-port-forward.ps1 after). Argo is optional.
# Usage: .\scripts\prd-tst\prd-tst-up.ps1 [-SkipComposeDown] [-SkipBuild]
param(
    [switch]$SkipComposeDown,
    [switch]$SkipBuild
)
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Set-Location $root

foreach ($cmd in @("kubectl", "nerdctl")) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        throw "$cmd not on PATH"
    }
}

if (-not $SkipComposeDown) {
    Write-Host "nerdctl compose down -v (harness must not share this machine with fleet-ops)"
    nerdctl compose down -v
}

if (-not $SkipBuild) {
    Write-Host "nerdctl --namespace k8s.io build -f Dockerfile.dev -t qllm:local ."
    nerdctl --namespace k8s.io build -f Dockerfile.dev -t qllm:local .
}

kubectl apply -k (Join-Path $root "deploy\prd-tst")
kubectl -n qllm-prd rollout restart deploy/qllm
kubectl -n qllm-prd rollout status deploy/qllm --timeout=180s

Write-Host ""
Write-Host "fleet-ops is up (namespace qllm-prd). Bearer: fleet-prd-token"
Write-Host "  .\scripts\prd-tst\prd-tst-port-forward.ps1"
Write-Host "HTTP http://127.0.0.1:18088  MCP http://127.0.0.1:18089"
Write-Host "Tear down: .\scripts\prd-tst\prd-tst-down.ps1"
