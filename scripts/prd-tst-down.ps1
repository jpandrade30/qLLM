# Remove the fleet-ops overlay. Stops Argo Application qllm-prd-sim if present.
# Does not uninstall Argo CD. Stop prd-tst-port-forward (Ctrl+C) first.
$ErrorActionPreference = "Continue"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $root

if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) {
    throw "kubectl not on PATH"
}

$overlay = Join-Path $root "deploy\prd-tst"
Write-Host "kubectl delete -k deploy/prd-tst"
kubectl delete -k $overlay --ignore-not-found
kubectl -n argocd delete application qllm-prd-sim --ignore-not-found
kubectl delete ns qllm-prd --ignore-not-found --wait=false

Write-Host ""
Write-Host "PRD sim removed. Ctrl+C any leftover port-forward windows."
Write-Host "Harness: nerdctl compose up --build"
