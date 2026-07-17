#Requires -Version 5.1
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
kubectl apply -k "$Root/deploy/dev"
kubectl -n qllm-dev rollout status deploy/postgres --timeout=180s
kubectl -n qllm-dev rollout status deploy/mysql --timeout=300s
kubectl -n qllm-dev rollout status deploy/mongodb --timeout=180s
kubectl -n qllm-dev rollout status deploy/test-api --timeout=300s
Write-Host "dev harness applied"
