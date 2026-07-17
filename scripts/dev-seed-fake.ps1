#Requires -Version 5.1
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

$py = Get-Command python -ErrorAction SilentlyContinue
if (-not $py) { throw "python not found" }

& python -m pip install -r "$Root/fixtures/seed/requirements.txt"
$env:QLLM_CRM_PG_HOST = if ($env:QLLM_CRM_PG_HOST) { $env:QLLM_CRM_PG_HOST } else { "127.0.0.1" }
$env:QLLM_CRM_PG_USER = if ($env:QLLM_CRM_PG_USER) { $env:QLLM_CRM_PG_USER } else { "qllm" }
$env:QLLM_CRM_PG_PASSWORD = if ($env:QLLM_CRM_PG_PASSWORD) { $env:QLLM_CRM_PG_PASSWORD } else { "qllm" }
$env:QLLM_BILLING_MYSQL_HOST = if ($env:QLLM_BILLING_MYSQL_HOST) { $env:QLLM_BILLING_MYSQL_HOST } else { "127.0.0.1" }
$env:QLLM_BILLING_MYSQL_USER = if ($env:QLLM_BILLING_MYSQL_USER) { $env:QLLM_BILLING_MYSQL_USER } else { "qllm" }
$env:QLLM_BILLING_MYSQL_PASSWORD = if ($env:QLLM_BILLING_MYSQL_PASSWORD) { $env:QLLM_BILLING_MYSQL_PASSWORD } else { "qllm" }
$env:QLLM_EVENTS_MONGO_URI = if ($env:QLLM_EVENTS_MONGO_URI) { $env:QLLM_EVENTS_MONGO_URI } else { "mongodb://127.0.0.1:27017" }

$customers = if ($args.Count -gt 0) { $args[0] } else { 200 }
& python "$Root/fixtures/seed/generate_and_load.py" --customers $customers --seed 42

kubectl apply -k "$Root/deploy/dev"
kubectl -n qllm-dev rollout restart deploy/test-api
kubectl -n qllm-dev rollout status deploy/test-api --timeout=180s
Write-Host "fake data loaded + test-api refreshed"
