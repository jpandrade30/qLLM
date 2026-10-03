# One window: qllm-prd (qLLM + DBs + crew API) and Argo CD UI if installed.
# Missing Services are skipped. Ctrl+C stops every kubectl this script started.
$ErrorActionPreference = "Continue"

$kubectl = Get-Command kubectl -ErrorAction SilentlyContinue
if (-not $kubectl) {
    throw "kubectl not on PATH"
}

function Test-Svc([string]$Namespace, [string]$Name) {
    kubectl get svc -n $Namespace $Name -o name 2>$null | Out-Null
    return ($LASTEXITCODE -eq 0)
}

$forwards = @(
    @{ Ns = "qllm-prd"; Svc = "svc/qllm"; Ports = @("18088:8088", "18089:8089"); Hint = "qLLM HTTP http://127.0.0.1:18088  MCP http://127.0.0.1:18089" }
    @{ Ns = "qllm-prd"; Svc = "svc/fleet-pg"; Ports = @("15432:5432"); Hint = "Postgres 127.0.0.1:15432" }
    @{ Ns = "qllm-prd"; Svc = "svc/fleet-ch"; Ports = @("19000:9000", "18123:8123"); Hint = "ClickHouse native 127.0.0.1:19000  HTTP http://127.0.0.1:18123" }
    @{ Ns = "qllm-prd"; Svc = "svc/fleet-ddb"; Ports = @("18000:8000"); Hint = "DynamoDB Local http://127.0.0.1:18000" }
    @{ Ns = "qllm-prd"; Svc = "svc/fleet-api"; Ports = @("18080:8080"); Hint = "crew API http://127.0.0.1:18080" }
    @{ Ns = "argocd"; Svc = "svc/argocd-server"; Ports = @("18081:80"); Hint = "Argo CD http://127.0.0.1:18081  (needs .\scripts\prd-tst\prd-tst-argocd-up.ps1 --insecure)" }
)

$procs = @()
$hints = @()
try {
    foreach ($f in $forwards) {
        $short = $f.Svc.Replace("svc/", "")
        if (-not (Test-Svc $f.Ns $short)) {
            Write-Host ("skip {0}/{1} (not in cluster)" -f $f.Ns, $short)
            if ($f.Ns -eq "argocd") {
                Write-Host "  Argo is not installed. HTTP :18081 will not work until:"
                Write-Host "  .\scripts\prd-tst\prd-tst-argocd-up.ps1"
            }
            continue
        }
        $args = @("-n", $f.Ns, "port-forward", $f.Svc) + $f.Ports
        $p = Start-Process -FilePath $kubectl.Source -ArgumentList $args -PassThru -WindowStyle Hidden
        $procs += $p
        $hints += $f.Hint
        Write-Host ("started pid {0}: kubectl {1}" -f $p.Id, ($args -join " "))
    }
    if ($procs.Count -eq 0) {
        throw "nothing to forward (is qllm-prd applied? is Argo in namespace argocd?)"
    }
    Write-Host ""
    $hints | ForEach-Object { Write-Host $_ }
    Write-Host ""
    Write-Host "Ctrl+C to stop all forwards."
    while ($true) {
        Start-Sleep -Seconds 3600
    }
}
finally {
    foreach ($p in $procs) {
        if ($p -and -not $p.HasExited) {
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        }
    }
}
