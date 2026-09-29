# Registers Application qllm-prd-sim in Argo CD. Run from repo root.
$ErrorActionPreference = "Stop"
$yaml = Join-Path $PSScriptRoot "..\deploy\prd\argocd\application.yaml"
if (-not (Test-Path $yaml)) {
    throw "missing $yaml — run from the qLLM clone"
}
kubectl apply -f $yaml
$branch = (git rev-parse --abbrev-ref HEAD).Trim()
if ($branch) {
    $patch = @{ spec = @{ source = @{ targetRevision = $branch } } } | ConvertTo-Json -Compress
    kubectl -n argocd patch application qllm-prd-sim --type merge -p $patch
    Write-Host "targetRevision set to branch: $branch"
}
Write-Host "Created Application qllm-prd-sim (namespace argocd)."
Write-Host "Refresh the Argo UI. Push this branch if deploy/prd is not on the remote yet."
Write-Host "Private GitHub: Settings -> Repositories in Argo, then Refresh."
