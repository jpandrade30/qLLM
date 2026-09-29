# Installs Argo CD and --insecure so the UI is plain HTTP.
# Stock argocd-server: container port 8080 is TLS. Service port 80 still hits TLS unless --insecure.
$ErrorActionPreference = "Stop"

kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
Write-Host "waiting for argocd-server..."
kubectl -n argocd rollout status deploy/argocd-server --timeout=300s

$deploy = kubectl -n argocd get deploy argocd-server -o json | ConvertFrom-Json
$c0 = $deploy.spec.template.spec.containers[0]
$patchContainer = @{ name = $c0.name }
$argList = @()
if ($c0.args) { $argList = @($c0.args) }
if ($argList -notcontains "--insecure") { $argList += "--insecure" }
$patchContainer.args = $argList

$tmp = Join-Path $env:TEMP "argocd-server-insecure.json"
$json = @{
    spec = @{
        template = @{
            spec = @{
                containers = @($patchContainer)
            }
        }
    }
} | ConvertTo-Json -Depth 12
[System.IO.File]::WriteAllText($tmp, $json)
kubectl -n argocd patch deploy argocd-server --type strategic --patch-file $tmp
Remove-Item $tmp -ErrorAction SilentlyContinue
kubectl -n argocd rollout status deploy/argocd-server --timeout=180s

$b64 = kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}"
$pass = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($b64))
Write-Host ""
Write-Host "Argo CD user: admin"
Write-Host "password:     $pass"
Write-Host "Stop port-forward (Ctrl+C) if it is running, then:"
Write-Host "  .\scripts\prd-port-forward.ps1"
Write-Host "Open: http://127.0.0.1:18081"
