$ErrorActionPreference = "Stop"
$b64 = kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}"
if (-not $b64) {
    throw "secret argocd-initial-admin-secret missing (install Argo: .\scripts\prd-tst\prd-tst-argocd-up.ps1)"
}
$pass = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($b64))
Write-Host "user:     admin"
Write-Host "password: $pass"
